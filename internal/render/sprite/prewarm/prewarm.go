// Package prewarm is the single background warm service for browse: chafa
// artefacts, native (Sixel/Kitty/iTerm) frames, official-artwork PNGs and
// dossiers. One Service runs one Kind; the queue, workers, phased size
// buckets, retry backoff and progress accounting are shared and only the
// per-species Warm hook (see kinds.go) differs.
//
// Scheduling: explicit hints (Prioritize = P0, SetNeighborsFromList = P1) sit
// in a heap and run latest-first. When the heap is empty and Background is set,
// the worker scans the index outward from the browse anchor (N+1..max, N-1..1).
package prewarm

import (
	"container/heap"
	"context"
	"errors"
	"maps"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Graphixa/Poke/internal/debuglog"
	"github.com/Graphixa/Poke/internal/domain"
	"github.com/Graphixa/Poke/internal/render/sprite"
)

// Kind selects what a Service warms.
type Kind string

const (
	ChafaArtefact Kind = "chafa"
	NativeFrame   Kind = "native"
	ArtworkAsset  Kind = "artwork"
	Dossier       Kind = "dossier"
)

const (
	prioritySelected = 0
	priorityNeighbor = 1
	auditYieldBatch  = 64
	idlePoll         = 250 * time.Millisecond
)

// ErrSkip is returned by a Warm hook when the species cannot be warmed and
// should not be retried (counted done, not ready).
var ErrSkip = errors.New("prewarm: skip")

// Progress reports warm work. App classifies it into TUI/CLI progress.
type Progress struct {
	Kind      Kind
	Done      int
	Total     int
	Message   string
	SpeciesID int
	Cells     int // active phase bucket (sized kinds only)
}

// Config configures a Service. Build it with the Chafa/Native/Artwork/Dossier
// constructors in kinds.go.
type Config struct {
	Kind  Kind
	Index []domain.SpeciesSummary
	// Background scans the whole index when no hints are pending.
	Background bool
	Workers    int
	// ThrottleOnActivity pauses the background scan briefly after NoteActivity.
	ThrottleOnActivity bool
	Message            string
	// Progress must be safe for concurrent calls.
	Progress func(Progress)
	// Warm warms one species at one size bucket (cells is 0 for unsized kinds).
	// nil = done, ErrSkip = done but not ready, anything else = retry with backoff.
	Warm func(ctx context.Context, id, cells int) error
	// Has optionally reports a species already warm; enables a disk audit that
	// seeds Done at the start of every phase.
	Has func(ctx context.Context, id, cells int) bool
}

type key struct{ id, cells int }

type hint struct {
	key
	pri int
	seq int64
}

type hintHeap []hint

func (h hintHeap) Len() int { return len(h) }
func (h hintHeap) Less(i, j int) bool {
	if h[i].pri != h[j].pri {
		return h[i].pri < h[j].pri
	}
	return h[i].seq > h[j].seq // latest selection wins
}
func (h hintHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *hintHeap) Push(x any)   { *h = append(*h, x.(hint)) }
func (h *hintHeap) Pop() any {
	old := *h
	it := old[len(old)-1]
	*h = old[:len(old)-1]
	return it
}

type job struct {
	key
	gen uint64
}

// Service is a prioritised background warmer for one Kind.
type Service struct {
	cfg      Config
	warmable []int
	pos      map[int]int // id -> index in warmable

	mu       sync.Mutex
	hints    hintHeap
	hinted   map[key]int64 // latest hint seq per key (older heap entries are stale)
	inflight map[key]bool
	seq      int64

	buckets    []int
	phase      int
	cells      int // active phase bucket
	focus      int // Prioritize bucket
	gen        uint64
	auditedGen uint64
	anchor     int

	done    map[int]bool // counted for the active phase
	ready   map[int]bool // counted and actually stored
	seen    map[int]bool // non-background: species requested this phase
	retryAt map[int]time.Time
	fails   map[int]int

	wake     chan struct{}
	started  atomic.Bool
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	activity sprite.ActivityGate
}

