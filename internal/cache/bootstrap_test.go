package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/domain"
)

func TestBootstrapUsesStoredIndex(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	clock := fixedClock{t: now}
	store := cache.NewStore(root, clock)
	list := []domain.SpeciesSummary{{ID: 25, Slug: "pikachu", DisplayName: "Pikachu", Types: []string{"electric"}}}
	if err := store.StoreSpeciesIndex(context.Background(), list, now); err != nil {
		t.Fatal(err)
	}
	api := &fakeAPI{
		list: list,
		pokemon: map[int]domain.Pokemon{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", Stats: domain.Stats{Speed: 90}, Types: []domain.TypeSlot{{Name: "electric"}}},
		},
		species: map[int]domain.Species{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", DefaultPokemonID: 25},
		},
	}
	svc := cache.NewService(store, api, testConfig(), clock)
	if err := svc.BootstrapDataset(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if api.listCalls.Load() != 0 {
		t.Fatalf("ListSpecies calls=%d want 0", api.listCalls.Load())
	}
}

func TestBootstrapReportsAllStages(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	clock := fixedClock{t: now}
	store := cache.NewStore(root, clock)
	api := &fakeAPI{
		list: []domain.SpeciesSummary{{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}},
		pokemon: map[int]domain.Pokemon{
			25: {
				ID: 25, Slug: "pikachu", DisplayName: "Pikachu", Stats: domain.Stats{Speed: 90},
				Types: []domain.TypeSlot{{Name: "electric"}},
				Moves: []domain.LearnedMove{{MoveID: 1, Learnings: []domain.MoveLearning{{Method: "level-up", Level: 1, VersionGroup: "red-blue", VersionGroupOrder: 1}}}},
			},
		},
		species: map[int]domain.Species{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", DefaultPokemonID: 25, EvolutionChainID: 10},
		},
		evolution: map[int]domain.EvolutionTree{
			10: {Root: domain.EvolutionNode{SpeciesID: 25, SpeciesSlug: "pikachu", DisplayName: "Pikachu"}},
		},
		moves: map[int]domain.Move{
			1: {ID: 1, Slug: "pound", DisplayName: "Pound"},
		},
	}
	svc := cache.NewService(store, api, testConfig(), clock)
	seen := map[string]bool{}
	progress := func(p cache.Progress) {
		if p.Stage != "" {
			seen[p.Stage] = true
		}
	}
	if err := svc.BootstrapDataset(context.Background(), progress); err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{"records", "evolution", "moves", "dataset"} {
		if !seen[stage] {
			t.Fatalf("missing progress stage %q seen=%v", stage, seen)
		}
	}
}

func TestBootstrapEncountersUseWorkers(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	clock := fixedClock{t: now}
	store := cache.NewStore(root, clock)
	const n = 4
	list := make([]domain.SpeciesSummary, n)
	pokemon := map[int]domain.Pokemon{}
	species := map[int]domain.Species{}
	for i := 0; i < n; i++ {
		id := i + 1
		list[i] = domain.SpeciesSummary{ID: id, Slug: "mon", DisplayName: "Mon", Types: []string{"normal"}}
		pokemon[id] = domain.Pokemon{ID: id, Stats: domain.Stats{Speed: 1}, Types: []domain.TypeSlot{{Name: "normal"}}}
		species[id] = domain.Species{ID: id, DefaultPokemonID: id}
	}
	api := &fakeAPI{list: list, pokemon: pokemon, species: species, delay: 40 * time.Millisecond}
	cfg := testConfig()
	cfg.Network.MaxConcurrency = 2
	svc := cache.NewService(store, api, cfg, clock)

	start := time.Now()
	if err := svc.BootstrapDataset(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if api.maxFlight.Load() < 2 {
		t.Fatalf("max in-flight=%d want >= 2", api.maxFlight.Load())
	}
	// 4 species × 3 delayed calls each, 2 workers → well under fully serial 12× delay.
	if elapsed := time.Since(start); elapsed >= 12*api.delay {
		t.Fatalf("bootstrap too slow: %v", elapsed)
	}
}

func TestBootstrapMoveAndEvoParallel(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	clock := fixedClock{t: now}
	store := cache.NewStore(root, clock)
	const n = 4
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
	api := &fakeAPI{list: list, pokemon: pokemon, species: species, evolution: evolution, moves: moves, delay: 40 * time.Millisecond}
	cfg := testConfig()
	cfg.Network.MaxConcurrency = 2
	svc := cache.NewService(store, api, cfg, clock)
	if err := svc.BootstrapDataset(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if api.maxFlight.Load() < 2 {
		t.Fatalf("max in-flight=%d want >= 2", api.maxFlight.Load())
	}
}
