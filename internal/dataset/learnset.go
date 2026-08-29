package dataset

import (
	"sort"

	"github.com/Graphixa/Pokedex/internal/domain"
)

// LearnsetFromPokemon extracts the level-up learnset for the latest version group,
// matching bootstrap and TUI semantics.
func LearnsetFromPokemon(p domain.Pokemon) []DatasetLearn {
	bestOrder := -1
	bestGroup := ""
	for _, lm := range p.Moves {
		for _, learn := range lm.Learnings {
			if learn.Method != "level-up" {
				continue
			}
			if learn.VersionGroupOrder >= bestOrder {
				bestOrder = learn.VersionGroupOrder
				bestGroup = learn.VersionGroup
			}
		}
	}
	if bestOrder < 0 {
		return nil
	}

	seen := map[int]struct{}{}
	var out []DatasetLearn
	for _, lm := range p.Moves {
		if lm.MoveID <= 0 {
			continue
		}
		for _, learn := range lm.Learnings {
			if learn.Method != "level-up" {
				continue
			}
			if learn.VersionGroup != bestGroup && learn.VersionGroupOrder != bestOrder {
				continue
			}
			if learn.VersionGroupOrder != bestOrder {
				continue
			}
			if _, ok := seen[lm.MoveID]; ok {
				break
			}
			seen[lm.MoveID] = struct{}{}
			out = append(out, DatasetLearn{Level: learn.Level, MoveID: lm.MoveID})
			break
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Level != out[j].Level {
			return out[i].Level < out[j].Level
		}
		return out[i].MoveID < out[j].MoveID
	})
	return out
}

// LevelUpMovesFromLearnset resolves persisted learnset rows using a move lookup.
func LevelUpMovesFromLearnset(rows []DatasetLearn, getMove func(int) (domain.Move, error)) ([]domain.LevelUpMove, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	out := make([]domain.LevelUpMove, 0, len(rows))
	for _, row := range rows {
		move, err := getMove(row.MoveID)
		if err != nil {
			return nil, err
		}
		out = append(out, domain.LevelUpMove{Level: row.Level, Move: move})
	}
	return out, nil
}
