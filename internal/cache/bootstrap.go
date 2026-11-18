package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Graphixa/Poke/internal/cache/typeseed"
	"github.com/Graphixa/Poke/internal/dataset"
	"github.com/Graphixa/Poke/internal/debuglog"
	"github.com/Graphixa/Poke/internal/domain"
	"github.com/Graphixa/Poke/internal/pokeapi"
)

const datasetWorkerDefault = 8

// BootstrapDataset fetches the TUI metadata graph into a staging snapshot and
// activates it only when complete. A failed run leaves the previous active
// snapshot untouched. Uses the cached species index when present.
func (s *Service) BootstrapDataset(ctx context.Context, progress DatasetProgressFunc) error {
	_, err := s.group.Do("dataset-bootstrap", func() (any, error) {
		return nil, s.bootstrapDataset(ctx, progress)
	})
	return err
}

// BootstrapDatasetFromIndex builds a snapshot from an already-loaded index.
func (s *Service) BootstrapDatasetFromIndex(ctx context.Context, summaries []domain.SpeciesSummary, progress DatasetProgressFunc) error {
	_, err := s.group.Do("dataset-bootstrap", func() (any, error) {
		return nil, s.bootstrapDatasetFromIndex(ctx, summaries, progress)
	})
	return err
}

func (s *Service) bootstrapDataset(ctx context.Context, progress DatasetProgressFunc) error {
	ctx = WithInteractivePriority(ctx)
	debuglog.Cachef("dataset.bootstrap.start")

	summaries, _, err := s.loadIndex()
	if err != nil || len(summaries) == 0 {
		if err := s.store.EnsureRoot(); err != nil {
			return err
		}
		report(progress, DatasetProgress{Stage: DatasetStageIndex, Message: "Fetching species index"})
		summaries, err = s.api.ListSpecies(ctx)
		if err != nil {
			return err
		}
		summaries, _ = typeseed.Apply(summaries)
		now := s.clock.Now()
		if err := s.store.StoreSpeciesIndex(ctx, summaries, now); err != nil {
			return err
		}
		report(progress, DatasetProgress{Stage: DatasetStageIndex, Done: 1, Total: 1, Message: fmt.Sprintf("Indexed %d species", len(summaries))})
	} else {
		summaries, _ = typeseed.Apply(summaries)
	}
	return s.bootstrapDatasetFromIndex(ctx, summaries, progress)
}

