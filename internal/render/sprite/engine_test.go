package sprite_test

import (
	"bytes"
	"context"
	"errors"
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
}

func newMemStore() *memStore {
	return &memStore{
		byURL:    make(map[string]string),
		png:      make(map[string][]byte),
		artefact: make(map[string]string),
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

func mustPNG(t *testing.T, c color.RGBA) []byte {
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
	key := sprite.CacheKey("abc", sprite.SourcePixel, "chafa", 24, 24, sprite.ColorTrueColor)
	hash, src, mode, w, h, color, ver, err := sprite.ParseCacheKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if hash != "abc" || src != sprite.SourcePixel || mode != "chafa" || w != 24 || h != 24 || color != sprite.ColorTrueColor || ver != sprite.AlgorithmVersion {
		t.Fatalf("parsed unexpected: %s %s %s %dx%d %s %s", hash, src, mode, w, h, color, ver)
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

func TestValidatePNG(t *testing.T) {
	if err := sprite.ValidatePNG(mustPNG(t, color.RGBA{1, 2, 3, 255})); err != nil {
		t.Fatal(err)
	}
	if err := sprite.ValidatePNG([]byte("xyz")); err == nil {
		t.Fatal("expected error")
	}
}
