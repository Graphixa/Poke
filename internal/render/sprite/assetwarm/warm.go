// Package assetwarm downloads named PNG sprite assets (pixel or official artwork)
// for the species index without requiring chafa artefact rendering.
package assetwarm

import (
	"container/heap"
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

const (
	PrioritySelected   = 0
	PriorityNeighbor   = 1
	PriorityBackground = 2
)

// PokemonLoader loads Pokémon detail (including sprite URLs) by species ID.
type PokemonLoader interface {
	Pokemon(ctx context.Context, id int) (domain.Pokemon, error)
}

// Resolver fetches and caches PNG bytes for a tier.
type Resolver interface {
	Resolve(ctx context.Context, pokemon domain.Pokemon, tier ...sprite.AssetTier) sprite.SourceResult
}

// ProgressFunc receives progress updates. Must be safe for concurrent calls.
type ProgressFunc func(cache.Progress)

// Mode selects which asset family to download.
type Mode string

const (
	ModePixel    Mode = "pixel"
	ModeArtwork  Mode = "artwork"
	ModeOff      Mode = "off"
)

// Config configures an asset warm Service.
type Config struct {
	Mode     Mode
	Workers  int
	Loader   PokemonLoader
	Resolver Resolver
	Index    []domain.SpeciesSummary
	Progress ProgressFunc
	// Stage label reported in Progress (default depends on Mode).
	Stage string
	Message string
}

// Service owns a priority download queue.
type Service struct {
	cfg Config

	mu      sync.Mutex
	pq      priorityQueue
	seq     int64
	queued  map[int]bool
	done    map[int]bool
	wake    chan struct{}
	started atomic.Bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	doneN   atomic.Int64
	totalN  atomic.Int64
}

// New creates an asset warm service. Call Start to begin workers.
func New(cfg Config) *Service {
	if cfg.Workers < 1 {
		cfg.Workers = runtime.GOMAXPROCS(0)
		if cfg.Workers < 1 {
			cfg.Workers = 2
		}
		if cfg.Workers > 8 {
			cfg.Workers = 8
		}
	}
	if cfg.Mode == "" {
		cfg.Mode = ModePixel
	}
	if cfg.Stage == "" {
		if cfg.Mode == ModeArtwork {
			cfg.Stage = "artwork"
		} else {
			cfg.Stage = "sprites"
		}
	}
	if cfg.Message == "" {
		if cfg.Mode == ModeArtwork {
			cfg.Message = "Downloading official artwork"
		} else {
			cfg.Message = "Downloading sprites"
		}
	}
	return &Service{
		cfg:    cfg,
		queued: make(map[int]bool),
		done:   make(map[int]bool),
		wake:   make(chan struct{}, 1),
	}
}

// Start launches workers and enqueues the full index.
func (s *Service) Start(ctx context.Context) {
	if s == nil || s.cfg.Mode == ModeOff || s.cfg.Loader == nil || s.cfg.Resolver == nil {
		return
	}
	if !s.started.CompareAndSwap(false, true) {
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.enqueueAll(PriorityBackground)
	for i := 0; i < s.cfg.Workers; i++ {
		s.wg.Add(1)
		go s.worker(runCtx)
	}
}

// Stop cancels workers and waits for them to exit.
func (s *Service) Stop() {
	if s == nil || !s.started.Load() {
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

// Prioritize bumps the selected species to P0.
func (s *Service) Prioritize(speciesID int) {
	if s == nil || speciesID < 1 || s.cfg.Mode == ModeOff {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done[speciesID] {
		return
	}
	s.pushLocked(job{speciesID: speciesID, priority: PrioritySelected})
	s.signal()
}

// SetNeighbors enqueues ±radius neighbors at P1.
func (s *Service) SetNeighbors(selectedID int, radius int) {
	if s == nil || s.cfg.Mode == ModeOff || radius < 1 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, sum := range s.cfg.Index {
		if sum.ID == selectedID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	lo := idx - radius
	if lo < 0 {
		lo = 0
	}
	hi := idx + radius
	if hi >= len(s.cfg.Index) {
		hi = len(s.cfg.Index) - 1
	}
	for i := lo; i <= hi; i++ {
		id := s.cfg.Index[i].ID
		if id == selectedID || s.done[id] {
			continue
		}
		s.pushLocked(job{speciesID: id, priority: PriorityNeighbor})
	}
	s.signal()
}

// Wait blocks until all queued work is done or ctx is cancelled.
func (s *Service) Wait(ctx context.Context) error {
	if s == nil {
		return nil
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		done, total := s.Snapshot()
		if total > 0 && done >= total {
			return nil
		}
		if total == 0 && len(s.cfg.Index) == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// Snapshot returns current done/total counts.
func (s *Service) Snapshot() (done, total int) {
	if s == nil {
		return 0, 0
	}
	return int(s.doneN.Load()), int(s.totalN.Load())
}

func (s *Service) enqueueAll(priority int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sum := range s.cfg.Index {
		if s.done[sum.ID] || s.queued[sum.ID] {
			continue
		}
		s.pushLocked(job{speciesID: sum.ID, priority: priority})
	}
	s.totalN.Store(int64(len(s.cfg.Index)))
	s.signal()
}

func (s *Service) pushLocked(j job) {
	if j.speciesID < 1 {
		return
	}
	s.seq++
	j.seq = s.seq
	heap.Push(&s.pq, j)
	s.queued[j.speciesID] = true
}

func (s *Service) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Service) worker(ctx context.Context) {
	defer s.wg.Done()
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		j, ok := s.pop()
		if !ok {
			select {
			case <-ctx.Done():
				return
			case <-s.wake:
			}
			continue
		}
		s.runJob(ctx, j)
	}
}

func (s *Service) pop() (job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for s.pq.Len() > 0 {
		j := heap.Pop(&s.pq).(job)
		if s.done[j.speciesID] {
			delete(s.queued, j.speciesID)
			continue
		}
		delete(s.queued, j.speciesID)
		return j, true
	}
	return job{}, false
}

func (s *Service) runJob(ctx context.Context, j job) {
	poke, err := s.cfg.Loader.Pokemon(ctx, j.speciesID)
	if err != nil {
		debuglog.Renderf("assetwarm skip species=%d err=%v", j.speciesID, err)
		s.markDone(j.speciesID)
		s.report()
		return
	}
	tier := sprite.AssetTierPixel
	if s.cfg.Mode == ModeArtwork {
		tier = sprite.AssetTierHD
	}
	if sprite.URLForTier(poke, tier) == "" {
		// Still try named disk; Resolve handles miss.
	}
	if err := ctx.Err(); err != nil {
		return
	}
	res := s.cfg.Resolver.Resolve(ctx, poke, tier)
	if res.UsePlaceholder || len(res.PNG) == 0 {
		debuglog.Renderf("assetwarm miss species=%d mode=%s", j.speciesID, s.cfg.Mode)
	}
	s.markDone(j.speciesID)
	s.report()
}

func (s *Service) markDone(speciesID int) {
	s.mu.Lock()
	if !s.done[speciesID] {
		s.done[speciesID] = true
		s.doneN.Add(1)
	}
	s.mu.Unlock()
}

func (s *Service) report() {
	if s.cfg.Progress == nil {
		return
	}
	s.cfg.Progress(cache.Progress{
		Stage:   s.cfg.Stage,
		Done:    int(s.doneN.Load()),
		Total:   int(s.totalN.Load()),
		Message: s.cfg.Message,
	})
}

type job struct {
	speciesID int
	priority  int
	seq       int64
}

type priorityQueue []job

func (pq priorityQueue) Len() int { return len(pq) }
func (pq priorityQueue) Less(i, j int) bool {
	if pq[i].priority != pq[j].priority {
		return pq[i].priority < pq[j].priority
	}
	return pq[i].seq < pq[j].seq
}
func (pq priorityQueue) Swap(i, j int) { pq[i], pq[j] = pq[j], pq[i] }
func (pq *priorityQueue) Push(x any)   { *pq = append(*pq, x.(job)) }
func (pq *priorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[:n-1]
	return item
}
