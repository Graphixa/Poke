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

// ManifestStore tracks per-size native cache completeness on disk.
type ManifestStore interface {
	EnsureManifest(protocol string) error
	NativeSizeComplete(cells, total int) bool
	InitNativeSizePhase(cells, total int) error
	MarkNativeSizeComplete(cells, total int) error
	RecountNativeSize(cells int) (int, error)
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
	Manifest ManifestStore
	Protocol string
	// OnlyIDs when non-empty restricts background encode to these species (poke update).
	OnlyIDs map[int]struct{}
}

// Service owns the native encode priority queue.
type Service struct {
	cfg Config

	mu        sync.Mutex
	pq        priorityQueue
	seq       int64
	queued    map[int]bool
	done      map[int]bool
	indexByID map[int]domain.SpeciesSummary
	cells     int
	wake      chan struct{}
	started   atomic.Bool
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	doneN     atomic.Int64
	totalN    atomic.Int64
	activity  sprite.ActivityGate
}

// New creates a native warm service. Call EnsureStarted when browse layout is known.
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
	indexByID := make(map[int]domain.SpeciesSummary, len(cfg.Index))
	for _, sum := range cfg.Index {
		if sum.ID > 0 {
			indexByID[sum.ID] = sum
		}
	}
	return &Service{
		cfg:       cfg,
		queued:    make(map[int]bool),
		done:      make(map[int]bool),
		indexByID: indexByID,
		cells:     cfg.Cells,
		wake:      make(chan struct{}, 1),
	}
}

// Start launches workers for cfg.Cells. Prefer EnsureStarted with the browse frame size.
func (s *Service) Start(ctx context.Context) {
	if s == nil {
		return
	}
	s.EnsureStarted(ctx, s.cells)
}

// EnsureStarted seeds on-disk hits, enqueues only missing species, and starts workers
// when there is work. No-op when the native cache is already complete for cells.
func (s *Service) EnsureStarted(ctx context.Context, cells int) {
	if s == nil || s.cfg.Renderer == nil || s.cfg.Loader == nil {
		return
	}
	cells = sprite.ClampFrameSide(cells)
	total := len(s.cfg.Index)
	if s.cfg.Manifest != nil {
		if err := s.cfg.Manifest.EnsureManifest(s.cfg.Protocol); err != nil {
			debuglog.Renderf("nativewarm manifest err=%v", err)
		}
		if s.cfg.Manifest.NativeSizeComplete(cells, total) {
			return
		}
	}

	s.mu.Lock()
	cellsChanged := s.cells != cells
	if cellsChanged {
		s.cells = cells
	}
	alreadyStarted := s.started.Load()
	s.mu.Unlock()

	if !alreadyStarted {
		if s.cfg.Manifest != nil {
			if err := s.cfg.Manifest.InitNativeSizePhase(cells, total); err != nil {
				debuglog.Renderf("nativewarm init phase err=%v", err)
			}
		}
		s.prepareQueueAtCells(ctx)
		if s.totalN.Load() == 0 {
			if s.cfg.Manifest != nil {
				_ = s.cfg.Manifest.MarkNativeSizeComplete(cells, total)
			}
			return
		}
		if !s.started.CompareAndSwap(false, true) {
			return
		}
		runCtx, cancel := context.WithCancel(ctx)
		s.cancel = cancel
		for i := 0; i < s.cfg.Workers; i++ {
			s.wg.Add(1)
			go s.worker(runCtx)
		}
		return
	}
	if cellsChanged {
		if s.cfg.Manifest != nil {
			if err := s.cfg.Manifest.InitNativeSizePhase(cells, total); err != nil {
				debuglog.Renderf("nativewarm init phase err=%v", err)
			}
		}
		s.prepareQueueAtCells(ctx)
		if s.totalN.Load() == 0 {
			if s.cfg.Manifest != nil {
				_ = s.cfg.Manifest.MarkNativeSizeComplete(cells, total)
			}
		}
	}
}

