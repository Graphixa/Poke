package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Graphixa/Poke/internal/bundled"
	"github.com/Graphixa/Poke/internal/cache/typeseed"
	"github.com/Graphixa/Poke/internal/config"
	"github.com/Graphixa/Poke/internal/debuglog"
	"github.com/Graphixa/Poke/internal/domain"
)

const interactiveReserve = 2

// DatasetProgress reports cache/dataset work without coupling to CLI or TUI rendering.
type DatasetProgress struct {
	Stage   DatasetStage
	Done    int
	Total   int
	Message string
}

// DatasetProgressFunc receives dataset progress updates. It must be safe for
// concurrent calls from enrichment workers.
type DatasetProgressFunc func(DatasetProgress)

// pokeAPI adapts app.PokeAPI plus optional slug lookup used during enrichment.
type pokeAPI interface {
	ListSpecies(context.Context) ([]domain.SpeciesSummary, error)
	GetPokemon(context.Context, int) (domain.Pokemon, error)
	GetSpecies(context.Context, int) (domain.Species, error)
	GetEvolutionChain(context.Context, int) (domain.EvolutionTree, error)
	GetEncounters(context.Context, int) ([]domain.Encounter, error)
	GetMove(context.Context, int) (domain.Move, error)
}

type pokemonBySluger interface {
	GetPokemonBySlug(context.Context, string) (domain.Pokemon, error)
}

// Service orchestrates cache-first reads, refresh, and startup policy.
type Service struct {
	store *Store
	api   pokeAPI
	cfg   config.Config
	clock Clock
	group singleFlight
	sema  chan struct{}
	// inFlight tracks active HTTP slots; background work stops at max-reserve.
	inFlight       int32
	maxConcurrency int
	localOnly      atomic.Bool
}

func NewService(store *Store, api pokeAPI, cfg config.Config, clock Clock) *Service {
	if clock == nil {
		clock = systemClock{}
	}
	concurrency := cfg.Network.MaxConcurrency
	if concurrency <= 0 {
		concurrency = 8
	}
	svc := &Service{
		store:          store,
		api:            api,
		cfg:            cfg,
		clock:          clock,
		sema:           make(chan struct{}, concurrency),
		maxConcurrency: concurrency,
	}
	return svc
}

// EnsureCompatibleCache resets incompatible prototype cache data before use.
func EnsureCompatibleCache(store *Store) error {
	return store.EnsureCompatibleDataset()
}

// MaxConcurrency returns the shared HTTP concurrency budget.
func (s *Service) MaxConcurrency() int {
	n := cap(s.sema)
	if n < 1 {
		return 8
	}
	return n
}

func (s *Service) Path() string {
	return s.store.Path()
}

// HasActiveDataset reports whether a complete snapshot is activated.
func (s *Service) HasActiveDataset() bool {
	return s.store.HasActiveDataset()
}

// LoadBrowse returns browse.json rows from the active dataset when present.
func (s *Service) LoadBrowse() ([]domain.BrowseEntry, error) {
	if !s.store.HasActiveDataset() {
		return nil, ErrNotFound
	}
	s.localOnly.Store(true)
	return s.store.LoadBrowse()
}

// PreloadDataset warms aggregates (learnsets/encounters/…) after browse is up.
func (s *Service) PreloadDataset() error {
	return s.store.PreloadDataset()
}

func (s *Service) Clear() error {
	return s.store.Clear()
}

