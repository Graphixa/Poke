package cache_test

import (
	"testing"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/config"
)

func TestDeferredImageSessionTrustedDoesNoAssetLookups(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	index := testIndex(4)
	spec := testImageSpec()
	for _, sum := range index {
		seedComplete(t, store, svc, spec, sum)
	}
	c1 := startSession(t, store, index, spec)
	if p := c1.Progress(); p.Total != 0 {
		t.Fatalf("setup total=%d", p.Total)
	}
	cache.ResetNamedAssetMetaCount()
	c, err := cache.StartImageCacheSessionDeferred(store, index, spec)
	if err != nil {
		t.Fatal(err)
	}
	if p := c.Progress(); p.Total != 0 {
		t.Fatalf("trusted deferred progress=%+v", p)
	}
	if n := cache.NamedAssetMetaCount(); n != 0 {
		t.Fatalf("NamedAssetMeta calls=%d want 0", n)
	}
}

func TestDeferredImageSessionTrustedDoesNoNativeStats(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	index := testIndex(3)
	spec := testImageSpec()
	for _, sum := range index {
		seedComplete(t, store, svc, spec, sum)
	}
	startSession(t, store, index, spec)
	cache.ResetNativeDirectoryScanCount()
	c, err := cache.StartImageCacheSessionDeferred(store, index, spec)
	if err != nil {
		t.Fatal(err)
	}
	c.Progress()
	if n := cache.NativeDirectoryScanCount(); n != 0 {
		t.Fatalf("native scans=%d want 0", n)
	}
}

func TestDeferredImageSessionUntrustedReturnsBeforeAudit(t *testing.T) {
	store := cache.NewStore(t.TempDir(), nil)
	index := testIndex(5)
	spec := testImageSpec()
	cache.ResetNamedAssetMetaCount()
	c, err := cache.StartImageCacheSessionDeferred(store, index, spec)
	if err != nil {
		t.Fatal(err)
	}
	if p := c.Progress(); p.Total != 5 {
		t.Fatalf("progress=%+v want total=5 before audit", p)
	}
	if n := cache.NamedAssetMetaCount(); n != 0 {
		t.Fatalf("deferred startup audited assets: %d lookups", n)
	}
}

func TestDeferredReconcileRunsOnce(t *testing.T) {
	store := cache.NewStore(t.TempDir(), nil)
	index := testIndex(3)
	spec := testImageSpec()
	c, err := cache.StartImageCacheSessionDeferred(store, index, spec)
	if err != nil {
		t.Fatal(err)
	}
	cache.ResetNamedAssetMetaCount()
	c.ReconcileImageCacheSession(index)
	first := cache.NamedAssetMetaCount()
	c.ReconcileImageCacheSession(index)
	second := cache.NamedAssetMetaCount()
	if first == 0 {
		t.Fatal("expected reconcile to audit")
	}
	if second != first {
		t.Fatalf("reconcile ran twice: first=%d second=%d", first, second)
	}
}

func TestDeferredReconcileStartsWarmerWhenWorkFound(t *testing.T) {
	store := cache.NewStore(t.TempDir(), nil)
	index := testIndex(2)
	spec := testImageSpec()
	for _, sum := range index {
		seedArtwork(t, store, sum.ID, sum.Slug, testHash(sum.ID))
	}
	c, err := cache.StartImageCacheSessionDeferred(store, index, spec)
	if err != nil {
		t.Fatal(err)
	}
	c.ReconcileImageCacheSession(index)
	if p := c.Progress(); p.Total != 2 || !p.Active {
		t.Fatalf("after reconcile progress=%+v", p)
	}
	if len(c.OutstandingIDs()) != 2 {
		t.Fatalf("outstanding=%v", c.OutstandingIDs())
	}
}

func TestNewDatasetOneNewPokemonCreatesOneOutstanding(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	spec := testImageSpec()
	old := testIndex(100)
	for _, sum := range old {
		seedComplete(t, store, svc, spec, sum)
	}
	c1 := startSession(t, store, old, spec)
	if p := c1.Progress(); p.Total != 0 {
		t.Fatalf("setup total=%d", p.Total)
	}
	man, _ := store.ReadImageCacheManifest()
	man.DatasetID = "old-dataset"
	_ = store.WriteImageCacheManifest(man)

	current := testIndex(101)
	c := startSession(t, store, current, spec)
	if p := c.Progress(); p.Total != 1 {
		t.Fatalf("progress=%+v want 1 outstanding", p)
	}
}

func TestDatasetIDChangeDoesNotInvalidateUnchangedPokemon(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	spec := testImageSpec()
	old := testIndex(50)
	for _, sum := range old {
		seedComplete(t, store, svc, spec, sum)
	}
	c1 := startSession(t, store, old, spec)
	if p := c1.Progress(); p.Total != 0 {
		t.Fatalf("setup total=%d", p.Total)
	}
	man, _ := store.ReadImageCacheManifest()
	man.DatasetID = "new-dataset-id"
	_ = store.WriteImageCacheManifest(man)

	c := startSession(t, store, old, spec)
	if p := c.Progress(); p.Total != 0 {
		t.Fatalf("dataset id change invalidated all: total=%d", p.Total)
	}
}

func TestChangedArtworkInvalidatesOnePokemon(t *testing.T) {
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
	seedArtwork(t, store, index[0].ID, index[0].Slug, testHash(99))
	c := startSession(t, store, index, spec)
	if p := c.Progress(); p.Total != 1 {
		t.Fatalf("progress=%+v want 1", p)
	}
}

func TestWarmStartupNoPerPokemonImageAudit(t *testing.T) {
	TestDeferredImageSessionUntrustedReturnsBeforeAudit(t)
}

func TestWarmStartupNoNativeDirScan(t *testing.T) {
	TestDeferredImageSessionTrustedDoesNoNativeStats(t)
}

func TestWarmStartupNoHTTP(t *testing.T) {
	TestOfflineReadsFromSnapshotOnly(t)
}
