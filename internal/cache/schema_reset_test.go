package cache_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/domain"
)

func TestIncompatibleSchemaReset(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	id := "2026-08-29T000000"
	if err := store.EnsureDatasetDir(id); err != nil {
		t.Fatal(err)
	}
	if err := store.StoreBrowse(id, []domain.BrowseEntry{{ID: 1, Slug: "a", Name: "A"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteDatasetManifest(cache.DatasetManifest{
		SchemaVersion:  2,
		DatasetVersion: id,
		PokemonCount:   1,
		Complete:       true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.ActivateDataset(id); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureCompatibleDataset(); err != nil {
		t.Fatal(err)
	}
	if store.HasActiveDataset() {
		t.Fatal("v2 active dataset must be reset")
	}
	if _, err := os.Stat(filepath.Join(root, "active.json")); !os.IsNotExist(err) {
		t.Fatal("active.json must be removed")
	}
}

func TestV3SchemaPreserved(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	api := &fakeAPI{
		list: []domain.SpeciesSummary{{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}},
		pokemon: map[int]domain.Pokemon{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", Stats: domain.Stats{Speed: 90}},
		},
		species: map[int]domain.Species{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", DefaultPokemonID: 25},
		},
	}
	svc := cache.NewService(store, api, testConfig(), clock)
	if err := svc.BootstrapDataset(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	if !svc.HasActiveDataset() {
		t.Fatal("v3 dataset must remain active")
	}
}
