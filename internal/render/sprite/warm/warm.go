// Package warm runs a priority queue that precomputes chafa sprite artefacts
// on the user's machine. Selection (P0) and list neighbors (P1) preempt
// background warm-all (P2). No sprite packs are shipped with the binary.
package warm

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

// Priority levels for warm jobs. Lower values run first.
const (
	PrioritySelected   = 0
	PriorityNeighbor   = 1
	PriorityBackground = 2
)

// PokemonLoader loads Pokémon detail (including sprite URLs) by species ID.
type PokemonLoader interface {
	Pokemon(ctx context.Context, id int) (domain.Pokemon, error)
}

// Renderer warms a single Pokémon into the artefact cache.
type Renderer interface {
	WarmRender(ctx context.Context, pokemon domain.Pokemon, cells int) sprite.RenderResult
}

// ProgressFunc receives warm progress updates. Must be safe for concurrent calls.
type ProgressFunc func(cache.Progress)

// Mode controls warm scope.
type Mode string

const (
	ModeAllPixel Mode = "all_pixel"
	ModeWindow   Mode = "window"
	ModeOff      Mode = "off"
)

// Config configures a warm Service.
type Config struct {
	Mode     Mode
	Cells    int
	Workers  int
	Loader   PokemonLoader
	Renderer Renderer
	Index    []domain.SpeciesSummary
	Progress ProgressFunc
}

// Service owns the priority warm queue.
type Service struct {
	cfg Config

	mu       sync.Mutex
	pq       priorityQueue
	seq      int64
	queued   map[int]bool // species currently in queue
	done     map[int]bool // species successfully warmed for current cells
	cells    int
	wake     chan struct{}
	started  atomic.Bool
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	doneN    atomic.Int64
	totalN   atomic.Int64
	activity sprite.ActivityGate
}

// New creates a warm service. Call Start to begin workers.
func New(cfg Config) *Service {
	if cfg.Workers < 1 {
		cfg.Workers = runtime.GOMAXPROCS(0)
		if cfg.Workers < 1 {
			cfg.Workers = 2
		}
		if cfg.Workers > 4 {
			cfg.Workers = 4
		}
	}
	if cfg.Cells < sprite.MinFrameSize {
		cfg.Cells = sprite.MaxFrameSize
	}
	cfg.Cells = sprite.ClampFrameSide(cfg.Cells)
	if cfg.Mode == "" {
		cfg.Mode = ModeAllPixel
	}
	return &Service{
		cfg:    cfg,
		queued: make(map[int]bool),
		done:   make(map[int]bool),
		cells:  cfg.Cells,
		wake:   make(chan struct{}, 1),
	}
}

// Start launches worker goroutines. Safe to call once; subsequent calls no-op.
func (s *Service) Start(ctx context.Context) {
	if s == nil || s.cfg.Mode == ModeOff || s.cfg.Renderer == nil || s.cfg.Loader == nil {
		return
	}
	if !s.started.CompareAndSwap(false, true) {
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	if s.cfg.Mode == ModeAllPixel {
		s.enqueueAll(PriorityBackground)
	}

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

// SetCells updates the frame bucket for subsequent jobs. Marks prior work stale
// when the bucket changes so warm can refill.
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
	if s.cfg.Mode == ModeAllPixel {
		// Re-queue background after size change.
		for _, sum := range s.cfg.Index {
			if s.queued[sum.ID] {
				continue
			}
			s.pushLocked(job{speciesID: sum.ID, priority: PriorityBackground})
		}
		s.totalN.Store(int64(len(s.cfg.Index)))
	}
	s.signal()
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

// NoteActivity records TUI selection motion so full-dex (P2) work pauses.
func (s *Service) NoteActivity() {
	if s == nil {
		return
	}
	s.activity.NoteActivity()
	s.signal()
}

// SetForegroundBusy pauses P2 work while a foreground cold encode is running.
func (s *Service) SetForegroundBusy(busy bool) {
	if s == nil {
		return
	}
	s.activity.SetForegroundBusy(busy)
	s.signal()
}

// SetNeighbors enqueues ±radius neighbors at P1 (window around selection).
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
		// Fall back to filtered-order unknown: still prioritize selection only.
		return
	}
	for d := 1; d <= radius; d++ {
		if i := idx - d; i >= 0 {
			id := s.cfg.Index[i].ID
			if !s.done[id] {
				s.pushLocked(job{speciesID: id, priority: PriorityNeighbor})
			}
		}
		if i := idx + d; i < len(s.cfg.Index) {
			id := s.cfg.Index[i].ID
			if !s.done[id] {
				s.pushLocked(job{speciesID: id, priority: PriorityNeighbor})
			}
		}
	}
	s.signal()
}

// SetNeighborsFromList uses the visible filtered list order for neighbor warm.
func (s *Service) SetNeighborsFromList(list []domain.SpeciesSummary, cursor, radius int) {
	if s == nil || s.cfg.Mode == ModeOff || radius < 1 || len(list) == 0 {
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
	// Promote existing queued job by pushing a higher-priority entry.
	// Workers skip if already done; duplicates are cheap.
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
			case <-time.After(sprite.InteractiveIdle):
			}
			continue
		}
		s.runJob(ctx, j)
	}
}

func (s *Service) pop() (job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	throttle := s.activity.ThrottleBackground()
	var skipped []job
	for s.pq.Len() > 0 {
		j := heap.Pop(&s.pq).(job)
		if s.done[j.speciesID] {
			delete(s.queued, j.speciesID)
			continue
		}
		if throttle && j.priority >= PriorityBackground {
			skipped = append(skipped, j)
			continue
		}
		delete(s.queued, j.speciesID)
		for _, sk := range skipped {
			heap.Push(&s.pq, sk)
		}
		return j, true
	}
	for _, sk := range skipped {
		heap.Push(&s.pq, sk)
	}
	return job{}, false
}

func (s *Service) runJob(ctx context.Context, j job) {
	if j.priority >= PriorityBackground && s.activity.ThrottleBackground() {
		s.requeue(j)
		return
	}
	cells := s.cells
	poke, err := s.cfg.Loader.Pokemon(ctx, j.speciesID)
	if err != nil || poke.Sprites.FrontDefault == "" {
		debuglog.Renderf("warm skip species=%d err=%v", j.speciesID, err)
		s.markDone(j.speciesID)
		s.report(j.speciesID)
		return
	}
	if err := ctx.Err(); err != nil {
		return
	}
	res := s.cfg.Renderer.WarmRender(ctx, poke, cells)
	if res.UsePlaceholder && res.Backend != "chafa" {
		debuglog.Renderf("warm placeholder species=%d", j.speciesID)
	}
	s.markDone(j.speciesID)
	s.report(j.speciesID)
}

func (s *Service) requeue(j job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done[j.speciesID] {
		return
	}
	s.pushLocked(j)
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
	done := int(s.doneN.Load())
	total := int(s.totalN.Load())
	s.cfg.Progress(cache.Progress{
		Stage:     "sprites_render",
		Done:      done,
		Total:     total,
		Message:   "Warming sprites",
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
