package dataset

import (
	"sort"

	"github.com/Graphixa/Poke/internal/domain"
)

// LearnsetFromPokemon persists every learning row for the Pokémon.
// Deduplicates only exact duplicate (move, vg, method, level) tuples.
func LearnsetFromPokemon(p domain.Pokemon) []DatasetLearn {
	type key struct {
		move, vg int
		method   string
		level    int
	}
	seen := map[key]struct{}{}
	var out []DatasetLearn
	for _, lm := range p.Moves {
		if lm.MoveID <= 0 {
			continue
		}
		for _, learn := range lm.Learnings {
			vgID := learn.VersionGroupID
			if vgID == 0 {
				vgID = learn.VersionGroupOrder
			}
			k := key{move: lm.MoveID, vg: vgID, method: learn.Method, level: learn.Level}
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = struct{}{}
			out = append(out, DatasetLearn{
				PokemonID:      p.ID,
				MoveID:         lm.MoveID,
				VersionGroupID: vgID,
				LearnMethod:    learn.Method,
				LevelLearnedAt: learn.Level,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].VersionGroupID != out[j].VersionGroupID {
			return out[i].VersionGroupID < out[j].VersionGroupID
		}
		if out[i].LearnMethod != out[j].LearnMethod {
			return out[i].LearnMethod < out[j].LearnMethod
		}
		if out[i].LevelLearnedAt != out[j].LevelLearnedAt {
			return out[i].LevelLearnedAt < out[j].LevelLearnedAt
		}
		return out[i].MoveID < out[j].MoveID
	})
	return out
}

// FilterLearnset returns rows for a version group. method empty = all methods.
func FilterLearnset(rows []DatasetLearn, versionGroupID int, method string) []DatasetLearn {
	if versionGroupID < 1 {
		return nil
	}
	var out []DatasetLearn
	for _, row := range rows {
		if row.VersionGroupID != versionGroupID {
			continue
		}
		if method != "" && row.LearnMethod != method {
			continue
		}
		out = append(out, row)
	}
	return out
}

// LevelUpMovesFromLearnset resolves level-up rows for one version group.
// If versionGroupID is 0, uses the highest version_group_id present (legacy TUI path).
func LevelUpMovesFromLearnset(rows []DatasetLearn, getMove func(int) (domain.Move, error), versionGroupID int) ([]domain.LevelUpMove, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	vg := versionGroupID
	if vg < 1 {
		for _, row := range rows {
			if row.LearnMethod != "level-up" {
				continue
			}
			if row.VersionGroupID > vg {
				vg = row.VersionGroupID
			}
		}
	}
	filtered := FilterLearnset(rows, vg, "level-up")
	if len(filtered) == 0 {
		return nil, nil
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].LevelLearnedAt != filtered[j].LevelLearnedAt {
			return filtered[i].LevelLearnedAt < filtered[j].LevelLearnedAt
		}
		return filtered[i].MoveID < filtered[j].MoveID
	})
	out := make([]domain.LevelUpMove, 0, len(filtered))
	for _, row := range filtered {
		move, err := getMove(row.MoveID)
		if err != nil {
			return nil, err
		}
		out = append(out, domain.LevelUpMove{Level: row.LevelLearnedAt, Method: "level-up", Move: move})
	}
	return out, nil
}

// MovesFromLearnset resolves all learn methods for one version group.
func MovesFromLearnset(rows []DatasetLearn, getMove func(int) (domain.Move, error), versionGroupID int) ([]domain.LevelUpMove, error) {
	filtered := FilterLearnset(rows, versionGroupID, "")
	if len(filtered) == 0 {
		return nil, nil
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].LearnMethod != filtered[j].LearnMethod {
			return methodOrder(filtered[i].LearnMethod) < methodOrder(filtered[j].LearnMethod)
		}
		if filtered[i].LevelLearnedAt != filtered[j].LevelLearnedAt {
			return filtered[i].LevelLearnedAt < filtered[j].LevelLearnedAt
		}
		return filtered[i].MoveID < filtered[j].MoveID
	})
	out := make([]domain.LevelUpMove, 0, len(filtered))
	for _, row := range filtered {
		move, err := getMove(row.MoveID)
		if err != nil {
			return nil, err
		}
		out = append(out, domain.LevelUpMove{
			Level:  row.LevelLearnedAt,
			Method: row.LearnMethod,
			Move:   move,
		})
	}
	return out, nil
}

func methodOrder(m string) int {
	switch m {
	case "level-up":
		return 0
	case "machine":
		return 1
	case "egg":
		return 2
	case "tutor":
		return 3
	default:
		return 9
	}
}

// ReferencedMoveIDs returns unique move IDs across all learn methods.
func ReferencedMoveIDs(p domain.Pokemon) []int {
	seen := map[int]struct{}{}
	var ids []int
	for _, lm := range p.Moves {
		if lm.MoveID <= 0 {
			continue
		}
		if len(lm.Learnings) == 0 {
			continue
		}
		if _, ok := seen[lm.MoveID]; ok {
			continue
		}
		seen[lm.MoveID] = struct{}{}
		ids = append(ids, lm.MoveID)
	}
	return ids
}
