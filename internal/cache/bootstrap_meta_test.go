package cache

import (
	"context"
	"testing"

	"github.com/Graphixa/Poke/internal/config"
	"github.com/Graphixa/Poke/internal/dataset"
	"github.com/Graphixa/Poke/internal/domain"
)

// formStub is only here so parallel form fetches can run without the network.
type formStub struct{}

func (formStub) ListSpecies(context.Context) ([]domain.SpeciesSummary, error) {
	return nil, nil
}
func (formStub) GetPokemon(context.Context, int) (domain.Pokemon, error) {
	return domain.Pokemon{}, nil
}
func (formStub) GetSpecies(context.Context, int) (domain.Species, error) {
	return domain.Species{}, nil
}
func (formStub) GetEvolutionChain(context.Context, int) (domain.EvolutionTree, error) {
	return domain.EvolutionTree{}, nil
}
func (formStub) GetEncounters(context.Context, int) ([]domain.Encounter, error) {
	return nil, nil
}
func (formStub) GetMove(context.Context, int) (domain.Move, error) { return domain.Move{}, nil }
func (formStub) GetPokemonForm(_ context.Context, id int) (domain.PokemonForm, error) {
	return domain.PokemonForm{ID: id, DisplayName: "form"}, nil
}

func TestParallelFormFetchWritesEveryForm(t *testing.T) {
	cfg := config.Default()
	cfg.Network.MaxConcurrency = 8
	svc := NewService(nil, formStub{}, cfg, nil)
	b := dataset.NewBuilder()
	ids := make([]int, 64)
	for i := range ids {
		ids[i] = i + 1
	}
	if err := svc.parallelFormFetch(context.Background(), ids, nil, b); err != nil {
		t.Fatal(err)
	}
	if len(b.Forms) != len(ids) {
		t.Fatalf("forms %d, want %d", len(b.Forms), len(ids))
	}
}
