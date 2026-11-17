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

	"github.com/Graphixa/Poke/internal/domain"
	"github.com/Graphixa/Poke/internal/render/sprite"
	spritechafa "github.com/Graphixa/Poke/internal/render/sprite/chafa"
	"github.com/Graphixa/Poke/internal/render/sprite/placeholder"
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

func (m *memStore) Artefact(_ context.Context, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.artefact[key]
	if !ok {
		return "", errors.New("not found")
	}
	return v, nil
}

func (m *memStore) StoreArtefact(_ context.Context, key, ansi string) error {
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

func (m *memStore) HasNativeFile(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.native[key]
	return ok
}

func (m *memStore) NamedSprite(context.Context, int, string, string) ([]byte, error) {
	return nil, errors.New("not found")
}

func (m *memStore) StoreNamedSprite(context.Context, int, string, string, string, []byte) error {
	return nil
}

func (m *memStore) StoreNamedSpriteWithURL(context.Context, int, string, string, string, string, []byte) error {
	return nil
}

func (m *memStore) NamedAssetMeta(context.Context, int, string, string) (string, string, bool, error) {
	return "", "", false, nil
}

func (m *memStore) ReadAssetBytes(context.Context, string) ([]byte, error) {
	return nil, errors.New("not found")
}

func flushNativePersist(t *testing.T, e *sprite.Engine, res sprite.RenderResult) {
	t.Helper()
	if e == nil || res.CacheKey == "" || res.ANSI == "" {
		return
	}
	if err := e.PersistFrame(context.Background(), res.CacheKey, res.ANSI); err != nil {
		t.Fatal(err)
	}
	e.ClosePersist()
}

type fakeAPI struct {
	data map[string][]byte
	err  error
	gets int
}

func (f *fakeAPI) GetSprite(_ context.Context, url string) (io.ReadCloser, error) {
	f.gets++
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

func TestModeResolverFullChain(t *testing.T) {
	native := &stubNativeRenderer{}
	modes := sprite.NewModeResolver(
		native,
		spritechafa.Renderer{},
		placeholder.Renderer{},
	)
	r := modes.Resolve(sprite.Capabilities{NativeImage: true, TrueColor: true}, sprite.ModeAuto)
	if r == nil || r.Name() != "image" {
		t.Fatalf("native auto = %v want image", r)
	}
	r = modes.Resolve(sprite.Capabilities{TrueColor: true, ANSI256: true}, sprite.ModeAuto)
	if r == nil || r.Name() != "chafa" {
		t.Fatalf("truecolor auto = %v want chafa", r)
	}
	r = modes.Resolve(sprite.Capabilities{}, sprite.ModeAuto)
	if r == nil || r.Name() != "placeholder" {
		t.Fatalf("empty auto = %v want placeholder", r)
	}
}

type countingRenderer struct {
	inner sprite.Renderer
	n     int
}

func (c *countingRenderer) Name() string { return c.inner.Name() }
func (c *countingRenderer) Supported(caps sprite.Capabilities) bool {
	return c.inner.Supported(caps)
}
func (c *countingRenderer) Render(ctx context.Context, src image.Image, opts sprite.RenderOptions) (sprite.Frame, error) {
	c.n++
	return c.inner.Render(ctx, src, opts)
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

func TestChafaWarmHitDoesNotRerunEncoder(t *testing.T) {
	store := newNamedMemStore()
	png := mustPNG(t, color.RGBA{0, 128, 255, 255})
	if err := store.StoreNamedSprite(context.Background(), 4, "charmander", "pixel", "", png); err != nil {
		t.Fatal(err)
	}
	count := &countingRenderer{inner: spritechafa.Renderer{}}
	engine := &sprite.Engine{
		Source:     &sprite.SourceResolver{Store: store},
		Modes:      sprite.NewModeResolver(count, placeholder.Renderer{}),
		Artefacts:  store,
		Caps:       sprite.Capabilities{TrueColor: true, ANSI256: true},
		Preference: sprite.ModeChafa,
		Color:      sprite.ColorTrueColor,
		SourceKind: sprite.SourcePixel,
		Background: color.RGBA{A: 255},
		HDOff:      true,
	}
	poke := domain.Pokemon{ID: 4, Slug: "charmander", DisplayName: "Charmander"}
	if res := engine.RenderCold(context.Background(), poke, 24); res.UsePlaceholder || res.Backend != "chafa" {
		t.Fatalf("cold: %+v", res)
	}
	if count.n != 1 {
		t.Fatalf("cold encodes=%d want 1", count.n)
	}
	res, hit := engine.LookupCached(context.Background(), poke, 24)
	if !hit || res.Backend != "chafa" {
		t.Fatalf("warm lookup hit=%v res=%+v", hit, res)
	}
	if engine.Render(context.Background(), poke, 24).UsePlaceholder {
		t.Fatal("warm render missed")
	}
	if count.n != 1 {
		t.Fatalf("chafa encoder ran on cache hit, encodes=%d", count.n)
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
	if sprite.ClampFrameSide(21) != sprite.SmallestFrameBucket() {
		t.Fatalf("clamp 21 = %d want %d", sprite.ClampFrameSide(21), sprite.SmallestFrameBucket())
	}
	if sprite.ClampFrameSide(45) != sprite.MaxFrameSize {
		t.Fatalf("clamp 45 = %d want %d", sprite.ClampFrameSide(45), sprite.MaxFrameSize)
	}
	if sprite.ClampFrameSide(30) != 24 {
		t.Fatalf("clamp 30 = %d want 24", sprite.ClampFrameSide(30))
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

func TestNamedHitUsesContentHash(t *testing.T) {
	png := mustPNG(t, color.RGBA{9, 9, 9, 255})
	store := newNamedMemStore()
	if err := store.StoreNamedSprite(context.Background(), 7, "squirtle", "pixel", "", png); err != nil {
		t.Fatal(err)
	}
	res := (&sprite.SourceResolver{Store: store}).Resolve(context.Background(), domain.Pokemon{
		ID: 7, Slug: "squirtle",
	}, sprite.AssetTierPixel)
	want := sprite.ContentHash(png)
	if res.ContentHash != want {
		t.Fatalf("hash=%q want %q", res.ContentHash, want)
	}
	if res.UsePlaceholder || len(res.PNG) == 0 {
		t.Fatalf("bad result %+v", res)
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
	hashes   map[string]string
	pngReads int
}

func newNamedMemStore() *namedMemStore {
	return &namedMemStore{
		memStore: newMemStore(),
		named:    make(map[string][]byte),
		hashes:   make(map[string]string),
	}
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
	k := namedKey(id, slug, kind)
	data, ok := m.named[k]
	if !ok {
		return "", "", false, nil
	}
	if h := m.hashes[k]; sprite.IsHexSHA256(h) {
		return h, "", true, nil
	}
	_ = data
	return "", "", true, nil
}

func (m *namedMemStore) StoreNamedSprite(_ context.Context, id int, slug, kind, contentHash string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := namedKey(id, slug, kind)
	m.named[k] = append([]byte(nil), data...)
	if sprite.IsHexSHA256(contentHash) {
		m.hashes[k] = contentHash
	} else {
		m.hashes[k] = sprite.ContentHash(data)
	}
	return nil
}

func (m *namedMemStore) StoreNamedSpriteWithURL(_ context.Context, id int, slug, kind, _, sha256 string, data []byte) error {
	return m.StoreNamedSprite(context.Background(), id, slug, kind, sha256, data)
}

func (m *namedMemStore) ReadAssetBytes(context.Context, string) ([]byte, error) {
	return nil, errors.New("not found")
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
	e1 := mk()
	res1 := e1.Render(context.Background(), poke, 24)
	if !res1.Native || res1.ANSI != "NATIVE" {
		t.Fatalf("first render: %+v", res1)
	}
	if native.renders != 1 {
		t.Fatalf("renders=%d after first", native.renders)
	}
	flushNativePersist(t, e1, res1)
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

func TestLookupCachedMissDoesNotHTTPOrReadPNG(t *testing.T) {
	store := newNamedMemStore()
	png := mustPNG(t, color.RGBA{1, 2, 3, 255})
	api := &fakeAPI{data: map[string][]byte{"https://example/s.png": png}}
	engine := &sprite.Engine{
		Source:     &sprite.SourceResolver{Store: store, API: api},
		Modes:      sprite.NewModeResolver(spritechafa.Renderer{}, placeholder.Renderer{}),
		Artefacts:  store,
		Caps:       sprite.Capabilities{TrueColor: true, ANSI256: true},
		Preference: sprite.ModeChafa,
		Color:      sprite.ColorTrueColor,
		SourceKind: sprite.SourcePixel,
		Background: color.RGBA{A: 255},
	}
	poke := domain.Pokemon{
		ID: 4, Slug: "charmander", DisplayName: "Charmander",
		Sprites: domain.Sprites{FrontDefault: "https://example/s.png"},
	}
	res, hit := engine.LookupCached(context.Background(), poke, 24)
	if hit || res.ANSI != "" {
		t.Fatalf("expected miss, got hit=%v %+v", hit, res)
	}
	if api.gets != 0 {
		t.Fatalf("LookupCached must not HTTP, gets=%d", api.gets)
	}
	if store.pngReads != 0 {
		t.Fatalf("LookupCached must not read PNG, pngReads=%d", store.pngReads)
	}
}

func TestLookupCachedHitsMemWithoutEncode(t *testing.T) {
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
	cold := engine.RenderCold(context.Background(), poke, 24)
	if !cold.Native || cold.ANSI != "NATIVE" {
		t.Fatalf("cold: %+v", cold)
	}
	if native.renders != 1 {
		t.Fatalf("renders=%d after cold", native.renders)
	}
	pngReads := store.pngReads
	res, hit := engine.LookupCached(context.Background(), poke, 24)
	if !hit || res.ANSI != "NATIVE" || !res.Native {
		t.Fatalf("lookup: hit=%v %+v", hit, res)
	}
	if native.renders != 1 {
		t.Fatalf("lookup must not encode, renders=%d", native.renders)
	}
	if store.pngReads != pngReads {
		t.Fatalf("lookup must not read PNG, pngReads=%d", store.pngReads)
	}
}

func TestLookupCachedHitsDiskOnFreshEngine(t *testing.T) {
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
	e1 := mk()
	cold := e1.RenderCold(context.Background(), poke, 24)
	if !cold.Native {
		t.Fatalf("cold: %+v", cold)
	}
	if native.renders != 1 {
		t.Fatalf("renders=%d after cold", native.renders)
	}
	flushNativePersist(t, e1, cold)
	pngReads := store.pngReads
	res, hit := mk().LookupCached(context.Background(), poke, 24)
	if !hit || res.ANSI != "NATIVE" || !res.Native {
		t.Fatalf("disk lookup: hit=%v %+v", hit, res)
	}
	if native.renders != 1 {
		t.Fatalf("disk lookup must not encode, renders=%d", native.renders)
	}
	if store.pngReads != pngReads {
		t.Fatalf("disk lookup must not read PNG, pngReads=%d", store.pngReads)
	}
}

func TestLookupCachedNativeHitDoesNotHTTP(t *testing.T) {
	store := newNamedMemStore()
	png := mustPNG(t, color.RGBA{255, 200, 0, 255})
	api := &fakeAPI{data: map[string][]byte{"https://example/s.png": png}}
	if err := store.StoreNamedSprite(context.Background(), 25, "pikachu", "pixel", "", png); err != nil {
		t.Fatal(err)
	}
	native := &stubNativeRenderer{}
	mk := func() *sprite.Engine {
		return &sprite.Engine{
			Source: &sprite.SourceResolver{Store: store, API: api},
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
	poke := domain.Pokemon{
		ID: 25, Slug: "pikachu", DisplayName: "Pikachu",
		Sprites: domain.Sprites{FrontDefault: "https://example/s.png"},
	}
	e1 := mk()
	cold := e1.RenderCold(context.Background(), poke, 24)
	if !cold.Native {
		t.Fatalf("cold: %+v", cold)
	}
	flushNativePersist(t, e1, cold)
	api.gets = 0
	if res, hit := e1.LookupCached(context.Background(), poke, 24); !hit || !res.Native {
		t.Fatalf("mem hit: hit=%v %+v", hit, res)
	}
	if api.gets != 0 {
		t.Fatalf("mem hit must not HTTP, gets=%d", api.gets)
	}
	api.gets = 0
	if res, hit := mk().LookupCached(context.Background(), poke, 24); !hit || !res.Native {
		t.Fatalf("disk hit: hit=%v %+v", hit, res)
	}
	if api.gets != 0 {
		t.Fatalf("disk hit must not HTTP, gets=%d", api.gets)
	}
}

func nativeEngine(store *namedMemStore, native *stubNativeRenderer, bg color.RGBA) *sprite.Engine {
	return nativeEngineAt(store, native, bg, true)
}

func artworkNativeEngine(store *namedMemStore, native *stubNativeRenderer, bg color.RGBA) *sprite.Engine {
	return nativeEngineAt(store, native, bg, false)
}

func nativeEngineAt(store *namedMemStore, native *stubNativeRenderer, bg color.RGBA, pixel bool) *sprite.Engine {
	kind := sprite.SourceArtwork
	if pixel {
		kind = sprite.SourcePixel
	}
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
		SourceKind: kind,
		Background: bg,
		HDOff:      pixel,
	}
}

func TestSixelCacheKeyStable(t *testing.T) {
	bg := color.RGBA{R: 0x31, G: 0x32, B: 0x44, A: 255}
	cases := []struct {
		id   int
		slug string
	}{
		{1, "bulbasaur"},
		{25, "pikachu"},
		{399, "starly"},
		{400, "staravia"},
		{401, "staraptor"},
		{1025, "pecharunt"},
	}
	for _, tc := range cases {
		for _, cells := range sprite.FrameSteps() {
			w, h := sprite.FrameSizeFor(cells)
			sha := fmt.Sprintf("%064x", tc.id)
			gen := sprite.NativeCacheKey(sha, "sixel", w, h, bg, 64)
			store := newNamedMemStore()
			png := mustPNG(t, color.RGBA{255, 200, 0, 255})
			if err := store.StoreNamedSprite(context.Background(), tc.id, tc.slug, "artwork", sha, png); err != nil {
				t.Fatal(err)
			}
			if err := store.StoreNative(context.Background(), gen, "FRAME"); err != nil {
				t.Fatal(err)
			}
			native := &stubNativeRenderer{}
			engine := artworkNativeEngine(store, native, bg)
			poke := domain.Pokemon{ID: tc.id, Slug: tc.slug, DisplayName: tc.slug}
			res, hit := engine.LookupCached(context.Background(), poke, cells)
			if !hit {
				t.Fatalf("lookup miss id=%d cells=%d hash=%s gen=%s", tc.id, cells, sha, gen)
			}
			if res.CacheKey != gen {
				t.Fatalf("lookup key %q != generated %q", res.CacheKey, gen)
			}
			if native.renders != 0 {
				t.Fatalf("stable key must not encode, renders=%d", native.renders)
			}
		}
	}
}

func TestWarmSelectionSkipsPNGRead(t *testing.T) {
	store := newNamedMemStore()
	png := mustPNG(t, color.RGBA{255, 200, 0, 255})
	if err := store.StoreNamedSprite(context.Background(), 25, "pikachu", "pixel", "", png); err != nil {
		t.Fatal(err)
	}
	native := &stubNativeRenderer{}
	bg := color.RGBA{A: 255}
	poke := domain.Pokemon{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}
	engine := nativeEngine(store, native, bg)
	sprite.ResetStats()
	cold := engine.RenderCold(context.Background(), poke, 24)
	if !cold.Native {
		t.Fatalf("cold: %+v", cold)
	}
	if store.pngReads != 1 {
		t.Fatalf("cold pngReads=%d", store.pngReads)
	}
	reads := store.pngReads
	decodes := sprite.Stats().PNGDecodes.Load()
	res, hit := engine.LookupCached(context.Background(), poke, 24)
	if !hit || res.ANSI != cold.ANSI {
		t.Fatalf("warm hit=%v %+v", hit, res)
	}
	if store.pngReads != reads {
		t.Fatalf("warm re-read PNG pngReads=%d", store.pngReads)
	}
	if sprite.Stats().PNGDecodes.Load() != decodes {
		t.Fatalf("warm decoded PNG")
	}
	if native.renders != 1 {
		t.Fatalf("warm encoded, renders=%d", native.renders)
	}
}

func TestWarmSelectionSkipsDecode(t *testing.T) {
	TestWarmSelectionSkipsPNGRead(t)
}

func TestWarmSelectionSkipsSixelEncode(t *testing.T) {
	store := newNamedMemStore()
	png := mustPNG(t, color.RGBA{255, 200, 0, 255})
	if err := store.StoreNamedSprite(context.Background(), 401, "kricketot", "pixel", "", png); err != nil {
		t.Fatal(err)
	}
	native := &stubNativeRenderer{}
	bg := color.RGBA{A: 255}
	poke := domain.Pokemon{ID: 401, Slug: "kricketot", DisplayName: "Kricketot"}
	first := nativeEngine(store, native, bg)
	cold := first.RenderCold(context.Background(), poke, 32)
	if !cold.Native {
		t.Fatalf("cold: %+v", cold)
	}
	if native.renders != 1 {
		t.Fatalf("renders=%d after cold", native.renders)
	}
	flushNativePersist(t, first, cold)
	fresh := nativeEngine(store, native, bg)
	res, hit := fresh.LookupCached(context.Background(), poke, 32)
	if !hit || !res.Native {
		t.Fatalf("disk hit=%v %+v", hit, res)
	}
	if native.renders != 1 {
		t.Fatalf("disk lookup encoded, renders=%d", native.renders)
	}
}

func TestMissingSingleArtifactColdRendersOnce(t *testing.T) {
	store := newNamedMemStore()
	native := &stubNativeRenderer{}
	bg := color.RGBA{A: 255}
	eng := nativeEngine(store, native, bg)
	for i, p := range []struct {
		id   int
		slug string
		r    uint8
	}{{399, "starly", 10}, {400, "staravia", 20}, {401, "kricketot", 30}, {402, "kricketune", 40}} {
		png := mustPNG(t, color.RGBA{p.r, 20, 30, 255})
		if err := store.StoreNamedSprite(context.Background(), p.id, p.slug, "pixel", "", png); err != nil {
			t.Fatal(err)
		}
		poke := domain.Pokemon{ID: p.id, Slug: p.slug, DisplayName: p.slug}
		res := eng.RenderCold(context.Background(), poke, 24)
		if !res.Native {
			t.Fatalf("seed %d: %+v", i, res)
		}
		if err := eng.PersistFrame(context.Background(), res.CacheKey, res.ANSI); err != nil {
			t.Fatal(err)
		}
	}
	eng.ClosePersist()
	if native.renders != 4 {
		t.Fatalf("seed renders=%d", native.renders)
	}
	sha := store.hashes[namedKey(401, "kricketot", "pixel")]
	w, h := sprite.FrameSizeFor(24)
	key := sprite.NativeCacheKey(sha, "sixel", w, h, bg, 64)
	store.mu.Lock()
	delete(store.native, key)
	store.mu.Unlock()

	eng2 := nativeEngine(store, native, bg)
	for _, id := range []int{399, 400, 402} {
		slug := map[int]string{399: "starly", 400: "staravia", 402: "kricketune"}[id]
		res, hit := eng2.LookupCached(context.Background(), domain.Pokemon{ID: id, Slug: slug}, 24)
		if !hit || !res.Native {
			t.Fatalf("neighbor %d must stay warm hit=%v", id, hit)
		}
	}
	if native.renders != 4 {
		t.Fatalf("neighbors encoded, renders=%d", native.renders)
	}
	miss, hit := eng2.LookupCached(context.Background(), domain.Pokemon{ID: 401, Slug: "kricketot"}, 24)
	if hit {
		t.Fatalf("deleted artefact must miss, got %+v", miss)
	}
	cold := eng2.RenderCold(context.Background(), domain.Pokemon{ID: 401, Slug: "kricketot", DisplayName: "Kricketot"}, 24)
	if !cold.Native {
		t.Fatalf("cold 401: %+v", cold)
	}
	if native.renders != 5 {
		t.Fatalf("401 should encode once, renders=%d", native.renders)
	}
	flushNativePersist(t, eng2, cold)
	res, hit := nativeEngine(store, native, bg).LookupCached(context.Background(), domain.Pokemon{ID: 401, Slug: "kricketot"}, 24)
	if !hit || res.ANSI != cold.ANSI {
		t.Fatalf("revisit 401 hit=%v %+v", hit, res)
	}
	if native.renders != 5 {
		t.Fatalf("revisit encoded, renders=%d", native.renders)
	}
}
