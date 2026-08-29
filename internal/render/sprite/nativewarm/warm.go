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

	maxWarmMisses             = 8
	outstandingRescanInterval = 2 * time.Second
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
	// RequiredSizes lists native cell buckets that must exist for Pokémon completeness.
	RequiredSizes []int
	// Completer tracks aggregate Pokémon-level image-cache progress (optional).
	Completer ImageCompleter
	// OnlyIDs when non-empty restricts background encode to these species (poke update).
	OnlyIDs map[int]struct{}
}

// ImageCompleter is the aggregate image-cache session (Pokémon completeness).
type ImageCompleter interface {
	OutstandingIDs() []int
	NoteVariant(speciesID, cells int)
	IsComplete(speciesID int) bool
	Progress() cache.ImageCacheProgress
}

// Service owns the native encode priority queue.
type Service struct {
	cfg Config

	mu          sync.Mutex
	pq          priorityQueue
	seq         int64
	queued      map[int]bool
	done        map[int]bool
	misses      map[int]int
	indexByID   map[int]domain.SpeciesSummary
	cells       int
	layoutCells int
	lastRescan  time.Time
	wake        chan struct{}
	started     atomic.Bool
	cancel      context.CancelFunc
	workerCtx   context.Context
	wg          sync.WaitGroup
	doneN       atomic.Int64
	totalN      atomic.Int64
	finished    atomic.Bool
	activity    sprite.ActivityGate
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
	if len(cfg.RequiredSizes) == 0 {
		cfg.RequiredSizes = sprite.FrameSteps()
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
		misses:    make(map[int]int),
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

// EnsureStarted seeds missing Pokémon and starts workers when image-cache work remains.
// Layout cell changes only affect P0/P1; they do not rebuild the P2 work set.
func (s *Service) EnsureStarted(ctx context.Context, cells int) {
	if s == nil || s.cfg.Renderer == nil || s.cfg.Loader == nil {
		return
	}
	cells = sprite.ClampFrameSide(cells)

	s.mu.Lock()
	s.layoutCells = cells
	s.cells = cells
	alreadyStarted := s.started.Load()
	s.mu.Unlock()

	if alreadyStarted {
		return
	}
	if s.noImageCacheWork(len(s.cfg.Index)) {
		return
	}

	s.prepareQueue(ctx)
	if s.totalN.Load() == 0 {
		s.finishSession()
		return
	}
	if !s.started.CompareAndSwap(false, true) {
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.workerCtx = runCtx
	for i := 0; i < s.cfg.Workers; i++ {
		s.wg.Add(1)
		go s.worker(runCtx)
	}
}

// WarmAllSizes encodes every required bucket, completing one Pokémon at a time. Blocks until done.
func (s *Service) WarmAllSizes(ctx context.Context, sizes []int) error {
	if s == nil {
		return nil
	}
	if len(sizes) > 0 {
		s.cfg.RequiredSizes = append([]int(nil), sizes...)
	}
	if s.cfg.Manifest != nil {
		if err := s.cfg.Manifest.EnsureManifest(s.cfg.Protocol); err != nil {
			return err
		}
	}
	if len(sizes) > 0 {
		s.EnsureStarted(ctx, sprite.ClampFrameSide(sizes[0]))
	} else {
		s.EnsureStarted(ctx, s.cells)
	}
	if err := s.Wait(ctx); err != nil {
		return err
	}
	s.markAllSizesComplete()
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
	if s.cfg.Completer != nil && s.cfg.Completer.IsComplete(speciesID) {
		return
	}
	s.pushLocked(job{speciesID: speciesID, priority: PrioritySelected, allSizes: false})
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
			if s.canQueue(id) {
				s.pushLocked(job{speciesID: id, priority: PriorityNeighbor, allSizes: false})
			}
		}
		if i := cursor + d; i < len(list) {
			id := list[i].ID
			if s.canQueue(id) {
				s.pushLocked(job{speciesID: id, priority: PriorityNeighbor, allSizes: false})
			}
		}
	}
	s.signal()
}

func (s *Service) canQueue(speciesID int) bool {
	if speciesID < 1 || s.done[speciesID] {
		return false
	}
	if s.cfg.Completer != nil && s.cfg.Completer.IsComplete(speciesID) {
		return false
	}
	return true
}

func (s *Service) prepareQueue(ctx context.Context) {
	s.mu.Lock()
	s.pq = priorityQueue{}
	s.queued = make(map[int]bool)
	s.done = make(map[int]bool)
	s.doneN.Store(0)
	s.mu.Unlock()

	ids := s.workIDs()
	s.mu.Lock()
	for _, id := range ids {
		if !s.shouldEnqueue(id) || s.done[id] || s.queued[id] {
			continue
		}
		if s.cfg.Completer != nil && s.cfg.Completer.IsComplete(id) {
			s.done[id] = true
			continue
		}
		s.pushLocked(job{speciesID: id, priority: PriorityBackground, allSizes: true})
	}
	remaining := s.countRemainingLocked()
	s.totalN.Store(int64(remaining))
	s.mu.Unlock()
	if remaining > 0 {
		s.signal()
	}
}

func (s *Service) workIDs() []int {
	if s.cfg.Completer != nil {
		return s.cfg.Completer.OutstandingIDs()
	}
	ids := make([]int, 0, len(s.cfg.Index))
	for _, sum := range s.cfg.Index {
		if sum.ID > 0 {
			ids = append(ids, sum.ID)
		}
	}
	return ids
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
			s.rescanOutstanding()
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
	poke, err := s.pokemonForJob(ctx, j.speciesID)
	if err != nil {
		debuglog.Renderf("nativewarm skip species=%d err=%v", j.speciesID, err)
		s.retryIncomplete(j.speciesID)
		s.report(j.speciesID)
		return
	}
	if err := ctx.Err(); err != nil {
		return
	}

	sizes := s.jobSizes(j)
	for _, cells := range sizes {
		if j.priority >= PriorityBackground && s.activity.ThrottleBackground() {
			s.requeue(job{speciesID: j.speciesID, priority: j.priority, allSizes: true})
			return
		}
		if err := ctx.Err(); err != nil {
			return
		}
		cached := s.cfg.Renderer.HasNativeCache(ctx, poke, cells)
		if cached {
			s.noteVariant(j.speciesID, cells)
			if s.cfg.Completer == nil || s.speciesFullyCached(ctx, j.speciesID) {
				continue
			}
		}
		res := s.cfg.Renderer.NativeWarmRender(ctx, poke, cells)
		if res.UsePlaceholder || !res.Native {
			debuglog.Renderf("nativewarm miss species=%d backend=%s", j.speciesID, res.Backend)
			continue
		}
		s.noteVariant(j.speciesID, cells)
	}

	if s.speciesFullyCached(ctx, j.speciesID) {
		s.markDone(j.speciesID)
	} else if j.allSizes || j.priority >= PriorityBackground {
		s.retryIncomplete(j.speciesID)
	} else {
		s.ensureBackground(j.speciesID)
	}
	s.report(j.speciesID)
}

func (s *Service) jobSizes(j job) []int {
	if !j.allSizes {
		cells := s.layoutCells
		if cells < sprite.MinFrameSize {
			cells = s.cells
		}
		return []int{sprite.ClampFrameSide(cells)}
	}
	return s.sizeOrder()
}

func (s *Service) sizeOrder() []int {
	layout := s.layoutCells
	req := s.requiredSizes()
	inReq := map[int]bool{}
	for _, cells := range req {
		inReq[cells] = true
	}
	out := make([]int, 0, len(req))
	if layout >= sprite.MinFrameSize && inReq[layout] {
		out = append(out, layout)
	}
	for _, cells := range sprite.FrameStepsMaxFirst() {
		if !inReq[cells] || cells == layout {
			continue
		}
		out = append(out, cells)
	}
	return out
}

func (s *Service) requiredSizes() []int {
	if len(s.cfg.RequiredSizes) > 0 {
		return s.cfg.RequiredSizes
	}
	return sprite.FrameSteps()
}

func (s *Service) noteVariant(speciesID, cells int) {
	if s.cfg.Completer != nil {
		s.cfg.Completer.NoteVariant(speciesID, cells)
	}
}

func (s *Service) speciesFullyCached(ctx context.Context, speciesID int) bool {
	if s.cfg.Completer != nil {
		return s.cfg.Completer.IsComplete(speciesID)
	}
	if s.cfg.Renderer == nil {
		return false
	}
	poke := s.indexPokemon(speciesID)
	for _, cells := range s.requiredSizes() {
		if !s.cfg.Renderer.HasNativeCache(ctx, poke, cells) {
			return false
		}
	}
	return true
}

func (s *Service) ensureBackground(speciesID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done[speciesID] {
		return
	}
	s.pushLocked(job{speciesID: speciesID, priority: PriorityBackground, allSizes: true})
	s.signal()
}

func (s *Service) retryIncomplete(speciesID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done[speciesID] {
		return
	}
	s.misses[speciesID]++
	n := s.misses[speciesID]
	if n > maxWarmMisses {
		debuglog.Renderf("nativewarm defer species=%d misses=%d", speciesID, n)
		s.lastRescan = time.Now()
		return
	}
	s.pushLocked(job{speciesID: speciesID, priority: PriorityBackground, allSizes: true})
	s.signal()
}

func (s *Service) rescanOutstanding() {
	if s.cfg.Completer == nil {
		return
	}
	p := s.cfg.Completer.Progress()
	if !p.Active {
		return
	}
	s.mu.Lock()
	if !s.lastRescan.IsZero() && time.Since(s.lastRescan) < outstandingRescanInterval {
		s.mu.Unlock()
		return
	}
	s.lastRescan = time.Now()
	s.mu.Unlock()

	ids := s.cfg.Completer.OutstandingIDs()
	s.mu.Lock()
	for _, id := range ids {
		if s.done[id] {
			delete(s.done, id)
		}
		delete(s.misses, id)
		if s.queued[id] {
			continue
		}
		s.pushLocked(job{speciesID: id, priority: PriorityBackground, allSizes: true})
	}
	s.mu.Unlock()
	s.signal()
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
	delete(s.misses, speciesID)
	s.mu.Unlock()
}

func (s *Service) report(speciesID int) {
	if s.cfg.Completer != nil {
		p := s.cfg.Completer.Progress()
		if p.Total <= 0 {
			return
		}
		if !p.Active {
			s.finishSession()
		}
		if s.cfg.Progress == nil {
			return
		}
		s.cfg.Progress(cache.Progress{
			Stage:     "native",
			Done:      p.Done,
			Total:     p.Total,
			Message:   "Building image cache",
			SpeciesID: speciesID,
		})
		return
	}
	done := int(s.doneN.Load())
	total := int(s.totalN.Load())
	if total <= 0 {
		return
	}
	if done >= total {
		s.finishSession()
	}
	if s.cfg.Progress == nil {
		return
	}
	s.cfg.Progress(cache.Progress{
		Stage:     "native",
		Done:      done,
		Total:     total,
		Message:   "Building image cache",
		SpeciesID: speciesID,
	})
}

func (s *Service) noImageCacheWork(indexTotal int) bool {
	if s.cfg.Completer != nil {
		p := s.cfg.Completer.Progress()
		return !p.Active
	}
	return s.cfg.Manifest != nil && s.allSizesComplete(indexTotal)
}

func (s *Service) finishSession() {
	if !s.finished.CompareAndSwap(false, true) {
		return
	}
	s.markAllSizesComplete()
}

func (s *Service) markAllSizesComplete() {
	if s.cfg.Manifest == nil {
		return
	}
	total := len(s.cfg.Index)
	for _, cells := range s.requiredSizes() {
		_ = s.cfg.Manifest.MarkNativeSizeComplete(cells, total)
	}
}

func (s *Service) allSizesComplete(total int) bool {
	if s.cfg.Manifest == nil {
		return false
	}
	for _, cells := range s.requiredSizes() {
		if !s.cfg.Manifest.NativeSizeComplete(cells, total) {
			return false
		}
	}
	return true
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
		if s.cfg.Completer != nil {
			if !s.cfg.Completer.Progress().Active {
				return nil
			}
		} else {
			done, total := s.Snapshot()
			if total == 0 || done >= total {
				return nil
			}
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
	allSizes  bool
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
