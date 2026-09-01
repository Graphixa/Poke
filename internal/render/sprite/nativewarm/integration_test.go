package nativewarm_test

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
	"time"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	spritechafa "github.com/Graphixa/Pokedex/internal/render/sprite/chafa"
	"github.com/Graphixa/Pokedex/internal/render/sprite/nativewarm"
	"github.com/Graphixa/Pokedex/internal/render/sprite/placeholder"
)

type countingNativeRenderer struct {
	mu      sync.Mutex
	renders int
}

func (r *countingNativeRenderer) Name() string         { return "image" }
func (r *countingNativeRenderer) ProtocolName() string { return "sixel" }
func (r *countingNativeRenderer) Supported(caps sprite.Capabilities) bool {
	return caps.NativeImage
}
func (r *countingNativeRenderer) Render(_ context.Context, _ image.Image, _ sprite.RenderOptions) (sprite.Frame, error) {
	r.mu.Lock()
	r.renders++
	r.mu.Unlock()
	return sprite.Frame{
		Text:    "DCS\x1bP\x1bq\x1b\"1;2;24;24\x1bP\x1b\\",
		Backend: "image",
		Native:  true,
	}, nil
}

func (r *countingNativeRenderer) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.renders
}

type integrationLoader struct {
	index []domain.SpeciesSummary
}

func (l integrationLoader) Pokemon(_ context.Context, id int) (domain.Pokemon, error) {
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
	svc    *cache.Service
	engine *sprite.Engine
	native *countingNativeRenderer
	index  []domain.SpeciesSummary
}

func mustTestPNG(t *testing.T, seed byte) []byte {
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

func newIntegrationEnv(t *testing.T, nSpecies int) *integrationEnv {
	t.Helper()
	if nSpecies < 1 {
		t.Fatal("nSpecies must be >= 1")
	}
	root := t.TempDir()
	store := cache.NewStore(root, nil)
	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		t.Fatal(err)
	}
	svc := cache.NewService(store, nil, config.Config{}, nil)
	native := &countingNativeRenderer{}
	engine := &sprite.Engine{
		Source: &sprite.SourceResolver{Store: svc},
		Modes: sprite.NewModeResolver(
			native,
			spritechafa.Renderer{},
			placeholder.Renderer{},
		),
		Artefacts:   svc,
		Caps:        sprite.Capabilities{NativeImage: true, TrueColor: true, ANSI256: true},
		Preference:  sprite.ModeAuto,
		Color:       sprite.ColorTrueColor,
		SourceKind:  sprite.SourceArtwork,
		Background:  sprite.DefaultBackground,
		HDOff:       true,
		SixelColors: 64,
	}
	index := make([]domain.SpeciesSummary, nSpecies)
	ctx := context.Background()
	for i := range index {
		id := i + 1
		slug := slugForID(id)
		index[i] = domain.SpeciesSummary{ID: id, Slug: slug, DisplayName: "M"}
		png := mustTestPNG(t, byte(id))
		hash := sprite.ContentHash(png)
		if err := svc.StoreNamedSprite(ctx, id, slug, cache.SpriteKindArtwork, hash, png); err != nil {
			t.Fatal(err)
		}
	}
	return &integrationEnv{root: root, store: store, svc: svc, engine: engine, native: native, index: index}
}

func slugForID(id int) string {
	return fmt.Sprintf("mon%d", id)
}

func (e *integrationEnv) warmer(progress cache.ProgressFunc) *nativewarm.Service {
	return nativewarm.New(nativewarm.Config{
		Loader:   integrationLoader{e.index},
		Renderer: e.engine,
		Index:    e.index,
		Progress: progress,
	})
}

