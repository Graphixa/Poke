package cache_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Graphixa/Poke/internal/cache"
	"github.com/Graphixa/Poke/internal/domain"
)

func TestIncompatibleSchemaDeactivatesWithoutDelete(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)

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
	// Simulate a previously activated incompatible snapshot (Activate refuses wrong schema).
	ptr, _ := json.Marshal(map[string]any{"schema_version": 2, "dataset_id": id})
	if err := os.WriteFile(filepath.Join(root, "active.json"), ptr, 0o600); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "datasets", id, "keep-me.txt")
	if err := os.WriteFile(marker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := store.EnsureCompatibleDataset(); err != nil {
		t.Fatal(err)
	}
	if store.HasActiveDataset() {
		t.Fatal("incompatible dataset must not stay active")
	}
	if _, err := os.Stat(filepath.Join(root, "active.json")); !os.IsNotExist(err) {
		t.Fatal("active.json must be removed")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("dataset snapshot must be preserved: %v", err)
	}
	notice, err := store.ReadDatasetNotice()
	if err != nil || !notice.NeedsUpdate || notice.DatasetID != id {
		t.Fatalf("notice=%+v err=%v", notice, err)
	}
	if notice.Reason == "" {
		t.Fatal("notice reason required")
	}
}

func TestActivateDatasetRefusesWrongSchema(t *testing.T) {
	root := t.TempDir()
	store := cache.NewStore(root, fixedClock{t: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)})
	id := "2026-08-29T000000"
	if err := store.EnsureDatasetDir(id); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteDatasetManifest(cache.DatasetManifest{
		SchemaVersion:  2,
		DatasetVersion: id,
		Complete:       true,
		PokemonCount:   1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.ActivateDataset(id); err == nil {
		t.Fatal("must refuse wrong schema")
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
