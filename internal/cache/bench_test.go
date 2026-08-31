package cache

import (
	"context"
	"testing"

	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/dataset"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

func benchDataset(b *testing.B) (*Store, string) {
	b.Helper()
	root := b.TempDir()
	store := NewStore(root, nil)
	id := "bench-dataset"
	_ = store.EnsureDatasetDir(id)
	builder := dataset.NewBuilder()
	for i := 1; i <= 1025; i++ {
		builder.Pokemon[i] = dataset.PokemonFromDomain(domain.Pokemon{
			ID: i, Slug: "p", DisplayName: "P", SpeciesID: i,
			Stats: domain.Stats{Speed: i % 255},
		})
		builder.Species[i] = dataset.SpeciesFromDomain(domain.Species{
			ID: i, Slug: "p", DisplayName: "P", DefaultPokemonID: i,
		})
		builder.Learnsets[i] = []dataset.DatasetLearn{{Level: 1, MoveID: 1}}
		builder.Moves[1] = dataset.DatasetMove{ID: 1, Slug: "pound", DisplayName: "Pound", PP: 35}
		builder.Evolutions[i] = dataset.EvolutionFromDomain(domain.EvolutionTree{
			Root: domain.EvolutionNode{SpeciesID: i, DisplayName: "P"},
		})
		builder.Encounters[i] = dataset.EncountersFromDomain([]domain.Encounter{{Area: "route", MinimumLevel: 1}})
	}
	if err := store.StoreAggregateBuilder(id, builder); err != nil {
		b.Fatal(err)
	}
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
	return store, id
}

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

func BenchmarkLoadPokemonDataset(b *testing.B) {
	store, id := benchDataset(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.LoadAggregatePokemon(id); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoadSpeciesDataset(b *testing.B) {
	store, id := benchDataset(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.LoadAggregateSpecies(id); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoadMovesDataset(b *testing.B) {
	store, id := benchDataset(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.LoadAggregateMoves(id); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoadEvolutionDataset(b *testing.B) {
	store, id := benchDataset(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.LoadAggregateEvolutions(id); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoadEncounterDataset(b *testing.B) {
	store, id := benchDataset(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.LoadAggregateEncounters(id); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPreloadDataset(b *testing.B) {
	store, _ := benchDataset(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.invalidateLoadedSnapshot()
		if err := store.PreloadDataset(); err != nil {
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

func BenchmarkStoreNativeMany(b *testing.B) {
	root := b.TempDir()
	store := NewStore(root, nil)
	svc := NewService(store, nil, config.Config{}, nil)
	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	bg := sprite.DefaultBackground
	w, h := sprite.FrameSizeFor(sprite.MaxFrameSize)
	ResetNativeDirectoryScanCount()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := sprite.NativeCacheKey("bench-store-"+string(rune(i)), "sixel", w, h, bg, 64)
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
	nkey := sprite.NativeCacheKey("hash", "sixel", 40, 20, sprite.DefaultBackground, 64)
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
