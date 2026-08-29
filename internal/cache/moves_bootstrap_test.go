package cache

import (
	"testing"

	"github.com/Graphixa/Pokedex/internal/domain"
)

func TestLevelUpMoveIDsIgnoresTMTutor(t *testing.T) {
	t.Parallel()
	pokemon := domain.Pokemon{
		Moves: []domain.LearnedMove{
			{
				MoveID: 1,
				Learnings: []domain.MoveLearning{{
					Method: "level-up", Level: 1, VersionGroup: "scarlet-violet", VersionGroupOrder: 27,
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
	ids := levelUpMoveIDs(pokemon)
	if len(ids) != 1 || ids[0] != 1 {
		t.Fatalf("ids=%v want [1]", ids)
	}
}
