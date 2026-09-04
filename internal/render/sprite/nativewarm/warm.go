// Package nativewarm opportunistically pre-encodes native frames for the
// active terminal bucket (24/32/40) while the user browses.
package nativewarm

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

const (
	maxPriorityHints = 4
	auditYieldBatch  = 64
)

// PokemonLoader loads Pokémon detail by species ID.
type PokemonLoader interface {
	Pokemon(ctx context.Context, id int) (domain.Pokemon, error)
}

// NativeRenderer encodes native graphics frames into the artefact cache.
type NativeRenderer interface {
	HasNativeCache(ctx context.Context, pokemon domain.Pokemon, cells int) bool
	HasNativeFile(ctx context.Context, pokemon domain.Pokemon, cells int) bool
	NativeWarmRender(ctx context.Context, pokemon domain.Pokemon, cells int) sprite.RenderResult
	PersistNativeFrame(ctx context.Context, cacheKey, frame string) error
	WaitPersistCapacity(ctx context.Context) error
}

// Progress reports native-cache warm work. Done counts persisted/readable
// .sixel.gz frames for the active bucket. App classifies this into TUI/CLI.
type Progress struct {
	Done    int
	Total   int
	Message string
	Cells   int
}

// Config configures the opportunistic native warm service.
type Config struct {
	Loader   PokemonLoader
	Renderer NativeRenderer
	Index    []domain.SpeciesSummary
	// OnlyIDs when non-empty restricts warm targets (poke update tooling).
	OnlyIDs map[int]struct{}
	// Progress must be safe for concurrent calls.
	Progress func(Progress)
}

// Service owns a single-lane background native encoder for the active bucket.
type Service struct {
	cfg Config

	mu          sync.Mutex
	cells       int
	bucketGen   uint64
	auditedGen  uint64
	anchor      int // index into warmable index slice
	fwdOff      int
	bwdOff      int
	warmable    []domain.SpeciesSummary
	indexByID   map[int]domain.SpeciesSummary
	priority    []int
	countedDone map[int]bool
	retryAt     map[int]time.Time
	failCount   map[int]int
	progress    warmProgress

	wake      chan struct{}
	started   atomic.Bool
	cancel    context.CancelFunc
	workerCtx context.Context
	wg        sync.WaitGroup
	activity  sprite.ActivityGate
}

type warmProgress struct {
	specKey string
	done    int
	total   int
}

// New creates a native warm service. Call EnsureStarted when browse layout is known.
func New(cfg Config) *Service {
	indexByID := make(map[int]domain.SpeciesSummary, len(cfg.Index))
	for _, sum := range cfg.Index {
		if sum.ID > 0 {
			indexByID[sum.ID] = sum
		}
	}
	s := &Service{
		cfg:         cfg,
		indexByID:   indexByID,
		countedDone: make(map[int]bool),
		retryAt:     make(map[int]time.Time),
		failCount:   make(map[int]int),
		wake:        make(chan struct{}, 1),
	}
	s.rebuildWarmableLocked()
	return s
}

// Start launches the worker using cells as the initial bucket.
func (s *Service) Start(ctx context.Context) {
	if s == nil {
		return
	}
	s.EnsureStarted(ctx, sprite.MaxFrameSize)
}

// EnsureStarted starts the single background worker when needed.
func (s *Service) EnsureStarted(ctx context.Context, cells int) {
	if s == nil || s.cfg.Renderer == nil || s.cfg.Loader == nil {
		return
	}
	s.SetCells(cells)
	if s.started.Load() {
		return
	}
	if !s.started.CompareAndSwap(false, true) {
		return
	}
	debuglog.StartupMark("native.discovery_start")
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.workerCtx = runCtx
	s.wg.Add(1)
	go s.worker(runCtx)
}