// New creates a Service. Call Start (or EnsureStarted) to launch workers.
func New(cfg Config) *Service {
	if cfg.Workers < 1 {
		cfg.Workers = 1
	}
	s := &Service{
		cfg:      cfg,
		pos:      make(map[int]int, len(cfg.Index)),
		hinted:   make(map[key]int64),
		inflight: make(map[key]bool),
		done:     make(map[int]bool),
		ready:    make(map[int]bool),
		seen:     make(map[int]bool),
		retryAt:  make(map[int]time.Time),
		fails:    make(map[int]int),
		wake:     make(chan struct{}, 1),
	}
	for _, sum := range cfg.Index {
		if sum.ID > 0 {
			s.pos[sum.ID] = len(s.warmable)
			s.warmable = append(s.warmable, sum.ID)
		}
	}
	if !s.sized() {
		s.buckets = []int{0}
	}
	return s
}

func (s *Service) sized() bool {
	return s.cfg.Kind == ChafaArtefact || s.cfg.Kind == NativeFrame
}

// Start launches the workers. Sized kinds default to the largest bucket when
// no size is known yet. Safe to call repeatedly.
func (s *Service) Start(ctx context.Context) {
	if s == nil || s.cfg.Warm == nil {
		return
	}
	if s.sized() {
		s.mu.Lock()
		unset := s.cells == 0
		s.mu.Unlock()
		if unset {
			s.SetCells(sprite.MaxFrameSize)
		}
	}
	if !s.started.CompareAndSwap(false, true) {
		return
	}
	debuglog.StartupMark(string(s.cfg.Kind) + ".discovery_start")
	runCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()
	for i := 0; i < s.cfg.Workers; i++ {
		s.wg.Add(1)
		go s.worker(runCtx)
	}
}

// EnsureStarted sets the focus size then starts workers.
func (s *Service) EnsureStarted(ctx context.Context, cells int) {
	if s == nil {
		return
	}
	s.SetCells(cells)
	s.Start(ctx)
}

// Stop cancels workers and waits for them to exit.
func (s *Service) Stop() {
	if s == nil || !s.started.Load() {
		return
	}
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
}

// SetBuckets configures the phase order (e.g. 16 then 40). Resets to phase 0
// when the list changes. Each bucket finishes before the next starts;
// Prioritize still prefers the focus bucket.
func (s *Service) SetBuckets(buckets []int) {
	if s == nil || !s.sized() {
		return
	}
	norm := sprite.NormalizeBuckets(buckets)
	if len(norm) == 0 {
		return
	}
	s.mu.Lock()
	if slices.Equal(s.buckets, norm) {
		s.mu.Unlock()
		return
	}
	s.buckets = norm
	s.phase = 0
	s.focus = norm[0]
	s.applyPhaseLocked(norm[0])
	s.mu.Unlock()
	s.signal()
}

// SetCells sets the focus bucket used by Prioritize. With a single (or unset)
// bucket list it also switches the background target; with several phases it
// does not abandon the current one.
func (s *Service) SetCells(cells int) {
	if s == nil || !s.sized() {
		return
	}
	cells = sprite.ClampFrameSide(cells)
	s.mu.Lock()
	s.focus = cells
	if len(s.buckets) <= 1 {
		s.buckets = []int{cells}
		s.phase = 0
		if s.cells != cells {
			s.applyPhaseLocked(cells)
		}
	}
	s.mu.Unlock()
	s.signal()
}

func (s *Service) applyPhaseLocked(cells int) {
	s.cells = cells
	s.gen++
	s.hints = nil
	s.hinted = make(map[key]int64)
	s.done = make(map[int]bool)
	s.ready = make(map[int]bool)
	s.seen = make(map[int]bool)
	s.retryAt = make(map[int]time.Time)
	s.fails = make(map[int]int)
}

// Prioritize bumps speciesID to P0 for the focus bucket, then other buckets.
func (s *Service) Prioritize(speciesID int) {
	if s == nil || speciesID < 1 {
		return
	}
	s.mu.Lock()
	s.hintLocked(speciesID, prioritySelected, true)
	s.mu.Unlock()
	s.signal()
}

// SetNeighborsFromList moves the scan anchor to list[cursor] and hints ±radius
// neighbours at P1.
func (s *Service) SetNeighborsFromList(list []domain.SpeciesSummary, cursor, radius int) {
	if s == nil || len(list) == 0 {
		return
	}
	cursor = min(max(cursor, 0), len(list)-1)
	s.mu.Lock()
	if i, ok := s.pos[list[cursor].ID]; ok {
		s.anchor = i
	}
	for d := max(radius, 0); d >= 1; d-- {
		if i := cursor - d; i >= 0 {
			s.hintLocked(list[i].ID, priorityNeighbor, false)
		}
		if i := cursor + d; i < len(list) {
			s.hintLocked(list[i].ID, priorityNeighbor, false)
		}
	}
	s.mu.Unlock()
	s.signal()
}