// EnsureIndex returns the browse index from the active dataset when present,
// otherwise the cached species index. An empty cache triggers a blocking thin
// index fetch only (full metadata bootstrap is explicit via BootstrapDataset).
func (s *Service) EnsureIndex(ctx context.Context, progress DatasetProgressFunc) (index []domain.SpeciesSummary, stale bool, err error) {
	if s.store.HasActiveDataset() {
		s.localOnly.Store(true)
		browse, berr := s.store.LoadBrowseSummaries()
		if berr != nil {
			return nil, false, berr
		}
		if len(browse) == 0 {
			return nil, false, ErrEmpty
		}
		return browse, false, nil
	}
	index, fetchedAt, err := s.loadIndex()
	if err == nil && len(index) > 0 {
		stale = s.isStale(fetchedAt, s.cfg.Cache.IndexTTL.Duration)
		index, _ = typeseed.Apply(index)
		if s.store.HasActiveDataset() {
			s.localOnly.Store(true)
			if browse, berr := s.store.LoadBrowseSummaries(); berr == nil && len(browse) > 0 {
				return browse, stale, nil
			}
		}
		return index, stale, nil
	}
	if err != nil && !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrCorrupt) {
		return nil, false, err
	}
	if refreshErr := s.refreshIndex(ctx, progress); refreshErr != nil {
		return nil, false, &OfflineError{Err: refreshErr}
	}
	index, _, err = s.loadIndex()
	if err != nil {
		return nil, false, err
	}
	if len(index) == 0 {
		return nil, false, ErrEmpty
	}
	index, missing := typeseed.Apply(index)
	if len(missing) == 0 {
		// Persist seeded types so disk matches what browse shows.
		_ = s.store.StoreSpeciesIndex(ctx, index, s.clock.Now())
		return index, false, nil
	}
	_ = s.store.StoreSpeciesIndex(ctx, index, s.clock.Now())
	// Only gap-fill IDs the seed does not know.
	s.BackgroundEnrich(ctx, progress)
	return index, false, nil
}

// ShouldBackgroundRefresh reports whether a stale index should refresh in the background.
func (s *Service) ShouldBackgroundRefresh(stale bool) bool {
	return stale && s.cfg.Cache.AutoRefresh
}

// NeedsTypeEnrichment reports whether any index row still lacks types after seeding.
func NeedsTypeEnrichment(index []domain.SpeciesSummary) bool {
	_, missing := typeseed.Apply(append([]domain.SpeciesSummary(nil), index...))
	return len(missing) > 0
}

// BackgroundRefresh starts at most one in-flight Refresh. It does not block the caller.
func (s *Service) BackgroundRefresh(ctx context.Context, progress DatasetProgressFunc) {
	go func() {
		debuglog.Cachef("background refresh start")
		err := s.Refresh(ctx, progress)
		if err != nil {
			debuglog.Cachef("background refresh err=%v", err)
		} else {
			debuglog.Cachef("background refresh done")
		}
	}()
}

// BackgroundEnrich starts type enrichment without blocking (single-flight).
func (s *Service) BackgroundEnrich(ctx context.Context, progress DatasetProgressFunc) {
	go func() {
		debuglog.Cachef("background enrich start")
		err := s.EnrichTypes(ctx, progress)
		if err != nil {
			debuglog.Cachef("background enrich err=%v", err)
		} else {
			debuglog.Cachef("background enrich done")
		}
	}()
}

// Refresh fetches the national species index and enriches types (CLI path; waits for enrich).
// Concurrent Refresh calls share a single in-flight run (single-flight).
func (s *Service) Refresh(ctx context.Context, progress DatasetProgressFunc) error {
	_, err := s.group.Do("refresh", func() (any, error) {
		if err := s.refreshIndexOnce(ctx, progress); err != nil {
			return nil, err
		}
		index, _, err := s.loadIndex()
		if err != nil {
			return nil, err
		}
		return nil, s.enrichTypes(ctx, index, progress)
	})
	return err
}

// RefreshDataset unpacks the embedded snapshot beside the active one and
// activates it when the manifest is complete. The same source_sha is a no-op.
// context.Canceled does not crawl. Any other unpack error uses BootstrapDataset.
// A failed unpack leaves the previous active snapshot in place.
func (s *Service) RefreshDataset(ctx context.Context, progress DatasetProgressFunc) error {
	before, _ := s.store.ActiveDatasetID()
	err := s.store.Unpack(ctx, bundled.Bytes())
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return err
		}
		return s.BootstrapDataset(ctx, progress)
	}
	after, _ := s.store.ActiveDatasetID()
	if after != "" && after != before {
		report(progress, DatasetProgress{Stage: DatasetStageImport, Done: 1, Total: 1})
	}
	return nil
}

