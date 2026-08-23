package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/domain"
)

// Progress reports refresh work without coupling to CLI or TUI rendering.
type Progress struct {
	Stage   string
	Done    int
	Total   int
	Message string
}

// ProgressFunc receives progress updates during refresh. It must be safe for
// concurrent calls from enrichment workers.
type ProgressFunc func(Progress)

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
}

func NewService(store *Store, api pokeAPI, cfg config.Config, clock Clock) *Service {
	if clock == nil {
		clock = systemClock{}
	}
	concurrency := cfg.Network.MaxConcurrency
	if concurrency <= 0 {
		concurrency = 4
	}
	return &Service{
		store: store,
		api:   api,
		cfg:   cfg,
		clock: clock,
		sema:  make(chan struct{}, concurrency),
	}
}

func (s *Service) Path() string {
	return s.store.Path()
}

func (s *Service) Clear() error {
	return s.store.Clear()
}

// EnsureIndex applies the startup cache policy:
//   - fresh cache: return immediately with no network
//   - stale cache: return immediately; caller may start BackgroundRefresh
//   - empty cache: block for bootstrap; return OfflineError when the network fails
//
// progress may be nil; it is used only when an empty cache triggers Refresh.
func (s *Service) EnsureIndex(ctx context.Context, progress ProgressFunc) (index []domain.SpeciesSummary, stale bool, err error) {
	index, fetchedAt, err := s.loadIndex()
	if err == nil && len(index) > 0 {
		stale = s.isStale(fetchedAt, s.cfg.Cache.IndexTTL.Duration)
		return index, stale, nil
	}
	if err != nil && !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrCorrupt) {
		return nil, false, err
	}
	if refreshErr := s.Refresh(ctx, progress); refreshErr != nil {
		return nil, false, &OfflineError{Err: refreshErr}
	}
	index, _, err = s.loadIndex()
	if err != nil {
		return nil, false, err
	}
	if len(index) == 0 {
		return nil, false, ErrEmpty
	}
	return index, false, nil
}

// ShouldBackgroundRefresh reports whether a stale index should refresh in the background.
func (s *Service) ShouldBackgroundRefresh(stale bool) bool {
	return stale && (s.cfg.Cache.RefreshOnStart || s.cfg.Cache.AutoRefresh)
}

// BackgroundRefresh starts at most one in-flight Refresh. It does not block the caller.
func (s *Service) BackgroundRefresh(ctx context.Context, progress ProgressFunc) {
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

// Refresh fetches the national species index and enriches types with bounded concurrency.
// Partial progress is persisted after the index write and after each enrichment update batch.
// Concurrent Refresh calls share a single in-flight run (single-flight).
func (s *Service) Refresh(ctx context.Context, progress ProgressFunc) error {
	_, err := s.group.Do("refresh", func() (any, error) {
		return nil, s.refreshOnce(ctx, progress)
	})
	return err
}

func (s *Service) refreshOnce(ctx context.Context, progress ProgressFunc) error {
	if err := s.store.EnsureRoot(); err != nil {
		return err
	}
	report(progress, Progress{Stage: "index", Message: "Fetching species index"})
	summaries, err := s.api.ListSpecies(ctx)
	if err != nil {
		return err
	}
	existing, _, loadErr := s.loadIndex()
	if loadErr == nil {
		summaries = mergeTypeHints(summaries, existing)
	}
	now := s.clock.Now()
	if err := s.store.StoreSpeciesIndex(ctx, summaries, now); err != nil {
		return err
	}
	report(progress, Progress{Stage: "index", Done: 1, Total: 1, Message: fmt.Sprintf("Indexed %d species", len(summaries))})

	return s.enrichTypes(ctx, summaries, progress)
}

func (s *Service) enrichTypes(ctx context.Context, summaries []domain.SpeciesSummary, progress ProgressFunc) error {
	type job struct {
		idx int
		sum domain.SpeciesSummary
	}
	jobs := make([]job, 0, len(summaries))
	for i, sum := range summaries {
		if len(sum.Types) == 0 {
			jobs = append(jobs, job{idx: i, sum: sum})
		}
	}
	total := len(jobs)
	if total == 0 {
		report(progress, Progress{Stage: "enrich", Done: 0, Total: 0, Message: "Types already enriched"})
		return nil
	}

	workers := cap(s.sema)
	if workers <= 0 {
		workers = 4
	}
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
				report(progress, Progress{Stage: "enrich", Done: current, Total: total, Message: j.sum.Slug})
				continue
			}
			types := make([]string, 0, len(pokemon.Types))
			for _, t := range pokemon.Types {
				types = append(types, t.Name)
			}

			mu.Lock()
			index[j.idx].Types = types
			snapshot := append([]domain.SpeciesSummary(nil), index...)
			mu.Unlock()

			_ = s.store.StorePokemon(ctx, pokemon, s.clock.Now())
			_ = s.store.StoreSpeciesIndex(ctx, snapshot, s.clock.Now())

			current := int(done.Add(1))
			report(progress, Progress{Stage: "enrich", Done: current, Total: total, Message: j.sum.DisplayName})
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
	final := append([]domain.SpeciesSummary(nil), index...)
	err := firstErr
	mu.Unlock()
	if storeErr := s.store.StoreSpeciesIndex(ctx, final, s.clock.Now()); storeErr != nil {
		return storeErr
	}
	if err != nil {
		return err
	}
	return ctx.Err()
}

func (s *Service) fetchDefaultPokemon(ctx context.Context, sum domain.SpeciesSummary) (domain.Pokemon, error) {
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
	select {
	case <-ctx.Done():
		return ctx.Err()
	case s.sema <- struct{}{}:
		return nil
	}
}

func (s *Service) release() {
	<-s.sema
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

func report(fn ProgressFunc, p Progress) {
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
