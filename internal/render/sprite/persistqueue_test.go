package sprite_test

import (
	"context"
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

func TestPersistQueueBoundedDropOldest(t *testing.T) {
	base := newMemStore()
	store := &laggyNativeStore{memStore: base, delay: 30 * time.Millisecond}

	engine := &sprite.Engine{Artefacts: store}
	for i := 0; i < 4; i++ {
		key := "k" + string(rune('a'+i))
		_ = engine.PersistFrame(context.Background(), key, "F")
	}
	engine.ClosePersist()

	if n := len(base.native); n > 2 {
		t.Fatalf("persisted %d keys want at most 2", n)
	}
}

func TestClosePersistWaitsForWorker(t *testing.T) {
	store := newMemStore()
	engine := &sprite.Engine{Artefacts: store}
	_ = engine.PersistFrame(context.Background(), "k", "FRAME")
	engine.ClosePersist()
	if _, ok := store.native["k"]; !ok {
		t.Fatal("expected frame persisted before close")
	}
}
