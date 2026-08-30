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

func TestDeferredProgressCountsOnlyOutstanding(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	spec := testImageSpec()
	index := testIndex(5)
	for i := 0; i < 3; i++ {
		seedComplete(t, store, svc, spec, index[i])
	}
	c1 := startSession(t, store, index, spec)
	if p := c1.Progress(); p.Total != 2 {
		t.Fatalf("setup progress=%+v want 2 outstanding", p)
	}
	cache.ResetNamedAssetMetaCount()
	c, err := cache.StartImageCacheSessionDeferred(store, index, spec)
	if err != nil {
		t.Fatal(err)
	}
	p := c.Progress()
	if p.Total != 2 || p.Done != 0 || !p.Active {
		t.Fatalf("progress=%+v want done=0 total=2 active", p)
	}
	if n := cache.NamedAssetMetaCount(); n != 0 {
		t.Fatalf("deferred startup audited assets: %d lookups", n)
	}
}

func TestDeferredTrustedReconcileInvalidatesChangedSourceHash(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	spec := testImageSpec()
	index := testIndex(2)
	for _, sum := range index {
		seedComplete(t, store, svc, spec, sum)
	}
	startSession(t, store, index, spec)
	c, err := cache.StartImageCacheSessionDeferred(store, index, spec)
	if err != nil {
		t.Fatal(err)
	}
	if p := c.Progress(); p.Total != 0 {
		t.Fatalf("trusted deferred progress=%+v", p)
	}
	seedArtwork(t, store, index[0].ID, index[0].Slug, testHash(99))
	c.ReconcileImageCacheSession(index)
	if p := c.Progress(); p.Total != 1 || p.Done != 0 || !p.Active {
		t.Fatalf("after reconcile progress=%+v want done=0 total=1 active", p)
	}
	if len(c.OutstandingIDs()) != 1 {
		t.Fatalf("outstanding=%v", c.OutstandingIDs())
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

func writeCompleteManifest(t *testing.T, store *cache.Store, spec cache.ImageCacheSpec, n int) {
	t.Helper()
	man := cache.ImageCacheManifest{
		Version:      cache.ImageCacheManifestVersion,
		SpecID:       spec.ID(),
		PokemonCount: n,
		Pokemon:      map[int]cache.PokemonImageCacheState{},
	}
	for i := 1; i <= n; i++ {
		man.Pokemon[i] = cache.PokemonImageCacheState{
			SpeciesID: i, SourceHash: testHash(i), SpecID: spec.ID(), Complete: true,
		}
	}
	if err := store.WriteImageCacheManifest(man); err != nil {
		t.Fatal(err)
	}
}

func TestDeferredReconcileOneNewPokemonRemainsOneTotal(t *testing.T) {
	store := cache.NewStore(t.TempDir(), nil)
	spec := testImageSpec()
	writeCompleteManifest(t, store, spec, 100)
	index := testIndex(101)
	c, err := cache.StartImageCacheSessionDeferred(store, index, spec)
	if err != nil {
		t.Fatal(err)
	}
	if p := c.Progress(); p.Total != 1 || p.Done != 0 {
		t.Fatalf("before reconcile progress=%+v want 0/1", p)
	}
	c.ReconcileImageCacheSession(index)
	if p := c.Progress(); p.Total != 1 {
		t.Fatalf("after reconcile progress=%+v want total=1", p)
	}
	if len(c.OutstandingIDs()) != 1 || c.OutstandingIDs()[0] != 101 {
		t.Fatalf("outstanding=%v want [101]", c.OutstandingIDs())
	}
}

func TestDeferredReconcileThreeOutstandingRemainsThreeTotal(t *testing.T) {
	store := cache.NewStore(t.TempDir(), nil)
	index := testIndex(3)
	spec := testImageSpec()
	c, err := cache.StartImageCacheSessionDeferred(store, index, spec)
	if err != nil {
		t.Fatal(err)
	}
	if p := c.Progress(); p.Total != 3 || p.Done != 0 {
		t.Fatalf("before reconcile progress=%+v want 0/3", p)
	}
	c.ReconcileImageCacheSession(index)
	if p := c.Progress(); p.Total != 3 || p.Done != 0 {
		t.Fatalf("after reconcile progress=%+v want 0/3", p)
	}
}

func TestDeferredReconcileAlreadyPresentOutstandingCountsDoneWithinSession(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	spec := testImageSpec()
	index := testIndex(1)
	c, err := cache.StartImageCacheSessionDeferred(store, index, spec)
	if err != nil {
		t.Fatal(err)
	}
	if p := c.Progress(); p.Total != 1 || p.Done != 0 {
		t.Fatalf("before seed progress=%+v want 0/1", p)
	}
	seedComplete(t, store, svc, spec, index[0])
	c.ReconcileImageCacheSession(index)
	if p := c.Progress(); p.Total != 1 || p.Done != 1 || p.Active {
		t.Fatalf("after reconcile progress=%+v want 1/1 inactive", p)
	}
	if len(c.OutstandingIDs()) != 0 {
		t.Fatalf("outstanding=%v want none", c.OutstandingIDs())
	}
}

func TestDeferredProgressNeverExceedsSessionTotal(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	spec := testImageSpec()
	index := testIndex(5)
	for i := 0; i < 3; i++ {
		seedComplete(t, store, svc, spec, index[i])
	}
	c, err := cache.StartImageCacheSessionDeferred(store, index, spec)
	if err != nil {
		t.Fatal(err)
	}
	total := c.Progress().Total
	c.ReconcileImageCacheSession(index)
	p := c.Progress()
	if p.Done > p.Total || p.Total != total {
		t.Fatalf("progress=%+v sessionTotal=%d", p, total)
	}
}

func TestDeferredProgressTotalNeverChangesAfterStart(t *testing.T) {
	store := cache.NewStore(t.TempDir(), nil)
	spec := testImageSpec()
	writeCompleteManifest(t, store, spec, 100)
	index := testIndex(101)
	c, err := cache.StartImageCacheSessionDeferred(store, index, spec)
	if err != nil {
		t.Fatal(err)
	}
	before := c.Progress().Total
	c.ReconcileImageCacheSession(index)
	after := c.Progress().Total
	if before != 1 || after != 1 {
		t.Fatalf("total changed: before=%d after=%d want 1", before, after)
	}
}