// Stop cancels the worker and waits for exit.
func (s *Service) Stop() {
	if s == nil || !s.started.Load() {
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

// SetCells switches the active bucket; future work targets the new size only.
func (s *Service) SetCells(cells int) {
	if s == nil {
		return
	}
	cells = sprite.ClampFrameSide(cells)
	s.mu.Lock()
	if s.cells != cells {
		s.cells = cells
		s.bucketGen++
		s.fwdOff = 1
		s.bwdOff = 1
		s.priority = nil
		s.countedDone = make(map[int]bool)
		s.retryAt = make(map[int]time.Time)
		s.failCount = make(map[int]int)
		s.resetProgressLocked(cells)
		s.mu.Unlock()
		s.signal()
		return
	}
	s.mu.Unlock()
	s.signal()
}

// ActiveCells returns the current warm bucket (for tests).
func (s *Service) ActiveCells() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cells
}

// Prioritize bumps speciesID to the front of the warm queue.
func (s *Service) Prioritize(speciesID int) {
	if s == nil || speciesID < 1 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.shouldWarm(speciesID) {
		return
	}
	s.enqueuePriorityLocked(speciesID)
	s.signal()
}

// NoteActivity is retained for interface compatibility; opportunistic warming
// does not throttle on scroll activity.
func (s *Service) NoteActivity() {}

// SetForegroundBusy pauses background encode while foreground cold work runs.
func (s *Service) SetForegroundBusy(busy bool) {
	if s == nil {
		return
	}
	s.activity.SetForegroundBusy(busy)
	s.signal()
}

// SetNeighborsFromList repositions the traversal anchor to the selected species.
// Radius is ignored; traversal continues progressively through the full warmable
// index for the active bucket (N+1..max, N-1..1).
func (s *Service) SetNeighborsFromList(list []domain.SpeciesSummary, cursor, _ int) {
	if s == nil || len(list) == 0 {
		return
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(list) {
		cursor = len(list) - 1
	}
	s.SetAnchorFromSpeciesID(list[cursor].ID)
}

// SetAnchorFromSpeciesID repositions traversal to speciesID in the warmable index.
func (s *Service) SetAnchorFromSpeciesID(speciesID int) {
	if s == nil || speciesID < 1 {
		return
	}
	s.mu.Lock()
	for i, sum := range s.warmable {
		if sum.ID == speciesID {
			if s.anchor != i {
				s.anchor = i
				s.fwdOff = 1
				s.bwdOff = 1
			}
			break
		}
	}
	s.mu.Unlock()
	s.signal()
}

// Snapshot returns persisted/readable coverage for the active render spec.
func (s *Service) Snapshot() (done, total int) {
	if s == nil {
		return 0, 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.progress.done, s.progress.total
}

func (s *Service) shouldWarm(speciesID int) bool {
	if speciesID < 1 {
		return false
	}
	if len(s.cfg.OnlyIDs) == 0 {
		return true
	}
	_, ok := s.cfg.OnlyIDs[speciesID]
	return ok
}

func (s *Service) rebuildWarmableLocked() {
	var warmable []domain.SpeciesSummary
	for _, sum := range s.cfg.Index {
		if s.shouldWarm(sum.ID) {
			warmable = append(warmable, sum)
		}
	}
	s.warmable = warmable
	s.progress.total = len(warmable)
}

func (s *Service) resetProgressLocked(cells int) {
	s.progress = warmProgress{
		specKey: specKey(cells),
		done:    0,
		total:   len(s.warmable),
	}
}

func specKey(cells int) string {
	return sprite.NativeCacheSpecKey(cells)
}

func (s *Service) enqueuePriorityLocked(speciesID int) {
	for i, id := range s.priority {
		if id == speciesID {
			copy(s.priority[1:i+1], s.priority[0:i])
			s.priority[0] = speciesID
			return
		}
	}
	if len(s.priority) >= maxPriorityHints {
		s.priority = s.priority[:len(s.priority)-1]
	}
	s.priority = append([]int{speciesID}, s.priority...)
}

func (s *Service) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Service) markDoneLocked(speciesID int) {
	if s.countedDone[speciesID] {
		return
	}
	first := s.progress.done == 0
	s.countedDone[speciesID] = true
	s.progress.done++
	if first {
		debuglog.StartupMark("native.discovery_first_batch")
	}
}

// markPersistedSpecies records that speciesID has a persisted/readable frame
// for the active bucket. Done increments only after disk verification or a
// successful PersistNativeFrame in this session.
func (s *Service) markPersistedSpecies(speciesID int, bucketGen uint64) {
	s.mu.Lock()
	if s.bucketGen != bucketGen {
		s.mu.Unlock()
		return
	}
	s.markDoneLocked(speciesID)
	delete(s.retryAt, speciesID)
	delete(s.failCount, speciesID)
	s.mu.Unlock()
	s.emitProgress()
}

func (s *Service) emitProgress() {
	if s.cfg.Progress == nil {
		return
	}
	s.mu.Lock()
	p := Progress{
		Done:    s.progress.done,
		Total:   s.progress.total,
		Message: "Building image cache",
		Cells:   s.cells,
	}
	s.mu.Unlock()
	s.cfg.Progress(p)
}

// maybeAuditDisk counts persisted native frames for the active bucket in the
// background worker. Returns true when the current bucketGen has been audited.
// Suppresses progress emission when every warmable species is already on disk.
func (s *Service) maybeAuditDisk(ctx context.Context) bool {
	s.mu.Lock()
	if s.auditedGen == s.bucketGen {
		s.mu.Unlock()
		return true
	}
	gen := s.bucketGen
	cells := s.cells
	warmable := append([]domain.SpeciesSummary(nil), s.warmable...)
	total := len(warmable)
	s.mu.Unlock()

	if s.cfg.Renderer == nil || cells < sprite.MinFrameSize || total == 0 {
		s.mu.Lock()
		if s.bucketGen == gen {
			s.auditedGen = gen
		}
		s.mu.Unlock()
		return true
	}

	countedDone := make(map[int]bool, total)
	done := 0
	for i, sum := range warmable {
		if err := ctx.Err(); err != nil {
			return false
		}
		if i > 0 && i%auditYieldBatch == 0 && s.activity.ForegroundBusy() {
			return false
		}
		poke := domain.Pokemon{ID: sum.ID, Slug: sum.Slug}
		if s.cfg.Renderer.HasNativeFile(ctx, poke, cells) {
			countedDone[sum.ID] = true
			done++
		}
	}

	s.mu.Lock()
	if s.bucketGen != gen {
		s.mu.Unlock()
		return false
	}
	s.countedDone = countedDone
	s.progress.done = done
	s.auditedGen = gen
	complete := done >= s.progress.total
	s.mu.Unlock()

	if !complete {
		s.emitProgress()
	}
	return true
}

func (s *Service) worker(ctx context.Context) {
	defer s.wg.Done()
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		if s.activity.ForegroundBusy() {
			if !s.wait(ctx) {
				return
			}
			continue
		}
		if !s.maybeAuditDisk(ctx) {
			if ctx.Err() != nil {
				return
			}
			if !s.wait(ctx) {
				return
			}
			continue
		}
		id, ok := s.popNext()
		if !ok {
			if anchorID, onlyAnchor := s.uncountedAnchorOnly(); onlyAnchor {
				s.warmOne(ctx, anchorID)
				continue
			}
			if id, retry := s.popUncountedRetry(); retry {
				s.warmOne(ctx, id)
				continue
			}
			if !s.wait(ctx) {
				return
			}
			continue
		}
		s.warmOne(ctx, id)
	}
}

func (s *Service) wait(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-s.wake:
		return true
	case <-time.After(250 * time.Millisecond):
		return true
	}
}

