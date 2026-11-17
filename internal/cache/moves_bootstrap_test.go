package cache

import (
	"testing"

	"github.com/Graphixa/Poke/internal/domain"
)

func TestReferencedMoveIDsIncludesMachine(t *testing.T) {
	t.Parallel()
	pokemon := domain.Pokemon{
		Moves: []domain.LearnedMove{
			{
				MoveID: 1,
				Learnings: []domain.MoveLearning{{
					Method: "level-up", Level: 1, VersionGroup: "scarlet-violet", VersionGroupID: 27,
				}},
			},
			{
				MoveID: 99,
				Learnings: []domain.MoveLearning{{
					Method: "machine", VersionGroup: "scarlet-violet", VersionGroupID: 27,
				}},
			},
		},
	}
	ids := referencedMoveIDs(pokemon)
	if len(ids) != 2 {
		t.Fatalf("ids=%v want both level-up and machine", ids)
	}
}
