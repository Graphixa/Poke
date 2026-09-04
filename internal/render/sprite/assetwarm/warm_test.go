package assetwarm_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/render/sprite/assetwarm"
)

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

func TestArtworkWorkers(t *testing.T) {
	if got := assetwarm.ArtworkWorkers(8); got != 16 {
		t.Fatalf("floor: got %d", got)
	}
	if got := assetwarm.ArtworkWorkers(20); got != 20 {
		t.Fatalf("mid: got %d", got)
	}
	if got := assetwarm.ArtworkWorkers(100); got != 24 {
		t.Fatalf("cap: got %d", got)
	}
}

func TestPixelWorkers(t *testing.T) {
	if got := assetwarm.PixelWorkers(8); got != 12 {
		t.Fatalf("floor: got %d", got)
	}
	if got := assetwarm.PixelWorkers(16); got != 16 {
		t.Fatalf("mid: got %d", got)
	}
	if got := assetwarm.PixelWorkers(100); got != 20 {
		t.Fatalf("cap: got %d", got)
	}
}

func TestCDNURLWarmSkipsLoader(t *testing.T) {
	pngBytes := mustPNG(t)
	store := &memNamed{data: make(map[string][]byte)}
	resolver := &sprite.SourceResolver{
		Store: store,
		API:   apiDL{png: pngBytes},
	}
	var loads atomic.Int32
	loader := countingLoader{n: &loads}
	index := []domain.SpeciesSummary{{ID: 1, Slug: "bulbasaur", DisplayName: "Bulbasaur"}}
	svc := assetwarm.New(assetwarm.Config{
		Mode:     assetwarm.ModePixel,
		Workers:  1,
		Loader:   loader,
		Resolver: resolver,
		Index:    index,
	})
	ctx := context.Background()
	svc.Start(ctx)
	if err := svc.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	svc.Stop()
	if loads.Load() != 0 {
		t.Fatalf("expected CDN/index path with 0 loader calls, got %d", loads.Load())
	}
	data, err := store.NamedSprite(ctx, 1, "bulbasaur", "pixel")
	if err != nil || len(data) == 0 {
		t.Fatalf("pixel not stored: err=%v len=%d", err, len(data))
	}
}

type countingLoader struct{ n *atomic.Int32 }

func (c countingLoader) Pokemon(_ context.Context, id int) (domain.Pokemon, error) {
	c.n.Add(1)
	return fakeLoader{}.Pokemon(context.Background(), id)
}

func TestEstimateParallelProbe(t *testing.T) {
	index := make([]domain.SpeciesSummary, 32)
	for i := range index {
		index[i].ID = i + 1
	}
	var probes atomic.Int32
	sampler := probeFunc(func(_ context.Context, _ string) (int64, error) {
		probes.Add(1)
		return 100_000, nil
	})
	got := assetwarm.EstimateArtworkBytes(context.Background(), index, fakeLoader{}, sampler)
	if got <= 0 {
		t.Fatalf("estimate=%d", got)
	}
	if probes.Load() < 1 {
		t.Fatal("expected HEAD probes")
	}
}

type probeFunc func(ctx context.Context, url string) (int64, error)

func (f probeFunc) ProbeBytes(ctx context.Context, url string) (int64, error) {
	return f(ctx, url)
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
