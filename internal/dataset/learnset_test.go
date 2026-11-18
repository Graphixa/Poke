package dataset_test

import (
	"testing"

	"github.com/Graphixa/Poke/internal/dataset"
	"github.com/Graphixa/Poke/internal/domain"
)

func TestLearnsetFromPokemonKeepsAllMethodsAndGroups(t *testing.T) {
	t.Parallel()
	pokemon := domain.Pokemon{
		ID: 25,
		Moves: []domain.LearnedMove{
			{
				MoveID: 1,
				Learnings: []domain.MoveLearning{{
					Method: "level-up", Level: 1, VersionGroupID: 1, VersionGroup: "red-blue",
				}},
			},
			{
				MoveID: 84,
				Learnings: []domain.MoveLearning{
					{Method: "level-up", Level: 5, VersionGroupID: 27, VersionGroup: "scarlet-violet"},
					{Method: "machine", Level: 0, VersionGroupID: 27, VersionGroup: "scarlet-violet"},
				},
			},
			{
				MoveID: 84,
				Learnings: []domain.MoveLearning{{
					Method: "level-up", Level: 5, VersionGroupID: 27, VersionGroup: "scarlet-violet",
				}},
			},
		},
	}
	rows := dataset.LearnsetFromPokemon(pokemon)
	if len(rows) != 3 {
		t.Fatalf("learnset len=%d want 3 (dedupe exact only): %+v", len(rows), rows)
	}
	filtered := dataset.FilterLearnset(rows, 27, "level-up")
	if len(filtered) != 1 || filtered[0].MoveID != 84 || filtered[0].LevelLearnedAt != 5 {
		t.Fatalf("vg27 level-up=%+v", filtered)
	}
	machines := dataset.FilterLearnset(rows, 27, "machine")
	if len(machines) != 1 || machines[0].MoveID != 84 {
		t.Fatalf("vg27 machine=%+v", machines)
	}
}

func TestLevelUpMovesFromLearnsetFiltersVersionGroup(t *testing.T) {
	t.Parallel()
	rows := []dataset.DatasetLearn{
		{PokemonID: 25, MoveID: 1, VersionGroupID: 1, LearnMethod: "level-up", LevelLearnedAt: 1},
		{PokemonID: 25, MoveID: 84, VersionGroupID: 27, LearnMethod: "level-up", LevelLearnedAt: 5},
		{PokemonID: 25, MoveID: 99, VersionGroupID: 27, LearnMethod: "machine", LevelLearnedAt: 0},
	}
	moves := map[int]domain.Move{
		1:  {ID: 1, Slug: "pound"},
		84: {ID: 84, Slug: "thunder-shock", DisplayName: "Thunder Shock"},
		99: {ID: 99, Slug: "tm"},
	}
	out, err := dataset.LevelUpMovesFromLearnset(rows, func(id int) (domain.Move, error) {
		return moves[id], nil
	}, 27)
	if err != nil || len(out) != 1 || out[0].Move.ID != 84 || out[0].Level != 5 {
		t.Fatalf("moves=%+v err=%v", out, err)
	}
	all, err := dataset.MovesFromLearnset(rows, func(id int) (domain.Move, error) {
		return moves[id], nil
	}, 27)
	if err != nil || len(all) != 2 {
		t.Fatalf("all methods=%+v err=%v", all, err)
	}
	if all[0].Method != "level-up" || all[1].Method != "machine" {
		t.Fatalf("method order=%+v", all)
	}
}

func TestDropMovesOmitsMissingOnly(t *testing.T) {
	t.Parallel()
	b := dataset.NewBuilder()
	b.Learnsets[10301] = []dataset.DatasetLearn{
		{PokemonID: 10301, MoveID: 34, LearnMethod: "machine"},
		{PokemonID: 10301, MoveID: 920, LearnMethod: "machine"},
	}
	if n := dataset.DropMoves(b, []int{920}); n != 1 {
		t.Fatalf("dropped %d", n)
	}
	rows := b.Learnsets[10301]
	if len(rows) != 1 || rows[0].MoveID != 34 {
		t.Fatalf("rows=%+v", rows)
	}
}

func TestReferencedMoveIDsIncludesMachine(t *testing.T) {
	t.Parallel()
	p := domain.Pokemon{Moves: []domain.LearnedMove{
		{MoveID: 1, Learnings: []domain.MoveLearning{{Method: "level-up"}}},
		{MoveID: 99, Learnings: []domain.MoveLearning{{Method: "machine"}}},
	}}
	ids := dataset.ReferencedMoveIDs(p)
	if len(ids) != 2 {
		t.Fatalf("ids=%v", ids)
	}
}

func TestNullPowerPreserved(t *testing.T) {
	t.Parallel()
	m := domain.Move{ID: 1, Slug: "growl", Power: nil, Accuracy: nil, PP: 40}
	compact := dataset.MoveFromDomain(m)
	if compact.Power != nil || compact.Accuracy != nil {
		t.Fatalf("nulls lost: %+v", compact)
	}
	back := dataset.MoveToDomain(compact)
	if back.Power != nil || back.Accuracy != nil {
		t.Fatalf("round-trip nulls: %+v", back)
	}
}
