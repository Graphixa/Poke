package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Graphixa/Pokedex/internal/cache/typeseed"
	"github.com/Graphixa/Pokedex/internal/dataset"
	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/pokeapi"
)

const datasetWorkerDefault = 8

// BootstrapDataset fetches the TUI metadata graph into a staging snapshot and
// activates it only when complete. A failed run leaves the previous active
// snapshot untouched. Uses the cached species index when present.
func (s *Service) BootstrapDataset(ctx context.Context, progress ProgressFunc) error {
	_, err := s.group.Do("dataset-bootstrap", func() (any, error) {
		return nil, s.bootstrapDataset(ctx, progress)
	})
	return err
}

// BootstrapDatasetFromIndex builds a snapshot from an already-loaded index.
func (s *Service) BootstrapDatasetFromIndex(ctx context.Context, summaries []domain.SpeciesSummary, progress ProgressFunc) error {
	_, err := s.group.Do("dataset-bootstrap", func() (any, error) {
		return nil, s.bootstrapDatasetFromIndex(ctx, summaries, progress)
	})
	return err
}

func (s *Service) bootstrapDataset(ctx context.Context, progress ProgressFunc) error {
	ctx = WithInteractivePriority(ctx)
	debuglog.Cachef("dataset.bootstrap.start")

	summaries, _, err := s.loadIndex()
	if err != nil || len(summaries) == 0 {
		if err := s.store.EnsureRoot(); err != nil {
			return err
		}
		report(progress, Progress{Stage: "index", Message: "Fetching species index"})
		summaries, err = s.api.ListSpecies(ctx)
		if err != nil {
			return err
		}
		summaries, _ = typeseed.Apply(summaries)
		now := s.clock.Now()
		if err := s.store.StoreSpeciesIndex(ctx, summaries, now); err != nil {
			return err
		}
		report(progress, Progress{Stage: "index", Done: 1, Total: 1, Message: fmt.Sprintf("Indexed %d species", len(summaries))})
	} else {
		summaries, _ = typeseed.Apply(summaries)
	}
	return s.bootstrapDatasetFromIndex(ctx, summaries, progress)
}

