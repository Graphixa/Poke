package cache

import (
	"context"
	"testing"

	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

func BenchmarkLoadBrowse(b *testing.B) {
	root := b.TempDir()
	store := NewStore(root, nil)
	id := "bench"
	_ = store.EnsureDatasetDir(id)
	entries := make([]domain.BrowseEntry, 1025)
	for i := range entries {
		entries[i] = domain.BrowseEntry{ID: i + 1, Slug: "p", Name: "P"}
	}
	if err := store.StoreBrowse(id, entries); err != nil {
		b.Fatal(err)
	}
	if err := store.WriteDatasetManifest(DatasetManifest{
		SchemaVersion: DatasetSchemaVersion, DatasetVersion: id, Complete: true, PokemonCount: len(entries),
	}); err != nil {
		b.Fatal(err)
	}
	if err := store.ActivateDataset(id); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.LoadBrowse(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNamedAssetMeta(b *testing.B) {
	root := b.TempDir()
	store := NewStore(root, nil)
	_ = store.StoreNamedSpriteWithMeta(context.Background(), 25, "pikachu", SpriteKindArtwork, "u", "deadbeef", []byte("png"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, ready, err := store.NamedAssetMeta(context.Background(), 25, "pikachu", SpriteKindArtwork)
		if err != nil || !ready {
			b.Fatal(err)
		}
	}
}

func BenchmarkNativeSizeComplete(b *testing.B) {
	root := b.TempDir()
	store := NewStore(root, nil)
	ctx := context.Background()
	bg := sprite.DefaultBackground
	w, h := sprite.FrameSizeFor(sprite.MaxFrameSize)
	for i := 0; i < 500; i++ {
		key := sprite.NativeCacheKey("bench-"+string(rune(i)), "sixel", w, h, bg)
		if err := store.StoreNative(ctx, key, []byte("SIXEL")); err != nil {
			b.Fatal(err)
		}
	}
	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		b.Fatal(err)
	}
	m, err := store.ReadNativeManifest()
	if err != nil {
		b.Fatal(err)
	}
	entry := m.Sizes[sprite.MaxFrameSize]
	entry.Done = 500
	entry.Total = 500
	m.Sizes[sprite.MaxFrameSize] = entry
	if err := store.WriteNativeManifest(m); err != nil {
		b.Fatal(err)
	}
	ResetNativeDirectoryScanCount()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !store.NativeSizeComplete(sprite.MaxFrameSize, 500) {
			b.Fatal("expected complete")
		}
	}
	if NativeDirectoryScanCount() != 0 {
		b.Fatalf("NativeSizeComplete scanned disk during benchmark")
	}
}

func BenchmarkStoreNativeMany(b *testing.B) {
	root := b.TempDir()
	store := NewStore(root, nil)
	svc := NewService(store, nil, config.Config{}, nil)
	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		b.Fatal(err)
	}
	if err := store.InitNativeSizePhase(sprite.MaxFrameSize, b.N); err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	bg := sprite.DefaultBackground
	w, h := sprite.FrameSizeFor(sprite.MaxFrameSize)
	ResetNativeDirectoryScanCount()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := sprite.NativeCacheKey("bench-store-"+string(rune(i)), "sixel", w, h, bg)
		if err := svc.StoreNative(ctx, key, "SIXEL"); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	if NativeDirectoryScanCount() != 0 {
		b.Fatalf("StoreNative scanned disk %d times during benchmark", NativeDirectoryScanCount())
	}
}

func BenchmarkRenderArtifactRoundTrip(b *testing.B) {
	root := b.TempDir()
	store := NewStore(root, nil)
	ctx := context.Background()
	nkey := sprite.NativeCacheKey("hash", "sixel", 40, 20, sprite.DefaultBackground)
	if err := store.StoreNative(ctx, nkey, []byte("SIXEL")); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.Native(ctx, nkey); err != nil {
			b.Fatal(err)
		}
	}
}
