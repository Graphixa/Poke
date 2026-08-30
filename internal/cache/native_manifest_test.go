package cache_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

func TestNativeSizeCompleteUsesManifestOnly(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		t.Fatal(err)
	}
	if err := store.InitNativeSizePhase(sprite.MaxFrameSize, 2); err != nil {
		t.Fatal(err)
	}
	m, err := store.ReadNativeManifest()
	if err != nil {
		t.Fatal(err)
	}
	entry := m.Sizes[sprite.MaxFrameSize]
	entry.Done = 2
	entry.Total = 2
	entry.Complete = true
	m.Sizes[sprite.MaxFrameSize] = entry
	if err := store.WriteNativeManifest(m); err != nil {
		t.Fatal(err)
	}
	// No .frame files on disk; fast path must trust manifest counts.
	if !store.NativeSizeComplete(sprite.MaxFrameSize, 2) {
		t.Fatal("expected complete from manifest without disk scan")
	}
}

func TestNativeManifestSizeComplete(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)

	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		t.Fatal(err)
	}
	if err := store.InitNativeSizePhase(sprite.MaxFrameSize, 2); err != nil {
		t.Fatal(err)
	}
	if store.NativeSizeComplete(sprite.MaxFrameSize, 2) {
		t.Fatal("expected incomplete before frames")
	}

	bg := sprite.DefaultBackground
	key := sprite.NativeCacheKey("named-25-pikachu-artwork", "sixel", 40, 20, bg)
	if err := svc.StoreNative(context.Background(), key, "FRAME"); err != nil {
		t.Fatal(err)
	}
	key2 := sprite.NativeCacheKey("named-26-raichu-artwork", "sixel", 40, 20, bg)
	if err := svc.StoreNative(context.Background(), key2, "FRAME2"); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkNativeSizeComplete(sprite.MaxFrameSize, 2); err != nil {
		t.Fatal(err)
	}
	if !store.NativeSizeComplete(sprite.MaxFrameSize, 2) {
		t.Fatal("expected complete after two frames")
	}
	if !svc.NativeSizeComplete(sprite.MaxFrameSize, 2) {
		t.Fatal("service wrapper expected complete")
	}
}

func TestInvalidateNativeSpeciesUpdatesManifest(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)

	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		t.Fatal(err)
	}
	if err := store.InitNativeSizePhase(sprite.MaxFrameSize, 1); err != nil {
		t.Fatal(err)
	}
	bg := sprite.DefaultBackground
	key := sprite.NativeCacheKey("named-25-pikachu-artwork", "sixel", 40, 20, bg)
	if err := svc.StoreNative(context.Background(), key, "FRAME"); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkNativeSizeComplete(sprite.MaxFrameSize, 1); err != nil {
		t.Fatal(err)
	}
	if err := svc.InvalidateNativeSpecies(25); err != nil {
		t.Fatal(err)
	}
	if store.NativeSizeComplete(sprite.MaxFrameSize, 1) {
		t.Fatal("expected incomplete after invalidate")
	}
	if _, err := store.Native(context.Background(), key); err == nil {
		t.Fatal("expected frame deleted")
	}
}

func TestNativeSizeCompleteStaleTotal(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		t.Fatal(err)
	}
	m, err := store.ReadNativeManifest()
	if err != nil {
		t.Fatal(err)
	}
	entry := m.Sizes[sprite.MaxFrameSize]
	entry.Done = 1025
	entry.Total = 1025
	entry.Complete = true
	m.Sizes[sprite.MaxFrameSize] = entry
	if err := store.WriteNativeManifest(m); err != nil {
		t.Fatal(err)
	}
	if store.NativeSizeComplete(sprite.MaxFrameSize, 1030) {
		t.Fatal("expected incomplete when index total grew beyond manifest total")
	}
	if !store.NativeSizeComplete(sprite.MaxFrameSize, 1025) {
		t.Fatal("expected complete when index total matches manifest total")
	}
}

