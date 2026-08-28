// Package nativewarm pre-encodes Kitty/iTerm/Sixel frames into the native disk cache.
package nativewarm

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

// PokemonLoader loads Pokémon detail by species ID.
type PokemonLoader interface {
	Pokemon(ctx context.Context, id int) (domain.Pokemon, error)
}

// NativeRenderer encodes native graphics frames into the artefact cache.
type NativeRenderer interface {
	HasNativeCache(ctx context.Context, pokemon domain.Pokemon, cells int) bool
	NativeWarmRender(ctx context.Context, pokemon domain.Pokemon, cells int) sprite.RenderResult
}

// ProgressFunc receives warm progress updates.
type ProgressFunc func(cache.Progress)

// Config configures a native warm Service.
type Config struct {
	Cells    int
	Workers  int
	Loader   PokemonLoader
	Renderer NativeRenderer
	Index    []domain.SpeciesSummary
	Progress ProgressFunc
	// OnlyIDs when non-empty restricts background encode to these species (poke update).
	OnlyIDs map[int]struct{}
}

// Service owns the native encode priority queue.
type Service struct {
	cfg Config

	mu      sync.Mutex
	pq      priorityQueue
	seq     int64
	queued  map[int]bool
	done    map[int]bool
	cells   int
	wake    chan struct{}
	started atomic.Bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	doneN   atomic.Int64
	totalN  atomic.Int64
}

// New creates a native warm service. Call Start to begin workers.
func New(cfg Config) *Service {
	if cfg.Workers < 1 {
		cfg.Workers = runtime.GOMAXPROCS(0)
		if cfg.Workers < 1 {
			cfg.Workers = 2
		}
		if cfg.Workers > 2 {
			cfg.Workers = 2
		}
	}
	if cfg.Cells < sprite.MinFrameSize {
		cfg.Cells = sprite.MaxFrameSize
	}
	cfg.Cells = sprite.ClampFrameSide(cfg.Cells)
	return &Service{
		cfg:    cfg,
		queued: make(map[int]bool),
		done:   make(map[int]bool),
		cells:  cfg.Cells,
		wake:   make(chan struct{}, 1),
	}
}

// Start launches worker goroutines.
func (s *Service) Start(ctx context.Context) {
	if s == nil || s.cfg.Renderer == nil || s.cfg.Loader == nil {
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

// SetCells updates the frame bucket for subsequent jobs.
func (s *Service) SetCells(cells int) {
	if s == nil {
		return
	}
	cells = sprite.ClampFrameSide(cells)
	s.mu.Lock()
	defer s.mu.Unlock()
	if cells == s.cells {
		return
	}
	s.cells = cells
	s.done = make(map[int]bool)
	s.doneN.Store(0)
	for _, sum := range s.cfg.Index {
		if s.shouldEnqueue(sum.ID) && !s.queued[sum.ID] {
			s.pushLocked(job{speciesID: sum.ID, priority: PriorityBackground})
		}
	}
	s.totalN.Store(int64(s.countEnqueueable()))
	s.signal()
}

// Prioritize bumps the selected species to P0.
func (s *Service) Prioritize(speciesID int) {
	if s == nil || speciesID < 1 {
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

// SetNeighborsFromList enqueues list neighbors at P1.
func (s *Service) SetNeighborsFromList(list []domain.SpeciesSummary, cursor, radius int) {
	if s == nil || radius < 1 || len(list) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(list) {
		cursor = len(list) - 1
	}
	for d := 1; d <= radius; d++ {
		if i := cursor - d; i >= 0 {
			id := list[i].ID
			if !s.done[id] {
				s.pushLocked(job{speciesID: id, priority: PriorityNeighbor})
			}
		}
		if i := cursor + d; i < len(list) {
			id := list[i].ID
			if !s.done[id] {
				s.pushLocked(job{speciesID: id, priority: PriorityNeighbor})
			}
		}
	}
	s.signal()
}

func (s *Service) enqueueAll(priority int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sum := range s.cfg.Index {
		if !s.shouldEnqueue(sum.ID) || s.done[sum.ID] || s.queued[sum.ID] {
			continue
		}
		s.pushLocked(job{speciesID: sum.ID, priority: priority})
	}
	s.totalN.Store(int64(s.countEnqueueable()))
	s.signal()
}

func (s *Service) shouldEnqueue(speciesID int) bool {
	if len(s.cfg.OnlyIDs) == 0 {
		return true
	}
	_, ok := s.cfg.OnlyIDs[speciesID]
	return ok
}

func (s *Service) countEnqueueable() int {
	if len(s.cfg.OnlyIDs) == 0 {
		return len(s.cfg.Index)
	}
	n := 0
	for _, sum := range s.cfg.Index {
		if s.shouldEnqueue(sum.ID) {
			n++
		}
	}
	return n
}

func (s *Service) pushLocked(j job) {
	if j.speciesID < 1 || !s.shouldEnqueue(j.speciesID) {
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
	cells := s.cells
	poke, err := s.cfg.Loader.Pokemon(ctx, j.speciesID)
	if err != nil {
		debuglog.Renderf("nativewarm skip species=%d err=%v", j.speciesID, err)
		s.markDone(j.speciesID)
		s.report(j.speciesID)
		return
	}
	if err := ctx.Err(); err != nil {
		return
	}
	if s.cfg.Renderer.HasNativeCache(ctx, poke, cells) {
		s.markDone(j.speciesID)
		s.report(j.speciesID)
		return
	}
	res := s.cfg.Renderer.NativeWarmRender(ctx, poke, cells)
	if res.UsePlaceholder || !res.Native {
		debuglog.Renderf("nativewarm miss species=%d backend=%s", j.speciesID, res.Backend)
	}
	s.markDone(j.speciesID)
	s.report(j.speciesID)
}

func (s *Service) markDone(speciesID int) {
	s.mu.Lock()
	if !s.done[speciesID] {
		s.done[speciesID] = true
		s.doneN.Add(1)
	}
	s.mu.Unlock()
}

func (s *Service) report(speciesID int) {
	if s.cfg.Progress == nil {
		return
	}
	s.cfg.Progress(cache.Progress{
		Stage:     "native",
		Done:      int(s.doneN.Load()),
		Total:     int(s.totalN.Load()),
		Message:   "Building image cache",
		SpeciesID: speciesID,
	})
}

// Snapshot returns current done/total counts.
func (s *Service) Snapshot() (done, total int) {
	if s == nil {
		return 0, 0
	}
	return int(s.doneN.Load()), int(s.totalN.Load())
}

// Wait blocks until queued work completes or ctx is cancelled.
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
