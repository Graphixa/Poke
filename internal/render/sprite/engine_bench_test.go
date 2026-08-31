package sprite_test

import (
	"context"
	"image/color"
	"testing"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	spritechafa "github.com/Graphixa/Pokedex/internal/render/sprite/chafa"
	"github.com/Graphixa/Pokedex/internal/render/sprite/halfblock"
	"github.com/Graphixa/Pokedex/internal/render/sprite/placeholder"
)

func benchEngine(store *namedMemStore, mode sprite.RenderMode, extra sprite.Renderer) *sprite.Engine {
	renderers := []sprite.Renderer{extra, placeholder.Renderer{}}
	return &sprite.Engine{
		Source:     &sprite.SourceResolver{Store: store},
		Modes:      sprite.NewModeResolver(renderers...),
		Artefacts:  store,
		Caps:       sprite.Capabilities{TrueColor: true, ANSI256: true},
		Preference: mode,
		Color:      sprite.ColorTrueColor,
		SourceKind: sprite.SourcePixel,
		Background: color.RGBA{A: 255},
	}
}

func BenchmarkChafaColdRender(b *testing.B) {
	png := mustPNG(b, color.RGBA{0, 128, 255, 255})
	poke := domain.Pokemon{ID: 4, Slug: "charmander", DisplayName: "Charmander"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		store := newNamedMemStore()
		if err := store.StoreNamedSprite(context.Background(), 4, "charmander", "pixel", "", png); err != nil {
			b.Fatal(err)
		}
		engine := benchEngine(store, sprite.ModeChafa, spritechafa.Renderer{})
		b.StartTimer()
		res := engine.Render(context.Background(), poke, 24)
		if res.UsePlaceholder {
			b.Fatal(res)
		}
	}
}

func BenchmarkChafaWarmRender(b *testing.B) {
	store := newNamedMemStore()
	png := mustPNG(b, color.RGBA{0, 128, 255, 255})
	if err := store.StoreNamedSprite(context.Background(), 4, "charmander", "pixel", "", png); err != nil {
		b.Fatal(err)
	}
	engine := benchEngine(store, sprite.ModeChafa, spritechafa.Renderer{})
	poke := domain.Pokemon{ID: 4, Slug: "charmander", DisplayName: "Charmander"}
	if res := engine.Render(context.Background(), poke, 24); res.UsePlaceholder {
		b.Fatal(res)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := engine.Render(context.Background(), poke, 24)
		if res.UsePlaceholder {
			b.Fatal(res)
		}
	}
}

func BenchmarkNativeWarmRender(b *testing.B) {
	store := newNamedMemStore()
	png := mustPNG(b, color.RGBA{255, 200, 0, 255})
	if err := store.StoreNamedSprite(context.Background(), 25, "pikachu", "pixel", "", png); err != nil {
		b.Fatal(err)
	}
	native := &stubNativeRenderer{}
	engine := &sprite.Engine{
		Source: &sprite.SourceResolver{Store: store},
		Modes: sprite.NewModeResolver(
			native,
			spritechafa.Renderer{},
			placeholder.Renderer{},
		),
		Artefacts:  store,
		Caps:       sprite.Capabilities{NativeImage: true, TrueColor: true, ANSI256: true},
		Preference: sprite.ModeAuto,
		Color:      sprite.ColorTrueColor,
		SourceKind: sprite.SourcePixel,
		Background: color.RGBA{A: 255},
		HDOff:      true,
	}
	poke := domain.Pokemon{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}
	if res := engine.Render(context.Background(), poke, 24); !res.Native {
		b.Fatal(res)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := engine.Render(context.Background(), poke, 24)
		if !res.Native {
			b.Fatal(res)
		}
	}
}

func BenchmarkHalfblockColdRender(b *testing.B) {
	png := mustPNG(b, color.RGBA{255, 0, 0, 255})
	poke := domain.Pokemon{ID: 1, Slug: "bulbasaur", DisplayName: "Bulbasaur"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		store := newNamedMemStore()
		if err := store.StoreNamedSprite(context.Background(), 1, "bulbasaur", "pixel", "", png); err != nil {
			b.Fatal(err)
		}
		engine := benchEngine(store, sprite.ModeHalfblock, halfblock.Renderer{})
		b.StartTimer()
		res := engine.Render(context.Background(), poke, 24)
		if res.UsePlaceholder {
			b.Fatal(res)
		}
	}
}

