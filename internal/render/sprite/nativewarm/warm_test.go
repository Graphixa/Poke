package nativewarm_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/render/sprite/nativewarm"
)

type manifestStub struct {
	complete map[int]bool
	init     []int
	marked   []int
}

func (m *manifestStub) EnsureManifest(_ string) error { return nil }
func (m *manifestStub) NativeSizeComplete(cells, _ int) bool {
	return m.complete[cells]
}
func (m *manifestStub) InitNativeSizePhase(cells, _ int) error {
	m.init = append(m.init, cells)
	return nil
}
func (m *manifestStub) MarkNativeSizeComplete(cells, _ int) error {
	m.complete[cells] = true
	m.marked = append(m.marked, cells)
	return nil
}
func (m *manifestStub) RecountNativeSize(cells int) (int, error) { return 0, nil }

type fakeNativeRenderer struct {
	mu     sync.Mutex
	cached map[int]bool
	render int
	ids    []int
}

func (f *fakeNativeRenderer) HasNativeCache(_ context.Context, pokemon domain.Pokemon, cells int) bool {
	if cells < sprite.MinFrameSize {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cached[pokemon.ID]
}

func (f *fakeNativeRenderer) NativeWarmRender(_ context.Context, pokemon domain.Pokemon, cells int) sprite.RenderResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.render++
	f.ids = append(f.ids, pokemon.ID)
	return sprite.RenderResult{ANSI: "X", Native: true, Backend: "image"}
}

func (f *fakeNativeRenderer) renders() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.render
}

type fakeLoader struct{}

func (fakeLoader) Pokemon(_ context.Context, id int) (domain.Pokemon, error) {
	return domain.Pokemon{ID: id}, nil
}