func (s *Service) bootstrapDatasetFromIndex(ctx context.Context, summaries []domain.SpeciesSummary, progress DatasetProgressFunc) error {
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
	abilityIDs := map[int]struct{}{}
	formIDs := map[int]struct{}{}
	rep := DatasetBootstrapReport{Expected: total}
	var done int
	for res := range results {
		done++
		report(progress, DatasetProgress{
			Stage: DatasetStageRecords, Done: done, Total: total,
			Message: res.sum.DisplayName,
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
		for _, aid := range res.abilityIDs {
			abilityIDs[aid] = struct{}{}
		}
		for _, fid := range res.formIDs {
			formIDs[fid] = struct{}{}
		}
		for _, alt := range res.alts {
			builder.Pokemon[alt.poke.ID] = dataset.PokemonFromDomain(alt.poke)
			builder.Learnsets[alt.poke.ID] = dataset.LearnsetFromPokemon(alt.poke)
			builder.Encounters[alt.poke.ID] = dataset.EncountersFromDomain(alt.enc)
			for _, mid := range referencedMoveIDs(alt.poke) {
				moveIDs[mid] = struct{}{}
			}
			for _, a := range alt.poke.Abilities {
				if a.ID > 0 {
					abilityIDs[a.ID] = struct{}{}
				}
			}
			for _, fid := range alt.poke.FormIDs {
				formIDs[fid] = struct{}{}
			}
		}
		entries = append(entries, res.entry)
	}
	if err := ctx.Err(); err != nil {
		s.finalizeIncomplete(id, staging, rep, err)
		return err
	}

	rep.EvolutionWanted = len(evoIDs)
	if len(evoIDs) == 0 {
		report(progress, DatasetProgress{Stage: DatasetStageEvolution})
	}
	if err := s.parallelEvolutionFetch(ctx, intSetKeys(evoIDs), progress, &rep, builder); err != nil {
		s.finalizeIncomplete(id, staging, rep, err)
		return err
	}

	rep.MoveWanted = len(moveIDs)
	if len(moveIDs) == 0 {
		report(progress, DatasetProgress{Stage: DatasetStageMoves})
	}
	if err := s.parallelMoveFetch(ctx, intSetKeys(moveIDs), progress, &rep, builder); err != nil {
		s.finalizeIncomplete(id, staging, rep, err)
		return err
	}

	report(progress, DatasetProgress{Stage: DatasetStageDataset, Done: 0, Total: 1, Message: "Fetching game catalog"})
	if err := s.fetchCatalogs(ctx, builder); err != nil {
		rep.add("catalog", 0, err, true)
		s.finalizeIncomplete(id, staging, rep, err)
		return err
	}
	report(progress, DatasetProgress{Stage: DatasetStageDataset, Done: 1, Total: 1, Message: "Game catalog ready"})
	if err := s.parallelAbilityFetch(ctx, intSetKeys(abilityIDs), progress, builder); err != nil {
		s.finalizeIncomplete(id, staging, rep, err)
		return err
	}
	if err := s.parallelFormFetch(ctx, intSetKeys(formIDs), progress, builder); err != nil {
		s.finalizeIncomplete(id, staging, rep, err)
		return err
	}

	if err := dataset.Validate(builder, total, entries); err != nil {
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
	report(progress, DatasetProgress{Stage: DatasetStageDataset, Done: 1, Total: 1, Message: "Activating dataset"})
	if err := s.store.ActivateDataset(id); err != nil {
		return err
	}
	s.localOnly.Store(true)
	debuglog.Cachef("dataset.bootstrap.complete id=%s count=%d", id, staging.PokemonCount)
	report(progress, DatasetProgress{Stage: DatasetStageDataset, Done: 1, Total: 1, Message: fmt.Sprintf("Dataset %s active", id)})
	return nil
}

func (s *Service) datasetWorkers() int {
	workers := s.MaxConcurrency()
	if workers < 1 {
		workers = datasetWorkerDefault
	}
	return workers
}

type altPokemon struct {
	poke domain.Pokemon
	enc  []domain.Encounter
}

type recordResult struct {
	sum          domain.SpeciesSummary
	entry        domain.BrowseEntry
	poke         domain.Pokemon
	sp           domain.Species
	enc          []domain.Encounter
	alts         []altPokemon
	evoID        int
	moveIDs      []int
	abilityIDs   []int
	formIDs      []int
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

	entry := domain.BrowseEntryFromRecord(sum, poke, sp)

	var alts []altPokemon
	for _, v := range sp.Varieties {
		if v.PokemonID < 1 || v.PokemonID == poke.ID {
			continue
		}
		alt, aerr := s.fetchPokemonByID(ctx, v.PokemonID)
		if aerr != nil {
			return recordResult{sum: sum, err: aerr, failResource: "pokemon", failID: v.PokemonID}
		}
		altEnc, encErr := s.fetchEncounters(ctx, alt.ID)
		if encErr := optionalEmpty(encErr); encErr != nil {
			return recordResult{sum: sum, err: encErr, failResource: "encounter", failID: alt.ID}
		}
		alts = append(alts, altPokemon{poke: alt, enc: altEnc})
	}

	var abilityIDs []int
	for _, a := range poke.Abilities {
		if a.ID > 0 {
			abilityIDs = append(abilityIDs, a.ID)
		}
	}
	evoID := 0
	if sp.EvolutionChainID != 0 {
		evoID = sp.EvolutionChainID
	}
	return recordResult{
		sum:        sum,
		entry:      entry,
		poke:       poke,
		sp:         sp,
		enc:        enc,
		alts:       alts,
		evoID:      evoID,
		moveIDs:    referencedMoveIDs(poke),
		abilityIDs: abilityIDs,
		formIDs:    append([]int(nil), poke.FormIDs...),
	}
}

type evolutionFetchResult struct {
	id   int
	tree domain.EvolutionTree
	err  error
}

type moveFetchResult struct {
	id   int
	move domain.Move
	err  error
}

func (s *Service) parallelEvolutionFetch(ctx context.Context, ids []int, progress DatasetProgressFunc, rep *DatasetBootstrapReport, builder *dataset.Builder) error {
	total := len(ids)
	if total == 0 {
		return nil
	}
	workers := s.datasetWorkers()
	if workers > total {
		workers = total
	}
	jobs := make(chan int, workers*2)
	results := make(chan evolutionFetchResult, workers*2)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				if ctx.Err() != nil {
					results <- evolutionFetchResult{id: id, err: ctx.Err()}
					continue
				}
				tree, err := s.fetchEvolution(ctx, id)
				results <- evolutionFetchResult{id: id, tree: tree, err: err}
			}
		}()
	}
	go func() {
		for _, id := range ids {
			select {
			case <-ctx.Done():
				close(jobs)
				return
			case jobs <- id:
			}
		}
		close(jobs)
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	done := 0
	for res := range results {
		done++
		report(progress, DatasetProgress{Stage: DatasetStageEvolution, Done: done, Total: total})
		if res.err != nil {
			rep.add("evolution", res.id, res.err, true)
			continue
		}
		builder.Evolutions[res.id] = dataset.EvolutionFromDomain(res.tree)
		rep.EvolutionOK++
	}
	return ctx.Err()
}

func (s *Service) parallelMoveFetch(ctx context.Context, ids []int, progress DatasetProgressFunc, rep *DatasetBootstrapReport, builder *dataset.Builder) error {
	total := len(ids)
	if total == 0 {
		return nil
	}
	workers := s.datasetWorkers()
	if workers > total {
		workers = total
	}
	jobs := make(chan int, workers*2)
	results := make(chan moveFetchResult, workers*2)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				if ctx.Err() != nil {
					results <- moveFetchResult{id: id, err: ctx.Err()}
					continue
				}
				mv, err := s.fetchMove(ctx, id)
				results <- moveFetchResult{id: id, move: mv, err: err}
			}
		}()
	}
	go func() {
		for _, id := range ids {
			select {
			case <-ctx.Done():
				close(jobs)
				return
			case jobs <- id:
			}
		}
		close(jobs)
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	done := 0
	var missing []int
	for res := range results {
		done++
		report(progress, DatasetProgress{Stage: DatasetStageMoves, Done: done, Total: total})
		if res.err != nil {
			// PokeAPI can name a move on a learnset and 404 the move itself
			// (zygarde-mega → nihil-light, id 920). That is not a failed download.
			if errors.Is(res.err, pokeapi.ErrNotFound) {
				rep.add("move", res.id, res.err, false)
				missing = append(missing, res.id)
				continue
			}
			rep.add("move", res.id, res.err, true)
			continue
		}
		builder.Moves[res.id] = dataset.MoveFromDomain(res.move)
		rep.MoveOK++
	}
	if n := dataset.DropMoves(builder, missing); n > 0 {
		rep.MoveWanted -= n
		debuglog.Cachef("dataset.bootstrap.moves omitted_missing=%d", n)
	}
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
	if err := s.store.RemoveDatasetDir(id); err != nil {
		debuglog.Cachef("dataset.bootstrap.cleanup err=%v", err)
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

func (s *Service) fetchPokemonByID(ctx context.Context, id int) (domain.Pokemon, error) {
	if err := s.acquire(ctx); err != nil {
		return domain.Pokemon{}, err
	}
	defer s.release()
	return s.api.GetPokemon(ctx, id)
}

func (s *Service) fetchEncounters(ctx context.Context, pokemonID int) ([]domain.Encounter, error) {
	if err := s.acquire(ctx); err != nil {
		return nil, err
	}
	defer s.release()
	return s.api.GetEncounters(ctx, pokemonID)
}

func (s *Service) parallelAbilityFetch(ctx context.Context, ids []int, progress DatasetProgressFunc, builder *dataset.Builder) error {
	var mu sync.Mutex
	return s.parallelMetaFetch(ctx, ids, "ability", progress, func(ctx context.Context, id int) error {
		a, err := s.fetchAbility(ctx, id)
		if err != nil {
			// Optional metadata — never block dataset activation.
			debuglog.Cachef("dataset.ability skip id=%d err=%v", id, err)
			return nil
		}
		if a.ID > 0 {
			got := dataset.AbilityFromDomain(a)
			mu.Lock()
			builder.Abilities[a.ID] = got
			mu.Unlock()
		}
		return nil
	})
}

func (s *Service) parallelFormFetch(ctx context.Context, ids []int, progress DatasetProgressFunc, builder *dataset.Builder) error {
	var mu sync.Mutex
	return s.parallelMetaFetch(ctx, ids, "form", progress, func(ctx context.Context, id int) error {
		f, err := s.fetchForm(ctx, id)
		if err != nil {
			debuglog.Cachef("dataset.form skip id=%d err=%v", id, err)
			return nil
		}
		if f.ID > 0 {
			got := dataset.FormFromDomain(f)
			mu.Lock()
			builder.Forms[f.ID] = got
			mu.Unlock()
		}
		return nil
	})
}

// parallelMetaFetch downloads optional metadata ids with worker pool + progress.
// kind is the progress part ("form", "ability"), not a label on screen.
func (s *Service) parallelMetaFetch(ctx context.Context, ids []int, kind string, progress DatasetProgressFunc, work func(context.Context, int) error) error {
	total := len(ids)
	if total == 0 {
		report(progress, DatasetProgress{Stage: DatasetStage(kind)})
		return nil
	}
	workers := s.datasetWorkers()
	if workers > total {
		workers = total
	}
	jobs := make(chan int, workers*2)
	results := make(chan error, workers*2)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				if ctx.Err() != nil {
					results <- ctx.Err()
					continue
				}
				results <- work(ctx, id)
			}
		}()
	}
	go func() {
		for _, id := range ids {
			select {
			case <-ctx.Done():
				close(jobs)
				return
			case jobs <- id:
			}
		}
		close(jobs)
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	done := 0
	var firstErr error
	for err := range results {
		done++
		report(progress, DatasetProgress{
			Stage: DatasetStage(kind), Done: done, Total: total,
		})
		if err != nil && firstErr == nil && !errors.Is(err, context.Canceled) {
			firstErr = err
		}
	}
	if firstErr != nil {
		return firstErr
	}
	return ctx.Err()
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
