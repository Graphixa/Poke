package sprite_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	spritechafa "github.com/Graphixa/Pokedex/internal/render/sprite/chafa"
	"github.com/Graphixa/Pokedex/internal/render/sprite/placeholder"
)

type memStore struct {
	mu       sync.Mutex
	byURL    map[string]string
	png      map[string][]byte
	artefact map[string]string
	native   map[string]string
}

func newMemStore() *memStore {
	return &memStore{
		byURL:    make(map[string]string),
		png:      make(map[string][]byte),
		artefact: make(map[string]string),
		native:   make(map[string]string),
	}
}

func (m *memStore) SpriteByURL(_ context.Context, url string) (string, []byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	hash, ok := m.byURL[url]
	if !ok {
		return "", nil, errors.New("not found")
	}
	data, ok := m.png[hash]
	if !ok {
		return "", nil, errors.New("not found")
	}
	return hash, append([]byte(nil), data...), nil
}

func (m *memStore) StoreSpriteByURL(_ context.Context, url, contentHash string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byURL[url] = contentHash
	m.png[contentHash] = append([]byte(nil), data...)
	return nil
}

func (m *memStore) Halfblock(_ context.Context, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.artefact[key]
	if !ok {
		return "", errors.New("not found")
	}
	return v, nil
}

func (m *memStore) StoreHalfblock(_ context.Context, key, ansi string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.artefact[key] = ansi
	return nil
}

func (m *memStore) Native(_ context.Context, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.native[key]
	if !ok {
		return "", errors.New("not found")
	}
	return v, nil
}

func (m *memStore) StoreNative(_ context.Context, key, frame string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.native[key] = frame
	return nil
}

type fakeAPI struct {
	data map[string][]byte
	err  error
}