func countNativeGzipFiles(t *testing.T, root string) int {
	t.Helper()
	dir := filepath.Join(root, "native")
	entries, err := os.ReadDir(dir)
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

func waitUntil(t *testing.T, timeout time.Duration, pred func() bool, hint string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if pred() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout: %s", hint)
}

func TestIntegrationStationaryWarmPersistence(t *testing.T) {
	if testing.Short() {
		t.Skip("slow integration test")
	}
	env := newIntegrationEnv(t, 8)
	ctx := context.Background()
	svc := env.warmer(nil)
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(env.index, 0, 1)

	waitUntil(t, 5*time.Second, func() bool {
		d, _ := svc.Snapshot()
		return d >= 2
	}, "expected background warm progress while stationary")

	done, total := svc.Snapshot()
	if done < 2 {
		t.Fatalf("progress done=%d want >= 2", done)
	}
	if total != len(env.index) {
		t.Fatalf("progress total=%d want %d", total, len(env.index))
	}

	final := countNativeGzipFiles(t, env.root)
	if final < 2 {
		t.Fatalf("file count=%d want >= 2", final)
	}

	dir := filepath.Join(env.root, "native")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	readable := 0
	for _, ent := range entries {
		if !strings.HasSuffix(ent.Name(), ".sixel.gz") {
			continue
		}
		key := strings.TrimSuffix(ent.Name(), ".sixel.gz")
		if _, err := env.store.Native(ctx, key); err != nil {
			t.Fatalf("gzip frame %q not readable: %v", ent.Name(), err)
		}
		readable++
	}
	if readable != final {
		t.Fatalf("readable=%d files=%d", readable, final)
	}

	time.Sleep(100 * time.Millisecond)
	if later := countNativeGzipFiles(t, env.root); later < final {
		t.Fatalf("file count regressed %d -> %d", final, later)
	}
	svc.Stop()
	env.engine.ClosePersist()
}

func TestIntegrationForegroundReadsBackgroundCache(t *testing.T) {
	env := newIntegrationEnv(t, 5)
	ctx := context.Background()
	svc := env.warmer(nil)
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(env.index, 0, 1)

	poke2 := domain.Pokemon{ID: 2, Slug: slugForID(2)}
	waitUntil(t, 5*time.Second, func() bool {
		return env.engine.HasNativeCache(ctx, poke2, 32)
	}, "background warm species 2")
	svc.Stop()

	renders := env.native.count()
	cache.ResetNamedAssetMetaCount()
	res, hit := env.engine.LookupCached(ctx, poke2, 32)
	if !hit || !res.Native || res.ANSI == "" {
		t.Fatalf("lookup miss: hit=%v res=%+v", hit, res)
	}
	if env.native.count() != renders {
		t.Fatalf("lookup must not encode, renders=%d -> %d", renders, env.native.count())
	}
	if cache.NamedAssetMetaCount() > 2 {
		t.Fatalf("lookup must not re-read PNG assets, meta reads=%d", cache.NamedAssetMetaCount())
	}
	env.engine.ClosePersist()
}

func TestIntegrationRestartUsesPersistedCache(t *testing.T) {
	if testing.Short() {
		t.Skip("slow integration test")
	}
	const warmN = 1
	env := newIntegrationEnv(t, 4)
	ctx := context.Background()
	svc := env.warmer(nil)
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(env.index, 0, 1)

	waitUntil(t, 5*time.Second, func() bool {
		return countNativeGzipFiles(t, env.root) >= warmN
	}, "persist first background species to disk")
	if n := countNativeGzipFiles(t, env.root); n < warmN {
		t.Fatalf("persisted files=%d want >= %d", n, warmN)
	}
	env.engine.ClosePersist()
	svc.Stop()

	native2 := &countingNativeRenderer{}
	engine2 := &sprite.Engine{
		Source: &sprite.SourceResolver{Store: env.svc},
		Modes: sprite.NewModeResolver(
			native2,
			spritechafa.Renderer{},
			placeholder.Renderer{},
		),
		Artefacts:   env.svc,
		Caps:        sprite.Capabilities{NativeImage: true, TrueColor: true, ANSI256: true},
		Preference:  sprite.ModeAuto,
		Color:       sprite.ColorTrueColor,
		SourceKind:  sprite.SourceArtwork,
		Background:  sprite.DefaultBackground,
		HDOff:       true,
		SixelColors: 64,
	}
	for id := 2; id <= warmN+1; id++ {
		poke := domain.Pokemon{ID: id, Slug: slugForID(id)}
		res, hit := engine2.LookupCached(ctx, poke, 32)
		if !hit || !res.Native {
			t.Fatalf("species %d disk miss: hit=%v res=%+v", id, hit, res)
		}
	}
	if n := native2.count(); n != 0 {
		t.Fatalf("restart lookups must not encode, renders=%d", n)
	}
}

func TestIntegrationResizeBucketSwitch(t *testing.T) {
	if testing.Short() {
		t.Skip("slow integration test")
	}
	env := newIntegrationEnv(t, 6)
	ctx := context.Background()
	svc := env.warmer(nil)
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(env.index, 0, 1)

	waitUntil(t, 5*time.Second, func() bool {
		done, _ := svc.Snapshot()
		return done >= 2
	}, "warm @32")
	files32 := countNativeGzipFiles(t, env.root)
	if files32 < 2 {
		t.Fatalf("@32 files=%d want >= 2", files32)
	}

	svc.SetCells(40)
	waitUntil(t, 5*time.Second, func() bool {
		return env.engine.HasNativeCache(ctx, domain.Pokemon{ID: 2, Slug: slugForID(2)}, 40)
	}, "warm @40")

	files40 := countNativeGzipFiles(t, env.root)
	if files40 <= files32 {
		t.Fatalf("file count should grow after resize: %d -> %d", files32, files40)
	}
	if later32 := countNativeGzipFiles(t, env.root); later32 < files32 {
		t.Fatalf("@32 files removed: had %d now %d", files32, later32)
	}

	poke2 := domain.Pokemon{ID: 2, Slug: slugForID(2)}
	res32, hit32 := env.engine.LookupCached(ctx, poke2, 32)
	if !hit32 || !res32.Native {
		t.Fatal("@32 reuse miss after resize")
	}
	res40, hit40 := env.engine.LookupCached(ctx, poke2, 40)
	if !hit40 || !res40.Native {
		t.Fatal("@40 miss after warm")
	}
	svc.Stop()
	env.engine.ClosePersist()
}

type slowNativeCache struct {
	*cache.Service
	delay time.Duration
}

func (s *slowNativeCache) StoreNative(ctx context.Context, key, frame string) error {
	time.Sleep(s.delay)
	return s.Service.StoreNative(ctx, key, frame)
}

func TestIntegrationProgressIncrementsAfterPersistence(t *testing.T) {
	root := t.TempDir()
	store := cache.NewStore(root, nil)
	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		t.Fatal(err)
	}
	base := cache.NewService(store, nil, config.Config{}, nil)
	svcCache := &slowNativeCache{Service: base, delay: 400 * time.Millisecond}

	nSpecies := 5
	index := make([]domain.SpeciesSummary, nSpecies)
	ctx := context.Background()
	for i := range index {
		id := i + 1
		slug := slugForID(id)
		index[i] = domain.SpeciesSummary{ID: id, Slug: slug, DisplayName: "M"}
		png := mustTestPNG(t, byte(id))
		hash := sprite.ContentHash(png)
		if err := base.StoreNamedSprite(ctx, id, slug, cache.SpriteKindArtwork, hash, png); err != nil {
			t.Fatal(err)
		}
	}

	native := &countingNativeRenderer{}
	engine := &sprite.Engine{
		Source: &sprite.SourceResolver{Store: base},
		Modes: sprite.NewModeResolver(
			native,
			spritechafa.Renderer{},
			placeholder.Renderer{},
		),
		Artefacts:   svcCache,
		Caps:        sprite.Capabilities{NativeImage: true, TrueColor: true, ANSI256: true},
		Preference:  sprite.ModeAuto,
		Color:       sprite.ColorTrueColor,
		SourceKind:  sprite.SourceArtwork,
		Background:  sprite.DefaultBackground,
		HDOff:       true,
		SixelColors: 64,
	}

	var lastDone int
	svc := nativewarm.New(nativewarm.Config{
		Loader:   integrationLoader{index},
		Renderer: engine,
		Index:    index,
		Progress: nil,
	})
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(index, 0, 1)

	waitUntil(t, 3*time.Second, func() bool {
		d, _ := svc.Snapshot()
		lastDone = d
		return native.count() >= 1 && d == 0
	}, "encode completes before persisted progress")

	waitUntil(t, 3*time.Second, func() bool {
		d, _ := svc.Snapshot()
		lastDone = d
		return d >= 1
	}, "progress after persist")
	if lastDone < 1 {
		t.Fatalf("progress done=%d want >=1 after persistence", lastDone)
	}
	if n := countNativeGzipFiles(t, root); n < 1 {
		t.Fatalf("file count=%d want >= 1 on disk", n)
	}
	svc.Stop()
	engine.ClosePersist()
}
