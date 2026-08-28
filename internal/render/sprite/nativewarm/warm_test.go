package nativewarm_test

import (
	"context"
	"testing"

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
	m.marked = append(m.marked, cells)
	return nil
}
func (m *manifestStub) RecountNativeSize(cells int) (int, error) { return 0, nil }

type fakeNativeRenderer struct {
	cached map[int]bool
	render int
}

func (f *fakeNativeRenderer) HasNativeCache(_ context.Context, pokemon domain.Pokemon, cells int) bool {
	if cells < sprite.MinFrameSize {
		return false
	}
	return f.cached[pokemon.ID]
}

func (f *fakeNativeRenderer) NativeWarmRender(_ context.Context, _ domain.Pokemon, cells int) sprite.RenderResult {
	f.render++
	return sprite.RenderResult{ANSI: "X", Native: true, Backend: "image"}
}

type fakeLoader struct{}

func (fakeLoader) Pokemon(_ context.Context, id int) (domain.Pokemon, error) {
	return domain.Pokemon{ID: id}, nil
}

func TestEnsureStartedSkipsWhenManifestComplete(t *testing.T) {
	index := []domain.SpeciesSummary{
		{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"},
	}
	manifest := &manifestStub{complete: map[int]bool{sprite.MaxFrameSize: true}}
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
	if rend.render != 1 {
		t.Fatalf("render calls=%d want 1", rend.render)
	}
}

func TestEnsureStartedMarksManifestComplete(t *testing.T) {
	index := []domain.SpeciesSummary{
		{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"},
		{ID: 26, Slug: "raichu", DisplayName: "Raichu"},
	}
	manifest := &manifestStub{complete: map[int]bool{}}
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
	if rend.render != 2 {
		t.Fatalf("render=%d want 2 sizes", rend.render)
	}
}

// compile-time check manifestStub implements interface
var _ nativewarm.ManifestStore = (*manifestStub)(nil)

// unused import guard
var _ = cache.Progress{}
