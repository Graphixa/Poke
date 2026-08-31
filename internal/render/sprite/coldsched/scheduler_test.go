package coldsched_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite/coldsched"
)

type fakeRenderer struct {
	mu        sync.Mutex
	blocks    map[int]chan struct{}
	released  map[int]bool
	started   []int
	persisted []string
	cached    map[string]string
	delay     time.Duration
	blockAll  bool
}

func newFakeRenderer() *fakeRenderer {
	return &fakeRenderer{
		blocks:   make(map[int]chan struct{}),
		released: make(map[int]bool),
		cached:   make(map[string]string),
	}
}

func (f *fakeRenderer) key(id, cells int) string {
	return fmt.Sprintf("%d:%d", id, cells)
}

func (f *fakeRenderer) setBlockAll(v bool) {
	f.mu.Lock()
	f.blockAll = v
	f.mu.Unlock()
}

func (f *fakeRenderer) block(speciesID int) chan struct{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.released[speciesID] {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	if ch, ok := f.blocks[speciesID]; ok {
		return ch
	}
	ch := make(chan struct{})
	f.blocks[speciesID] = ch
	return ch
}

func (f *fakeRenderer) release(speciesID int) {
	f.mu.Lock()
	f.released[speciesID] = true
	ch, ok := f.blocks[speciesID]
	if ok {
		delete(f.blocks, speciesID)
	}
	f.mu.Unlock()
	if ok {
		close(ch)
	}
}

func (f *fakeRenderer) releaseAllStarted() {
	f.mu.Lock()
	ids := append([]int(nil), f.started...)
	f.mu.Unlock()
	for _, id := range ids {
		f.release(id)
	}
}

func (f *fakeRenderer) LookupCached(_ context.Context, pokemon domain.Pokemon, cells int) (coldsched.ColdResult, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if frame, ok := f.cached[f.key(pokemon.ID, cells)]; ok {
		return coldsched.ColdResult{Frame: frame, CacheState: "mem"}, true
	}
	return coldsched.ColdResult{}, false
}

func (f *fakeRenderer) RenderCold(ctx context.Context, pokemon domain.Pokemon, cells int) coldsched.ColdResult {
	f.mu.Lock()
	f.started = append(f.started, pokemon.ID)
	block := f.blockAll
	f.mu.Unlock()

	if block {
		ch := f.block(pokemon.ID)
		select {
		case <-ch:
		case <-ctx.Done():
			return coldsched.ColdResult{Err: ctx.Err()}
		}
	}

	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return coldsched.ColdResult{Err: ctx.Err()}
		}
	}

	frame := "frame-" + f.key(pokemon.ID, cells)
	return coldsched.ColdResult{
		Frame:      frame,
		CacheKey:   f.key(pokemon.ID, cells),
		CacheState: "cold",
		Timings: coldsched.RenderTimings{
			SixelEncode: 10 * time.Millisecond,
			Total:       10 * time.Millisecond,
		},
	}
}

func (f *fakeRenderer) Persist(_ context.Context, cacheKey, frame string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cached[cacheKey] = frame
	f.persisted = append(f.persisted, cacheKey)
	return nil
}

func poke(id int) domain.Pokemon {
	return domain.Pokemon{ID: id, Slug: "mon", DisplayName: "Mon"}
}

func newTestScheduler(f *fakeRenderer) *coldsched.Scheduler {
	return coldsched.NewScheduler(f, func(_ coldsched.Request, frame string) {})
}

func TestColdSchedulerAtMostOneActive(t *testing.T) {
	f := newFakeRenderer()
	f.setBlockAll(true)
	s := newTestScheduler(f)
	ctx := context.Background()

	s.Select(ctx, 1, poke(1), 32, false)
	time.Sleep(5 * time.Millisecond)
	m := s.Metrics()
	if m.MaxActive > 1 {
		t.Fatalf("max active=%d want <=1", m.MaxActive)
	}
	f.release(1)
	_ = s.WaitIdle(ctx)
}

func TestColdSchedulerAtMostOnePending(t *testing.T) {
	f := newFakeRenderer()
	f.setBlockAll(true)
	s := newTestScheduler(f)
	ctx := context.Background()

	s.Select(ctx, 1, poke(1), 32, false)
	s.Select(ctx, 2, poke(2), 32, false)
	s.Select(ctx, 3, poke(3), 32, false)
	time.Sleep(5 * time.Millisecond)
	m := s.Metrics()
	if m.MaxPending > 1 {
		t.Fatalf("max pending=%d want <=1", m.MaxPending)
	}
	f.release(1)
	f.release(3)
	_ = s.WaitIdle(ctx)
}

func TestColdSchedulerLatestPendingWins(t *testing.T) {
	f := newFakeRenderer()
	f.setBlockAll(true)
	s := newTestScheduler(f)
	ctx := context.Background()

	s.Select(ctx, 1, poke(1), 32, false)
	s.Select(ctx, 2, poke(2), 32, false)
	s.Select(ctx, 3, poke(3), 32, false)
	s.Select(ctx, 4, poke(4), 32, false)
	s.Select(ctx, 5, poke(5), 32, false)

	f.release(1)
	f.release(5)
	if err := s.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	lastStarted := f.started[len(f.started)-1]
	if lastStarted != 5 {
		t.Fatalf("last render=%d want 5 (latest pending wins)", lastStarted)
	}
}

