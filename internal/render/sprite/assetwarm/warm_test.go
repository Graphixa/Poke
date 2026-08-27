package assetwarm_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"sync"
	"testing"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/render/sprite/assetwarm"
)

func TestNeedsModePrompt(t *testing.T) {
	if assetwarm.NeedsModePrompt(assetwarm.ArtworkSizeThreshold - 1) {
		t.Fatal("under threshold should not prompt")
	}
	if !assetwarm.NeedsModePrompt(assetwarm.ArtworkSizeThreshold) {
		t.Fatal("at threshold should prompt")
	}
}

func TestEstimateFallback(t *testing.T) {
	index := make([]domain.SpeciesSummary, 10)
	for i := range index {
		index[i].ID = i + 1
	}
	got := assetwarm.EstimateArtworkBytes(context.Background(), index, nil, nil)
	want := int64(10) * assetwarm.FallbackBytesPerArtwork
	if got != want {
		t.Fatalf("estimate = %d, want %d", got, want)
	}
}

func TestFormatBytes(t *testing.T) {
	if got := assetwarm.FormatBytes(1500); got == "" {
		t.Fatal("empty format")
	}
	if got := assetwarm.FormatBytes(2 * 1024 * 1024); got == "" {
		t.Fatal("empty MB format")
	}
}

type memNamed struct {
	mu   sync.Mutex
	data map[string][]byte
}

func (m *memNamed) NamedSprite(_ context.Context, id int, slug, kind string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.data[kind+":"+slug]
	if !ok {
		return nil, io.EOF
	}
	return append([]byte(nil), b...), nil
}

func (m *memNamed) StoreNamedSprite(_ context.Context, id int, slug, kind, _ string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data == nil {
		m.data = make(map[string][]byte)
	}
	m.data[kind+":"+slug] = append([]byte(nil), data...)
	return nil
}

func (m *memNamed) SpriteByURL(context.Context, string) (string, []byte, error) {
	return "", nil, io.EOF
}
func (m *memNamed) StoreSpriteByURL(context.Context, string, string, []byte) error { return nil }

type fakeLoader struct{}

func (fakeLoader) Pokemon(_ context.Context, id int) (domain.Pokemon, error) {
	return domain.Pokemon{
		ID: id, Slug: "bulbasaur",
		Sprites: domain.Sprites{
			FrontDefault:    "https://example/pixel.png",
			OfficialArtwork: "https://example/art.png",
		},
	}, nil
}

type apiDL struct{ png []byte }

func (a apiDL) GetSprite(_ context.Context, _ string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(a.png)), nil
}

func mustPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestArtworkWarmStoresNamed(t *testing.T) {
	pngBytes := mustPNG(t)
	store := &memNamed{data: make(map[string][]byte)}
	resolver := &sprite.SourceResolver{
		Store: store,
		API:   apiDL{png: pngBytes},
	}
	index := []domain.SpeciesSummary{{ID: 1, Slug: "bulbasaur"}}
	svc := assetwarm.New(assetwarm.Config{
		Mode:     assetwarm.ModeArtwork,
		Workers:  1,
		Loader:   fakeLoader{},
		Resolver: resolver,
		Index:    index,
	})
	ctx := context.Background()
	svc.Start(ctx)
	if err := svc.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	svc.Stop()
	data, err := store.NamedSprite(ctx, 1, "bulbasaur", "artwork")
	if err != nil || len(data) == 0 {
		t.Fatalf("artwork not stored: err=%v len=%d", err, len(data))
	}
}
