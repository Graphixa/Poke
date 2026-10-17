package dataset_test

import (
	"strings"
	"testing"

	"github.com/Graphixa/Pokedex/internal/dataset"
	"github.com/Graphixa/Pokedex/internal/domain"
)

func TestValidateRejectsMissingMove(t *testing.T) {
	b := dataset.NewBuilder()
	b.Pokemon[1] = dataset.DatasetPokemon{ID: 1, SpeciesID: 1}
	b.Species[1] = dataset.DatasetSpecies{ID: 1, DefaultPokemonID: 1}
	b.Encounters[1] = []dataset.DatasetEncounter{}
	b.Learnsets[1] = []dataset.DatasetLearn{{
		PokemonID: 1, LevelLearnedAt: 1, MoveID: 99, LearnMethod: "level-up", VersionGroupID: 1,
	}}
	browse := []domain.BrowseEntry{{ID: 1, Slug: "x", Name: "X"}}
	err := dataset.Validate(b, 1, browse)
	if err == nil || !strings.Contains(err.Error(), "missing move") {
		t.Fatalf("expected missing move error, got %v", err)
	}
}
