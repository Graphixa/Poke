package cache_test

import (
	"context"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

func TestCompleteDatasetActivates(t *testing.T) {
	TestBootstrapDatasetActivatesSnapshot(t)
}

func TestIncompleteDatasetDoesNotActivate(t *testing.T) {
	TestActivateDatasetRefusesIncomplete(t)
}

func TestOfficialArtworkResolvedLocally(t *testing.T) {
	root := t.TempDir()
	store := cache.NewStore(root, nil)
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	if err := store.StoreNamedSpriteWithMeta(context.Background(), 401, "kricketot", cache.SpriteKindArtwork,
		"https://example/401.png", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", png); err != nil {
		t.Fatal(err)
	}
	hash, path, ready, err := store.NamedAssetMeta(context.Background(), 401, "kricketot", cache.SpriteKindArtwork)
	if err != nil || !ready || hash == "" || path == "" {
		t.Fatalf("meta hash=%q path=%q ready=%v err=%v", hash, path, ready, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("artwork missing on disk: %v", err)
	}
	got, err := store.ReadAssetBytes(path)
	if err != nil || len(got) != len(png) {
		t.Fatalf("ReadAssetBytes err=%v len=%d", err, len(got))
	}
}

func TestSixelCacheExistsForEverySupportedSize(t *testing.T) {
	root := t.TempDir()
	store := cache.NewStore(root, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	bg := color.RGBA{R: 0x31, G: 0x32, B: 0x44, A: 255}
	index := []domain.BrowseEntry{
		{ID: 1, Slug: "bulbasaur"},
		{ID: 25, Slug: "pikachu"},
		{ID: 400, Slug: "staravia"},
		{ID: 401, Slug: "kricketot"},
	}
	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		t.Fatal(err)
	}
	for _, cells := range sprite.FrameSteps() {
		w, h := sprite.FrameSizeFor(cells)
		for _, e := range index {
			key := sprite.NativeCacheKey(sprite.NamedAssetKey(e.ID, e.Slug, "artwork"), "sixel", w, h, bg, 64)
			if err := svc.StoreNative(context.Background(), key, "FRAME"); err != nil {
				t.Fatal(err)
			}
		}
		if err := store.MarkNativeSizeComplete(cells, len(index)); err != nil {
			t.Fatal(err)
		}
		got, err := store.CountNativeSpeciesForCells(cells, "sixel")
		if err != nil {
			t.Fatal(err)
		}
		if got != len(index) {
			t.Fatalf("size %d count=%d want %d", cells, got, len(index))
		}
		if !store.NativeSizeComplete(cells, len(index)) {
			t.Fatalf("size %d not complete", cells)
		}
	}
}

func TestCompleteManifestMatchesFilesystem(t *testing.T) {
	root := t.TempDir()
	store := cache.NewStore(root, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	bg := color.RGBA{R: 0x31, G: 0x32, B: 0x44, A: 255}
	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		t.Fatal(err)
	}
	index := []domain.BrowseEntry{
		{ID: 25, Slug: "pikachu"},
		{ID: 401, Slug: "kricketot"},
	}
	cells := sprite.MaxFrameSize
	w, h := sprite.FrameSizeFor(cells)
	for _, e := range index {
		key := sprite.NativeCacheKey(sprite.NamedAssetKey(e.ID, e.Slug, "artwork"), "sixel", w, h, bg, 64)
		if err := svc.StoreNative(context.Background(), key, "FRAME"); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.MarkNativeSizeComplete(cells, len(index)); err != nil {
		t.Fatal(err)
	}
	man, err := store.ReadNativeManifest()
	if err != nil {
		t.Fatal(err)
	}
	st := man.Sizes[cells]
	if !st.Complete || st.Done != len(index) {
		t.Fatalf("manifest size %d = %+v", cells, st)
	}
	for _, e := range index {
		key := sprite.NativeCacheKey(sprite.NamedAssetKey(e.ID, e.Slug, "artwork"), "sixel", w, h, bg, 64)
		if _, err := store.Native(context.Background(), key); err != nil {
			t.Fatalf("size %d marked complete but pokemon %d missing", cells, e.ID)
		}
	}

	key := sprite.NativeCacheKey(sprite.NamedAssetKey(401, "kricketot", "artwork"), "sixel", w, h, bg, 64)
	path := filepath.Join(root, "native", key+".frame")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	got, err := store.CountNativeSpeciesForCells(cells, "sixel")
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Fatalf("after delete count=%d want 1", got)
	}
	if err := store.MarkNativeSizeComplete(cells, len(index)); err != nil {
		t.Fatal(err)
	}
	if store.NativeSizeComplete(cells, len(index)) {
		t.Fatal("manifest must not stay complete after a missing artefact")
	}
}

func TestCountNativeSpeciesIncludesSHA256Keys(t *testing.T) {
	root := t.TempDir()
	store := cache.NewStore(root, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	bg := color.RGBA{R: 0x31, G: 0x32, B: 0x44, A: 255}
	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		t.Fatal(err)
	}
	sha := "0013d5f503037eb0fe545ea553c781f751df8f385382d4042e0a60a2060fde14"
	w, h := sprite.FrameSizeFor(sprite.MaxFrameSize)
	key := sprite.NativeCacheKey(sha, "sixel", w, h, bg, 64)
	if err := svc.StoreNative(context.Background(), key, "FRAME"); err != nil {
		t.Fatal(err)
	}
	got, err := store.CountNativeSpeciesForCells(sprite.MaxFrameSize, "sixel")
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Fatalf("sha256-keyed frame count=%d want 1 (key=%s)", got, key)
	}
	if err := store.MarkNativeSizeComplete(sprite.MaxFrameSize, 1); err != nil {
		t.Fatal(err)
	}
	if !store.NativeSizeComplete(sprite.MaxFrameSize, 1) {
		t.Fatal("sha256-keyed size must count as complete")
	}
}

func TestOfflineStartupUsesActiveDataset(t *testing.T) {
	TestOfflineReadsAfterBootstrap(t)
}

func TestOfflineBrowseUsesNoNetwork(t *testing.T) {
	TestOfflineReadsFromSnapshotOnly(t)
}

func TestSixelCacheKeyStableAcrossSizes(t *testing.T) {
	bg := color.RGBA{R: 0x31, G: 0x32, B: 0x44, A: 255}
	hash := sprite.NamedAssetKey(401, "kricketot", "artwork")
	for _, cells := range sprite.FrameSteps() {
		w, h := sprite.FrameSizeFor(cells)
		a := sprite.NativeCacheKey(hash, "sixel", w, h, bg, 64)
		b := sprite.NativeCacheKey(hash, "sixel", w, h, bg, 64)
		if a != b {
			t.Fatalf("cells=%d key unstable %q vs %q", cells, a, b)
		}
		parsed, proto, pxW, pxH, bgHash, ver, _, err := sprite.ParseNativeCacheKey(a)
		if err != nil {
			t.Fatal(err)
		}
		wantW, wantH := sprite.PixelSize(w, h)
		if parsed != hash || proto != "sixel" || pxW != wantW || pxH != wantH || bgHash != "313244" || ver != sprite.AlgorithmVersion {
			t.Fatalf("cells=%d parsed hash=%s proto=%s %dx%d bg=%s ver=%s", cells, parsed, proto, pxW, pxH, bgHash, ver)
		}
	}
}
