package cache_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

func testImageSpec() cache.ImageCacheSpec {
	return cache.DefaultImageCacheSpec("sixel", string(sprite.SourceArtwork), sprite.DefaultBackground)
}

func testHash(id int) string {
	return fmt.Sprintf("%064x", id)
}

func testIndex(n int) []domain.SpeciesSummary {
	out := make([]domain.SpeciesSummary, n)
	for i := 0; i < n; i++ {
		id := i + 1
		out[i] = domain.SpeciesSummary{ID: id, Slug: fmt.Sprintf("p%d", id), DisplayName: fmt.Sprintf("P%d", id)}
	}
	return out
}

func seedArtwork(t *testing.T, store *cache.Store, id int, slug, hash string) {
	t.Helper()
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, byte(id)}
	if err := store.StoreNamedSpriteWithMeta(context.Background(), id, slug, cache.SpriteKindArtwork, "", hash, png); err != nil {
		t.Fatal(err)
	}
}

func seedNativeSizes(t *testing.T, svc *cache.Service, spec cache.ImageCacheSpec, hash string, sizes []int) {
	t.Helper()
	bg := spec.BackgroundRGBA()
	ctx := context.Background()
	for _, cells := range sizes {
		w, h := sprite.FrameSizeFor(cells)
		key := sprite.NativeCacheKey(hash, spec.Protocol, w, h, bg)
		if err := svc.StoreNative(ctx, key, "FRAME"); err != nil {
			t.Fatal(err)
		}
	}
}

func seedComplete(t *testing.T, store *cache.Store, svc *cache.Service, spec cache.ImageCacheSpec, sum domain.SpeciesSummary) {
	t.Helper()
	hash := testHash(sum.ID)
	seedArtwork(t, store, sum.ID, sum.Slug, hash)
	seedNativeSizes(t, svc, spec, hash, spec.RequiredSizes)
}