// WarmAllSizes encodes every bucket in order (typically max-first). Blocks until done.
func (s *Service) WarmAllSizes(ctx context.Context, sizes []int) error {
	if s == nil {
		return nil
	}
	total := len(s.cfg.Index)
	if s.cfg.Manifest != nil {
		if err := s.cfg.Manifest.EnsureManifest(s.cfg.Protocol); err != nil {
			return err
		}
	}
	for _, cells := range sizes {
		cells = sprite.ClampFrameSide(cells)
		if s.cfg.Manifest != nil && s.cfg.Manifest.NativeSizeComplete(cells, total) {
			continue
		}
		if err := s.runSizePhase(ctx, cells, total); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) runSizePhase(ctx context.Context, cells, total int) error {
	s.mu.Lock()
	s.cells = cells
	s.mu.Unlock()

	if s.cfg.Manifest != nil {
		if err := s.cfg.Manifest.InitNativeSizePhase(cells, total); err != nil {
			return err
		}
	}

	s.prepareQueueAtCells(ctx)
	if s.totalN.Load() == 0 {
		if s.cfg.Manifest != nil {
			return s.cfg.Manifest.MarkNativeSizeComplete(cells, total)
		}
		return nil
	}

	if !s.started.Load() {
		if !s.started.CompareAndSwap(false, true) {
			s.signal()
		} else {
			runCtx, cancel := context.WithCancel(ctx)
			s.cancel = cancel
			for i := 0; i < s.cfg.Workers; i++ {
				s.wg.Add(1)
				go s.worker(runCtx)
			}
		}
	} else {
		s.signal()
	}

	if err := s.Wait(ctx); err != nil {
		return err
	}
	if s.cfg.Manifest != nil {
		if _, err := s.cfg.Manifest.RecountNativeSize(cells); err != nil {
			return err
		}
		return s.cfg.Manifest.MarkNativeSizeComplete(cells, total)
	}
	return nil
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

// SetCells updates the frame bucket and re-queues only species missing disk cache.
func (s *Service) SetCells(cells int) {
	if s == nil {
		return
	}
	s.EnsureStarted(context.Background(), cells)
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

func (s *Service) prepareQueueAtCells(ctx context.Context) {
	s.mu.Lock()
	s.pq = priorityQueue{}
	s.queued = make(map[int]bool)
	s.done = make(map[int]bool)
	s.doneN.Store(0)
	s.mu.Unlock()

	s.seedDoneFromDisk(ctx)

	s.mu.Lock()
	for _, sum := range s.cfg.Index {
		if !s.shouldEnqueue(sum.ID) || s.done[sum.ID] || s.queued[sum.ID] {
			continue
		}
		s.pushLocked(job{speciesID: sum.ID, priority: PriorityBackground})
	}
	remaining := s.countRemainingLocked()
	s.totalN.Store(int64(remaining))
	s.mu.Unlock()
	if remaining > 0 {
		s.signal()
	}
}

func (s *Service) seedDoneFromDisk(ctx context.Context) {
	if s.cfg.Renderer == nil {
		return
	}
	cells := s.cells
	for _, sum := range s.cfg.Index {
		if !s.shouldEnqueue(sum.ID) {
			continue
		}
		poke := s.indexPokemon(sum.ID)
		if s.cfg.Renderer.HasNativeCache(ctx, poke, cells) {
			s.mu.Lock()
			s.done[sum.ID] = true
			s.mu.Unlock()
		}
	}
}

func (s *Service) indexPokemon(speciesID int) domain.Pokemon {
	if sum, ok := s.indexByID[speciesID]; ok {
		return domain.Pokemon{ID: sum.ID, Slug: sum.Slug, DisplayName: sum.DisplayName}
	}
	return domain.Pokemon{ID: speciesID}
}

func (s *Service) pokemonForJob(ctx context.Context, speciesID int) (domain.Pokemon, error) {
	poke, err := s.cfg.Loader.Pokemon(ctx, speciesID)
	if err != nil {
		return domain.Pokemon{}, err
	}
	if sum, ok := s.indexByID[speciesID]; ok && sum.Slug != "" {
		poke.Slug = sum.Slug
	}
	if poke.ID == 0 {
		poke.ID = speciesID
	}
	return poke, nil
}

func (s *Service) shouldEnqueue(speciesID int) bool {
	if len(s.cfg.OnlyIDs) == 0 {
		return true
	}
	_, ok := s.cfg.OnlyIDs[speciesID]
	return ok
}

func (s *Service) countRemainingLocked() int {
	n := 0
	for _, sum := range s.cfg.Index {
		if s.shouldEnqueue(sum.ID) && !s.done[sum.ID] {
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
			wait := sprite.InteractiveIdle
			select {
			case <-ctx.Done():
				return
			case <-s.wake:
			case <-time.After(wait):
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
	poke, err := s.pokemonForJob(ctx, j.speciesID)
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
	done := int(s.doneN.Load())
	total := int(s.totalN.Load())
	if total <= 0 {
		return
	}
	if done >= total {
		if s.cfg.Manifest != nil {
			_ = s.cfg.Manifest.MarkNativeSizeComplete(s.cells, len(s.cfg.Index))
		}
		return
	}
	if s.cfg.Progress == nil {
		return
	}
	s.cfg.Progress(cache.Progress{
		Stage:     "native",
		Done:      done,
		Total:     total,
		Cells:     s.cells,
		Message:   "Building image cache",
		SpeciesID: speciesID,
	})
}

// Snapshot returns current done/total counts for in-flight warm work.
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
		if total == 0 || done >= total {
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
