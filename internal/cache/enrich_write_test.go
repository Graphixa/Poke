package cache

import (
	"context"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/pokeapi"
)

type enrichTestClock struct{ t time.Time }

func (c enrichTestClock) Now() time.Time { return c.t }

type enrichTestAPI struct {
	pokemon map[int]domain.Pokemon
}

func (f *enrichTestAPI) ListSpecies(context.Context) ([]domain.SpeciesSummary, error) {
	return nil, pokeapi.ErrNotFound
}
func (f *enrichTestAPI) GetPokemon(_ context.Context, id int) (domain.Pokemon, error) {
	p, ok := f.pokemon[id]
	if !ok {
		return domain.Pokemon{}, pokeapi.ErrNotFound
	}
	return p, nil
}
func (f *enrichTestAPI) GetSpecies(context.Context, int) (domain.Species, error) {
	return domain.Species{}, pokeapi.ErrNotFound
}
func (f *enrichTestAPI) GetEvolutionChain(context.Context, int) (domain.EvolutionTree, error) {
	return domain.EvolutionTree{}, pokeapi.ErrNotFound
}
func (f *enrichTestAPI) GetEncounters(context.Context, int) ([]domain.Encounter, error) {
	return nil, pokeapi.ErrNotFound
}
func (f *enrichTestAPI) GetMove(context.Context, int) (domain.Move, error) {
	return domain.Move{}, pokeapi.ErrNotFound
}

func TestEnrichTypesWritesIndexOnce(t *testing.T) {
	root := t.TempDir()
	clock := enrichTestClock{t: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)}
	store := NewStore(root, clock)
	summaries := []domain.SpeciesSummary{
		{ID: 1, Slug: "bulbasaur", DisplayName: "Bulbasaur"},
		{ID: 4, Slug: "charmander", DisplayName: "Charmander"},
		{ID: 7, Slug: "squirtle", DisplayName: "Squirtle"},
	}
	if err := store.StoreSpeciesIndex(context.Background(), summaries, clock.Now()); err != nil {
		t.Fatal(err)
	}
	before := store.SpeciesIndexWriteCountForTest()
	cfg := config.Default()
	cfg.Network.MaxConcurrency = 2
	api := &enrichTestAPI{
		pokemon: map[int]domain.Pokemon{
			1: {ID: 1, Slug: "bulbasaur", Types: []domain.TypeSlot{{Slot: 1, Name: "grass"}}},
			4: {ID: 4, Slug: "charmander", Types: []domain.TypeSlot{{Slot: 1, Name: "fire"}}},
			7: {ID: 7, Slug: "squirtle", Types: []domain.TypeSlot{{Slot: 1, Name: "water"}}},
		},
	}
	svc := NewService(store, api, cfg, clock)
	if err := svc.EnrichTypes(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	delta := store.SpeciesIndexWriteCountForTest() - before
	if delta != 1 {
		t.Fatalf("enrich wrote index %d times during enrich, want 1", delta)
	}
	index, err := store.SpeciesIndex(context.Background())
	if err != nil || len(index[0].Types) == 0 || index[1].Types[0] != "fire" {
		t.Fatalf("index = %#v err=%v", index, err)
	}
}
