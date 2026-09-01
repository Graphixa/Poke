// Package coldsched implements latest-selection-wins foreground cold-render
// scheduling for the TUI selection path.
package coldsched

import (
	"context"
	"sync"
	"time"

	"github.com/Graphixa/Pokedex/internal/domain"
)

// Request identifies one cold-render job.
type Request struct {
	SpeciesID int
	Gen       int
	Cells     int
	Pokemon   domain.Pokemon
}

// Same reports whether two requests target the same render identity.
func (a Request) Same(b Request) bool {
	return a.SpeciesID == b.SpeciesID && a.Gen == b.Gen && a.Cells == b.Cells
}

// Selection is the current user selection.
type Selection struct {
	Gen       int
	SpeciesID int
	Cells     int
}

// Matches reports whether req is still the selected render target.
func (s Selection) Matches(req Request) bool {
	return s.Gen == req.Gen && s.SpeciesID == req.SpeciesID && s.Cells == req.Cells
}

// RenderTimings breaks down cold-render stage durations.
type RenderTimings struct {
	SourceRead  time.Duration
	PNGDecode   time.Duration
	Resize      time.Duration
	SixelEncode time.Duration
	Persist     time.Duration
	Total       time.Duration
}

// ColdResult is the output of a cache lookup or cold render.
type ColdResult struct {
	Frame       string
	CacheKey    string
	CacheState  string
	Native      bool
	AssetKind   string
	Placeholder bool
	Timings     RenderTimings
	Err         error
}

// Renderer performs cache lookups and cold encodes for the scheduler.
type Renderer interface {
	LookupCached(ctx context.Context, pokemon domain.Pokemon, cells int) (ColdResult, bool)
	RenderCold(ctx context.Context, pokemon domain.Pokemon, cells int) ColdResult
	Persist(ctx context.Context, cacheKey, frame string) error
}

// EventKind classifies a selection/render lifecycle event.
type EventKind int

const (
	EventSelection EventKind = iota
	EventCacheHit
	EventSkippedBeforeRender
	EventRenderStarted
	EventRenderCompleted
	EventDisplayed
	EventStaleCompleted
	EventPendingReplaced
)

// Event records one measured selection/render lifecycle point.
type Event struct {
	Seq             int
	Kind            EventKind
	SpeciesID       int
	SelectedAt      time.Time
	BecameActiveAt  time.Time
	CompletedAt     time.Time
	VisibleAt       time.Time
	CacheState      string
	Stale           bool
	PendingReplaced bool
	Timings         RenderTimings
}

// MetricsSnapshot aggregates scheduler invariants and latency samples.
type MetricsSnapshot struct {
	SelectionEvents       int
	CacheHits             int
	ColdMisses            int
	RendersStarted        int
	RendersCompleted      int
	RendersDisplayed      int
	StaleCompleted        int
	PendingReplaced       int
	SkippedBeforeRender   int
	MaxActive             int
	MaxPending            int
	MaxQueueDepth         int
	DisplayedLatencyMS    []float64
	StaleEncodeCPU        time.Duration
	TotalEncodeCPU        time.Duration
	FinalSettleMS         float64
	SchedulingInvariantOK bool
	Events                []Event
}

type activeJob struct {
	req          Request
	reqID        int
	ctx          context.Context
	cancel       context.CancelFunc
	selectedAt   time.Time
	becameActive time.Time
}

// DisplayFunc receives a ready frame for the selected render target.
type DisplayFunc func(req Request, res ColdResult)

// Scheduler enforces at most one active and one pending cold render.
type Scheduler struct {
	renderer Renderer

	mu      sync.Mutex
	sel     Selection
	reqID   int
	active  *activeJob
	pending *Request

	events []Event
	seq    int

	maxActive, maxPending, maxQueue  int
	cacheHits, coldMisses            int
	rendersStarted, rendersCompleted int
	rendersDisplayed, staleCompleted int
	pendingReplaced, skipped         int
	staleEncodeCPU, totalEncodeCPU   time.Duration
	displayedLatencyMS               []float64

	lastSelectedAt time.Time
	finalSettleMS  float64

	onDisplay DisplayFunc

	ownerCtx context.Context
	stop     context.CancelFunc

	renderWG sync.WaitGroup
	stopped  bool
}