func (s *Service) popNext() (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.warmable) == 0 {
		return 0, false
	}
	for len(s.priority) > 0 {
		id := s.priority[0]
		s.priority = s.priority[1:]
		if s.shouldWarm(id) && !s.countedDone[id] && s.canRetryLocked(id) {
			return id, true
		}
	}
	anchor := s.anchor
	if anchor < 0 {
		anchor = 0
	}
	if anchor >= len(s.warmable) {
		anchor = len(s.warmable) - 1
	}
	for {
		if idx := anchor + s.fwdOff; idx < len(s.warmable) {
			s.fwdOff++
			id := s.warmable[idx].ID
			if !s.countedDone[id] && s.canRetryLocked(id) {
				return id, true
			}
			continue
		}
		if idx := anchor - s.bwdOff; idx >= 0 {
			s.bwdOff++
			id := s.warmable[idx].ID
			if !s.countedDone[id] && s.canRetryLocked(id) {
				return id, true
			}
			continue
		}
		anchorID := s.warmable[anchor].ID
		if !s.countedDone[anchorID] && s.canRetryLocked(anchorID) {
			return anchorID, true
		}
		return 0, false
	}
}

func (s *Service) uncountedAnchorOnly() (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.anchor < 0 || s.anchor >= len(s.warmable) {
		return 0, false
	}
	anchorID := s.warmable[s.anchor].ID
	if s.countedDone[anchorID] {
		return 0, false
	}
	uncounted := 0
	for _, sum := range s.warmable {
		if !s.countedDone[sum.ID] {
			uncounted++
			if uncounted > 1 {
				return 0, false
			}
		}
	}
	return anchorID, true
}

