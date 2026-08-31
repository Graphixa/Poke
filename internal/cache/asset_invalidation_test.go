package cache_test

import (
	"context"
	"testing"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

func TestStoreContentAssetSameHashDoesNotInvalidateImageState(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	spec := testImageSpec()
	index := testIndex(1)
	seedComplete(t, store, svc, spec, index[0])
	startSession(t, store, index, spec)
	manBefore, err := store.ReadImageCacheManifest()
	if err != nil || !manBefore.Pokemon[1].Complete {
		t.Fatalf("manifest=%+v err=%v", manBefore, err)
	}
	hash := testHash(1)
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 1}
	if err := store.StoreContentAsset(1, cache.SpriteKindArtwork, "", hash, png); err != nil {
		t.Fatal(err)
	}
	manAfter, err := store.ReadImageCacheManifest()
	if err != nil || !manAfter.Pokemon[1].Complete {
		t.Fatalf("same hash invalidated manifest: %+v err=%v", manAfter, err)
	}
}

func TestStoreContentAssetChangedHashInvalidatesOnlyPokemon(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	spec := testImageSpec()
	index := testIndex(2)
	seedComplete(t, store, svc, spec, index[0])
	seedComplete(t, store, svc, spec, index[1])
	startSession(t, store, index, spec)
	seedArtwork(t, store, index[0].ID, index[0].Slug, testHash(99))
	man, err := store.ReadImageCacheManifest()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := man.Pokemon[1]; ok {
		t.Fatal("pokemon 1 should be invalidated")
	}
	if !man.Pokemon[2].Complete {
		t.Fatal("pokemon 2 should remain complete")
	}
}

func TestDeferredSessionChangedArtworkCreatesOneOutstanding(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	spec := testImageSpec()
	index := testIndex(2)
	seedComplete(t, store, svc, spec, index[0])
	seedComplete(t, store, svc, spec, index[1])
	startSession(t, store, index, spec)
	seedArtwork(t, store, index[0].ID, index[0].Slug, testHash(99))
	c, err := cache.StartImageCacheSessionDeferred(store, index, spec)
	if err != nil {
		t.Fatal(err)
	}
	if p := c.Progress(); p.Total != 1 {
		t.Fatalf("progress=%+v want total=1", p)
	}
	ids := c.OutstandingIDs()
	if len(ids) != 1 || ids[0] != index[0].ID {
		t.Fatalf("outstanding=%v want [%d]", ids, index[0].ID)
	}
}

func TestChangedArtworkDoesNotInvalidateOtherPokemon(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	spec := testImageSpec()
	index := testIndex(2)
	seedComplete(t, store, svc, spec, index[0])
	seedComplete(t, store, svc, spec, index[1])
	startSession(t, store, index, spec)
	seedArtwork(t, store, index[0].ID, index[0].Slug, testHash(99))
	c := startSession(t, store, index, spec)
	if p := c.Progress(); p.Total != 1 {
		t.Fatalf("progress=%+v want 1 outstanding", p)
	}
	man, _ := store.ReadImageCacheManifest()
	if !man.Pokemon[2].Complete {
		t.Fatal("pokemon 2 should remain complete in manifest")
	}
}

func TestHashInvalidationDuringActiveSessionNotRestoredByFlush(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	spec := testImageSpec()
	index := testIndex(2)
	seedComplete(t, store, svc, spec, index[0])
	seedComplete(t, store, svc, spec, index[1])
	c := startSession(t, store, index, spec)
	if p := c.Progress(); p.Total != 0 {
		t.Fatalf("setup progress=%+v want idle session", p)
	}
	seedArtwork(t, store, index[1].ID, index[1].Slug, testHash(99))
	if err := c.Flush(); err != nil {
		t.Fatal(err)
	}
	man, err := store.ReadImageCacheManifest()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := man.Pokemon[2]; ok {
		t.Fatal("coordinator flush must not restore hash-invalidated pokemon 2")
	}
	if !man.Pokemon[1].Complete {
		t.Fatal("pokemon 1 should remain complete after flush")
	}
}

func TestChangedArtworkRebuildsRequiredNativeSizes(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	spec := testImageSpec()
	index := testIndex(1)
	oldHash := testHash(1)
	seedComplete(t, store, svc, spec, index[0])
	bg := spec.BackgroundRGBA()
	for _, cells := range spec.RequiredSizes {
		w, h := sprite.FrameSizeFor(cells)
		key := sprite.NativeCacheKey(oldHash, spec.Protocol, w, h, bg, 64)
		if !store.HasNativeFile(key) {
			t.Fatalf("missing native for old hash size=%d", cells)
		}
	}
	newHash := testHash(99)
	seedArtwork(t, store, index[0].ID, index[0].Slug, newHash)
	for _, cells := range spec.RequiredSizes {
		w, h := sprite.FrameSizeFor(cells)
		oldKey := sprite.NativeCacheKey(oldHash, spec.Protocol, w, h, bg, 64)
		if store.HasNativeFile(oldKey) {
			t.Fatalf("old native size=%d should be removed after hash change", cells)
		}
	}
	c := startSession(t, store, index, spec)
	if p := c.Progress(); p.Total != 1 {
		t.Fatalf("progress=%+v want 1 outstanding native rebuild", p)
	}
	for _, cells := range spec.RequiredSizes {
		w, h := sprite.FrameSizeFor(cells)
		newKey := sprite.NativeCacheKey(newHash, spec.Protocol, w, h, bg, 64)
		if store.HasNativeFile(newKey) {
			t.Fatalf("new native size=%d should not exist yet", cells)
		}
	}
	_ = context.Background()
}
