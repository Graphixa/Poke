package cache_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Graphixa/Poke/internal/cache"
	"github.com/Graphixa/Poke/internal/render/sprite"
)

func TestNativeGzipRoundTrip(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	key := sprite.NativeCacheKey("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "sixel", 32, 16, sprite.DefaultBackground, 64)
	raw := []byte("DCS\x1bP\x1bq\x1b\"1;2;24;24\x1bP\x1b\\")
	if err := store.StoreNative(context.Background(), key, raw); err != nil {
		t.Fatal(err)
	}
	got, err := store.Native(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("round-trip mismatch: got %q want %q", got, raw)
	}
	path := filepath.Join(root, "native", key+".sixel.gz")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("expected non-empty gzip file")
	}
}

func TestNativeCorruptGzipIsCacheMiss(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	key := sprite.NativeCacheKey("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "sixel", 32, 16, sprite.DefaultBackground, 64)
	path := filepath.Join(root, "native", key+".sixel.gz")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not-gzip"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Native(context.Background(), key); err != cache.ErrNotFound {
		t.Fatalf("corrupt gzip err=%v want ErrNotFound", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("corrupt artefact should be quarantined/removed")
	}
}

func TestStoreNativeAtomicNoPartialFile(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	key := sprite.NativeCacheKey("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "sixel", 40, 20, sprite.DefaultBackground, 64)
	if err := store.StoreNative(context.Background(), key, []byte("SIXEL")); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "native")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == key+".sixel.gz" {
			continue
		}
		if filepath.Ext(e.Name()) == ".gz" || e.Name() == ".tmp-" {
			t.Fatalf("unexpected partial file %q", e.Name())
		}
	}
}

func TestGzipDecompressMatchesRaw(t *testing.T) {
	raw := []byte("raw-sixel-bytes")
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, _ = w.Write(raw)
	_ = w.Close()
	// sanity: store path uses same stdlib gzip
	store := cache.NewStore(t.TempDir(), fixedClock{t: time.Now()})
	key := sprite.NativeCacheKey("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "sixel", 24, 12, sprite.DefaultBackground, 64)
	_ = store.StoreNative(context.Background(), key, raw)
	got, err := store.Native(context.Background(), key)
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("got=%v err=%v", got, err)
	}
}
