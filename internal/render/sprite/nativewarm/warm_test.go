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

type fakeKey struct {
	id    int
	cells int
}

type fakeNativeRenderer struct {
	mu       sync.Mutex
	cached   map[fakeKey]bool
	render   int
	ids      []int
	cells    []int
	pngReads int
}

func (f *fakeNativeRenderer) HasNativeCache(_ context.Context, pokemon domain.Pokemon, cells int) bool {
	if cells < sprite.MinFrameSize {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cached[fakeKey{pokemon.ID, cells}]
}

func (f *fakeNativeRenderer) NativeWarmRender(_ context.Context, pokemon domain.Pokemon, cells int) sprite.RenderResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.render++
	f.pngReads++
	f.ids = append(f.ids, pokemon.ID)
	f.cells = append(f.cells, cells)
	if f.cached == nil {
		f.cached = map[fakeKey]bool{}
	}
	f.cached[fakeKey{pokemon.ID, cells}] = true
	return sprite.RenderResult{ANSI: "X", Native: true, Backend: "image"}
}

func (f *fakeNativeRenderer) renderedIDs() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int(nil), f.ids...)
}

func (f *fakeNativeRenderer) renders() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.render
}

func (f *fakeNativeRenderer) renderedCells() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int(nil), f.cells...)
}

type fakeLoader struct{}

func (fakeLoader) Pokemon(_ context.Context, id int) (domain.Pokemon, error) {
	return domain.Pokemon{ID: id}, nil
}

func indexN(n int) []domain.SpeciesSummary {
	out := make([]domain.SpeciesSummary, n)
	for i := range out {
		id := i + 1
		out[i] = domain.SpeciesSummary{ID: id, Slug: "s", DisplayName: "P"}
	}
	return out
}

func waitRenders(t *testing.T, rend *fakeNativeRenderer, n int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if rend.renders() >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("renders=%d want >=%d cells=%v", rend.renders(), n, rend.renderedCells())
}

func TestWarmerOnlyCurrentBucket(t *testing.T) {
	index := indexN(5)
	rend := &fakeNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: rend, Index: index})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(index, 2, 2)
	waitRenders(t, rend, 1)
	for _, c := range rend.renderedCells() {
		if c != 32 {
			t.Fatalf("cell=%d want only active bucket 32", c)
		}
	}
	svc.Stop()
}

func TestWarmerDoesNotGenerateOtherBuckets(t *testing.T) {
	index := indexN(3)
	rend := &fakeNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: rend, Index: index})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 24)
	svc.SetNeighborsFromList(index, 0, 1)
	waitRenders(t, rend, 1)
	svc.SetCells(40)
	svc.SetNeighborsFromList(index, 0, 1)
	waitRenders(t, rend, 2)
	cells := rend.renderedCells()
	if len(cells) != 2 || cells[0] != 24 || cells[1] != 40 {
		t.Fatalf("cells=%v want [24 40] one encode per bucket", cells)
	}
	svc.Stop()
}

func TestForegroundBusyPausesBackground(t *testing.T) {
	index := indexN(5)
	rend := &fakeNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: rend, Index: index})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	svc.SetForegroundBusy(true)
	svc.SetNeighborsFromList(index, 2, 2)
	time.Sleep(200 * time.Millisecond)
	if rend.renders() != 0 {
		t.Fatalf("renders=%d want 0 while foreground busy", rend.renders())
	}
	svc.SetForegroundBusy(false)
	waitRenders(t, rend, 1)
	svc.Stop()
}

func TestCacheHitSkipsRender(t *testing.T) {
	index := indexN(3)
	rend := &fakeNativeRenderer{cached: map[fakeKey]bool{{3, 32}: true}}
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: rend, Index: index})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(index, 1, 1)
	waitRenders(t, rend, 1)
	if rend.renders() != 1 {
		t.Fatalf("renders=%d want 1 (skip cached neighbor)", rend.renders())
	}
	svc.Stop()
}

func TestNeighborOrderNPlusThenNMinus(t *testing.T) {
	index := indexN(5)
	rend := &fakeNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: rend, Index: index})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(index, 2, 2)
	waitRenders(t, rend, 4)
	// cursor=2 (id 3): order 4,5,2,1
	want := []int{4, 5, 2, 1}
	got := rend.renderedIDs()
	if len(got) != len(want) {
		t.Fatalf("ids=%v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ids=%v want order %v", got, want)
		}
	}
	svc.Stop()
}

func TestShutdownStopsCleanly(t *testing.T) {
	index := indexN(20)
	rend := &fakeNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: rend, Index: index})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(index, 10, 5)
	time.Sleep(50 * time.Millisecond)
	svc.Stop()
}

func TestSnapshotReturnsZero(t *testing.T) {
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: &fakeNativeRenderer{}})
	done, total := svc.Snapshot()
	if done != 0 || total != 0 {
		t.Fatalf("snapshot=%d/%d want 0/0", done, total)
	}
}

func TestIntegrationWarmerQueueBounded(t *testing.T) {
	index := indexN(100)
	rend := &fakeNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: rend, Index: index})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(index, 50, 50)
	time.Sleep(300 * time.Millisecond)
	if n := rend.renders(); n > 20 {
		t.Fatalf("renders=%d want bounded opportunistic work", n)
	}
	svc.Stop()
}

func TestIntegrationForegroundPreemptsWarmer(t *testing.T) {
	index := indexN(10)
	rend := &fakeNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: rend, Index: index})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	svc.SetForegroundBusy(true)
	svc.SetNeighborsFromList(index, 5, 5)
	time.Sleep(200 * time.Millisecond)
	if rend.renders() != 0 {
		t.Fatalf("renders=%d want 0 while foreground busy", rend.renders())
	}
	svc.SetForegroundBusy(false)
	waitRenders(t, rend, 1)
	svc.Stop()
}
