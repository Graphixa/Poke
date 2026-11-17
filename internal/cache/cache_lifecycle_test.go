package cache_test

import (
	"context"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/Graphixa/Poke/internal/cache"
	"github.com/Graphixa/Poke/internal/config"
	"github.com/Graphixa/Poke/internal/render/sprite"
)

func TestCacheLifecycleBucketSwitchRetainsArtefacts(t *testing.T) {
	root := t.TempDir()
	store := cache.NewStore(root, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	bg := color.RGBA{R: 0x31, G: 0x32, B: 0x44, A: 255}
	hash := sprite.NamedAssetKey(25, "pikachu", "artwork")

	var keys []string
	for _, cells := range []int{32, 40} {
		w, h := sprite.FrameSizeFor(cells)
		key := sprite.NativeCacheKey(hash, "sixel", w, h, bg, 64)
		if err := svc.StoreNative(context.Background(), key, "FRAME"); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, key)
	}
	for _, key := range keys {
		if !store.HasNativeFile(key) {
			t.Fatalf("missing key %s", key)
		}
	}
}

func TestCacheLifecyclePaletteChangeDistinctKeys(t *testing.T) {
	bg := color.RGBA{R: 0x31, G: 0x32, B: 0x44, A: 255}
	hash := sprite.NamedAssetKey(1, "bulbasaur", "artwork")
	w, h := sprite.FrameSizeFor(32)
	k64 := sprite.NativeCacheKey(hash, "sixel", w, h, bg, 64)
	k128 := sprite.NativeCacheKey(hash, "sixel", w, h, bg, 128)
	if k64 == k128 {
		t.Fatalf("palette must change cache identity: %q", k64)
	}
}

func TestCacheLifecycleUpgradePurgesIncompatible(t *testing.T) {
	root := t.TempDir()
	store := cache.NewStore(root, nil)
	oldPath := filepath.Join(root, "native", "legacy.frame")
	if err := os.MkdirAll(filepath.Dir(oldPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath, []byte("OLD"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatal("upgrade must purge incompatible native artefacts")
	}
}

func TestCacheLifecycleCorruptGzipTreatedAsMiss(t *testing.T) {
	root := t.TempDir()
	store := cache.NewStore(root, nil)
	bg := color.RGBA{R: 0x31, G: 0x32, B: 0x44, A: 255}
	w, h := sprite.FrameSizeFor(32)
	key := sprite.NativeCacheKey(sprite.NamedAssetKey(25, "pikachu", "artwork"), "sixel", w, h, bg, 64)
	path := filepath.Join(root, "native", key+".sixel.gz")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not-gzip"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Native(context.Background(), key); err == nil {
		t.Fatal("corrupt gzip must not succeed as cache hit")
	}
}
