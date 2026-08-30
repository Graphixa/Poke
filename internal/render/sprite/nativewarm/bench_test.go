package nativewarm_test

import (
	"context"
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
	return sprite.RenderResult{ANSI: "X", Native: true, Backend: "image"}
}

func (s *slowNativeRenderer) NativeWarmBatch(ctx context.Context, pokemon domain.Pokemon, sizes []int) sprite.NativeWarmBatchResult {
	var encoded []int
	var last sprite.RenderResult
	for _, cells := range sizes {
		last = s.NativeWarmRender(ctx, pokemon, cells)
		if last.Native {
			encoded = append(encoded, cells)
		}
	}
	return sprite.NativeWarmBatchResult{EncodedSizes: encoded, Encoded: len(encoded), Last: last}
}

func (s *slowNativeRenderer) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.render
}

func benchmarkNativeWarmWorkers(b *testing.B, workers int) {
	const species = 24
	index := make([]domain.SpeciesSummary, species)
	for i := range index {
		id := i + 1
		index[i] = domain.SpeciesSummary{ID: id, Slug: "mon", DisplayName: "Mon"}
	}
	renderer := &slowNativeRenderer{delay: time.Millisecond}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		renderer.render = 0
		renderer.cached = nil
		svc := nativewarm.New(nativewarm.Config{
			Workers:       workers,
			Loader:        fakeLoader{},
			Renderer:      renderer,
			Index:         index,
			RequiredSizes: []int{sprite.MaxFrameSize},
			Protocol:      "sixel",
		})
		runCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		b.StartTimer()
		svc.EnsureStarted(runCtx, sprite.MaxFrameSize)
		if err := svc.Wait(runCtx); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		cancel()
		svc.Stop()
		b.StopTimer()
		if got := renderer.count(); got != species {
			b.Fatalf("workers=%d renders=%d want %d", workers, got, species)
		}
	}
}

func BenchmarkNativeWarmWorkers1(b *testing.B) {
	benchmarkNativeWarmWorkers(b, 1)
}

func BenchmarkNativeWarmWorkers4(b *testing.B) {
	benchmarkNativeWarmWorkers(b, 4)
}
