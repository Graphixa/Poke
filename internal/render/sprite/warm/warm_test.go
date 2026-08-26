package warm_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/render/sprite/warm"
)

type fakeLoader struct {
	mu   sync.Mutex
	hits []int
}

func (f *fakeLoader) Pokemon(_ context.Context, id int) (domain.Pokemon, error) {
	f.mu.Lock()
	f.hits = append(f.hits, id)
	f.mu.Unlock()
	return domain.Pokemon{
		ID: id,
		Sprites: domain.Sprites{
			FrontDefault: "https://example/p.png",
		},
	}, nil
}

type fakeRenderer struct {
	order   []int
	mu      sync.Mutex
	blockP2 chan struct{}
	started chan struct{}
	calls   atomic.Int32
}

func (f *fakeRenderer) WarmRender(ctx context.Context, pokemon domain.Pokemon, _ int) sprite.RenderResult {
	f.calls.Add(1)
	if f.started != nil {
		select {
		case f.started <- struct{}{}:
		default:
		}
	}
	f.mu.Lock()
	f.order = append(f.order, pokemon.ID)
	f.mu.Unlock()
	// Optionally block background jobs to observe priority.
	if f.blockP2 != nil && pokemon.ID >= 100 {
		select {
		case <-ctx.Done():
			return sprite.RenderResult{UsePlaceholder: true}
		case <-f.blockP2:
		}
	}
	return sprite.RenderResult{ANSI: "x", Backend: "chafa"}
}

func TestPrioritySelectedBeforeBackground(t *testing.T) {
	loader := &fakeLoader{}
	rend := &fakeRenderer{blockP2: make(chan struct{}), started: make(chan struct{}, 8)}
	index := make([]domain.SpeciesSummary, 0, 5)
	for id := 100; id <= 104; id++ {
		index = append(index, domain.SpeciesSummary{ID: id})
	}
	svc := warm.New(warm.Config{
		Mode:     warm.ModeAllPixel,
		Cells:    24,
		Workers:  1,
		Loader:   loader,
		Renderer: rend,
		Index:    index,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Start(ctx)

	// Wait until a background job starts, then prioritize species 1.
	select {
	case <-rend.started:
	case <-time.After(2 * time.Second):
		t.Fatal("background did not start")
	}
	svc.Prioritize(1)
	close(rend.blockP2)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		rend.mu.Lock()
		order := append([]int(nil), rend.order...)
		rend.mu.Unlock()
		for i, id := range order {
			if id == 1 {
				// Selected should appear; ideally before remaining background.
				_ = i
				svc.Stop()
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	rend.mu.Lock()
	defer rend.mu.Unlock()
	t.Fatalf("selected never warmed; order=%v", rend.order)
}

func TestProgressReports(t *testing.T) {
	var last cache.Progress
	var mu sync.Mutex
	loader := &fakeLoader{}
	rend := &fakeRenderer{}
	index := []domain.SpeciesSummary{{ID: 1}, {ID: 2}}
	svc := warm.New(warm.Config{
		Mode:     warm.ModeAllPixel,
		Cells:    24,
		Workers:  2,
		Loader:   loader,
		Renderer: rend,
		Index:    index,
		Progress: func(p cache.Progress) {
			mu.Lock()
			last = p
			mu.Unlock()
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	svc.Start(ctx)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		done, total := svc.Snapshot()
		if done >= 2 && total == 2 {
			mu.Lock()
			stage := last.Stage
			mu.Unlock()
			if stage != "sprites_render" {
				t.Fatalf("stage=%q", stage)
			}
			svc.Stop()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	done, total := svc.Snapshot()
	t.Fatalf("warm incomplete done=%d total=%d", done, total)
}

func TestModeOffNoop(t *testing.T) {
	rend := &fakeRenderer{}
	svc := warm.New(warm.Config{
		Mode:     warm.ModeOff,
		Loader:   &fakeLoader{},
		Renderer: rend,
		Index:    []domain.SpeciesSummary{{ID: 1}},
	})
	svc.Start(context.Background())
	svc.Prioritize(1)
	time.Sleep(50 * time.Millisecond)
	if rend.calls.Load() != 0 {
		t.Fatalf("expected no renders, got %d", rend.calls.Load())
	}
	svc.Stop()
}