func BenchmarkHalfblockWarmRender(b *testing.B) {
	store := newNamedMemStore()
	png := mustPNG(b, color.RGBA{255, 0, 0, 255})
	if err := store.StoreNamedSprite(context.Background(), 1, "bulbasaur", "pixel", "", png); err != nil {
		b.Fatal(err)
	}
	engine := benchEngine(store, sprite.ModeHalfblock, halfblock.Renderer{})
	poke := domain.Pokemon{ID: 1, Slug: "bulbasaur", DisplayName: "Bulbasaur"}
	if res := engine.Render(context.Background(), poke, 24); res.UsePlaceholder {
		b.Fatal(res)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := engine.Render(context.Background(), poke, 24)
		if res.UsePlaceholder {
			b.Fatal(res)
		}
	}
}

func BenchmarkLookupCachedMem(b *testing.B) {
	store := newNamedMemStore()
	png := mustPNG(b, color.RGBA{0, 128, 255, 255})
	if err := store.StoreNamedSprite(context.Background(), 4, "charmander", "pixel", "", png); err != nil {
		b.Fatal(err)
	}
	engine := benchEngine(store, sprite.ModeChafa, spritechafa.Renderer{})
	poke := domain.Pokemon{ID: 4, Slug: "charmander", DisplayName: "Charmander"}
	if res := engine.Render(context.Background(), poke, 24); res.UsePlaceholder {
		b.Fatal(res)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, hit := engine.LookupCached(context.Background(), poke, 24)
		if !hit || res.UsePlaceholder {
			b.Fatal(res)
		}
	}
}

func BenchmarkLookupCachedDisk(b *testing.B) {
	store := newNamedMemStore()
	png := mustPNG(b, color.RGBA{0, 128, 255, 255})
	if err := store.StoreNamedSprite(context.Background(), 4, "charmander", "pixel", "", png); err != nil {
		b.Fatal(err)
	}
	warm := benchEngine(store, sprite.ModeChafa, spritechafa.Renderer{})
	poke := domain.Pokemon{ID: 4, Slug: "charmander", DisplayName: "Charmander"}
	if res := warm.Render(context.Background(), poke, 24); res.UsePlaceholder {
		b.Fatal(res)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		engine := benchEngine(store, sprite.ModeChafa, spritechafa.Renderer{})
		b.StartTimer()
		res, hit := engine.LookupCached(context.Background(), poke, 24)
		if !hit || res.UsePlaceholder {
			b.Fatal(res)
		}
	}
}

func BenchmarkColdNativeRender(b *testing.B) {
	png := mustPNG(b, color.RGBA{255, 200, 0, 255})
	poke := domain.Pokemon{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		store := newNamedMemStore()
		if err := store.StoreNamedSprite(context.Background(), 25, "pikachu", "pixel", "", png); err != nil {
			b.Fatal(err)
		}
		native := &stubNativeRenderer{}
		engine := &sprite.Engine{
			Source: &sprite.SourceResolver{Store: store},
			Modes: sprite.NewModeResolver(
				native,
				spritechafa.Renderer{},
				placeholder.Renderer{},
			),
			Artefacts:  store,
			Caps:       sprite.Capabilities{NativeImage: true, TrueColor: true, ANSI256: true},
			Preference: sprite.ModeAuto,
			Color:      sprite.ColorTrueColor,
			SourceKind: sprite.SourcePixel,
			Background: color.RGBA{A: 255},
			HDOff:      true,
		}
		b.StartTimer()
		res := engine.RenderCold(context.Background(), poke, 24)
		if !res.Native {
			b.Fatal(res)
		}
	}
}

