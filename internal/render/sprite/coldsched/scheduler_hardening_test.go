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

type hardeningRenderer struct {
	mu             sync.Mutex
	blocks         map[int]chan struct{}
	released       map[int]bool
	started        []int
	persisted      []string
	blockAll       bool
	persistGate    chan struct{}
	persistEntered chan struct{}
}

func newHardeningRenderer() *hardeningRenderer {
	return &hardeningRenderer{
		blocks:   make(map[int]chan struct{}),
		released: make(map[int]bool),
	}
}

func (f *hardeningRenderer) key(id, cells int) string {
	return fmt.Sprintf("%d:%d", id, cells)
}

func (f *hardeningRenderer) setBlockAll(v bool) {
	f.mu.Lock()
	f.blockAll = v
	f.mu.Unlock()
}

func (f *hardeningRenderer) block(speciesID int) chan struct{} {
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

func (f *hardeningRenderer) release(speciesID int) {
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

func (f *hardeningRenderer) releaseAllStarted() {
	f.mu.Lock()
	ids := append([]int(nil), f.started...)
	f.mu.Unlock()
	for _, id := range ids {
		f.release(id)
	}
}

func (f *hardeningRenderer) startedIDs() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int(nil), f.started...)
}

func (f *hardeningRenderer) LookupCached(context.Context, domain.Pokemon, int) (coldsched.ColdResult, bool) {
	return coldsched.ColdResult{}, false
}

func (f *hardeningRenderer) RenderCold(ctx context.Context, pokemon domain.Pokemon, cells int) coldsched.ColdResult {
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
	frame := "frame"
	return coldsched.ColdResult{
		Frame:    frame,
		CacheKey: f.key(pokemon.ID, cells),
		Timings:  coldsched.RenderTimings{SixelEncode: time.Millisecond},
	}
}

func (f *hardeningRenderer) Persist(_ context.Context, cacheKey, _ string) error {
	if f.persistEntered != nil {
		select {
		case f.persistEntered <- struct{}{}:
		default:
		}
	}
	if f.persistGate != nil {
		<-f.persistGate
	}
	f.mu.Lock()
	f.persisted = append(f.persisted, cacheKey)
	f.mu.Unlock()
	return nil
}

func newHardeningScheduler(r coldsched.Renderer) *coldsched.Scheduler {
	return coldsched.NewScheduler(context.Background(), r, nil)
}

func TestSchedulerStopWaitsForBlockedRenderer(t *testing.T) {
	f := newHardeningRenderer()
	f.setBlockAll(true)
	s := newHardeningScheduler(f)
	ctx := context.Background()
	s.Select(ctx, 1, poke(1), 32, false, false)
	done := make(chan error, 1)
	go func() { done <- s.Stop(ctx) }()
	time.Sleep(20 * time.Millisecond)
	f.release(1)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not wait for blocked renderer")
	}
}

func TestSchedulerStopPreventsPendingAfterStop(t *testing.T) {
	f := newHardeningRenderer()
	f.setBlockAll(true)
	s := newHardeningScheduler(f)
	ctx := context.Background()
	s.Select(ctx, 1, poke(1), 32, false, false)
	s.Select(ctx, 2, poke(2), 32, false, false)
	if err := s.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	f.releaseAllStarted()
	time.Sleep(50 * time.Millisecond)
	if len(f.startedIDs()) > 1 {
		t.Fatalf("pending render started after stop: %v", f.startedIDs())
	}
}

func TestPersistenceDoesNotBlockPendingPromotion(t *testing.T) {
	f := newHardeningRenderer()
	f.persistGate = make(chan struct{})
	f.persistEntered = make(chan struct{}, 1)
	s := newHardeningScheduler(f)
	ctx := context.Background()

	s.Select(ctx, 1, poke(1), 32, false, false)
	select {
	case <-f.persistEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("persist for render #1 did not start")
	}

	s.Select(ctx, 2, poke(2), 32, false, false)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, id := range f.startedIDs() {
			if id == 2 {
				close(f.persistGate)
				if err := s.WaitIdle(ctx); err != nil {
					t.Fatal(err)
				}
				return
			}
		}
		time.Sleep(2 * time.Millisecond)
	}
	close(f.persistGate)
	t.Fatal("render #2 did not start before blocked persistence released")
}
