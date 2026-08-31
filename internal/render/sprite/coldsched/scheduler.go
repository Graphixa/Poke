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
	Frame      string
	CacheKey   string
	CacheState string
	Timings    RenderTimings
	Err        error
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

	maxActive, maxPending, maxQueue int
	cacheHits, coldMisses           int
	rendersStarted, rendersCompleted int
	rendersDisplayed, staleCompleted int
	pendingReplaced, skipped        int
	staleEncodeCPU, totalEncodeCPU  time.Duration
	displayedLatencyMS              []float64

	lastSelectedAt time.Time
	finalSettleMS  float64

	onDisplay func(frame string)
}

// NewScheduler creates a scheduler with optional display callback.
func NewScheduler(renderer Renderer, onDisplay func(frame string)) *Scheduler {
	return &Scheduler{
		renderer:  renderer,
		onDisplay: onDisplay,
	}
}

// Select handles a selection change.
func (s *Scheduler) Select(ctx context.Context, gen int, pokemon domain.Pokemon, cells int) {
	now := time.Now()
	req := Request{SpeciesID: pokemon.ID, Gen: gen, Cells: cells, Pokemon: pokemon}

	s.mu.Lock()
	s.sel = Selection{Gen: gen, SpeciesID: pokemon.ID, Cells: cells}
	s.lastSelectedAt = now
	s.seq++
	s.recordEventLocked(Event{Seq: s.seq, Kind: EventSelection, SpeciesID: pokemon.ID, SelectedAt: now})

	if res, hit := s.renderer.LookupCached(ctx, pokemon, cells); hit && res.Frame != "" && res.Err == nil {
		frame := s.recordCacheHitLocked(now, pokemon.ID, res)
		s.mu.Unlock()
		s.display(frame)
		return
	}
	s.coldMisses++

	if s.active != nil {
		if s.active.req.Same(req) {
			s.mu.Unlock()
			return
		}
		cp := req
		if s.pending != nil && !s.pending.Same(req) {
			s.pendingReplaced++
			s.recordEventLocked(Event{Seq: s.seq, Kind: EventPendingReplaced, SpeciesID: pokemon.ID, PendingReplaced: true})
		}
		s.pending = &cp
		s.trackPendingLocked()
		s.abortActiveLocked()
		outcome := s.takeStartLocked(ctx, now)
		s.mu.Unlock()
		if outcome.cacheFrame != "" {
			s.display(outcome.cacheFrame)
		}
		if outcome.job != nil {
			go s.runRender(*outcome.job)
		}
		return
	}

	outcome := s.takeStartLocked(ctx, now, req)
	s.mu.Unlock()
	if outcome.cacheFrame != "" {
		s.display(outcome.cacheFrame)
	}
	if outcome.job != nil {
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
			return startOutcome{cacheFrame: frame}
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
	res := s.renderer.RenderCold(job.ctx, job.req.Pokemon, job.req.Cells)
	completed := time.Now()

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
			frame := res.Frame
			s.mu.Unlock()
			s.display(frame)
			s.mu.Lock()
			if res.CacheKey != "" {
				go func(key, f string) { _ = s.renderer.Persist(context.Background(), key, f) }(res.CacheKey, res.Frame)
			}
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
			if res.CacheKey != "" {
				go func(key, f string) { _ = s.renderer.Persist(context.Background(), key, f) }(res.CacheKey, res.Frame)
			}
		}
	}

	outcome := s.takeStartLocked(context.Background(), s.lastSelectedAt)
	s.mu.Unlock()
	if outcome.cacheFrame != "" {
		s.display(outcome.cacheFrame)
	}
	if outcome.job != nil {
		go s.runRender(*outcome.job)
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

func (s *Scheduler) display(frame string) {
	if s.onDisplay != nil && frame != "" {
		s.onDisplay(frame)
	}
}

// WaitIdle blocks until no active or pending jobs remain.
func (s *Scheduler) WaitIdle(ctx context.Context) error {
	ticker := time.NewTicker(2 * time.Millisecond)
	defer ticker.Stop()
	for {
		s.mu.Lock()
		idle := s.active == nil && s.pending == nil
		s.mu.Unlock()
		if idle {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
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