func startSession(t *testing.T, store *cache.Store, index []domain.SpeciesSummary, spec cache.ImageCacheSpec) *cache.ImageCacheCoordinator {
	t.Helper()
	c, err := cache.StartImageCacheSession(store, index, spec)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestImageCacheSessionFirstRunIncludesAllPokemon(t *testing.T) {
	store := cache.NewStore(t.TempDir(), nil)
	index := testIndex(8)
	spec := testImageSpec()
	c := startSession(t, store, index, spec)
	p := c.Progress()
	if p.Total != 8 || p.Done != 0 || !p.Active {
		t.Fatalf("progress=%+v want total=8 done=0 active", p)
	}
}

func TestImageCacheSessionNoChangesHasZeroWork(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	index := testIndex(4)
	spec := testImageSpec()
	for _, sum := range index {
		seedComplete(t, store, svc, spec, sum)
	}
	c := startSession(t, store, index, spec)
	p := c.Progress()
	if p.Total != 0 || p.Active {
		t.Fatalf("progress=%+v want zero work", p)
	}
}

func TestImageCacheSessionIncludesOnlyNewPokemon(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	spec := testImageSpec()
	known := testIndex(100)
	for i := 0; i < 100; i++ {
		seedComplete(t, store, svc, spec, known[i])
	}
	c1 := startSession(t, store, known, spec)
	if p := c1.Progress(); p.Total != 0 {
		t.Fatalf("setup total=%d want 0", p.Total)
	}

	current := testIndex(103)
	c := startSession(t, store, current, spec)
	p := c.Progress()
	if p.Total != 3 || p.Done != 0 {
		t.Fatalf("progress=%+v want 0/3", p)
	}
}

func TestImageCacheSessionDetectsChangedSourceHash(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	spec := testImageSpec()
	index := testIndex(2)
	seedComplete(t, store, svc, spec, index[0])
	seedComplete(t, store, svc, spec, index[1])
	c1 := startSession(t, store, index, spec)
	if p := c1.Progress(); p.Total != 0 {
		t.Fatalf("setup total=%d", p.Total)
	}

	newHash := testHash(99)
	seedArtwork(t, store, index[0].ID, index[0].Slug, newHash)
	c := startSession(t, store, index, spec)
	p := c.Progress()
	if p.Total != 1 {
		t.Fatalf("progress=%+v want only changed Pokémon", p)
	}
}

func TestImageCacheSessionDetectsMissingVariant(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	spec := testImageSpec()
	index := testIndex(2)
	seedComplete(t, store, svc, spec, index[0])
	hash := testHash(index[1].ID)
	seedArtwork(t, store, index[1].ID, index[1].Slug, hash)
	partial := spec.RequiredSizes[:len(spec.RequiredSizes)-1]
	seedNativeSizes(t, svc, spec, hash, partial)

	c := startSession(t, store, index, spec)
	if p := c.Progress(); p.Total != 1 {
		t.Fatalf("progress=%+v want missing-variant Pokémon only", p)
	}
}

func TestImageCacheSessionDetectsSpecChange(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	spec := testImageSpec()
	index := testIndex(3)
	for _, sum := range index {
		seedComplete(t, store, svc, spec, sum)
	}
	c1 := startSession(t, store, index, spec)
	if p := c1.Progress(); p.Total != 0 {
		t.Fatalf("setup total=%d", p.Total)
	}

	changed := spec
	changed.RendererVersion = spec.RendererVersion + "-next"
	c := startSession(t, store, index, changed)
	if p := c.Progress(); p.Total != 3 {
		t.Fatalf("progress=%+v want all outstanding after spec change", p)
	}
}

func TestImageCacheProgressCountsPokemonNotSizeBuckets(t *testing.T) {
	store := cache.NewStore(t.TempDir(), nil)
	spec := testImageSpec()
	index := testIndex(2)
	for _, sum := range index {
		seedArtwork(t, store, sum.ID, sum.Slug, testHash(sum.ID))
	}
	c := startSession(t, store, index, spec)
	var last cache.ImageCacheProgress
	c.SetProgressFunc(func(p cache.Progress) {
		last = cache.ImageCacheProgress{Done: p.Done, Total: p.Total, Active: p.Total > 0 && p.Done < p.Total}
	})
	for _, cells := range spec.RequiredSizes {
		c.NoteVariant(index[0].ID, cells)
	}
	if last.Done != 1 || last.Total != 2 {
		t.Fatalf("after one Pokémon complete last=%+v want done=1 total=2", last)
	}
	p := c.Progress()
	if p.Done != 1 || p.Total != 2 {
		t.Fatalf("progress=%+v", p)
	}
}

func TestImageCacheProgressRemainsActiveWhileThrottled(t *testing.T) {
	store := cache.NewStore(t.TempDir(), nil)
	c := startSession(t, store, testIndex(5), testImageSpec())
	p := c.Progress()
	if !p.Active || p.Done != 0 || p.Total != 5 {
		t.Fatalf("progress=%+v want active 0/5", p)
	}
}

func TestImageCacheProgressCompletesExactlyOncePerPokemon(t *testing.T) {
	store := cache.NewStore(t.TempDir(), nil)
	spec := testImageSpec()
	index := testIndex(1)
	seedArtwork(t, store, index[0].ID, index[0].Slug, testHash(index[0].ID))
	c := startSession(t, store, index, spec)
	emits := 0
	c.SetProgressFunc(func(cache.Progress) { emits++ })
	for i := 0; i < 3; i++ {
		for _, cells := range spec.RequiredSizes {
			c.NoteVariant(index[0].ID, cells)
		}
	}
	p := c.Progress()
	if p.Done != 1 || emits != 1 {
		t.Fatalf("done=%d emits=%d want 1/1", p.Done, emits)
	}
}

func TestImageCacheRestartResumesOnlyOutstandingPokemon(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	spec := testImageSpec()
	index := testIndex(5)
	for i := 0; i < 2; i++ {
		seedComplete(t, store, svc, spec, index[i])
	}
	c1 := startSession(t, store, index, spec)
	if p := c1.Progress(); p.Total != 3 {
		t.Fatalf("first session total=%d want 3", p.Total)
	}
	seedComplete(t, store, svc, spec, index[2])
	c2 := startSession(t, store, index, spec)
	if p := c2.Progress(); p.Total != 2 || p.Done != 0 {
		t.Fatalf("restart progress=%+v want 0/2", p)
	}
}

func TestImageCachePartialArtworkAndSizesAreOutstanding(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	spec := testImageSpec()
	index := testIndex(10)
	for i := 0; i < 5; i++ {
		seedComplete(t, store, svc, spec, index[i])
	}
	for i := 5; i < 8; i++ {
		seedArtwork(t, store, index[i].ID, index[i].Slug, testHash(index[i].ID))
	}
	for i := 8; i < 10; i++ {
		hash := testHash(index[i].ID)
		seedArtwork(t, store, index[i].ID, index[i].Slug, hash)
		seedNativeSizes(t, svc, spec, hash, spec.RequiredSizes[:2])
	}
	c := startSession(t, store, index, spec)
	if p := c.Progress(); p.Total != 5 {
		t.Fatalf("progress=%+v want outstanding=5", p)
	}
}

func TestImageCacheCompletePokemonHasAllRequiredSizes(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	spec := testImageSpec()
	index := testIndex(3)
	for _, sum := range index {
		seedComplete(t, store, svc, spec, sum)
	}
	c := startSession(t, store, index, spec)
	if p := c.Progress(); p.Active {
		t.Fatal("expected no outstanding work")
	}
	bg := spec.BackgroundRGBA()
	for _, id := range c.CompletePokemonIDs() {
		var sum domain.SpeciesSummary
		for _, s := range index {
			if s.ID == id {
				sum = s
				break
			}
		}
		hash, _, ready, err := store.NamedAssetMeta(context.Background(), sum.ID, sum.Slug, cache.SpriteKindArtwork)
		if err != nil || !ready {
			t.Fatalf("id=%d meta ready=%v err=%v", id, ready, err)
		}
		for _, cells := range spec.RequiredSizes {
			w, h := sprite.FrameSizeFor(cells)
			key := sprite.NativeCacheKey(hash, spec.Protocol, w, h, bg)
			if !storeHasNative(svc, key) {
				t.Fatalf("complete id=%d missing size %d key=%s", id, cells, key)
			}
		}
	}
}

func storeHasNative(svc *cache.Service, key string) bool {
	_, err := svc.Native(context.Background(), key)
	return err == nil
}

func TestImageCacheSpecIDChangesWithRendererVersion(t *testing.T) {
	a := testImageSpec()
	b := a
	b.RendererVersion = "other"
	if a.ID() == b.ID() {
		t.Fatal("spec identity must change with renderer version")
	}
}

func TestImageCachePublishStartDoesNotScan(t *testing.T) {
	store := cache.NewStore(t.TempDir(), nil)
	c := startSession(t, store, testIndex(3), testImageSpec())
	cache.ResetNativeDirectoryScanCount()
	c.PublishStart()
	c.Progress()
	if n := cache.NativeDirectoryScanCount(); n != 0 {
		t.Fatalf("progress path scans=%d want 0", n)
	}
}
