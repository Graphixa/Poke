package cache_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Graphixa/Poke/internal/cache"
	"github.com/Graphixa/Poke/internal/config"
	"github.com/Graphixa/Poke/internal/domain"
	"github.com/Graphixa/Poke/internal/render/sprite"
)

func TestEnsureNativeManifestPurgesIncompatibleFrames(t *testing.T) {
	root := t.TempDir()
	store := cache.NewStore(root, nil)
	bg := sprite.DefaultBackground
	w, h := sprite.FrameSizeFor(40)

	oldKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef-image-sixel-256x128-313244-v12"
	oldPath := filepath.Join(root, "native", oldKey+".frame")
	if err := os.MkdirAll(filepath.Dir(oldPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath, []byte("OLD"), 0o600); err != nil {
		t.Fatal(err)
	}

	newKey := sprite.NativeCacheKey("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "sixel", w, h, bg, 64)
	if err := store.StoreNative(context.Background(), newKey, []byte("NEW")); err != nil {
		t.Fatal(err)
	}

	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatal("expected legacy v12 frame deleted")
	}
	if _, err := os.Stat(filepath.Join(root, "native", newKey+".sixel.gz")); err != nil {
		t.Fatalf("current frame missing: %v", err)
	}
	man, err := store.ReadNativeManifest()
	if err != nil {
		t.Fatal(err)
	}
	if man.Version != sprite.AlgorithmVersion || man.Protocol != "sixel" {
		t.Fatalf("manifest=%+v", man)
	}
}

func TestEnsureNativeManifestNoRescanWhenUnchanged(t *testing.T) {
	root := t.TempDir()
	store := cache.NewStore(root, nil)
	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		t.Fatal(err)
	}
	cache.ResetNativeDirectoryScanCount()
	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		t.Fatal(err)
	}
	if n := cache.NativeDirectoryScanCount(); n != 0 {
		t.Fatalf("EnsureNativeManifest scans=%d want 0 when version matches", n)
	}
}

func TestCountNativeSpeciesForCells(t *testing.T) {
	root := t.TempDir()
	store := cache.NewStore(root, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	bg := sprite.DefaultBackground
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
	got, err := store.CountNativeSpeciesForCells(cells, "sixel")
	if err != nil {
		t.Fatal(err)
	}
	if got != len(index) {
		t.Fatalf("count=%d want %d", got, len(index))
	}
}

func TestStoreNativeDoesNotUpdateCompletionManifest(t *testing.T) {
	root := t.TempDir()
	store := cache.NewStore(root, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		t.Fatal(err)
	}
	bg := sprite.DefaultBackground
	w, h := sprite.FrameSizeFor(sprite.MaxFrameSize)
	key := sprite.NativeCacheKey("bench", "sixel", w, h, bg, 64)
	cache.ResetNativeDirectoryScanCount()
	if err := svc.StoreNative(context.Background(), key, "SIXEL"); err != nil {
		t.Fatal(err)
	}
	if n := cache.NativeDirectoryScanCount(); n != 0 {
		t.Fatalf("StoreNative scanned native dir %d times", n)
	}
	if !store.HasNativeFile(key) {
		t.Fatal("expected frame on disk")
	}
}
