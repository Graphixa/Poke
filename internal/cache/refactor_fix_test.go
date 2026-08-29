package cache_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/dataset"
	"github.com/Graphixa/Pokedex/internal/domain"
)

func TestBootstrapParallelMoveReport(t *testing.T) {
	testBootstrapParallelReport(t, "moves")
}

func TestBootstrapParallelEvolutionReport(t *testing.T) {
	testBootstrapParallelReport(t, "evolution")
}

func TestBootstrapParallelFailureReport(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	api := &fakeAPI{
		list: []domain.SpeciesSummary{
			{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"},
			{ID: 26, Slug: "raichu", DisplayName: "Raichu"},
		},
		pokemon: map[int]domain.Pokemon{
			25: {
				ID: 25, Stats: domain.Stats{Speed: 90},
				Moves: []domain.LearnedMove{{MoveID: 1, Learnings: []domain.MoveLearning{{Method: "level-up", Level: 1, VersionGroup: "red-blue", VersionGroupOrder: 1}}}},
			},
		},
		species: map[int]domain.Species{
			25: {ID: 25, DefaultPokemonID: 25, EvolutionChainID: 10},
			26: {ID: 26, DefaultPokemonID: 26},
		},
		evolution: map[int]domain.EvolutionTree{
			10: {Root: domain.EvolutionNode{SpeciesID: 25}},
		},
		moves: map[int]domain.Move{
			1: {ID: 1, Slug: "pound", DisplayName: "Pound"},
		},
	}
	svc := cache.NewService(store, api, testConfig(), clock)
	if err := svc.BootstrapDataset(context.Background(), nil); err == nil {
		t.Fatal("expected bootstrap failure")
	}
	if svc.HasActiveDataset() {
		t.Fatal("incomplete bootstrap must not activate")
	}
}

func testBootstrapParallelReport(t *testing.T, stage string) {
	t.Helper()
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	const n = 8
	list := make([]domain.SpeciesSummary, n)
	pokemon := map[int]domain.Pokemon{}
	species := map[int]domain.Species{}
	evolution := map[int]domain.EvolutionTree{}
	moves := map[int]domain.Move{}
	for i := 0; i < n; i++ {
		id := i + 1
		evoID := 100 + id
		moveID := 200 + id
		list[i] = domain.SpeciesSummary{ID: id, Slug: "mon", DisplayName: "Mon"}
		pokemon[id] = domain.Pokemon{
			ID: id, Stats: domain.Stats{Speed: 1},
			Moves: []domain.LearnedMove{{MoveID: moveID, Learnings: []domain.MoveLearning{{Method: "level-up", Level: 1, VersionGroup: "red-blue", VersionGroupOrder: 1}}}},
		}
		species[id] = domain.Species{ID: id, DefaultPokemonID: id, EvolutionChainID: evoID}
		evolution[evoID] = domain.EvolutionTree{Root: domain.EvolutionNode{SpeciesID: id}}
		moves[moveID] = domain.Move{ID: moveID, Slug: "move", DisplayName: "Move"}
	}
	api := &fakeAPI{list: list, pokemon: pokemon, species: species, evolution: evolution, moves: moves}
	cfg := testConfig()
	cfg.Network.MaxConcurrency = 4
	svc := cache.NewService(store, api, cfg, clock)
	seen := map[string]bool{}
	if err := svc.BootstrapDataset(context.Background(), func(p cache.Progress) {
		if p.Stage == stage {
			seen[stage] = true
		}
	}); err != nil {
		t.Fatal(err)
	}
	if !seen[stage] {
		t.Fatalf("missing progress stage %q", stage)
	}
	id := cache.DatasetIDForTime(clock.t)
	rep, err := store.ReadBootstrapReport(id)
	if err != nil {
		t.Fatal(err)
	}
	if stage == "moves" && rep.MoveOK != rep.MoveWanted {
		t.Fatalf("moves=%d/%d", rep.MoveOK, rep.MoveWanted)
	}
	if stage == "evolution" && rep.EvolutionOK != rep.EvolutionWanted {
		t.Fatalf("evo=%d/%d", rep.EvolutionOK, rep.EvolutionWanted)
	}
}

func TestBootstrapParallelReportRace(t *testing.T) {
	testBootstrapParallelReport(t, "moves")
}

func TestConcurrentDatasetLoadAndReset(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	seedActiveDataset(t, store, clock)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = store.Pokemon(context.Background(), 25)
			_ = store.EnsureCompatibleDataset()
		}()
	}
	wg.Wait()
}

func TestConcurrentDatasetLoadAndClear(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	seedActiveDataset(t, store, clock)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				_, _ = store.Pokemon(context.Background(), 25)
			} else {
				_ = store.Clear()
			}
		}(i)
	}
	wg.Wait()
}

func TestConcurrentActivateAndRead(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	seedActiveDataset(t, store, clock)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = store.Pokemon(context.Background(), 25)
			_, _ = store.Species(context.Background(), 25)
			store.InvalidateLoadedSnapshotForTest()
		}()
	}
	wg.Wait()
}

