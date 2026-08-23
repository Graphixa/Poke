package sprites_test

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
	"github.com/Graphixa/Pokedex/internal/render/halfblock"
	"github.com/Graphixa/Pokedex/internal/render/sprites"
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

func TestResolverFrontDefaultAndFallback(t *testing.T) {
	pngData := mustPNG(t, color.RGBA{255, 0, 0, 255})
	store := newMemStore()
	api := &fakeAPI{data: map[string][]byte{"https://example/p.png": pngData}}
	r := &sprites.Resolver{Store: store, API: api}

	ok := r.Resolve(context.Background(), domain.Pokemon{
		Sprites: domain.Sprites{FrontDefault: "https://example/p.png"},
	})
	if ok.UsePlaceholder || len(ok.PNG) == 0 || ok.ContentHash == "" {
		t.Fatalf("resolve = %#v", ok)
	}

	missing := r.Resolve(context.Background(), domain.Pokemon{})
	if !missing.UsePlaceholder {
		t.Fatal("empty url should placeholder")
	}

	bad := &sprites.Resolver{Store: store, API: &fakeAPI{data: map[string][]byte{"https://example/bad": []byte("nope")}}}
	inv := bad.Resolve(context.Background(), domain.Pokemon{
		Sprites: domain.Sprites{FrontDefault: "https://example/bad"},
	})
	if !inv.UsePlaceholder {
		t.Fatal("invalid png should placeholder")
	}
}

func TestRendererCachesArtefactAndInvalidates(t *testing.T) {
	pngData := mustPNG(t, color.RGBA{0, 128, 255, 255})
	store := newMemStore()
	api := &fakeAPI{data: map[string][]byte{"https://example/p.png": pngData}}
	renderer := &sprites.Renderer{
		Resolver:  &sprites.Resolver{Store: store, API: api},
		Artefacts: store,
		Enabled:   true,
		Palette:   halfblock.PaletteTrueColor,
	}
	p := domain.Pokemon{Sprites: domain.Sprites{FrontDefault: "https://example/p.png"}}

	first := renderer.Render(context.Background(), p, 4)
	if first.UsePlaceholder || first.ANSI == "" || first.CacheKey == "" {
		t.Fatalf("first = %#v", first)
	}
	if len(store.artefact) != 1 {
		t.Fatalf("artefacts = %d", len(store.artefact))
	}

	// Corrupt stored ANSI; second render at same size should hit cache.
	store.artefact[first.CacheKey] = "CACHED"
	second := renderer.Render(context.Background(), p, 4)
	if second.ANSI != "CACHED" {
		t.Fatalf("expected cache hit, got %q", second.ANSI)
	}

	// Different cell size → new key (cache invalidation by dimensions).
	third := renderer.Render(context.Background(), p, 6)
	if third.CacheKey == first.CacheKey {
		t.Fatal("size change must produce new cache key")
	}
	if third.ANSI == "CACHED" || third.UsePlaceholder {
		t.Fatalf("third = %#v", third)
	}
}

func TestRendererDisabledAndBadImage(t *testing.T) {
	r := &sprites.Renderer{Enabled: false}
	res := r.Render(context.Background(), domain.Pokemon{
		Sprites: domain.Sprites{FrontDefault: "https://x"},
	}, 4)
	if !res.UsePlaceholder {
		t.Fatal("disabled must placeholder")
	}

	store := newMemStore()
	renderer := &sprites.Renderer{
		Resolver: &sprites.Resolver{
			Store: store,
			API:   &fakeAPI{data: map[string][]byte{"https://x": []byte("bad")}},
		},
		Artefacts: store,
		Enabled:   true,
	}
	bad := renderer.Render(context.Background(), domain.Pokemon{
		Sprites: domain.Sprites{FrontDefault: "https://x"},
	}, 4)
	if !bad.UsePlaceholder {
		t.Fatal("bad image must placeholder")
	}
}

func TestValidatePNG(t *testing.T) {
	if err := sprites.ValidatePNG(mustPNG(t, color.RGBA{1, 2, 3, 255})); err != nil {
		t.Fatal(err)
	}
	if err := sprites.ValidatePNG([]byte("xyz")); err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(sprites.ContentHash([]byte("a")), "ca9781") {
		t.Fatal("unexpected hash prefix")
	}
}
