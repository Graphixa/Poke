package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Graphixa/Pokedex/internal/cache/typeseed"
	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/pokeapi"
)

const datasetWorkerDefault = 6

// BootstrapDataset fetches the TUI metadata graph into a staging snapshot and
// activates it only when complete. A failed run leaves the previous active
// snapshot untouched.
func (s *Service) BootstrapDataset(ctx context.Context, progress ProgressFunc) error {
	_, err := s.group.Do("dataset-bootstrap", func() (any, error) {
		return nil, s.bootstrapDataset(ctx, progress)
	})
	return err
}

func (s *Service) bootstrapDataset(ctx context.Context, progress ProgressFunc) error {
	debuglog.Cachef("dataset.bootstrap.start")
	if err := s.store.EnsureRoot(); err != nil {
		return err
	}
	id := DatasetIDForTime(s.clock.Now())
	if err := s.store.EnsureDatasetDir(id); err != nil {
		return err
	}
	now := s.clock.Now()
	staging := DatasetManifest{
		SchemaVersion:  DatasetSchemaVersion,
		DatasetVersion: id,
		CreatedAt:      now.UTC(),
		PokeAPIBase:    "https://pokeapi.co/api/v2",
		AppDataVersion: appDataVersion,
		Complete:       false,
	}
	if err := s.store.WriteDatasetManifest(staging); err != nil {
		return err
	}

	report(progress, Progress{Stage: "dataset", Message: "Fetching species index"})
	summaries, err := s.api.ListSpecies(ctx)
	if err != nil {
		return err
	}
	summaries, _ = typeseed.Apply(summaries)
	if err := s.store.StoreSpeciesIndex(ctx, summaries, now); err != nil {
		return err
	}

	workers := s.MaxConcurrency()
	if workers < 1 {
		workers = datasetWorkerDefault
	}
	if workers > 8 {
		workers = 8
	}

	type pokeResult struct {
		sum     domain.SpeciesSummary
		pokemon domain.Pokemon
		species domain.Species
		err     error
	}

	jobs := make(chan domain.SpeciesSummary, workers*2)
	results := make(chan pokeResult, workers*2)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for sum := range jobs {
				if ctx.Err() != nil {
					results <- pokeResult{sum: sum, err: ctx.Err()}
					continue
				}
				poke, perr := s.fetchDefaultPokemon(ctx, sum)
				sp, serr := s.fetchSpecies(ctx, sum.ID)
				err := perr
				if err == nil {
					err = serr
				}
				results <- pokeResult{sum: sum, pokemon: poke, species: sp, err: err}
			}
		}()
	}
	go func() {
		for _, sum := range summaries {
			select {
			case <-ctx.Done():
				close(jobs)
				return
			case jobs <- sum:
			}
		}
		close(jobs)
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	entries := make([]domain.BrowseEntry, 0, len(summaries))
	evoIDs := map[int]struct{}{}
	moveIDs := map[int]struct{}{}
	rep := DatasetBootstrapReport{Expected: len(summaries)}
	var done int
	total := len(summaries)
	for res := range results {
		done++
		report(progress, Progress{Stage: "dataset", Done: done, Total: total, Message: res.sum.DisplayName, SpeciesID: res.sum.ID})
		if err := ctx.Err(); err != nil {
			rep.add("pokemon", res.sum.ID, err, true)
			continue
		}
		if res.err != nil {
			rep.add("pokemon", res.sum.ID, res.err, true)
			continue
		}
		if res.pokemon.ID == 0 || res.species.ID == 0 {
			rep.add("pokemon", res.sum.ID, fmt.Errorf("cache: missing pokemon or species record"), true)
			continue
		}
		if err := s.store.StoreSnapshotPokemon(id, res.pokemon, now); err != nil {
			rep.StorageFailures++
			rep.add("pokemon", res.pokemon.ID, err, true)
			continue
		}
		rep.PokemonOK++
		if err := s.store.StoreSnapshotSpecies(id, res.species, now); err != nil {
			rep.StorageFailures++
			rep.add("species", res.species.ID, err, true)
			continue
		}
		rep.SpeciesOK++
		if res.species.EvolutionChainID != 0 {
			evoIDs[res.species.EvolutionChainID] = struct{}{}
		}
		for _, m := range res.pokemon.Moves {
			if m.MoveID > 0 {
				moveIDs[m.MoveID] = struct{}{}
			}
		}
		types := res.sum.Types
		if len(types) == 0 {
			for _, t := range res.pokemon.Types {
				types = append(types, t.Name)
			}
		}
		name := res.sum.DisplayName
		if name == "" {
			name = res.pokemon.DisplayName
		}
		entries = append(entries, domain.BrowseEntry{
			ID:        res.sum.ID,
			Slug:      res.sum.Slug,
			Name:      name,
			Types:     types,
			HP:        res.pokemon.Stats.HP,
			Attack:    res.pokemon.Stats.Attack,
			Defence:   res.pokemon.Stats.Defense,
			SpAttack:  res.pokemon.Stats.SpecialAttack,
			SpDefence: res.pokemon.Stats.SpecialDefense,
			Speed:     res.pokemon.Stats.Speed,
		})
		enc, encErr := s.fetchEncounters(ctx, res.pokemon.ID)
		if encErr := optionalEmpty(encErr); encErr != nil {
			rep.add("encounter", res.pokemon.ID, encErr, true)
			continue
		}
		if err := s.store.StoreSnapshotEncounters(id, res.pokemon.ID, enc, now); err != nil {
			rep.StorageFailures++
			rep.add("encounter", res.pokemon.ID, err, true)
			continue
		}
		rep.EncounterOK++
	}
	if err := ctx.Err(); err != nil {
		s.finalizeIncomplete(id, staging, rep, err)
		return err
	}

	rep.EvolutionWanted = len(evoIDs)
	for evoID := range evoIDs {
		if err := ctx.Err(); err != nil {
			s.finalizeIncomplete(id, staging, rep, err)
			return err
		}
		tree, err := s.fetchEvolution(ctx, evoID)
		if err != nil {
			rep.add("evolution", evoID, err, true)
			continue
		}
		if err := s.store.StoreSnapshotEvolution(id, evoID, tree, now); err != nil {
			rep.StorageFailures++
			rep.add("evolution", evoID, err, true)
			continue
		}
		rep.EvolutionOK++
	}
	rep.MoveWanted = len(moveIDs)
	for moveID := range moveIDs {
		if err := ctx.Err(); err != nil {
			s.finalizeIncomplete(id, staging, rep, err)
			return err
		}
		mv, err := s.fetchMove(ctx, moveID)
		if err != nil {
			rep.add("move", moveID, err, true)
			continue
		}
		if err := s.store.StoreSnapshotMove(id, mv, now); err != nil {
			rep.StorageFailures++
			rep.add("move", moveID, err, true)
			continue
		}
		rep.MoveOK++
	}

	if err := s.store.StoreBrowse(id, entries); err != nil {
		rep.StorageFailures++
		rep.add("browse", 0, err, true)
		s.finalizeIncomplete(id, staging, rep, err)
		return err
	}
	rep.BrowseOK = len(entries)
	staging.PokemonCount = rep.PokemonOK
	staging.SpeciesCount = rep.SpeciesOK
	staging.EncounterCount = rep.EncounterOK
	staging.EvolutionCount = rep.EvolutionOK
	staging.MoveCount = rep.MoveOK
	staging.StorageFailures = rep.StorageFailures
	staging.Complete = rep.Ready()
	if err := s.store.WriteBootstrapReport(id, rep); err != nil {
		return err
	}
	if err := s.store.WriteDatasetManifest(staging); err != nil {
		return err
	}
	if !staging.Complete {
		return fmt.Errorf("cache: dataset %s incomplete (%s)", id, rep.summary())
	}
	if err := s.store.ActivateDataset(id); err != nil {
		return err
	}
	s.localOnly.Store(true)
	debuglog.Cachef("dataset.bootstrap.complete id=%s count=%d", id, staging.PokemonCount)
	report(progress, Progress{Stage: "dataset", Done: 1, Total: 1, Message: fmt.Sprintf("Dataset %s active", id)})
	return nil
}

