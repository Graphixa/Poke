package sprite_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"testing"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

type memNamedStore struct {
	byKind map[string][]byte
	byURL  map[string][]byte
}

func newMemNamed() *memNamedStore {
	return &memNamedStore{
		byKind: map[string][]byte{},
		byURL:  map[string][]byte{},
	}
}

func (m *memNamedStore) key(id int, slug, kind string) string {
	return sprite.NamedAssetKey(id, slug, kind)
}

func (m *memNamedStore) NamedSprite(_ context.Context, id int, slug, kind string) ([]byte, error) {
	data, ok := m.byKind[m.key(id, slug, kind)]
	if !ok {
		return nil, io.EOF
	}
	return data, nil
}

func (m *memNamedStore) StoreNamedSprite(_ context.Context, id int, slug, kind, _ string, data []byte) error {
	m.byKind[m.key(id, slug, kind)] = data
	return nil
}

func (m *memNamedStore) SpriteByURL(_ context.Context, url string) (string, []byte, error) {
	data, ok := m.byURL[url]
	if !ok {
		return "", nil, io.EOF
	}
	return "hash", data, nil
}

func (m *memNamedStore) StoreSpriteByURL(_ context.Context, url, hash string, data []byte) error {
	m.byURL[url] = data
	return nil
}

type stubAPI struct {
	byURL map[string][]byte
}

func (a stubAPI) GetSprite(_ context.Context, url string) (io.ReadCloser, error) {
	data, ok := a.byURL[url]
	if !ok {
		return nil, io.EOF
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func mustTinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{1, 2, 3, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestHDPixelFallbackDoesNotPoisonArtworkTree(t *testing.T) {
	pixelPNG := mustTinyPNG(t)
	store := newMemNamed()
	api := stubAPI{byURL: map[string][]byte{
		// Artwork CDN missing; only pixel CDN available.
		sprite.CDNPixelURL(25): pixelPNG,
	}}
	r := &sprite.SourceResolver{Store: store, API: api}
	p := domain.Pokemon{ID: 25, Slug: "pikachu", Sprites: domain.Sprites{
		FrontDefault: sprite.CDNPixelURL(25),
	}}
	res := r.Resolve(context.Background(), p, sprite.AssetTierHD)
	if res.UsePlaceholder || len(res.PNG) == 0 {
		t.Fatal("expected pixel fallback success")
	}
	if res.AssetKind != "pixel" {
		t.Fatalf("AssetKind=%q want pixel", res.AssetKind)
	}
	if _, err := store.NamedSprite(context.Background(), 25, "pikachu", "artwork"); err == nil {
		t.Fatal("pixel fallback must not write official-artwork named path")
	}
	if data, err := store.NamedSprite(context.Background(), 25, "pikachu", "pixel"); err != nil || len(data) == 0 {
		t.Fatal("pixel fallback should land in pixel named tree")
	}
}

func TestResolveURLsPreferCDNArtworkOverFrontDefault(t *testing.T) {
	p := domain.Pokemon{ID: 25, Sprites: domain.Sprites{
		FrontDefault: "https://pixel-only.png",
	}}
	if got := sprite.URLForTier(p, sprite.AssetTierHD); got != sprite.CDNArtworkURL(25) {
		t.Fatalf("URLForTier=%q want CDN artwork", got)
	}
}