func (f *fakeAPI) GetSprite(_ context.Context, url string) (io.ReadCloser, error) {
	if f.err != nil {
		return nil, f.err
	}
	data, ok := f.data[url]
	if !ok {
		return nil, errors.New("missing")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func mustPNG(t testing.TB, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestSourceResolve(t *testing.T) {
	store := newMemStore()
	png := mustPNG(t, color.RGBA{255, 0, 0, 255})
	api := &fakeAPI{data: map[string][]byte{"https://example/a.png": png}}
	r := &sprite.SourceResolver{Store: store, API: api}
	res := r.Resolve(context.Background(), domain.Pokemon{
		Sprites: domain.Sprites{FrontDefault: "https://example/a.png"},
	})
	if res.UsePlaceholder || len(res.PNG) == 0 || res.ContentHash == "" {
		t.Fatalf("unexpected result: %+v", res)
	}
	// Second resolve hits store.
	res2 := r.Resolve(context.Background(), domain.Pokemon{
		Sprites: domain.Sprites{FrontDefault: "https://example/a.png"},
	})
	if res2.ContentHash != res.ContentHash {
		t.Fatalf("cache miss: %s vs %s", res2.ContentHash, res.ContentHash)
	}
}

func TestModeResolverPriority(t *testing.T) {
	modes := sprite.NewModeResolver(
		spritechafa.Renderer{},
		placeholder.Renderer{},
	)
	caps := sprite.Capabilities{TrueColor: true, ANSI256: true}
	r := modes.Resolve(caps, sprite.ModeAuto)
	if r == nil || r.Name() != "chafa" {
		t.Fatalf("auto = %v", r)
	}
	r = modes.Resolve(caps, sprite.ModeChafa)
	if r == nil || r.Name() != "chafa" {
		t.Fatalf("chafa = %v", r)
	}
	r = modes.Resolve(sprite.Capabilities{}, sprite.ModeAuto)
	if r == nil || r.Name() != "placeholder" {
		t.Fatalf("empty caps auto = %v, want placeholder", r)
	}
	r = modes.Resolve(sprite.Capabilities{}, sprite.ModeChafa)
	if r == nil || r.Name() != "chafa" {
		t.Fatalf("forced chafa without caps = %v, want chafa", r)
	}
}

func TestEngineChafaAndCache(t *testing.T) {
	store := newMemStore()
	png := mustPNG(t, color.RGBA{0, 128, 255, 255})
	api := &fakeAPI{data: map[string][]byte{"https://example/s.png": png}}
	engine := &sprite.Engine{
		Source: &sprite.SourceResolver{Store: store, API: api},
		Modes: sprite.NewModeResolver(
			spritechafa.Renderer{},
			placeholder.Renderer{},
		),
		Artefacts:  store,
		Caps:       sprite.Capabilities{TrueColor: true, ANSI256: true},
		Preference: sprite.ModeChafa,
		Color:      sprite.ColorTrueColor,
		SourceKind: sprite.SourcePixel,
		Background: color.RGBA{A: 255},
	}
	poke := domain.Pokemon{
		ID: 4, DisplayName: "Charmander",
		Sprites: domain.Sprites{FrontDefault: "https://example/s.png"},
	}
	res := engine.Render(context.Background(), poke, 24)
	if res.UsePlaceholder || res.ANSI == "" || res.Backend != "chafa" {
		t.Fatalf("result=%+v", res)
	}
	if res.CacheKey == "" {
		t.Fatal("expected cache key")
	}
	// Second render hits artefact cache.
	res2 := engine.Render(context.Background(), poke, 24)
	if res2.ANSI != res.ANSI || res2.CacheKey != res.CacheKey {
		t.Fatalf("cache mismatch")
	}
}

func TestEnginePlaceholder(t *testing.T) {
	engine := &sprite.Engine{
		Source:     &sprite.SourceResolver{},
		Modes:      sprite.NewModeResolver(placeholder.Renderer{}),
		Preference: sprite.ModeAuto,
	}
	res := engine.Render(context.Background(), domain.Pokemon{DisplayName: "Bulbasaur"}, 24)
	if !res.UsePlaceholder {
		t.Fatalf("expected placeholder: %+v", res)
	}
}

func TestPlaceholderRenderer(t *testing.T) {
	opts := sprite.RenderOptions{
		Width: 24, Height: 8,
		BackgroundColor: color.RGBA{A: 255},
		Source:          sprite.SourcePixel,
		Label:           "Test",
	}
	pf, err := (placeholder.Renderer{}).Render(context.Background(), nil, opts)
	if err != nil || !strings.Contains(pf.Text, "Test") {
		t.Fatalf("placeholder: %+v err=%v", pf, err)
	}
}

func TestCacheKeyRoundTrip(t *testing.T) {
	bg := color.RGBA{R: 1, G: 2, B: 3, A: 255}
	key := sprite.CacheKey("abc", sprite.SourcePixel, "chafa", 24, 24, sprite.ColorTrueColor, bg)
	hash, src, mode, w, h, colorMode, bgHash, ver, err := sprite.ParseCacheKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if hash != "abc" || src != sprite.SourcePixel || mode != "chafa" || w != 24 || h != 24 || colorMode != sprite.ColorTrueColor || bgHash != "010203" || ver != sprite.AlgorithmVersion+"-"+sprite.ChafaRenderVersion {
		t.Fatalf("parsed unexpected: %s %s %s %dx%d %s %s %s", hash, src, mode, w, h, colorMode, bgHash, ver)
	}
	if sprite.ClampFrameSide(21) != sprite.MinFrameSize {
		t.Fatalf("clamp 21 = %d want %d", sprite.ClampFrameSide(21), sprite.MinFrameSize)
	}
	if sprite.ClampFrameSide(45) != sprite.MaxFrameSize {
		t.Fatalf("clamp 45 = %d want %d", sprite.ClampFrameSide(45), sprite.MaxFrameSize)
	}
	if sprite.ClampFrameSide(30) != 28 && sprite.ClampFrameSide(30) != 32 {
		t.Fatalf("clamp 30 = %d", sprite.ClampFrameSide(30))
	}
}

func TestURLForTier(t *testing.T) {
	p := domain.Pokemon{
		ID: 25,
		Sprites: domain.Sprites{
			FrontDefault:    "https://pixel.png",
			HomeFront:       "https://home.png",
			OfficialArtwork: "https://art.png",
		},
	}
	if got := sprite.URLForTier(p, sprite.AssetTierPixel); got != "https://pixel.png" {
		t.Fatalf("pixel = %q", got)
	}
	if got := sprite.URLForTier(p, sprite.AssetTierHD); got != "https://art.png" {
		t.Fatalf("hd art = %q", got)
	}
	p.Sprites.OfficialArtwork = ""
	// Without OfficialArtwork, prefer CDN artwork over Home/FrontDefault.
	if got := sprite.URLForTier(p, sprite.AssetTierHD); got != sprite.CDNArtworkURL(25) {
		t.Fatalf("hd cdn = %q want %q", got, sprite.CDNArtworkURL(25))
	}
	p.Sprites.FrontDefault = ""
	p.Sprites.HomeFront = ""
	if got := sprite.URLForTier(p, sprite.AssetTierPixel); got != sprite.CDNPixelURL(25) {
		t.Fatalf("cdn pixel = %q", got)
	}
	if got := sprite.URLForTier(p, sprite.AssetTierHD); got != sprite.CDNArtworkURL(25) {
		t.Fatalf("cdn art = %q", got)
	}
}

func TestNamedHitUsesNamedAssetKey(t *testing.T) {
	png := mustPNG(t, color.RGBA{9, 9, 9, 255})
	store := newNamedMemStore()
	if err := store.StoreNamedSprite(context.Background(), 7, "squirtle", "pixel", "", png); err != nil {
		t.Fatal(err)
	}
	res := (&sprite.SourceResolver{Store: store}).Resolve(context.Background(), domain.Pokemon{
		ID: 7, Slug: "squirtle",
	}, sprite.AssetTierPixel)
	want := sprite.NamedAssetKey(7, "squirtle", "pixel")
	if res.ContentHash != want {
		t.Fatalf("hash=%q want %q", res.ContentHash, want)
	}
	if res.UsePlaceholder || len(res.PNG) == 0 {
		t.Fatalf("bad result %+v", res)
	}
	// Must not equal raw SHA of PNG (named path skips ContentHash).
	if res.ContentHash == sprite.ContentHash(png) {
		t.Fatal("named hit should not use SHA content hash")
	}
}

func TestValidatePNG(t *testing.T) {
	if err := sprite.ValidatePNG(mustPNG(t, color.RGBA{1, 2, 3, 255})); err != nil {
		t.Fatal(err)
	}
	if err := sprite.ValidatePNG([]byte("xyz")); err == nil {
		t.Fatal("expected error")
	}
}

// namedMemStore adds NamedPNGStore on top of memStore.
type namedMemStore struct {
	*memStore
	named    map[string][]byte
	pngReads int
}

func newNamedMemStore() *namedMemStore {
	return &namedMemStore{memStore: newMemStore(), named: make(map[string][]byte)}
}

func namedKey(id int, slug, kind string) string {
	return fmt.Sprintf("%d/%s/%s", id, slug, kind)
}

func (m *namedMemStore) NamedSprite(_ context.Context, id int, slug, kind string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pngReads++
	data, ok := m.named[namedKey(id, slug, kind)]
	if !ok {
		return nil, errors.New("not found")
	}
	return append([]byte(nil), data...), nil
}

func (m *namedMemStore) NamedAssetMeta(_ context.Context, id int, slug, kind string) (string, string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.named[namedKey(id, slug, kind)]
	if !ok {
		return "", "", false, nil
	}
	return sprite.NamedAssetKey(id, slug, kind), "", true, nil
}

func (m *namedMemStore) StoreNamedSprite(_ context.Context, id int, slug, kind, _ string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.named[namedKey(id, slug, kind)] = append([]byte(nil), data...)
	return nil
}

type stubNativeRenderer struct {
	renders int
	err     error
}

func (s *stubNativeRenderer) Name() string         { return "image" }
func (s *stubNativeRenderer) ProtocolName() string { return "sixel" }
func (s *stubNativeRenderer) Supported(caps sprite.Capabilities) bool {
	return caps.NativeImage
}
func (s *stubNativeRenderer) Render(_ context.Context, _ image.Image, _ sprite.RenderOptions) (sprite.Frame, error) {
	s.renders++
	if s.err != nil {
		return sprite.Frame{}, s.err
	}
	return sprite.Frame{Text: "NATIVE", Backend: "image", Native: true}, nil
}

func TestEngineNativeMemCacheSkipsSecondEncode(t *testing.T) {
	store := newNamedMemStore()
	png := mustPNG(t, color.RGBA{255, 200, 0, 255})
	if err := store.StoreNamedSprite(context.Background(), 25, "pikachu", "pixel", "", png); err != nil {
		t.Fatal(err)
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
	res1 := engine.Render(context.Background(), poke, 24)
	if !res1.Native || res1.ANSI != "NATIVE" {
		t.Fatalf("first render: %+v", res1)
	}
	if native.renders != 1 {
		t.Fatalf("renders=%d after first", native.renders)
	}
	res2 := engine.Render(context.Background(), poke, 24)
	if !res2.Native || res2.ANSI != "NATIVE" {
		t.Fatalf("second render: %+v", res2)
	}
	if native.renders != 1 {
		t.Fatalf("mem hit must skip encode, renders=%d", native.renders)
	}
}

func TestEngineNativeDiskCacheSkipsEncodeOnFreshEngine(t *testing.T) {
	store := newNamedMemStore()
	png := mustPNG(t, color.RGBA{255, 200, 0, 255})
	if err := store.StoreNamedSprite(context.Background(), 25, "pikachu", "pixel", "", png); err != nil {
		t.Fatal(err)
	}
	native := &stubNativeRenderer{}
	mk := func() *sprite.Engine {
		return &sprite.Engine{
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
	}
	poke := domain.Pokemon{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}
	res1 := mk().Render(context.Background(), poke, 24)
	if !res1.Native || res1.ANSI != "NATIVE" {
		t.Fatalf("first render: %+v", res1)
	}
	if native.renders != 1 {
		t.Fatalf("renders=%d after first", native.renders)
	}
	if _, err := store.Native(context.Background(), res1.CacheKey); err != nil {
		t.Fatalf("expected native disk store: %v", err)
	}
	res2 := mk().Render(context.Background(), poke, 24)
	if !res2.Native || res2.ANSI != "NATIVE" {
		t.Fatalf("second engine render: %+v", res2)
	}
	if native.renders != 1 {
		t.Fatalf("disk hit must skip encode, renders=%d", native.renders)
	}
	if store.pngReads != 1 {
		t.Fatalf("warm native render must not re-read PNG, pngReads=%d", store.pngReads)
	}
}

func TestEngineNativeCancelDoesNotDisable(t *testing.T) {
	store := newNamedMemStore()
	png := mustPNG(t, color.RGBA{255, 200, 0, 255})
	if err := store.StoreNamedSprite(context.Background(), 25, "pikachu", "pixel", "", png); err != nil {
		t.Fatal(err)
	}
	native := &stubNativeRenderer{err: context.Canceled}
	disabled := false
	engine := &sprite.Engine{
		Source: &sprite.SourceResolver{Store: store},
		Modes: sprite.NewModeResolver(
			native,
			spritechafa.Renderer{},
			placeholder.Renderer{},
		),
		Artefacts:    store,
		Caps:         sprite.Capabilities{NativeImage: true, TrueColor: true, ANSI256: true},
		Preference:   sprite.ModeAuto,
		Color:        sprite.ColorTrueColor,
		SourceKind:   sprite.SourcePixel,
		Background:   color.RGBA{A: 255},
		OnNativeFail: func() { disabled = true },
	}
	poke := domain.Pokemon{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}
	_ = engine.Render(context.Background(), poke, 24)
	if disabled {
		t.Fatal("context.Canceled must not disable native for the session")
	}
}

func TestEngineNativeUsesPixelWhenHDMissingWithoutDisabling(t *testing.T) {
	store := newNamedMemStore()
	png := mustPNG(t, color.RGBA{255, 200, 0, 255})
	if err := store.StoreNamedSprite(context.Background(), 25, "pikachu", "pixel", "", png); err != nil {
		t.Fatal(err)
	}
	native := &stubNativeRenderer{}
	disabled := false
	engine := &sprite.Engine{
		Source: &sprite.SourceResolver{Store: store},
		Modes: sprite.NewModeResolver(
			native,
			spritechafa.Renderer{},
			placeholder.Renderer{},
		),
		Artefacts:    store,
		Caps:         sprite.Capabilities{NativeImage: true, TrueColor: true, ANSI256: true},
		Preference:   sprite.ModeAuto,
		Color:        sprite.ColorTrueColor,
		SourceKind:   sprite.SourcePixel,
		Background:   color.RGBA{A: 255},
		HDOff:        false,
		OnNativeFail: func() { disabled = true },
	}
	// Browse selection: id/slug only — no sprite URLs, no artwork on disk.
	poke := domain.Pokemon{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}
	res := engine.Render(context.Background(), poke, 24)
	if res.UsePlaceholder || !res.Native || res.Backend != "image" || res.ANSI != "NATIVE" {
		t.Fatalf("want native image result, got %+v", res)
	}
	if native.renders != 1 {
		t.Fatalf("native renders=%d", native.renders)
	}
	if disabled {
		t.Fatal("OnNativeFail must not run on HD asset miss when pixel is available")
	}
	// Second Pokémon with no assets at all: fall through to chafa/placeholder
	// without disabling native for the session.
	res2 := engine.Render(context.Background(), domain.Pokemon{ID: 999, Slug: "missing", DisplayName: "Missing"}, 24)
	if disabled {
		t.Fatal("OnNativeFail must not run on total asset miss")
	}
	if res2.Backend == "image" && res2.Native {
		t.Fatal("missing asset should not claim native success")
	}
	// Native still works afterward.
	res3 := engine.Render(context.Background(), poke, 24)
	if !res3.Native || res3.Backend != "image" {
		t.Fatalf("native should remain available, got %+v", res3)
	}
}

func TestEngineChafaWarmSkipsPNGRead(t *testing.T) {
	store := newNamedMemStore()
	png := mustPNG(t, color.RGBA{0, 128, 255, 255})
	if err := store.StoreNamedSprite(context.Background(), 4, "charmander", "pixel", "", png); err != nil {
		t.Fatal(err)
	}
	engine := &sprite.Engine{
		Source: &sprite.SourceResolver{Store: store},
		Modes: sprite.NewModeResolver(
			spritechafa.Renderer{},
			placeholder.Renderer{},
		),
		Artefacts:  store,
		Caps:       sprite.Capabilities{TrueColor: true, ANSI256: true},
		Preference: sprite.ModeChafa,
		Color:      sprite.ColorTrueColor,
		SourceKind: sprite.SourcePixel,
		Background: color.RGBA{A: 255},
	}
	poke := domain.Pokemon{ID: 4, Slug: "charmander", DisplayName: "Charmander"}
	res := engine.Render(context.Background(), poke, 24)
	if res.UsePlaceholder || res.Backend != "chafa" || res.ANSI == "" {
		t.Fatalf("cold render: %+v", res)
	}
	if store.pngReads != 1 {
		t.Fatalf("cold pngReads=%d want 1", store.pngReads)
	}
	res2 := engine.Render(context.Background(), poke, 24)
	if res2.ANSI != res.ANSI {
		t.Fatal("warm ansi mismatch")
	}
	if store.pngReads != 1 {
		t.Fatalf("warm chafa must not re-read PNG, pngReads=%d", store.pngReads)
	}
}
