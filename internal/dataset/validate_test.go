package dataset_test

import (
	"testing"

	"github.com/Graphixa/Pokedex/internal/dataset"
)

func TestValidateRejectsMissingMove(t *testing.T) {
	b := dataset.NewBuilder()
	b.Pokemon[1] = dataset.DatasetPokemon{ID: 1, SpeciesID: 1}
	b.Species[1] = dataset.DatasetSpecies{ID: 1, DefaultPokemonID: 1}
	b.Learnsets[1] = []dataset.DatasetLearn{{Level: 1, MoveID: 99}}
	if err := dataset.Validate(b, 1); err == nil {
		t.Fatal("expected missing move error")
	}
}