// LocalOnly reports whether normal reads must not hit the network.
func (s *Service) LocalOnly() bool {
	return s.localOnly.Load()
}

// EnrichTypes fills missing type hints on the cached index (sync).
func (s *Service) EnrichTypes(ctx context.Context, progress DatasetProgressFunc) error {
	_, err := s.group.Do("enrich", func() (any, error) {
		index, _, err := s.loadIndex()
		if err != nil {
			return nil, err
		}
		return nil, s.enrichTypes(ctx, index, progress)
	})
	return err
}

func (s *Service) refreshIndex(ctx context.Context, progress DatasetProgressFunc) error {
	_, err := s.group.Do("refresh-index", func() (any, error) {
		return nil, s.refreshIndexOnce(ctx, progress)
	})
	return err
}

func (s *Service) refreshIndexOnce(ctx context.Context, progress DatasetProgressFunc) error {
	if err := s.store.EnsureRoot(); err != nil {
		return err
	}
	report(progress, DatasetProgress{Stage: DatasetStageIndex, Message: "Fetching species index"})
	summaries, err := s.api.ListSpecies(ctx)
	if err != nil {
		return err
	}
	existing, _, loadErr := s.loadIndex()
	if loadErr == nil {
		summaries = mergeTypeHints(summaries, existing)
	}
	summaries, _ = typeseed.Apply(summaries)
	now := s.clock.Now()
	if err := s.store.StoreSpeciesIndex(ctx, summaries, now); err != nil {
		return err
	}
	report(progress, DatasetProgress{Stage: DatasetStageIndex, Done: 1, Total: 1, Message: fmt.Sprintf("Indexed %d species", len(summaries))})
	return nil
}

func (s *Service) enrichTypes(ctx context.Context, summaries []domain.SpeciesSummary, progress DatasetProgressFunc) error {
	type job struct {
		idx int
		sum domain.SpeciesSummary
	}
	summaries, _ = typeseed.Apply(summaries)
	jobs := make([]job, 0)
	for i, sum := range summaries {
		if len(sum.Types) == 0 {
			jobs = append(jobs, job{idx: i, sum: sum})
		}
	}
	total := len(jobs)
	if total == 0 {
		_ = s.store.StoreSpeciesIndex(ctx, summaries, s.clock.Now())
		report(progress, DatasetProgress{Stage: DatasetStageEnrich, Done: 0, Total: 0, Message: "Types already enriched"})
		return nil
	}

	workers := s.MaxConcurrency()
	queue := make(chan job, workers*2)

	var (
		done     atomic.Int64
		mu       sync.Mutex
		index    = append([]domain.SpeciesSummary(nil), summaries...)
		firstErr error
	)

	var wg sync.WaitGroup
	worker := func() {
		defer wg.Done()
		for j := range queue {
			if err := ctx.Err(); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			pokemon, err := s.fetchDefaultPokemon(ctx, j.sum)
			if err != nil {
				if ctx.Err() != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = ctx.Err()
					}
					mu.Unlock()
				}
				current := int(done.Add(1))
				report(progress, DatasetProgress{Stage: DatasetStageEnrich, Done: current, Total: total, Message: j.sum.Slug})
				continue
			}
			types := make([]string, 0, len(pokemon.Types))
			for _, t := range pokemon.Types {
				types = append(types, t.Name)
			}

			mu.Lock()
			index[j.idx].Types = types
			mu.Unlock()

			current := int(done.Add(1))
			report(progress, DatasetProgress{Stage: DatasetStageEnrich, Done: current, Total: total, Message: j.sum.DisplayName})
		}
	}

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go worker()
	}

	for _, j := range jobs {
		if err := ctx.Err(); err != nil {
			close(queue)
			wg.Wait()
			return err
		}
		mu.Lock()
		abort := firstErr != nil
		mu.Unlock()
		if abort {
			break
		}
		select {
		case <-ctx.Done():
			close(queue)
			wg.Wait()
			return ctx.Err()
		case queue <- j:
		}
	}
	close(queue)
	wg.Wait()

	mu.Lock()
	snapshot := append([]domain.SpeciesSummary(nil), index...)
	err := firstErr
	mu.Unlock()
	_ = s.store.StoreSpeciesIndex(ctx, snapshot, s.clock.Now())
	if err != nil {
		return err
	}
	return ctx.Err()
}