// NewScheduler creates a scheduler with optional display callback.
func NewScheduler(ownerCtx context.Context, renderer Renderer, onDisplay DisplayFunc) *Scheduler {
	if ownerCtx == nil {
		ownerCtx = context.Background()
	}
	ctx, cancel := context.WithCancel(ownerCtx)
	return &Scheduler{
		renderer:  renderer,
		onDisplay: onDisplay,
		ownerCtx:  ctx,
		stop:      cancel,
	}
}

func (s *Scheduler) runCtx() context.Context {
	if s == nil || s.ownerCtx == nil {
		return context.Background()
	}
	return s.ownerCtx
}

// Stop cancels in-flight work and waits until all render goroutines exit.
func (s *Scheduler) Stop(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return s.waitRenderWG(ctx)
	}
	s.stopped = true
	if s.active != nil {
		s.active.cancel()
	}
	s.pending = nil
	if s.stop != nil {
		s.stop()
	}
	s.mu.Unlock()
	return s.waitRenderWG(ctx)
}

func (s *Scheduler) waitRenderWG(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		s.renderWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// AbortForeground cancels active and pending foreground work without stopping
// the scheduler owner context.
func (s *Scheduler) AbortForeground() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active != nil {
		s.active.cancel()
		s.active = nil
	}
	s.pending = nil
}

// Select handles a selection change. When force is true, an in-flight encode for
// the same target is aborted so a fresh cold render can start (artwork upgrade).
// When skipLookup is true the caller already performed an authoritative cache miss.
func (s *Scheduler) Select(ctx context.Context, gen int, pokemon domain.Pokemon, cells int, force bool, skipLookup bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	stopped := s.stopped
	s.mu.Unlock()
	if stopped {
		return
	}
	now := time.Now()
	req := Request{SpeciesID: pokemon.ID, Gen: gen, Cells: cells, Pokemon: pokemon}

	s.mu.Lock()
	s.sel = Selection{Gen: gen, SpeciesID: pokemon.ID, Cells: cells}
	s.lastSelectedAt = now
	s.seq++
	s.recordEventLocked(Event{Seq: s.seq, Kind: EventSelection, SpeciesID: pokemon.ID, SelectedAt: now})

	if !skipLookup {
		if res, hit := s.renderer.LookupCached(ctx, pokemon, cells); hit && res.Frame != "" && res.Err == nil {
			frame := s.recordCacheHitLocked(now, pokemon.ID, res)
			s.mu.Unlock()
			s.display(req, resWithFrame(res, frame))
			return
		}
	}
	s.coldMisses++

	if s.active != nil {
		if s.active.req.Same(req) && !force {
			s.mu.Unlock()
			return
		}
		if force && s.active.req.Same(req) {
			s.abortActiveLocked()
		}
		cp := req
		if s.pending != nil && !s.pending.Same(req) {
			s.pendingReplaced++
			s.recordEventLocked(Event{Seq: s.seq, Kind: EventPendingReplaced, SpeciesID: pokemon.ID, PendingReplaced: true})
		}
		s.pending = &cp
		s.trackPendingLocked()
		s.mu.Unlock()
		return
	}

	outcome := s.takeStartLocked(ctx, now, req)
	s.mu.Unlock()
	if outcome.cacheFrame != "" {
		s.display(outcome.cacheReq, outcome.cacheRes)
	}
	if outcome.job != nil {
		s.renderWG.Add(1)
		go s.runRender(*outcome.job)
	}
}

func (s *Scheduler) recordCacheHitLocked(selectedAt time.Time, speciesID int, res ColdResult) string {
	s.cacheHits++
	s.rendersDisplayed++
	visible := time.Now()
	latMS := float64(visible.Sub(selectedAt)) / float64(time.Millisecond)
	s.displayedLatencyMS = append(s.displayedLatencyMS, latMS)
	s.finalSettleMS = latMS
	s.recordEventLocked(Event{
		Seq:        s.seq,
		Kind:       EventCacheHit,
		SpeciesID:  speciesID,
		SelectedAt: selectedAt,
		VisibleAt:  visible,
		CacheState: res.CacheState,
	})
	return res.Frame
}

type startOutcome struct {
	job        *activeJob
	cacheFrame string
	cacheReq   Request
	cacheRes   ColdResult
}

func resWithFrame(res ColdResult, frame string) ColdResult {
	out := res
	out.Frame = frame
	return out
}