func TestColdSchedulerDoesNotFIFOQueue(t *testing.T) {
	f := newFakeRenderer()
	f.setBlockAll(true)
	s := newTestScheduler(f)
	ctx := context.Background()

	s.Select(ctx, 1, poke(1), 32, false)
	for id := 2; id <= 5; id++ {
		s.Select(ctx, id, poke(id), 32, false)
	}
	f.release(1)
	f.release(5)
	if err := s.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int{2, 3, 4} {
		for _, started := range f.started {
			if started == id {
				t.Fatalf("species %d should not have been rendered (no FIFO queue)", id)
			}
		}
	}
}

func TestColdSchedulerStaleResultNotDisplayed(t *testing.T) {
	f := newFakeRenderer()
	f.setBlockAll(true)
	var displayed []string
	s := coldsched.NewScheduler(f, func(_ coldsched.Request, frame string) { displayed = append(displayed, frame) })
	ctx := context.Background()

	s.Select(ctx, 1, poke(1), 32, false)
	s.Select(ctx, 2, poke(2), 32, false)
	f.release(1)
	f.release(2)
	if err := s.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	for _, frame := range displayed {
		if frame == "frame-1:32" {
			t.Fatal("stale frame displayed")
		}
	}
	if len(displayed) == 0 || displayed[len(displayed)-1] != "frame-2:32" {
		t.Fatalf("displayed=%v want final frame-2:32", displayed)
	}
}

func TestColdSchedulerFinalSelectionDisplayed(t *testing.T) {
	f := newFakeRenderer()
	f.setBlockAll(true)
	var lastFrame string
	s := coldsched.NewScheduler(f, func(_ coldsched.Request, frame string) { lastFrame = frame })
	ctx := context.Background()

	for id := 1; id <= 5; id++ {
		s.Select(ctx, id, poke(id), 32, false)
	}
	f.release(1)
	f.release(5)
	if err := s.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	if lastFrame != "frame-5:32" {
		t.Fatalf("last frame=%q want frame-5:32", lastFrame)
	}
}

func TestColdSchedulerCachedHitBypassesColdWorker(t *testing.T) {
	f := newFakeRenderer()
	f.cached[f.key(7, 32)] = "cached-frame"
	s := newTestScheduler(f)
	ctx := context.Background()

	s.Select(ctx, 1, poke(7), 32, false)
	if err := s.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	if len(f.started) != 0 {
		t.Fatalf("cold worker started=%v want none", f.started)
	}
}

func TestColdSchedulerCachedHitNotDebounced(t *testing.T) {
	f := newFakeRenderer()
	f.cached[f.key(7, 32)] = "cached-frame"
	s := newTestScheduler(f)
	ctx := context.Background()

	start := time.Now()
	for i := 0; i < 20; i++ {
		s.Select(ctx, i, poke(7), 32, false)
	}
	if err := s.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("20 cache hits took %v; debounce suspected", elapsed)
	}
}

func TestColdSchedulerStaleValidResultCanPersist(t *testing.T) {
	f := newFakeRenderer()
	f.setBlockAll(true)
	s := newTestScheduler(f)
	ctx := context.Background()

	s.Select(ctx, 1, poke(1), 32, false)
	s.Select(ctx, 2, poke(2), 32, false)
	f.release(1)
	f.release(2)
	if err := s.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	if len(f.persisted) == 0 {
		t.Fatal("expected stale valid result persisted")
	}
}

func TestColdSchedulerWheel20RemainsBounded(t *testing.T) {
	f := newFakeRenderer()
	f.delay = 25 * time.Millisecond
	s := newTestScheduler(f)
	ctx := context.Background()

	for id := 1; id <= 20; id++ {
		s.Select(ctx, id, poke(id), 32, false)
		time.Sleep(50 * time.Millisecond)
	}
	if err := s.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	m := s.Metrics()
	if !m.SchedulingInvariantOK {
		t.Fatalf("invariants failed: active=%d pending=%d queue=%d", m.MaxActive, m.MaxPending, m.MaxQueueDepth)
	}
}

func TestColdSchedulerBurst40RemainsBounded(t *testing.T) {
	f := newFakeRenderer()
	f.delay = 10 * time.Millisecond
	s := newTestScheduler(f)
	ctx := context.Background()

	for id := 1; id <= 50; id++ {
		s.Select(ctx, id, poke(id), 32, false)
		time.Sleep(25 * time.Millisecond)
	}
	if err := s.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	m := s.Metrics()
	if !m.SchedulingInvariantOK {
		t.Fatalf("invariants failed: active=%d pending=%d queue=%d", m.MaxActive, m.MaxPending, m.MaxQueueDepth)
	}
	if m.FinalSettleMS <= 0 {
		t.Fatal("expected final selection to settle")
	}
}
