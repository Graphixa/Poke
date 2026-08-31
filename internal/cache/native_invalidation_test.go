package cache_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

func TestEnsureNativeManifestPurgesIncompatibleFrames(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
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
}
