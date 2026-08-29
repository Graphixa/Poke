package dataset_test

import (
	"testing"

	"github.com/Graphixa/Pokedex/internal/dataset"
	"github.com/Graphixa/Pokedex/internal/domain"
)

func TestLearnsetFromPokemonUsesLatestVersionGroup(t *testing.T) {
	t.Parallel()
	pokemon := domain.Pokemon{
		Moves: []domain.LearnedMove{
			{
				MoveID: 1,
				Learnings: []domain.MoveLearning{{
					Method: "level-up", Level: 1, VersionGroup: "red-blue", VersionGroupOrder: 1,
				}},
			},
			{
				MoveID: 84,
				Learnings: []domain.MoveLearning{{
					Method: "level-up", Level: 5, VersionGroup: "scarlet-violet", VersionGroupOrder: 27,
				}},
			},
			{
				MoveID: 99,
				Learnings: []domain.MoveLearning{{
					Method: "machine", VersionGroup: "scarlet-violet", VersionGroupOrder: 27,
				}},
			},
		},
	}
	rows := dataset.LearnsetFromPokemon(pokemon)
	if len(rows) != 1 || rows[0].MoveID != 84 || rows[0].Level != 5 {
		t.Fatalf("learnset=%+v want scarlet-violet move 84", rows)
	}
}

func TestLevelUpMovesFromLearnset(t *testing.T) {
	t.Parallel()
	rows := []dataset.DatasetLearn{{Level: 1, MoveID: 84}}
	moves := map[int]domain.Move{84: {ID: 84, Slug: "thunder-shock", DisplayName: "Thunder Shock"}}
	out, err := dataset.LevelUpMovesFromLearnset(rows, func(id int) (domain.Move, error) {
		return moves[id], nil
	})
	if err != nil || len(out) != 1 || out[0].Move.ID != 84 || out[0].Level != 1 {
		t.Fatalf("moves=%+v err=%v", out, err)
	}
}
