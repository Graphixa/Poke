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

	d := domain.Dossier{
		Pokemon:    domain.Pokemon{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"},
		Species:    domain.Species{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"},
		Evolution:  &domain.EvolutionTree{Root: domain.EvolutionNode{DisplayName: "Pikachu"}},
		Encounters: []domain.Encounter{},
		LevelUpMoves: []domain.LevelUpMove{{
			Level: 1,
			Move:  domain.Move{Slug: "tail-whip", DisplayName: "Tail Whip"},
		}},
	}
	if err := store.StoreDossier(context.Background(), d, clock.Now()); err != nil {
		t.Fatal(err)
	}

	svc := cache.NewService(store, &fakeAPI{}, testConfig(), clock)
	start := time.Now()
	res, err := svc.DossierCore(context.Background(), 25, "pikachu")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Complete {
		t.Fatal("expected complete dossier from disk")
	}
	if res.Dossier.Pokemon.DisplayName != "Pikachu" {
		t.Fatalf("name = %q", res.Dossier.Pokemon.DisplayName)
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("cached dossier core took %v want <200ms", elapsed)
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
		t.Fatal("expected incomplete core on miss")
	}
	if api.moveCalls.Load() != 0 {
		t.Fatalf("core path fetched moves: %d", api.moveCalls.Load())
	}
}

func TestResolveLevelUpMovesParallel(t *testing.T) {
	api := &countingAPI{fakeAPI: fakeAPI{delay: 20 * time.Millisecond}}
	store := cache.NewStore(t.TempDir(), fixedClock{t: time.Now()})
	cfg := testConfig()
	cfg.Network.MaxConcurrency = 4
	svc := cache.NewService(store, api, cfg, fixedClock{t: time.Now()})

	pokemon := domain.Pokemon{Moves: make([]domain.LearnedMove, 0, 8)}
	api.moves = make(map[int]domain.Move, 8)
	for i := 1; i <= 8; i++ {
		pokemon.Moves = append(pokemon.Moves, domain.LearnedMove{
			MoveID: i,
			Slug:   "move",
			Learnings: []domain.MoveLearning{{
				VersionGroup: "scarlet-violet", VersionGroupOrder: 25, Method: "level-up", Level: i,
			}},
		})
		api.moves[i] = domain.Move{ID: i, Slug: "move", DisplayName: "Move"}
	}

	_, err := svc.DossierExtras(context.Background(), 1, "test", domain.Dossier{
		Pokemon: pokemon,
		Species: domain.Species{ID: 1, DefaultPokemonID: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if max := api.maxFlight.Load(); max < 2 {
		t.Fatalf("max concurrent move fetches = %d want >= 2", max)
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

func (c *countingAPI) GetSpecies(ctx context.Context, id int) (domain.Species, error) {
	if s, ok := c.fakeAPI.species[id]; ok {
		return s, nil
	}
	return c.fakeAPI.GetSpecies(ctx, id)
}

func (c *countingAPI) GetPokemon(ctx context.Context, id int) (domain.Pokemon, error) {
	if p, ok := c.fakeAPI.pokemon[id]; ok {
		return p, nil
	}
	return c.fakeAPI.GetPokemon(ctx, id)
}

func (c *countingAPI) GetEvolutionChain(context.Context, int) (domain.EvolutionTree, error) {
	return domain.EvolutionTree{Root: domain.EvolutionNode{DisplayName: "Root"}}, nil
}

func (c *countingAPI) GetEncounters(context.Context, int) ([]domain.Encounter, error) {
	return []domain.Encounter{}, nil
}