func optionalEmpty(err error) error {
	if err == nil || errors.Is(err, pokeapi.ErrNotFound) {
		return nil
	}
	return err
}

func (s *Service) finalizeIncomplete(id string, staging DatasetManifest, rep DatasetBootstrapReport, cause error) {
	if cause != nil {
		rep.add("bootstrap", 0, cause, true)
	}
	staging.PokemonCount = rep.PokemonOK
	staging.SpeciesCount = rep.SpeciesOK
	staging.EncounterCount = rep.EncounterOK
	staging.EvolutionCount = rep.EvolutionOK
	staging.MoveCount = rep.MoveOK
	staging.StorageFailures = rep.StorageFailures
	staging.Complete = false
	if err := s.store.WriteBootstrapReport(id, rep); err != nil {
		debuglog.Cachef("dataset.bootstrap.report err=%v", err)
	}
	if err := s.store.WriteDatasetManifest(staging); err != nil {
		debuglog.Cachef("dataset.bootstrap.manifest err=%v", err)
	}
}

func (s *Service) fetchSpecies(ctx context.Context, id int) (domain.Species, error) {
	if err := s.acquire(ctx); err != nil {
		return domain.Species{}, err
	}
	defer s.release()
	return s.api.GetSpecies(ctx, id)
}

func (s *Service) fetchEvolution(ctx context.Context, id int) (domain.EvolutionTree, error) {
	if err := s.acquire(ctx); err != nil {
		return domain.EvolutionTree{}, err
	}
	defer s.release()
	return s.api.GetEvolutionChain(ctx, id)
}

func (s *Service) fetchEncounters(ctx context.Context, pokemonID int) ([]domain.Encounter, error) {
	if err := s.acquire(ctx); err != nil {
		return nil, err
	}
	defer s.release()
	return s.api.GetEncounters(ctx, pokemonID)
}

func (s *Service) fetchMove(ctx context.Context, id int) (domain.Move, error) {
	if err := s.acquire(ctx); err != nil {
		return domain.Move{}, err
	}
	defer s.release()
	return s.api.GetMove(ctx, id)
}
