package cache_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/domain"
)

func TestIncompatibleSchemaDestructiveReset(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)

	legacyPaths := []string{
		filepath.Join("pokemon", "25.json"),
		filepath.Join("species", "25.json"),
		filepath.Join("moves", "1.json"),
		filepath.Join("evolution", "10.json"),
		filepath.Join("encounters", "25.json"),
		filepath.Join("pokemon", "025-pikachu", "dossier.json"),
		"manifest.json",
	}
	for _, rel := range legacyPaths {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	id := "2026-08-29T000000"
	if err := store.EnsureDatasetDir(id); err != nil {
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
		t.Fatal("incompatible dataset must be reset")
	}
	for _, rel := range legacyPaths {
		if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
			t.Fatalf("legacy file remains: %s", rel)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "active.json")); !os.IsNotExist(err) {
		t.Fatal("active.json must be removed")
	}
	if _, err := os.Stat(filepath.Join(root, "datasets", id)); !os.IsNotExist(err) {
		t.Fatal("incompatible dataset dir must be removed")
	}
}

func TestDatasetSchemaPreserved(t *testing.T) {
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
	if err := cache.EnsureCompatibleCache(store); err != nil {
		t.Fatal(err)
	}
	svc := cache.NewService(store, api, testConfig(), clock)
	if err := svc.BootstrapDataset(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if !svc.HasActiveDataset() {
		t.Fatal("active dataset must remain active")
	}
}