func seedNativeBench(b *testing.B, cells int) (*namedMemStore, domain.Pokemon, string) {
	b.Helper()
	store := newNamedMemStore()
	png := mustPNG(b, color.RGBA{255, 200, 0, 255})
	if err := store.StoreNamedSprite(context.Background(), 25, "pikachu", "pixel", "", png); err != nil {
		b.Fatal(err)
	}
	native := &stubNativeRenderer{}
	engine := &sprite.Engine{
		Source: &sprite.SourceResolver{Store: store},
		Modes: sprite.NewModeResolver(
			native,
			spritechafa.Renderer{},
			placeholder.Renderer{},
		),
		Artefacts:  store,
		Caps:       sprite.Capabilities{NativeImage: true, TrueColor: true, ANSI256: true},
		Preference: sprite.ModeAuto,
		Color:      sprite.ColorTrueColor,
		SourceKind: sprite.SourcePixel,
		Background: color.RGBA{A: 255},
		HDOff:      true,
	}
	poke := domain.Pokemon{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}
	res := engine.RenderCold(context.Background(), poke, cells)
	if !res.Native {
		b.Fatal(res)
	}
	flushNativePersistBench(b, engine, res)
	return store, poke, res.CacheKey
}

func flushNativePersistBench(b *testing.B, e *sprite.Engine, res sprite.RenderResult) {
	b.Helper()
	if err := e.PersistFrame(context.Background(), res.CacheKey, res.ANSI); err != nil {
		b.Fatal(err)
	}
	e.ClosePersist()
}

func benchNativeEngine(store *namedMemStore) *sprite.Engine {
	return &sprite.Engine{
		Source: &sprite.SourceResolver{Store: store},
		Modes: sprite.NewModeResolver(
			&stubNativeRenderer{},
			spritechafa.Renderer{},
			placeholder.Renderer{},
		),
		Artefacts:  store,
		Caps:       sprite.Capabilities{NativeImage: true, TrueColor: true, ANSI256: true},
		Preference: sprite.ModeAuto,
		Color:      sprite.ColorTrueColor,
		SourceKind: sprite.SourcePixel,
		Background: color.RGBA{A: 255},
		HDOff:      true,
	}
}

func BenchmarkLookupCachedNativeMem24(b *testing.B) {
	store, poke, _ := seedNativeBench(b, 24)
	engine := benchNativeEngine(store)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, hit := engine.LookupCached(context.Background(), poke, 24)
		if !hit || !res.Native {
			b.Fatal(res)
		}
	}
}

func BenchmarkLookupCachedNativeMem32(b *testing.B) {
	store, poke, _ := seedNativeBench(b, 32)
	engine := benchNativeEngine(store)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, hit := engine.LookupCached(context.Background(), poke, 32)
		if !hit || !res.Native {
			b.Fatal(res)
		}
	}
}

func BenchmarkLookupCachedNativeMem40(b *testing.B) {
	store, poke, _ := seedNativeBench(b, 40)
	engine := benchNativeEngine(store)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, hit := engine.LookupCached(context.Background(), poke, 40)
		if !hit || !res.Native {
			b.Fatal(res)
		}
	}
}

func BenchmarkLookupCachedNativeDisk24(b *testing.B) {
	store, poke, _ := seedNativeBench(b, 24)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		engine := benchNativeEngine(store)
		b.StartTimer()
		res, hit := engine.LookupCached(context.Background(), poke, 24)
		if !hit || !res.Native {
			b.Fatal(res)
		}
	}
}

func BenchmarkLookupCachedNativeDisk32(b *testing.B) {
	store, poke, _ := seedNativeBench(b, 32)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		engine := benchNativeEngine(store)
		b.StartTimer()
		res, hit := engine.LookupCached(context.Background(), poke, 32)
		if !hit || !res.Native {
			b.Fatal(res)
		}
	}
}

func BenchmarkLookupCachedNativeDisk40(b *testing.B) {
	store, poke, _ := seedNativeBench(b, 40)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		engine := benchNativeEngine(store)
		b.StartTimer()
		res, hit := engine.LookupCached(context.Background(), poke, 40)
		if !hit || !res.Native {
			b.Fatal(res)
		}
	}
}