func (s *Service) fetchDefaultPokemon(ctx context.Context, sum domain.SpeciesSummary) (domain.Pokemon, error) {
	if err := s.acquire(ctx); err != nil {
		return domain.Pokemon{}, err
	}
	defer s.release()

	pokemon, err := s.api.GetPokemon(ctx, sum.ID)
	if err == nil {
		return pokemon, nil
	}
	if bySlug, ok := s.api.(pokemonBySluger); ok {
		return bySlug.GetPokemonBySlug(ctx, sum.Slug)
	}
	return domain.Pokemon{}, err
}

func (s *Service) acquire(ctx context.Context) error {
	reserve := interactiveReserve
	if reserve >= s.maxConcurrency {
		reserve = 0
	}
	bgLimit := int32(s.maxConcurrency - reserve)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !interactivePriority(ctx) {
			if atomic.LoadInt32(&s.inFlight) >= bgLimit {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(5 * time.Millisecond):
					continue
				}
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case s.sema <- struct{}{}:
			atomic.AddInt32(&s.inFlight, 1)
			return nil
		}
	}
}

func (s *Service) release() {
	<-s.sema
	atomic.AddInt32(&s.inFlight, -1)
}

func (s *Service) loadIndex() ([]domain.SpeciesSummary, time.Time, error) {
	index, err := s.store.SpeciesIndex(context.Background())
	if err != nil {
		return nil, time.Time{}, err
	}
	fetchedAt, manErr := s.store.IndexFetchedAt()
	if manErr != nil {
		// Index file exists but manifest missing: treat as stale epoch.
		return index, time.Time{}, nil
	}
	return index, fetchedAt, nil
}

func (s *Service) isStale(fetchedAt time.Time, ttl time.Duration) bool {
	if fetchedAt.IsZero() || ttl <= 0 {
		return true
	}
	return s.clock.Now().After(fetchedAt.Add(ttl))
}

func mergeTypeHints(fresh, existing []domain.SpeciesSummary) []domain.SpeciesSummary {
	byID := make(map[int][]string, len(existing))
	for _, item := range existing {
		if len(item.Types) > 0 {
			byID[item.ID] = item.Types
		}
	}
	for i := range fresh {
		if types, ok := byID[fresh[i].ID]; ok && len(fresh[i].Types) == 0 {
			fresh[i].Types = types
		}
	}
	return fresh
}

func report(fn DatasetProgressFunc, p DatasetProgress) {
	if fn != nil {
		fn(p)
	}
}

// singleFlight deduplicates in-flight work by key.
type singleFlight struct {
	mu    sync.Mutex
	calls map[string]*flightCall
}

type flightCall struct {
	wg  sync.WaitGroup
	val any
	err error
}

func (g *singleFlight) Do(key string, fn func() (any, error)) (any, error) {
	g.mu.Lock()
	if g.calls == nil {
		g.calls = make(map[string]*flightCall)
	}
	if c, ok := g.calls[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}
	c := &flightCall{}
	c.wg.Add(1)
	g.calls[key] = c
	g.mu.Unlock()

	c.val, c.err = fn()
	c.wg.Done()

	g.mu.Lock()
	delete(g.calls, key)
	g.mu.Unlock()
	return c.val, c.err
}