func TestEnsureStartedSkipsWhenManifestComplete(t *testing.T) {
	index := []domain.SpeciesSummary{
		{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"},
	}
	complete := map[int]bool{}
	for _, cells := range sprite.FrameSteps() {
		complete[cells] = true
	}
	manifest := &manifestStub{complete: complete}
	svc := nativewarm.New(nativewarm.Config{
		Cells:    sprite.MaxFrameSize,
		Workers:  1,
		Loader:   fakeLoader{},
		Renderer: &fakeNativeRenderer{cached: map[int]bool{}},
		Index:    index,
		Manifest: manifest,
		Protocol: "sixel",
	})
	svc.EnsureStarted(context.Background(), sprite.MaxFrameSize)
	done, total := svc.Snapshot()
	if total != 0 || done != 0 {
		t.Fatalf("snapshot done=%d total=%d want skip", done, total)
	}
}

func TestEnsureStartedOnlyEncodesMissing(t *testing.T) {
	index := []domain.SpeciesSummary{
		{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"},
		{ID: 26, Slug: "raichu", DisplayName: "Raichu"},
	}
	rend := &fakeNativeRenderer{cached: map[int]bool{25: true}}
	svc := nativewarm.New(nativewarm.Config{
		Cells:    sprite.MaxFrameSize,
		Workers:  1,
		Loader:   fakeLoader{},
		Renderer: rend,
		Index:    index,
	})
	ctx := context.Background()
	svc.EnsureStarted(ctx, sprite.MaxFrameSize)
	if err := svc.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	if rend.renders() != 1 {
		t.Fatalf("render calls=%d want 1", rend.renders())
	}
}

func TestEnsureStartedMarksManifestComplete(t *testing.T) {
	index := []domain.SpeciesSummary{
		{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"},
		{ID: 26, Slug: "raichu", DisplayName: "Raichu"},
	}
	complete := map[int]bool{}
	for _, cells := range sprite.FrameSteps() {
		if cells != sprite.MaxFrameSize {
			complete[cells] = true
		}
	}
	manifest := &manifestStub{complete: complete}
	rend := &fakeNativeRenderer{cached: map[int]bool{}}
	svc := nativewarm.New(nativewarm.Config{
		Cells:    sprite.MaxFrameSize,
		Workers:  1,
		Loader:   fakeLoader{},
		Renderer: rend,
		Index:    index,
		Manifest: manifest,
		Protocol: "sixel",
	})
	ctx := context.Background()
	svc.EnsureStarted(ctx, sprite.MaxFrameSize)
	if err := svc.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	if len(manifest.marked) != 1 || manifest.marked[0] != sprite.MaxFrameSize {
		t.Fatalf("marked=%v want [%d]", manifest.marked, sprite.MaxFrameSize)
	}
}

func TestEnsureStartedChainsBackgroundSizeAfterLayoutComplete(t *testing.T) {
	index := []domain.SpeciesSummary{{ID: 25, Slug: "pikachu"}}
	complete := map[int]bool{sprite.MaxFrameSize: true}
	manifest := &manifestStub{complete: complete}
	rend := &fakeNativeRenderer{cached: map[int]bool{}}
	svc := nativewarm.New(nativewarm.Config{
		Workers:  1,
		Loader:   fakeLoader{},
		Renderer: rend,
		Index:    index,
		Manifest: manifest,
		Protocol: "sixel",
	})
	ctx := context.Background()
	svc.EnsureStarted(ctx, sprite.MaxFrameSize)
	waitNativeRenders(t, rend, 1)
	if len(manifest.init) == 0 || manifest.init[0] != 36 {
		t.Fatalf("init phases=%v want first background size 36", manifest.init)
	}
	svc.Stop()
}

func TestWarmAllSizesMaxFirst(t *testing.T) {
	index := []domain.SpeciesSummary{{ID: 25, Slug: "pikachu"}}
	manifest := &manifestStub{complete: map[int]bool{}}
	rend := &fakeNativeRenderer{cached: map[int]bool{}}
	svc := nativewarm.New(nativewarm.Config{
		Workers:  1,
		Loader:   fakeLoader{},
		Renderer: rend,
		Index:    index,
		Manifest: manifest,
		Protocol: "sixel",
	})
	sizes := []int{40, 32}
	ctx := context.Background()
	if err := svc.WarmAllSizes(ctx, sizes); err != nil {
		t.Fatal(err)
	}
	if len(manifest.init) != 2 || manifest.init[0] != 40 || manifest.init[1] != 32 {
		t.Fatalf("init phases=%v want [40 32]", manifest.init)
	}
	if rend.renders() != 2 {
		t.Fatalf("render=%d want 2 sizes", rend.renders())
	}
}

func (f *fakeNativeRenderer) renderedIDs() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int(nil), f.ids...)
}

func waitNativeRenders(t *testing.T, rend *fakeNativeRenderer, n int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if rend.renders() >= n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("renders=%d want >=%d ids=%v", rend.renders(), n, rend.renderedIDs())
}

func TestForegroundBusySkipsBackgroundKeepsSelected(t *testing.T) {
	index := []domain.SpeciesSummary{
		{ID: 25, Slug: "pikachu"},
		{ID: 26, Slug: "raichu"},
		{ID: 27, Slug: "sandshrew"},
	}
	rend := &fakeNativeRenderer{cached: map[int]bool{}}
	svc := nativewarm.New(nativewarm.Config{
		Cells:    sprite.MaxFrameSize,
		Workers:  1,
		Loader:   fakeLoader{},
		Renderer: rend,
		Index:    index,
	})
	svc.SetForegroundBusy(true)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	svc.EnsureStarted(ctx, sprite.MaxFrameSize)
	time.Sleep(80 * time.Millisecond)
	if n := rend.renders(); n != 0 {
		t.Fatalf("P2 ran while foreground busy; renders=%d ids=%v", n, rend.renderedIDs())
	}
	svc.Prioritize(25)
	svc.SetNeighborsFromList(index, 0, 1)
	waitNativeRenders(t, rend, 2)
	for _, id := range rend.renderedIDs() {
		if id == 27 {
			t.Fatalf("P2 species 27 encoded while busy; ids=%v", rend.renderedIDs())
		}
	}
	svc.SetForegroundBusy(false)
	if err := svc.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	if rend.renders() != 3 {
		t.Fatalf("after idle render=%d want 3 ids=%v", rend.renders(), rend.renderedIDs())
	}
}

func TestActivityPausesBackgroundUntilIdle(t *testing.T) {
	index := []domain.SpeciesSummary{
		{ID: 25, Slug: "pikachu"},
		{ID: 26, Slug: "raichu"},
	}
	rend := &fakeNativeRenderer{cached: map[int]bool{}}
	svc := nativewarm.New(nativewarm.Config{
		Cells:    sprite.MaxFrameSize,
		Workers:  1,
		Loader:   fakeLoader{},
		Renderer: rend,
		Index:    index,
	})
	svc.NoteActivity()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	svc.EnsureStarted(ctx, sprite.MaxFrameSize)
	time.Sleep(80 * time.Millisecond)
	if n := rend.renders(); n != 0 {
		t.Fatalf("P2 ran during activity window; renders=%d", n)
	}
	if err := svc.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	if rend.renders() != 2 {
		t.Fatalf("after idle render=%d want 2", rend.renders())
	}
}

// compile-time check manifestStub implements interface
var _ nativewarm.ManifestStore = (*manifestStub)(nil)

// unused import guard
var _ = cache.Progress{}
