package cache_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/domain"
)

func TestDossierCoreHitUnder200ms(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	api := &fakeAPI{
		list: []domain.SpeciesSummary{{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}},
		pokemon: map[int]domain.Pokemon{
			25: {
				ID: 25, Slug: "pikachu", DisplayName: "Pikachu", Stats: domain.Stats{Speed: 90},
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
			25: {{Area: "forest"}},
		},
		moves: map[int]domain.Move{
			1: {ID: 1, Slug: "pound", DisplayName: "Pound"},
		},
	}
	svc := bootstrapOne(t, store, api, clock)

	start := time.Now()
	res, err := svc.DossierCore(context.Background(), 25, "pikachu")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if res.Complete {
		t.Fatal("core path must not load extras eagerly")
	}
	if res.Dossier.Evolution != nil || res.Dossier.Encounters != nil || len(res.Dossier.LevelUpMoves) > 0 {
		t.Fatalf("core loaded extras: %+v", res.Dossier)
	}
	if res.Dossier.Pokemon.DisplayName != "Pikachu" {
		t.Fatalf("name = %q", res.Dossier.Pokemon.DisplayName)
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("dataset dossier core took %v want <200ms", elapsed)
	}
}

func TestDossierExtrasLoadsFromDataset(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	api := &fakeAPI{
		list: []domain.SpeciesSummary{{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}},
		pokemon: map[int]domain.Pokemon{
			25: {
				ID: 25, Slug: "pikachu", DisplayName: "Pikachu", Stats: domain.Stats{Speed: 90},
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
			25: {{Area: "forest"}},
		},
		moves: map[int]domain.Move{
			1: {ID: 1, Slug: "pound", DisplayName: "Pound"},
		},
	}
	svc := bootstrapOne(t, store, api, clock)
	core, err := svc.DossierCore(context.Background(), 25, "pikachu")
	if err != nil || core.Complete {
		t.Fatalf("core=%+v err=%v", core, err)
	}
	extras, err := svc.DossierExtras(context.Background(), 25, "pikachu", core.Dossier)
	if err != nil {
		t.Fatal(err)
	}
	if extras.Evolution == nil || len(extras.Encounters) != 1 || len(extras.LevelUpMoves) != 1 {
		t.Fatalf("extras=%+v", extras)
	}
	before := api.calls.Load()
	_, err = svc.DossierExtras(context.Background(), 25, "pikachu", core.Dossier)
	if err != nil {
		t.Fatal(err)
	}
	if api.calls.Load() != before {
		t.Fatal("extras must stay offline after dataset bootstrap")
	}
}

func TestDossierCoreMissSkipsMoves(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	api := &countingAPI{
		fakeAPI: fakeAPI{
			species: map[int]domain.Species{
				4: {ID: 4, Slug: "charmander", DisplayName: "Charmander", DefaultPokemonID: 4, EvolutionChainID: 1},
			},
			pokemon: map[int]domain.Pokemon{
				4: {ID: 4, Slug: "charmander", DisplayName: "Charmander", Moves: []domain.LearnedMove{{
					MoveID: 10, Slug: "scratch",
					Learnings: []domain.MoveLearning{{VersionGroup: "scarlet-violet", VersionGroupOrder: 25, Method: "level-up", Level: 1}},
				}}},
			},
		},
	}
	svc := cache.NewService(store, api, testConfig(), clock)

	res, err := svc.DossierCore(context.Background(), 4, "charmander")
	if err != nil {
		t.Fatal(err)
	}
	if res.Complete {
		t.Fatal("expected incomplete core without dataset")
	}
	if api.moveCalls.Load() != 0 {
		t.Fatalf("core path fetched moves: %d", api.moveCalls.Load())
	}
}

func TestResolveLevelUpMovesParallel(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	api := &countingAPI{
		fakeAPI: fakeAPI{
			list: []domain.SpeciesSummary{{ID: 1, Slug: "test", DisplayName: "Test"}},
			pokemon: map[int]domain.Pokemon{
				1: {
					ID: 1, Moves: []domain.LearnedMove{
						{MoveID: 1, Learnings: []domain.MoveLearning{{Method: "level-up", Level: 1, VersionGroup: "scarlet-violet", VersionGroupOrder: 25}}},
						{MoveID: 2, Learnings: []domain.MoveLearning{{Method: "level-up", Level: 2, VersionGroup: "scarlet-violet", VersionGroupOrder: 25}}},
						{MoveID: 3, Learnings: []domain.MoveLearning{{Method: "level-up", Level: 3, VersionGroup: "scarlet-violet", VersionGroupOrder: 25}}},
						{MoveID: 4, Learnings: []domain.MoveLearning{{Method: "level-up", Level: 4, VersionGroup: "scarlet-violet", VersionGroupOrder: 25}}},
					},
				},
			},
			species: map[int]domain.Species{1: {ID: 1, DefaultPokemonID: 1}},
			moves: map[int]domain.Move{
				1: {ID: 1, Slug: "move", DisplayName: "Move"},
				2: {ID: 2, Slug: "move", DisplayName: "Move"},
				3: {ID: 3, Slug: "move", DisplayName: "Move"},
				4: {ID: 4, Slug: "move", DisplayName: "Move"},
			},
		},
	}
	cfg := testConfig()
	cfg.Network.MaxConcurrency = 4
	svc := cache.NewService(store, api, cfg, clock)
	if err := svc.BootstrapDataset(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	d, err := svc.DossierExtras(context.Background(), 1, "test", domain.Dossier{
		Pokemon: domain.Pokemon{ID: 1},
		Species: domain.Species{ID: 1, DefaultPokemonID: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.LevelUpMoves) != 4 {
		t.Fatalf("level-up moves=%d want 4", len(d.LevelUpMoves))
	}
}

type countingAPI struct {
	fakeAPI
	moveCalls atomic.Int64
	maxFlight atomic.Int64
	inFlight  atomic.Int64
	moves     map[int]domain.Move
}

func (c *countingAPI) GetMove(ctx context.Context, id int) (domain.Move, error) {
	c.moveCalls.Add(1)
	cur := c.inFlight.Add(1)
	for {
		prev := c.maxFlight.Load()
		if cur <= prev || c.maxFlight.CompareAndSwap(prev, cur) {
			break
		}
	}
	defer c.inFlight.Add(-1)
	if c.fakeAPI.delay > 0 {
		select {
		case <-ctx.Done():
			return domain.Move{}, ctx.Err()
		case <-time.After(c.fakeAPI.delay):
		}
	}
	if m, ok := c.moves[id]; ok {
		return m, nil
	}
	return domain.Move{ID: id, Slug: "move", DisplayName: "Move"}, nil
}

func bootstrapOne(t *testing.T, store *cache.Store, api *fakeAPI, clock fixedClock) *cache.Service {
	t.Helper()
	svc := cache.NewService(store, api, testConfig(), clock)
	if err := svc.BootstrapDataset(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	return svc
}
