package sprite_test

import (
	"context"
	"image/color"
	"testing"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	spritechafa "github.com/Graphixa/Pokedex/internal/render/sprite/chafa"
)

func TestNativeWarmBatchSingleDecode(t *testing.T) {
	TestNativeWarmBatchColdSourceSingleDecode(t)
}

func TestNativeWarmBatchColdSourceSingleDecode(t *testing.T) {
	store := newNamedMemStore()
	png := mustPNG(t, color.RGBA{255, 200, 0, 255})
	if err := store.StoreNamedSprite(context.Background(), 25, "pikachu", "artwork", "", png); err != nil {
		t.Fatal(err)
	}
	native := &stubNativeRenderer{}
	engine := artworkNativeEngine(store, native, color.RGBA{A: 255})
	poke := domain.Pokemon{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}
	sizes := sprite.FrameSteps()
	sprite.ResetStats()
	store.pngReads = 0

	res := engine.NativeWarmBatch(context.Background(), poke, sizes)
	if res.Encoded != len(sizes) {
		t.Fatalf("encoded=%d want %d", res.Encoded, len(sizes))
	}
	if store.pngReads != 1 {
		t.Fatalf("pngReads=%d want 1", store.pngReads)
	}
	if got := sprite.Stats().PNGDecodes.Load(); got != 1 {
		t.Fatalf("pngDecodes=%d want 1", got)
	}
	if native.renders != len(sizes) {
		t.Fatalf("renders=%d want %d", native.renders, len(sizes))
	}
}

func TestNativeWarmBatchPartialSizes(t *testing.T) {
	store := newNamedMemStore()
	png := mustPNG(t, color.RGBA{255, 200, 0, 255})
	if err := store.StoreNamedSprite(context.Background(), 401, "kricketot", "artwork", "", png); err != nil {
		t.Fatal(err)
	}
	native := &stubNativeRenderer{}
	bg := color.RGBA{A: 255}
	engine := artworkNativeEngine(store, native, bg)
	poke := domain.Pokemon{ID: 401, Slug: "kricketot", DisplayName: "Kricketot"}
	existing := []int{24, 32}
	for _, cells := range existing {
		res := engine.NativeWarmRender(context.Background(), poke, cells)
		if !res.Native {
			t.Fatalf("seed size=%d failed", cells)
		}
	}
	sprite.ResetStats()
	store.pngReads = 0
	beforeRenders := native.renders

	all := append(append([]int(nil), existing...), 40)
	res := engine.NativeWarmBatch(context.Background(), poke, all)
	if res.Encoded != 1 {
		t.Fatalf("encoded=%d want 1 missing size", res.Encoded)
	}
	if store.pngReads != 1 {
		t.Fatalf("pngReads=%d want 1", store.pngReads)
	}
	if got := sprite.Stats().PNGDecodes.Load(); got != 1 {
		t.Fatalf("pngDecodes=%d want 1", got)
	}
	if native.renders != beforeRenders+1 {
		t.Fatalf("renders=%d want %d", native.renders, beforeRenders+1)
	}
}

func TestNativeWarmBatchColdPathSinglePNGRead(t *testing.T) {
	TestNativeWarmBatchColdSourceUsesResolvedPNGWithoutSecondRead(t)
}

func TestNativeWarmBatchColdSourceUsesResolvedPNGWithoutSecondRead(t *testing.T) {
	store := newNamedMemStore()
	png := mustPNG(t, color.RGBA{255, 200, 0, 255})
	hash := sprite.ContentHash(png)
	url := "https://example/pikachu.png"
	if err := store.StoreSpriteByURL(context.Background(), url, hash, png); err != nil {
		t.Fatal(err)
	}
	native := &stubNativeRenderer{}
	engine := artworkNativeEngine(store, native, color.RGBA{A: 255})
	poke := domain.Pokemon{
		ID: 25, Slug: "pikachu", DisplayName: "Pikachu",
		Sprites: domain.Sprites{OfficialArtwork: url},
	}
	sprite.ResetStats()
	store.pngReads = 0

	res := engine.NativeWarmBatch(context.Background(), poke, sprite.FrameSteps())
	if res.Encoded != len(sprite.FrameSteps()) {
		t.Fatalf("encoded=%d want %d", res.Encoded, len(sprite.FrameSteps()))
	}
	if store.pngReads != 1 {
		t.Fatalf("pngReads=%d want 1", store.pngReads)
	}
	if got := sprite.Stats().PNGReads.Load(); got != 1 {
		t.Fatalf("stats pngReads=%d want 1", got)
	}
	if got := sprite.Stats().PNGDecodes.Load(); got != 1 {
		t.Fatalf("pngDecodes=%d want 1", got)
	}
}

func TestNativeWarmBatchFallsBackToSingleSizeRenderer(t *testing.T) {
	store := newNamedMemStore()
	png := mustPNG(t, color.RGBA{10, 20, 30, 255})
	if err := store.StoreNamedSprite(context.Background(), 4, "charmander", "pixel", "", png); err != nil {
		t.Fatal(err)
	}
	chafa := spritechafa.Renderer{}
	engine := &sprite.Engine{
		Source:     &sprite.SourceResolver{Store: store},
		Modes:      sprite.NewModeResolver(chafa, spritechafa.Renderer{}),
		Artefacts:  store,
		Caps:       sprite.Capabilities{TrueColor: true, ANSI256: true},
		Preference: sprite.ModeChafa,
		Color:      sprite.ColorTrueColor,
		SourceKind: sprite.SourcePixel,
		Background: color.RGBA{A: 255},
	}
	poke := domain.Pokemon{ID: 4, Slug: "charmander", DisplayName: "Charmander"}
	res := engine.NativeWarmBatch(context.Background(), poke, []int{24, 32})
	if res.Encoded != 0 {
		t.Fatalf("non-native renderer should not batch encode: %+v", res)
	}
}