func seedActiveDataset(t *testing.T, store *cache.Store, clock fixedClock) {
	t.Helper()
	api := &fakeAPI{
		list: []domain.SpeciesSummary{{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}},
		pokemon: map[int]domain.Pokemon{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", SpeciesID: 25, Stats: domain.Stats{Speed: 90}},
		},
		species: map[int]domain.Species{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", DefaultPokemonID: 25},
		},
	}
	svc := cache.NewService(store, api, testConfig(), clock)
	if err := svc.BootstrapDataset(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}

func TestPokemonDoesNotFallbackToLegacy(t *testing.T) {
	testIgnoresLegacyMetadata(t, "pokemon", func(svc *cache.Service) error {
		_, err := svc.Pokemon(context.Background(), 25)
		return err
	})
}

func TestSpeciesDoesNotFallbackToLegacy(t *testing.T) {
	testIgnoresLegacyMetadata(t, "species", func(svc *cache.Service) error {
		_, err := svc.Species(context.Background(), 25)
		return err
	})
}

func TestMoveDoesNotFallbackToLegacy(t *testing.T) {
	testIgnoresLegacyMetadata(t, "move", func(svc *cache.Service) error {
		_, err := svc.Move(context.Background(), 1)
		return err
	})
}

func TestEvolutionDoesNotFallbackToLegacy(t *testing.T) {
	testIgnoresLegacyMetadata(t, "evolution", func(svc *cache.Service) error {
		_, err := svc.EvolutionChain(context.Background(), 10)
		return err
	})
}

func TestEncountersDoesNotFallbackToLegacy(t *testing.T) {
	testIgnoresLegacyMetadata(t, "encounters", func(svc *cache.Service) error {
		_, err := svc.Encounters(context.Background(), 25)
		return err
	})
}

func testIgnoresLegacyMetadata(t *testing.T, kind string, read func(*cache.Service) error) {
	t.Helper()
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	api := &fakeAPI{
		list: []domain.SpeciesSummary{{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}},
		pokemon: map[int]domain.Pokemon{
			25: {
				ID: 25, Slug: "pikachu", DisplayName: "Pikachu", SpeciesID: 25, Stats: domain.Stats{Speed: 90},
				Moves: []domain.LearnedMove{{MoveID: 1, Learnings: []domain.MoveLearning{{Method: "level-up", Level: 1, VersionGroup: "red-blue", VersionGroupOrder: 1}}}},
			},
		},
		species: map[int]domain.Species{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", DefaultPokemonID: 25, EvolutionChainID: 10},
		},
		evolution: map[int]domain.EvolutionTree{
			10: {Root: domain.EvolutionNode{SpeciesID: 25, SpeciesSlug: "pikachu", DisplayName: "Pikachu"}},
		},
		encounters: map[int][]domain.Encounter{
			25: {{Area: "forest", MinimumLevel: 5}},
		},
		moves: map[int]domain.Move{
			1: {ID: 1, Slug: "pound", DisplayName: "Pound", PP: 35},
		},
	}
	svc := cache.NewService(store, api, testConfig(), clock)
	if err := svc.BootstrapDataset(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(root, kind)
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	switch kind {
	case "pokemon":
		_ = os.WriteFile(filepath.Join(legacy, "25.json"), []byte(`{"schema_version":2,"source":"pokeapi","fetched_at":"2026-01-01T00:00:00Z","data":{"id":999,"display_name":"WRONG"}}`), 0o600)
	case "species":
		_ = os.WriteFile(filepath.Join(legacy, "25.json"), []byte(`{"schema_version":2,"source":"pokeapi","fetched_at":"2026-01-01T00:00:00Z","data":{"id":999,"display_name":"WRONG"}}`), 0o600)
	case "move":
		_ = os.WriteFile(filepath.Join(legacy, "1.json"), []byte(`{"schema_version":2,"source":"pokeapi","fetched_at":"2026-01-01T00:00:00Z","data":{"id":999,"display_name":"WRONG"}}`), 0o600)
	case "evolution":
		_ = os.WriteFile(filepath.Join(legacy, "10.json"), []byte(`{"schema_version":2,"source":"pokeapi","fetched_at":"2026-01-01T00:00:00Z","data":{"root":{"species_id":999}}}`), 0o600)
	case "encounters":
		_ = os.WriteFile(filepath.Join(legacy, "25.json"), []byte(`{"schema_version":2,"source":"pokeapi","fetched_at":"2026-01-01T00:00:00Z","data":[{"area":"wrong"}]}`), 0o600)
	}
	store.InvalidateLoadedSnapshotForTest()
	if err := read(svc); err != nil {
		t.Fatal(err)
	}
}

func TestPokemonCoreDoesNotLoadMoves(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	api := &fakeAPI{
		list: []domain.SpeciesSummary{{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}},
		pokemon: map[int]domain.Pokemon{
			25: {
				ID: 25, Slug: "pikachu", DisplayName: "Pikachu", SpeciesID: 25, Stats: domain.Stats{Speed: 90},
				Moves: []domain.LearnedMove{{MoveID: 1, Learnings: []domain.MoveLearning{{Method: "level-up", Level: 1, VersionGroup: "red-blue", VersionGroupOrder: 1}}}},
			},
		},
		species: map[int]domain.Species{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", DefaultPokemonID: 25},
		},
		moves: map[int]domain.Move{
			1: {ID: 1, Slug: "pound", DisplayName: "Pound"},
		},
	}
	svc := cache.NewService(store, api, testConfig(), clock)
	if err := svc.BootstrapDataset(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	id, _ := store.ActiveDatasetID()
	dir := filepath.Join(root, "datasets", id)
	for _, file := range []string{dataset.FileLearnsets, dataset.FileMoves} {
		if err := os.Remove(filepath.Join(dir, file)); err != nil {
			t.Fatal(err)
		}
	}
	store.InvalidateLoadedSnapshotForTest()
	p, err := svc.Pokemon(context.Background(), 25)
	if err != nil || p.ID != 25 {
		t.Fatalf("pokemon=%#v err=%v", p, err)
	}
	if len(p.Moves) != 0 {
		t.Fatalf("core pokemon must not load moves: %#v", p.Moves)
	}
}