func (s *Service) bootstrapDatasetFromIndex(ctx context.Context, summaries []domain.SpeciesSummary, progress ProgressFunc) error {
	ctx = WithInteractivePriority(ctx)
	if len(summaries) == 0 {
		return ErrEmpty
	}
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
		Source:         "pokeapi",
		CreatedAt:      now.UTC(),
		PokeAPIBase:    "https://pokeapi.co/api/v2",
		AppDataVersion: appDataVersion,
		Complete:       false,
	}
	if err := s.store.WriteDatasetManifest(staging); err != nil {
		return err
	}

	builder := dataset.NewBuilder()
	workers := s.datasetWorkers()
	total := len(summaries)

	jobs := make(chan domain.SpeciesSummary, workers*2)
	results := make(chan recordResult, workers*8)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for sum := range jobs {
				if ctx.Err() != nil {
					results <- recordResult{sum: sum, err: ctx.Err()}
					continue
				}
				results <- s.fetchRecord(ctx, sum)
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

	entries := make([]domain.BrowseEntry, 0, total)
	evoIDs := map[int]struct{}{}
	moveIDs := map[int]struct{}{}
	rep := DatasetBootstrapReport{Expected: total}
	var done int
	for res := range results {
		done++
		report(progress, Progress{
			Stage: "records", Done: done, Total: total,
			Message: res.sum.DisplayName, SpeciesID: res.sum.ID,
		})
		if err := ctx.Err(); err != nil {
			rep.add("pokemon", res.sum.ID, err, true)
			continue
		}
		if res.err != nil {
			if res.storageFail {
				rep.StorageFailures++
			}
			rep.add(res.failResource, res.failID, res.err, true)
			continue
		}
		builder.Pokemon[res.poke.ID] = dataset.PokemonFromDomain(res.poke)
		builder.Species[res.sp.ID] = dataset.SpeciesFromDomain(res.sp)
		builder.Learnsets[res.poke.ID] = dataset.LearnsetFromPokemon(res.poke)
		builder.Encounters[res.poke.ID] = dataset.EncountersFromDomain(res.enc)
		rep.PokemonOK++
		rep.SpeciesOK++
		rep.EncounterOK++
		if res.evoID != 0 {
			evoIDs[res.evoID] = struct{}{}
		}
		for _, mid := range res.moveIDs {
			moveIDs[mid] = struct{}{}
		}
		entries = append(entries, res.entry)
	}
	if err := ctx.Err(); err != nil {
		s.finalizeIncomplete(id, staging, rep, err)
		return err
	}

	rep.EvolutionWanted = len(evoIDs)
	if err := s.parallelEvolutionFetch(ctx, intSetKeys(evoIDs), progress, &rep, builder); err != nil {
		s.finalizeIncomplete(id, staging, rep, err)
		return err
	}

	rep.MoveWanted = len(moveIDs)
	if err := s.parallelMoveFetch(ctx, intSetKeys(moveIDs), progress, &rep, builder); err != nil {
		s.finalizeIncomplete(id, staging, rep, err)
		return err
	}

	if err := dataset.Validate(builder, total); err != nil {
		rep.add("validate", 0, err, true)
		s.finalizeIncomplete(id, staging, rep, err)
		return err
	}

	if err := s.store.StoreAggregateBuilder(id, builder); err != nil {
		rep.StorageFailures++
		rep.add("aggregate", 0, err, true)
		s.finalizeIncomplete(id, staging, rep, err)
		return err
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
	report(progress, Progress{Stage: "dataset", Done: 1, Total: 1, Message: "Activating dataset"})
	if err := s.store.ActivateDataset(id); err != nil {
		return err
	}
	s.localOnly.Store(true)
	debuglog.Cachef("dataset.bootstrap.complete id=%s count=%d", id, staging.PokemonCount)
	report(progress, Progress{Stage: "dataset", Done: 1, Total: 1, Message: fmt.Sprintf("Dataset %s active", id)})
	return nil
}

func (s *Service) datasetWorkers() int {
	workers := s.MaxConcurrency()
	if workers < 1 {
		workers = datasetWorkerDefault
	}
	return workers
}

type recordResult struct {
	sum          domain.SpeciesSummary
	entry        domain.BrowseEntry
	poke         domain.Pokemon
	sp           domain.Species
	enc          []domain.Encounter
	evoID        int
	moveIDs      []int
	err          error
	storageFail  bool
	failResource string
	failID       int
}

func (s *Service) fetchRecord(ctx context.Context, sum domain.SpeciesSummary) recordResult {
	var (
		poke domain.Pokemon
		sp   domain.Species
		perr error
		serr error
		wg   sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		poke, perr = s.fetchDefaultPokemon(ctx, sum)
	}()
	go func() {
		defer wg.Done()
		sp, serr = s.fetchSpecies(ctx, sum.ID)
	}()
	wg.Wait()

	err := perr
	if err == nil {
		err = serr
	}
	if err != nil {
		return recordResult{sum: sum, err: err, failResource: "pokemon", failID: sum.ID}
	}
	if poke.ID == 0 || sp.ID == 0 {
		return recordResult{sum: sum, err: fmt.Errorf("cache: missing pokemon or species record"), failResource: "pokemon", failID: sum.ID}
	}

	enc, encErr := s.fetchEncounters(ctx, poke.ID)
	if encErr := optionalEmpty(encErr); encErr != nil {
		return recordResult{sum: sum, err: encErr, failResource: "encounter", failID: poke.ID}
	}

	types := sum.Types
	if len(types) == 0 {
		for _, t := range poke.Types {
			types = append(types, t.Name)
		}
	}
	name := sum.DisplayName
	if name == "" {
		name = poke.DisplayName
	}
	entry := domain.BrowseEntry{
		ID:        sum.ID,
		Slug:      sum.Slug,
		Name:      name,
		Types:     types,
		HP:        poke.Stats.HP,
		Attack:    poke.Stats.Attack,
		Defence:   poke.Stats.Defense,
		SpAttack:  poke.Stats.SpecialAttack,
		SpDefence: poke.Stats.SpecialDefense,
		Speed:     poke.Stats.Speed,
	}

	evoID := 0
	if sp.EvolutionChainID != 0 {
		evoID = sp.EvolutionChainID
	}
	return recordResult{
		sum:     sum,
		entry:   entry,
		poke:    poke,
		sp:      sp,
		enc:     enc,
		evoID:   evoID,
		moveIDs: levelUpMoveIDs(poke),
	}
}

func (s *Service) parallelEvolutionFetch(ctx context.Context, ids []int, progress ProgressFunc, rep *DatasetBootstrapReport, builder *dataset.Builder) error {
	var mu sync.Mutex
	return s.parallelIntFetch(ctx, ids, "evolution", progress, func(ctx context.Context, id int) error {
		tree, err := s.fetchEvolution(ctx, id)
		if err != nil {
			rep.add("evolution", id, err, true)
			return nil
		}
		mu.Lock()
		builder.Evolutions[id] = dataset.EvolutionFromDomain(tree)
		mu.Unlock()
		rep.EvolutionOK++
		return nil
	})
}

func (s *Service) parallelMoveFetch(ctx context.Context, ids []int, progress ProgressFunc, rep *DatasetBootstrapReport, builder *dataset.Builder) error {
	var mu sync.Mutex
	return s.parallelIntFetch(ctx, ids, "moves", progress, func(ctx context.Context, id int) error {
		mv, err := s.fetchMove(ctx, id)
		if err != nil {
			rep.add("move", id, err, true)
			return nil
		}
		mu.Lock()
		builder.Moves[id] = dataset.MoveFromDomain(mv)
		mu.Unlock()
		rep.MoveOK++
		return nil
	})
}

func (s *Service) parallelIntFetch(ctx context.Context, ids []int, stage string, progress ProgressFunc, fn func(context.Context, int) error) error {
	total := len(ids)
	if total == 0 {
		return nil
	}
	workers := s.datasetWorkers()
	if workers > total {
		workers = total
	}
	jobs := make(chan int, workers*2)
	var wg sync.WaitGroup
	var done int
	var doneMu sync.Mutex

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				if ctx.Err() != nil {
					return
				}
				_ = fn(ctx, id)
				doneMu.Lock()
				done++
				current := done
				doneMu.Unlock()
				report(progress, Progress{Stage: stage, Done: current, Total: total})
			}
		}()
	}
	for _, id := range ids {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return ctx.Err()
		case jobs <- id:
		}
	}
	close(jobs)
	wg.Wait()
	return ctx.Err()
}

func intSetKeys(set map[int]struct{}) []int {
	out := make([]int, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return out
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
	v, err := s.group.Do(fmt.Sprintf("evo:%d", id), func() (any, error) {
		if err := s.acquire(ctx); err != nil {
			return domain.EvolutionTree{}, err
		}
		defer s.release()
		return s.api.GetEvolutionChain(ctx, id)
	})
	if err != nil {
		return domain.EvolutionTree{}, err
	}
	return v.(domain.EvolutionTree), nil
}

func (s *Service) fetchEncounters(ctx context.Context, pokemonID int) ([]domain.Encounter, error) {
	if err := s.acquire(ctx); err != nil {
		return nil, err
	}
	defer s.release()
	return s.api.GetEncounters(ctx, pokemonID)
}

func (s *Service) fetchMove(ctx context.Context, id int) (domain.Move, error) {
	v, err := s.group.Do(fmt.Sprintf("move:%d", id), func() (any, error) {
		if err := s.acquire(ctx); err != nil {
			return domain.Move{}, err
		}
		defer s.release()
		return s.api.GetMove(ctx, id)
	})
	if err != nil {
		return domain.Move{}, err
	}
	return v.(domain.Move), nil
}
