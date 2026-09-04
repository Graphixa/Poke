package nativewarm_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/render/sprite/nativewarm"
)

type slowNativeRenderer struct {
	mu     sync.Mutex
	delay  time.Duration
	render int
	cached map[cacheKey]bool
}

type cacheKey struct {
	id    int
	cells int
}

func (s *slowNativeRenderer) HasNativeCache(_ context.Context, pokemon domain.Pokemon, cells int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cached[cacheKey{pokemon.ID, cells}]
}

func (s *slowNativeRenderer) HasNativeFile(_ context.Context, pokemon domain.Pokemon, cells int) bool {
	return s.HasNativeCache(context.Background(), pokemon, cells)
}

func (s *slowNativeRenderer) NativeWarmRender(_ context.Context, pokemon domain.Pokemon, cells int) sprite.RenderResult {
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	s.mu.Lock()
	if s.cached == nil {
		s.cached = map[cacheKey]bool{}
	}
	s.cached[cacheKey{pokemon.ID, cells}] = true
	s.render++
	s.mu.Unlock()
	return sprite.RenderResult{
		ANSI:     "X",
		Native:   true,
		CacheKey: fmt.Sprintf("k:%d:%d", pokemon.ID, cells),
		Backend:  "image",
	}
}

func (s *slowNativeRenderer) PersistNativeFrame(_ context.Context, _, _ string) error { return nil }
func (s *slowNativeRenderer) WaitPersistCapacity(context.Context) error               { return nil }

func (s *slowNativeRenderer) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.render
}

func BenchmarkOpportunisticNativeWarm(b *testing.B) {
	index := make([]domain.SpeciesSummary, 24)
	for i := range index {
		id := i + 1
		index[i] = domain.SpeciesSummary{ID: id, Slug: "mon", DisplayName: "Mon"}
	}
	renderer := &slowNativeRenderer{delay: time.Millisecond}
	for b.Loop() {
		b.StopTimer()
		renderer.render = 0
		renderer.cached = nil
		svc := nativewarm.New(nativewarm.Config{
			Loader:   fakeLoader{},
			Renderer: renderer,
			Index:    index,
		})
		runCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		b.StartTimer()
		svc.EnsureStarted(runCtx, sprite.MaxFrameSize)
		svc.SetNeighborsFromList(index, 12, 11)
		deadline := time.Now().Add(4 * time.Second)
		for renderer.count() < 22 && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		b.StopTimer()
		cancel()
		svc.Stop()
	}
}