func TestNativeSizeCompleteHotPathNoScan(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		t.Fatal(err)
	}
	m, err := store.ReadNativeManifest()
	if err != nil {
		t.Fatal(err)
	}
	entry := m.Sizes[sprite.MaxFrameSize]
	entry.Done = 100
	entry.Total = 100
	m.Sizes[sprite.MaxFrameSize] = entry
	if err := store.WriteNativeManifest(m); err != nil {
		t.Fatal(err)
	}
	cache.ResetNativeDirectoryScanCount()
	for i := 0; i < 1000; i++ {
		_ = store.NativeSizeComplete(sprite.MaxFrameSize, 100)
	}
	if n := cache.NativeDirectoryScanCount(); n != 0 {
		t.Fatalf("NativeSizeComplete scans=%d want 0", n)
	}
}

func TestMarkAllNativeSizesCompleteSingleScan(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		t.Fatal(err)
	}
	bg := sprite.DefaultBackground
	w, h := sprite.FrameSizeFor(sprite.MaxFrameSize)
	for i := 0; i < 3; i++ {
		key := sprite.NativeCacheKey(fmt.Sprintf("all-scan-%d", i), "sixel", w, h, bg)
		if err := svc.StoreNative(context.Background(), key, "F"); err != nil {
			t.Fatal(err)
		}
	}
	cache.ResetNativeDirectoryScanCount()
	sizes := sprite.FrameSteps()
	if err := store.MarkAllNativeSizesComplete(sizes, 3); err != nil {
		t.Fatal(err)
	}
	if n := cache.NativeDirectoryScanCount(); n != 1 {
		t.Fatalf("MarkAllNativeSizesComplete scans=%d want 1", n)
	}
}

func TestEnsureNativeManifestSingleDirectoryPass(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		t.Fatal(err)
	}
	bg := sprite.DefaultBackground
	w, h := sprite.FrameSizeFor(sprite.MaxFrameSize)
	for i := 0; i < 20; i++ {
		hash := sprite.NativeCacheKey(fmt.Sprintf("hash%02d", i), "sixel", w, h, bg)
		if err := svc.StoreNative(context.Background(), hash, "F"); err != nil {
			t.Fatal(err)
		}
	}
	cache.ResetNativeDirectoryScanCount()
	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		t.Fatal(err)
	}
	if n := cache.NativeDirectoryScanCount(); n != 0 {
		t.Fatalf("EnsureNativeManifest scans=%d want 0 (trust matching manifest)", n)
	}
}

func TestStoreNativeNoPerFrameDirectoryScan(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		t.Fatal(err)
	}
	if err := store.InitNativeSizePhase(sprite.MaxFrameSize, 100); err != nil {
		t.Fatal(err)
	}
	cache.ResetNativeDirectoryScanCount()
	bg := sprite.DefaultBackground
	w, h := sprite.FrameSizeFor(sprite.MaxFrameSize)
	for i := 0; i < 100; i++ {
		key := sprite.NativeCacheKey(fmt.Sprintf("store-test-%03d", i), "sixel", w, h, bg)
		if err := svc.StoreNative(context.Background(), key, "FRAME"); err != nil {
			t.Fatal(err)
		}
	}
	if n := cache.NativeDirectoryScanCount(); n != 0 {
		t.Fatalf("StoreNative scans=%d want 0 during stores", n)
	}
	m, err := store.ReadNativeManifest()
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Sizes[sprite.MaxFrameSize].Done; got != 100 {
		t.Fatalf("manifest done=%d want 100", got)
	}
	key := sprite.NativeCacheKey("store-test-dup", "sixel", w, h, bg)
	if err := svc.StoreNative(context.Background(), key, "A"); err != nil {
		t.Fatal(err)
	}
	if err := svc.StoreNative(context.Background(), key, "B"); err != nil {
		t.Fatal(err)
	}
	m, err = store.ReadNativeManifest()
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Sizes[sprite.MaxFrameSize].Done; got != 101 {
		t.Fatalf("after overwrite manifest done=%d want 101", got)
	}
}
