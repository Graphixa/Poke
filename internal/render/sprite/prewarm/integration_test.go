package prewarm_test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Graphixa/Poke/internal/cache"
	"github.com/Graphixa/Poke/internal/config"
	"github.com/Graphixa/Poke/internal/domain"
	"github.com/Graphixa/Poke/internal/render/sprite"
	spritechafa "github.com/Graphixa/Poke/internal/render/sprite/chafa"
	"github.com/Graphixa/Poke/internal/render/sprite/placeholder"
	"github.com/Graphixa/Poke/internal/render/sprite/prewarm"
)

// countingNative is a fake native (sixel) image renderer for the real Engine.
type countingNative struct {
	mu      sync.Mutex
	renders int
}

func (r *countingNative) Name() string         { return "image" }
func (r *countingNative) ProtocolName() string { return "sixel" }
func (r *countingNative) Supported(caps sprite.Capabilities) bool {
	return caps.NativeImage
}
func (r *countingNative) Render(_ context.Context, _ image.Image, _ sprite.RenderOptions) (sprite.Frame, error) {
	r.mu.Lock()
	r.renders++
	r.mu.Unlock()
	return sprite.Frame{Text: "DCS\x1bP\x1bq\x1b\"1;2;24;24\x1bP\x1b\\", Backend: "image", Native: true}, nil
}

func (r *countingNative) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.renders
}

type indexLoader struct{ index []domain.SpeciesSummary }

func (l indexLoader) Pokemon(_ context.Context, id int) (domain.Pokemon, error) {
	for _, sum := range l.index {
		if sum.ID == id {
			return domain.Pokemon{ID: sum.ID, Slug: sum.Slug, DisplayName: sum.DisplayName}, nil
		}
	}
	return domain.Pokemon{ID: id}, nil
}

type integrationEnv struct {
	root   string
	store  *cache.Store
	engine *sprite.Engine
	native *countingNative
	index  []domain.SpeciesSummary
}

func testPNG(t *testing.T, seed byte) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.SetRGBA(x, y, color.RGBA{seed, uint8(x * 40), uint8(y * 40), 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func slugFor(id int) string { return fmt.Sprintf("mon%d", id) }

func newIntegrationEnv(t *testing.T, n int) *integrationEnv {
	t.Helper()
	root := t.TempDir()
	store := cache.NewStore(root, nil)
	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		t.Fatal(err)
	}
	svc := cache.NewService(store, nil, config.Config{}, nil)
	native := &countingNative{}
	engine := &sprite.Engine{
		Source:      &sprite.SourceResolver{Store: svc},
		Modes:       sprite.NewModeResolver(native, spritechafa.Renderer{}, placeholder.Renderer{}),
		Artefacts:   svc,
		Caps:        sprite.Capabilities{NativeImage: true, TrueColor: true, ANSI256: true},
		Preference:  sprite.ModeAuto,
		Color:       sprite.ColorTrueColor,
		SourceKind:  sprite.SourceArtwork,
		Background:  sprite.DefaultBackground,
		HDOff:       true,
		SixelColors: 64,
	}
	index := make([]domain.SpeciesSummary, n)
	for i := range index {
		id := i + 1
		index[i] = domain.SpeciesSummary{ID: id, Slug: slugFor(id), DisplayName: "M"}
		png := testPNG(t, byte(id))
		if err := svc.StoreNamedSprite(context.Background(), id, slugFor(id), cache.SpriteKindArtwork, sprite.ContentHash(png), png); err != nil {
			t.Fatal(err)
		}
	}
	return &integrationEnv{root: root, store: store, engine: engine, native: native, index: index}
}

func countGzip(t *testing.T, root string) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "native"))
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, ent := range entries {
		if strings.HasSuffix(ent.Name(), ".sixel.gz") {
			n++
		}
	}
	return n
}

// Real Engine + cache: warm persists readable .sixel.gz frames, Done follows
// the persisted files, and foreground LookupCached then hits without encoding.
func TestIntegrationNativeWarmPersistsAndForegroundHits(t *testing.T) {
	env := newIntegrationEnv(t, 5)
	ctx := context.Background()
	svc := prewarm.NewNative(env.index, indexLoader{env.index}, env.engine, nil)
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(env.index, 0, 1)
	poke2 := domain.Pokemon{ID: 2, Slug: slugFor(2)}
	waitFor(t, "all species warmed", func() bool { d, tot := svc.Snapshot(); return tot == 5 && d == 5 })
	svc.Stop()
	defer env.engine.ClosePersist()

	files := countGzip(t, env.root)
	if files != 5 {
		t.Fatalf("gzip files=%d want 5", files)
	}
	entries, _ := os.ReadDir(filepath.Join(env.root, "native"))
	for _, ent := range entries {
		key, ok := strings.CutSuffix(ent.Name(), ".sixel.gz")
		if !ok {
			continue
		}
		if _, err := env.store.Native(ctx, key); err != nil {
			t.Fatalf("frame %q not readable: %v", ent.Name(), err)
		}
	}

	renders := env.native.count()
	res, hit := env.engine.LookupCached(ctx, poke2, 32)
	if !hit || !res.Native || res.ANSI == "" {
		t.Fatalf("lookup miss: hit=%v res=%+v", hit, res)
	}
	if env.native.count() != renders {
		t.Fatalf("lookup must not encode: %d -> %d", renders, env.native.count())
	}
}