func (s *Service) popUncountedRetry() (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	anchorID := 0
	if s.anchor >= 0 && s.anchor < len(s.warmable) {
		anchorID = s.warmable[s.anchor].ID
	}
	for _, sum := range s.warmable {
		if !s.countedDone[sum.ID] && sum.ID != anchorID && s.canRetryLocked(sum.ID) {
			return sum.ID, true
		}
	}
	return 0, false
}

func (s *Service) canRetryLocked(speciesID int) bool {
	until, ok := s.retryAt[speciesID]
	return !ok || time.Now().After(until)
}

func (s *Service) noteFailure(speciesID int) {
	n := s.failCount[speciesID] + 1
	s.failCount[speciesID] = n
	var wait time.Duration
	switch {
	case n == 1:
		wait = time.Second
	case n == 2:
		wait = 5 * time.Second
	default:
		wait = 30 * time.Second
	}
	s.retryAt[speciesID] = time.Now().Add(wait)
}

func (s *Service) warmOne(ctx context.Context, speciesID int) {
	s.mu.Lock()
	cells := s.cells
	bucketGen := s.bucketGen
	s.mu.Unlock()
	if cells < sprite.MinFrameSize {
		return
	}
	if s.cfg.Renderer != nil {
		if err := s.cfg.Renderer.WaitPersistCapacity(ctx); err != nil {
			return
		}
	}
	poke, err := s.cfg.Loader.Pokemon(ctx, speciesID)
	if err != nil {
		debuglog.Renderf("nativewarm skip species=%d err=%v", speciesID, err)
		s.mu.Lock()
		if s.bucketGen == bucketGen {
			s.noteFailure(speciesID)
		}
		s.mu.Unlock()
		return
	}
	if sum, ok := s.indexByID[speciesID]; ok && sum.Slug != "" {
		poke.Slug = sum.Slug
	}
	if poke.ID == 0 {
		poke.ID = speciesID
	}
	if err := ctx.Err(); err != nil {
		return
	}
	if s.cfg.Renderer.HasNativeFile(ctx, poke, cells) {
		s.markPersistedSpecies(speciesID, bucketGen)
		return
	}
	res := s.cfg.Renderer.NativeWarmRender(ctx, poke, cells)
	if res.UsePlaceholder || !res.Native || res.ANSI == "" || res.CacheKey == "" {
		debuglog.Renderf("nativewarm miss species=%d backend=%s", speciesID, res.Backend)
		s.mu.Lock()
		if s.bucketGen == bucketGen {
			s.noteFailure(speciesID)
		}
		s.mu.Unlock()
		return
	}
	if err := s.cfg.Renderer.PersistNativeFrame(ctx, res.CacheKey, res.ANSI); err != nil {
		debuglog.Renderf("nativewarm persist species=%d err=%v", speciesID, err)
		s.mu.Lock()
		if s.bucketGen == bucketGen {
			s.noteFailure(speciesID)
		}
		s.mu.Unlock()
		return
	}
	if s.cfg.Renderer.HasNativeFile(ctx, poke, cells) {
		s.markPersistedSpecies(speciesID, bucketGen)
		return
	}
	debuglog.Renderf("nativewarm persist species=%d unverified on disk", speciesID)
	s.mu.Lock()
	if s.bucketGen == bucketGen {
		s.noteFailure(speciesID)
	}
	s.mu.Unlock()
}
