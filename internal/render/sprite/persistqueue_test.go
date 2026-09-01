package sprite_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

type slowNativeStore struct {
	*memStore
	block chan struct{}
	calls atomic.Int32
}

func (s *slowNativeStore) StoreNative(_ context.Context, key, frame string) error {
	s.calls.Add(1)
	if s.block != nil {
		<-s.block
	}
	return s.memStore.StoreNative(context.Background(), key, frame)
}

func TestPersistFrameReturnsBeforeSlowStore(t *testing.T) {
	block := make(chan struct{})
	store := &slowNativeStore{memStore: newMemStore(), block: block}
	engine := &sprite.Engine{Artefacts: store}

	done := make(chan struct{})
	var persistErr error
	go func() {
		persistErr = engine.PersistFrame(context.Background(), "key1", "SIXEL")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("PersistFrame blocked on slow store")
	}
	if persistErr != nil {
		t.Fatalf("PersistFrame err=%v", persistErr)
	}
	if store.calls.Load() != 0 {
		t.Fatalf("store invoked synchronously, calls=%d", store.calls.Load())
	}

	close(block)
	deadline := time.Now().Add(2 * time.Second)
	for store.calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if store.calls.Load() != 1 {
		t.Fatalf("async store calls=%d want 1", store.calls.Load())
	}
	engine.ClosePersist()
}

type laggyNativeStore struct {
	*memStore
	delay time.Duration
}

func (s *laggyNativeStore) StoreNative(ctx context.Context, key, frame string) error {
	time.Sleep(s.delay)
	return s.memStore.StoreNative(ctx, key, frame)
}

func TestPersistQueueDoesNotDropAcceptedBackgroundFrames(t *testing.T) {
	base := newMemStore()
	store := &laggyNativeStore{memStore: base, delay: 20 * time.Millisecond}

	engine := &sprite.Engine{Artefacts: store}
	for i := 0; i < 4; i++ {
		key := "k" + string(rune('a'+i))
		if err := engine.PersistFrame(context.Background(), key, "F"); err != nil {
			t.Fatal(err)
		}
	}
	engine.ClosePersist()

	if n := len(base.native); n != 4 {
		t.Fatalf("persisted %d keys want all 4 accepted jobs", n)
	}
}

func TestClosePersistWaitsForWorker(t *testing.T) {
	store := newMemStore()
	engine := &sprite.Engine{Artefacts: store}
	if err := engine.PersistFrame(context.Background(), "k", "FRAME"); err != nil {
		t.Fatal(err)
	}
	engine.ClosePersist()
	if _, ok := store.native["k"]; !ok {
		t.Fatal("expected frame persisted before close")
	}
}

func TestShutdownDrainsAcceptedPersistenceJobs(t *testing.T) {
	store := newMemStore()
	engine := &sprite.Engine{Artefacts: store}
	_ = engine.PersistFrame(context.Background(), "a", "A")
	_ = engine.PersistFrame(context.Background(), "b", "B")
	engine.ClosePersist()
	if len(store.native) != 2 {
		t.Fatalf("native keys=%d want 2 drained", len(store.native))
	}
}

func TestSubmitAfterCloseReturnsCanceled(t *testing.T) {
	store := newMemStore()
	engine := &sprite.Engine{Artefacts: store}
	_ = engine.PersistFrame(context.Background(), "a", "A")
	engine.ClosePersist()
	if err := engine.PersistFrame(context.Background(), "b", "B"); !errors.Is(err, context.Canceled) {
		t.Fatalf("PersistFrame after close err=%v want canceled", err)
	}
}

func TestConcurrentSubmitAndClose(t *testing.T) {
	for iter := 0; iter < 100; iter++ {
		store := newMemStore()
		engine := &sprite.Engine{Artefacts: store}
		done := make(chan struct{})
		go func() {
			for i := 0; i < 20; i++ {
				key := fmt.Sprintf("k%d", i)
				_ = engine.PersistFrame(context.Background(), key, "F")
			}
			close(done)
		}()
		time.Sleep(time.Millisecond)
		engine.ClosePersist()
		<-done
	}
}

func TestCloseBeforeFirstSubmit(t *testing.T) {
	engine := &sprite.Engine{Artefacts: newMemStore()}
	engine.ClosePersist()
	if err := engine.PersistFrame(context.Background(), "a", "A"); !errors.Is(err, context.Canceled) {
		t.Fatalf("PersistFrame err=%v want canceled", err)
	}
}

func TestClosePersistIdempotent(t *testing.T) {
	engine := &sprite.Engine{Artefacts: newMemStore()}
	_ = engine.PersistFrame(context.Background(), "a", "A")
	engine.ClosePersist()
	engine.ClosePersist()
	if err := engine.PersistFrame(context.Background(), "b", "B"); !errors.Is(err, context.Canceled) {
		t.Fatalf("PersistFrame after double close err=%v want canceled", err)
	}
}

func TestCloseWhileQueueFullDrainsAccepted(t *testing.T) {
	block := make(chan struct{})
	store := &slowNativeStore{memStore: newMemStore(), block: block}
	engine := &sprite.Engine{Artefacts: store}
	_ = engine.PersistFrame(context.Background(), "a", "A")
	_ = engine.PersistFrame(context.Background(), "b", "B")
	go func() {
		time.Sleep(20 * time.Millisecond)
		close(block)
	}()
	engine.ClosePersist()
	if len(store.native) != 2 {
		t.Fatalf("native keys=%d want 2 drained", len(store.native))
	}
}

func TestPersistFrameWaitCompletesAcceptedJob(t *testing.T) {
	store := newMemStore()
	engine := &sprite.Engine{Artefacts: store}
	if err := engine.PersistFrameWait(context.Background(), "k", "FRAME"); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.native["k"]; !ok {
		t.Fatal("expected frame persisted")
	}
}