func (s *Scheduler) takeStartLocked(ctx context.Context, selectedAt time.Time, req ...Request) startOutcome {
	if s.active != nil {
		return startOutcome{}
	}
	var startReq Request
	if len(req) > 0 {
		startReq = req[0]
	} else if s.pending != nil {
		startReq = *s.pending
		s.pending = nil
		if !s.sel.Matches(startReq) {
			s.skipped++
			s.recordEventLocked(Event{Seq: s.seq, Kind: EventSkippedBeforeRender, SpeciesID: startReq.SpeciesID})
			return s.takeStartLocked(ctx, selectedAt)
		}
		if res, hit := s.renderer.LookupCached(ctx, startReq.Pokemon, startReq.Cells); hit && res.Frame != "" && res.Err == nil {
			frame := s.recordCacheHitLocked(selectedAt, startReq.SpeciesID, res)
			return startOutcome{cacheFrame: frame, cacheReq: startReq, cacheRes: resWithFrame(res, frame)}
		}
	} else {
		return startOutcome{}
	}

	s.reqID++
	reqID := s.reqID
	jobCtx, cancel := context.WithCancel(ctx)
	cp := startReq
	job := &activeJob{
		req:          cp,
		reqID:        reqID,
		cancel:       cancel,
		selectedAt:   selectedAt,
		becameActive: time.Now(),
	}
	s.active = job
	s.rendersStarted++
	s.trackActiveLocked()
	s.recordEventLocked(Event{
		Seq:            s.seq,
		Kind:           EventRenderStarted,
		SpeciesID:      cp.SpeciesID,
		SelectedAt:     selectedAt,
		BecameActiveAt: job.becameActive,
	})
	out := *job
	out.ctx = jobCtx
	out.cancel = cancel
	return startOutcome{job: &out}
}

func (s *Scheduler) runRender(job activeJob) {
	defer s.renderWG.Done()

	res := s.renderer.RenderCold(job.ctx, job.req.Pokemon, job.req.Cells)
	completed := time.Now()

	var (
		displayReq   Request
		displayRes   ColdResult
		doDisplay    bool
		persistKey   string
		persistFrame string
		persistCtx   context.Context
	)

	s.mu.Lock()
	if s.active != nil && s.active.reqID == job.reqID {
		s.active = nil
	}
	s.rendersCompleted++
	s.totalEncodeCPU += res.Timings.SixelEncode
	s.recordEventLocked(Event{
		Seq:            s.seq,
		Kind:           EventRenderCompleted,
		SpeciesID:      job.req.SpeciesID,
		SelectedAt:     job.selectedAt,
		BecameActiveAt: job.becameActive,
		CompletedAt:    completed,
		Timings:        res.Timings,
	})

	current := s.sel.Matches(job.req)
	if res.Err == nil && res.Frame != "" {
		if current {
			visible := time.Now()
			latMS := float64(visible.Sub(job.selectedAt)) / float64(time.Millisecond)
			s.displayedLatencyMS = append(s.displayedLatencyMS, latMS)
			s.rendersDisplayed++
			s.finalSettleMS = latMS
			s.recordEventLocked(Event{
				Seq:         s.seq,
				Kind:        EventDisplayed,
				SpeciesID:   job.req.SpeciesID,
				SelectedAt:  job.selectedAt,
				CompletedAt: completed,
				VisibleAt:   visible,
				Timings:     res.Timings,
			})
			displayReq = job.req
			displayRes = res
			doDisplay = true
		} else {
			s.staleCompleted++
			s.staleEncodeCPU += res.Timings.SixelEncode
			s.recordEventLocked(Event{
				Seq:         s.seq,
				Kind:        EventStaleCompleted,
				SpeciesID:   job.req.SpeciesID,
				SelectedAt:  job.selectedAt,
				CompletedAt: completed,
				Stale:       true,
				Timings:     res.Timings,
			})
		}
		if res.CacheKey != "" {
			persistKey = res.CacheKey
			persistFrame = res.Frame
			persistCtx = job.ctx
		}
	}

	outcome := s.takeStartLocked(s.runCtx(), s.lastSelectedAt)
	s.mu.Unlock()

	if doDisplay {
		s.display(displayReq, displayRes)
	}
	if outcome.cacheFrame != "" {
		s.display(outcome.cacheReq, outcome.cacheRes)
	}
	if outcome.job != nil {
		s.renderWG.Add(1)
		go s.runRender(*outcome.job)
	}
	if persistKey != "" && persistFrame != "" {
		_ = s.renderer.Persist(persistCtx, persistKey, persistFrame)
	}
}

