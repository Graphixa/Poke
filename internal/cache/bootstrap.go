package cache

import (
	"context"
	"fmt"
	"sync"

	"github.com/Graphixa/Pokedex/internal/cache/typeseed"
	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/domain"
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
	var done int
	total := len(summaries)
	for res := range results {
		done++
		report(progress, Progress{Stage: "dataset", Done: done, Total: total, Message: res.sum.DisplayName, SpeciesID: res.sum.ID})
		if res.err != nil || ctx.Err() != nil {
			continue
		}
		_ = s.store.StoreSnapshotPokemon(id, res.pokemon, now)
		if res.species.ID != 0 {
			_ = s.store.StoreSnapshotSpecies(id, res.species, now)
			if res.species.EvolutionChainID != 0 {
				evoIDs[res.species.EvolutionChainID] = struct{}{}
			}
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
		if res.pokemon.ID != 0 {
			if enc, err := s.fetchEncounters(ctx, res.pokemon.ID); err == nil {
				_ = s.store.StoreSnapshotEncounters(id, res.pokemon.ID, enc, now)
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	for evoID := range evoIDs {
		if err := ctx.Err(); err != nil {
			return err
		}
		tree, err := s.fetchEvolution(ctx, evoID)
		if err != nil {
			continue
		}
		_ = s.store.StoreSnapshotEvolution(id, evoID, tree, now)
	}
	for moveID := range moveIDs {
		if err := ctx.Err(); err != nil {
			return err
		}
		mv, err := s.fetchMove(ctx, moveID)
		if err != nil {
			continue
		}
		_ = s.store.StoreSnapshotMove(id, mv, now)
	}

	if err := s.store.StoreBrowse(id, entries); err != nil {
		return err
	}
	staging.PokemonCount = len(entries)
	staging.Complete = len(entries) > 0
	if err := s.store.WriteDatasetManifest(staging); err != nil {
		return err
	}
	if !staging.Complete {
		return fmt.Errorf("cache: dataset %s has no pokemon records", id)
	}
	if err := s.store.ActivateDataset(id); err != nil {
		return err
	}
	s.localOnly.Store(true)
	debuglog.Cachef("dataset.bootstrap.complete id=%s count=%d", id, staging.PokemonCount)
	report(progress, Progress{Stage: "dataset", Done: 1, Total: 1, Message: fmt.Sprintf("Dataset %s active", id)})
	return nil
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
