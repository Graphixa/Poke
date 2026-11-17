package cache_test

import (
	"os"
	"testing"
	"time"

	"github.com/Graphixa/Poke/internal/cache"
	"github.com/Graphixa/Poke/internal/domain"
)

func TestTruncatedStagingManifestDoesNotActivate(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	good := "2026-08-29T000000"
	if err := store.EnsureDatasetDir(good); err != nil {
		t.Fatal(err)
	}
	if err := store.StoreBrowse(good, []domain.BrowseEntry{{ID: 1, Slug: "a", Name: "A"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteDatasetManifest(cache.DatasetManifest{
		SchemaVersion:  cache.DatasetSchemaVersion,
		DatasetVersion: good,
		PokemonCount:   1,
		Complete:       true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.ActivateDataset(good); err != nil {
		t.Fatal(err)
	}

	bad := "2026-08-29T010000"
	if err := store.EnsureDatasetDir(bad); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteDatasetManifest(cache.DatasetManifest{
		SchemaVersion:  cache.DatasetSchemaVersion,
		DatasetVersion: bad,
		Complete:       false,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.ActivateDataset(bad); err == nil {
		t.Fatal("incomplete staging must not activate")
	}
	got, err := store.ActiveDatasetID()
	if err != nil || got != good {
		t.Fatalf("active=%q err=%v want %s", got, err, good)
	}
}

func TestMissingAggregateFileResetsActive(t *testing.T) {
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
		SchemaVersion:  cache.DatasetSchemaVersion,
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
		t.Fatal("manifest without aggregates must reset")
	}
}

func TestActiveAggregateCorruptionReturnsError(t *testing.T) {
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
	activeID, _ := store.ActiveDatasetID()
	if err := os.WriteFile(store.AggregatePokemonPath(activeID), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	store.InvalidateLoadedSnapshotForTest()
	_, err := svc.Pokemon(t.Context(), 25)
	if err == nil {
		t.Fatal("corrupt active aggregate must return error")
	}
}