func (s *Scheduler) abortActiveLocked() {
	if s.active == nil {
		return
	}
	s.active.cancel()
}

func (s *Scheduler) trackActiveLocked() {
	n := 0
	if s.active != nil {
		n = 1
	}
	if n > s.maxActive {
		s.maxActive = n
	}
	s.trackQueueLocked()
}

func (s *Scheduler) trackPendingLocked() {
	n := 0
	if s.pending != nil {
		n = 1
	}
	if n > s.maxPending {
		s.maxPending = n
	}
	s.trackQueueLocked()
}

func (s *Scheduler) trackQueueLocked() {
	depth := 0
	if s.active != nil {
		depth++
	}
	if s.pending != nil {
		depth++
	}
	if depth > s.maxQueue {
		s.maxQueue = depth
	}
}

func (s *Scheduler) recordEventLocked(ev Event) {
	s.events = append(s.events, ev)
}

// ForegroundBusy reports whether a cold render is active or pending.
func (s *Scheduler) ForegroundBusy() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active != nil || s.pending != nil
}

// ActiveSpecies returns the species ID of the active cold render, or 0.
func (s *Scheduler) ActiveSpecies() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active != nil {
		return s.active.req.SpeciesID
	}
	return 0
}

// PendingSpecies returns the species ID of the pending cold render, or 0.
func (s *Scheduler) PendingSpecies() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending != nil {
		return s.pending.SpeciesID
	}
	return 0
}

func (s *Scheduler) display(req Request, res ColdResult) {
	if s.onDisplay != nil && res.Frame != "" {
		s.onDisplay(req, res)
	}
}

// WaitIdle blocks until no active or pending jobs remain and render goroutines exit.
func (s *Scheduler) WaitIdle(ctx context.Context) error {
	ticker := time.NewTicker(2 * time.Millisecond)
	defer ticker.Stop()
	for {
		s.mu.Lock()
		queueIdle := s.active == nil && s.pending == nil
		s.mu.Unlock()
		if !queueIdle {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
			}
			continue
		}
		if err := s.waitRenderWG(ctx); err != nil {
			return err
		}
		s.mu.Lock()
		stillIdle := s.active == nil && s.pending == nil
		s.mu.Unlock()
		if stillIdle {
			return nil
		}
	}
}

// Metrics returns a snapshot of collected scheduler metrics.
func (s *Scheduler) Metrics() MetricsSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	eventsCopy := append([]Event(nil), s.events...)
	latCopy := append([]float64(nil), s.displayedLatencyMS...)
	invariantOK := s.maxActive <= 1 && s.maxPending <= 1 && s.maxQueue <= 2
	selections := 0
	for _, ev := range eventsCopy {
		if ev.Kind == EventSelection {
			selections++
		}
	}
	return MetricsSnapshot{
		SelectionEvents:       selections,
		CacheHits:             s.cacheHits,
		ColdMisses:            s.coldMisses,
		RendersStarted:        s.rendersStarted,
		RendersCompleted:      s.rendersCompleted,
		RendersDisplayed:      s.rendersDisplayed,
		StaleCompleted:        s.staleCompleted,
		PendingReplaced:       s.pendingReplaced,
		SkippedBeforeRender:   s.skipped,
		MaxActive:             s.maxActive,
		MaxPending:            s.maxPending,
		MaxQueueDepth:         s.maxQueue,
		DisplayedLatencyMS:    latCopy,
		StaleEncodeCPU:        s.staleEncodeCPU,
		TotalEncodeCPU:        s.totalEncodeCPU,
		FinalSettleMS:         s.finalSettleMS,
		SchedulingInvariantOK: invariantOK,
		Events:                eventsCopy,
	}
}

// Reset clears scheduler state for a new trace iteration.
func (s *Scheduler) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active != nil {
		s.active.cancel()
	}
	s.active = nil
	s.pending = nil
	s.events = nil
	s.seq = 0
	s.maxActive, s.maxPending, s.maxQueue = 0, 0, 0
	s.cacheHits, s.coldMisses = 0, 0
	s.rendersStarted, s.rendersCompleted = 0, 0
	s.rendersDisplayed, s.staleCompleted = 0, 0
	s.pendingReplaced, s.skipped = 0, 0
	s.staleEncodeCPU, s.totalEncodeCPU = 0, 0
	s.displayedLatencyMS = nil
	s.finalSettleMS = 0
}
