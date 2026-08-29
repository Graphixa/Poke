package cache

import (
	"context"
	"testing"

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