// NoteActivity records selection motion (throttles background when configured).
func (s *Service) NoteActivity() {
	if s == nil {
		return
	}
	s.activity.NoteActivity()
	s.signal()
}

// SetForegroundBusy pauses all warm work while a foreground cold render runs.
func (s *Service) SetForegroundBusy(busy bool) {
	if s == nil {
		return
	}
	s.activity.SetForegroundBusy(busy)
	s.signal()
}

// Ready reports whether speciesID was successfully warmed for the active phase.
func (s *Service) Ready(speciesID int) bool {
	if s == nil || speciesID < 1 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ready[speciesID]
}

// Snapshot returns done/total for the active phase.
func (s *Service) Snapshot() (done, total int) {
	if s == nil {
		return 0, 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.done), s.totalLocked()
}

// Wait blocks until the active phase is complete or ctx is cancelled.
func (s *Service) Wait(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if done, total := s.Snapshot(); total == 0 || done >= total {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (s *Service) totalLocked() int {
	if s.cfg.Background {
		return len(s.warmable)
	}
	return len(s.seen)
}

func (s *Service) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// hintLocked queues id for the focus bucket (last, so it runs first) and,
// when allBuckets, every other bucket.
func (s *Service) hintLocked(id, pri int, allBuckets bool) {
	focus := s.focus
	if s.sized() && focus < sprite.SmallestFrameBucket() {
		focus = s.cells
	}
	if allBuckets {
		for _, b := range s.buckets {
			if b != focus {
				s.pushLocked(id, pri, b)
			}
		}
	}
	s.pushLocked(id, pri, focus)
}

func (s *Service) pushLocked(id, pri, cells int) {
	if id < 1 || (s.sized() && cells < sprite.SmallestFrameBucket()) {
		return
	}
	k := key{id, cells}
	s.seq++
	s.hinted[k] = s.seq
	heap.Push(&s.hints, hint{key: k, pri: pri, seq: s.seq})
	if !s.cfg.Background && cells == s.cells {
		s.seen[id] = true
	}
}

func (s *Service) canRetryLocked(id int) bool {
	until, ok := s.retryAt[id]
	return !ok || time.Now().After(until)
}

func (s *Service) claimLocked(k key) job {
	s.inflight[k] = true
	return job{key: k, gen: s.gen}
}

// nextLocked pops the next hint, else (Background) the nearest unfinished
// species outward from the anchor.
func (s *Service) nextLocked() (job, bool) {
	for s.hints.Len() > 0 {
		h := heap.Pop(&s.hints).(hint)
		if s.hinted[h.key] != h.seq {
			continue // superseded by a newer hint
		}
		delete(s.hinted, h.key)
		if s.inflight[h.key] {
			continue
		}
		// Phase-bucket hints skip stored species; off-phase hints always run
		// (the hook itself is cache-aware).
		if h.cells == s.cells && (s.ready[h.id] || !s.canRetryLocked(h.id)) {
			continue
		}
		return s.claimLocked(h.key), true
	}
	if !s.cfg.Background || len(s.warmable) == 0 ||
		(s.cfg.ThrottleOnActivity && s.activity.ThrottleBackground()) {
		return job{}, false
	}
	try := func(i int) (job, bool) {
		id := s.warmable[i]
		k := key{id, s.cells}
		if s.done[id] || !s.canRetryLocked(id) || s.inflight[k] {
			return job{}, false
		}
		return s.claimLocked(k), true
	}
	a := min(s.anchor, len(s.warmable)-1)
	for i := a + 1; i < len(s.warmable); i++ {
		if j, ok := try(i); ok {
			return j, true
		}
	}
	for i := a - 1; i >= 0; i-- {
		if j, ok := try(i); ok {
			return j, true
		}
	}
	return try(a)
}

func (s *Service) worker(ctx context.Context) {
	defer s.wg.Done()
	for ctx.Err() == nil {
		if s.activity.ForegroundBusy() || !s.audit(ctx) {
			s.wait(ctx)
			continue
		}
		s.mu.Lock()
		j, ok := s.nextLocked()
		s.mu.Unlock()
		if !ok {
			s.wait(ctx)
			continue
		}
		s.run(ctx, j)
	}
}

func (s *Service) wait(ctx context.Context) {
	select {
	case <-ctx.Done():
	case <-s.wake:
	case <-time.After(idlePoll):
	}
}

func (s *Service) run(ctx context.Context, j job) {
	err := s.cfg.Warm(ctx, j.id, j.cells)
	s.mu.Lock()
	delete(s.inflight, j.key)
	if ctx.Err() != nil || j.gen != s.gen {
		s.mu.Unlock()
		return
	}
	var p Progress
	switch {
	case err == nil || errors.Is(err, ErrSkip):
		p = s.countLocked(j, err == nil)
	case j.cells == s.cells:
		n := s.fails[j.id] + 1
		s.fails[j.id] = n
		s.retryAt[j.id] = time.Now().Add(backoff(n))
		s.mu.Unlock()
		return
	default:
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	s.report(p)
}

func backoff(n int) time.Duration {
	switch n {
	case 1:
		return time.Second
	case 2:
		return 5 * time.Second
	}
	return 30 * time.Second
}

// countLocked records a finished species and returns the progress to emit.
// On phase completion it emits Done>=Total for the finished bucket and then
// advances — never an interim 0/N for the next phase.
func (s *Service) countLocked(j job, stored bool) Progress {
	if j.cells == s.cells && !s.done[j.id] {
		if len(s.done) == 0 {
			debuglog.StartupMark(string(s.cfg.Kind) + ".discovery_first_batch")
		}
		s.done[j.id] = true
		delete(s.retryAt, j.id)
		delete(s.fails, j.id)
	}
	if j.cells == s.cells && stored {
		s.ready[j.id] = true
	}
	p := s.progressLocked(j.id)
	if s.cfg.Background && p.Total > 0 && p.Done >= p.Total {
		s.advanceLocked()
	}
	return p
}

func (s *Service) progressLocked(speciesID int) Progress {
	return Progress{
		Kind:      s.cfg.Kind,
		Done:      len(s.done),
		Total:     s.totalLocked(),
		Message:   s.cfg.Message,
		SpeciesID: speciesID,
		Cells:     s.cells,
	}
}

func (s *Service) advanceLocked() bool {
	if s.phase+1 >= len(s.buckets) {
		return false
	}
	s.phase++
	s.applyPhaseLocked(s.buckets[s.phase])
	return true
}

func (s *Service) report(p Progress) {
	if s.cfg.Progress != nil {
		s.cfg.Progress(p)
	}
}

// audit seeds Done from Has for the active phase. It returns false when the
// worker should re-loop (foreground busy, cancelled, or phase advanced).
// A fully warm last phase stays silent.
func (s *Service) audit(ctx context.Context) bool {
	if s.cfg.Has == nil {
		return true
	}
	s.mu.Lock()
	if s.auditedGen == s.gen {
		s.mu.Unlock()
		return true
	}
	gen, cells := s.gen, s.cells
	ids := slices.Clone(s.warmable)
	s.mu.Unlock()

	if cells < sprite.SmallestFrameBucket() || len(ids) == 0 {
		s.mu.Lock()
		if s.gen == gen {
			s.auditedGen = gen
		}
		s.mu.Unlock()
		return true
	}
	counted := make(map[int]bool, len(ids))
	for i, id := range ids {
		if ctx.Err() != nil || (i > 0 && i%auditYieldBatch == 0 && s.activity.ForegroundBusy()) {
			return false
		}
		if s.cfg.Has(ctx, id, cells) {
			counted[id] = true
		}
	}

	s.mu.Lock()
	if s.gen != gen {
		s.mu.Unlock()
		return false
	}
	s.auditedGen = gen
	s.done, s.ready = counted, maps.Clone(counted)
	p := s.progressLocked(0)
	complete := p.Total > 0 && p.Done >= p.Total
	advanced := complete && s.advanceLocked()
	s.mu.Unlock()
	switch {
	case advanced:
		s.report(p) // close out the finished phase for the footer
		s.signal()
		return false
	case complete:
		return true
	}
	s.report(p)
	return true
}
