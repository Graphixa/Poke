package cache

import "github.com/Graphixa/Pokedex/internal/domain"

// levelUpMoveIDs returns unique move IDs from the latest level-up version group.
// Bootstrap only caches moves the browse UI can display.
func levelUpMoveIDs(pokemon domain.Pokemon) []int {
	bestOrder := -1
	bestGroup := ""
	for _, lm := range pokemon.Moves {
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
	var ids []int
	for _, lm := range pokemon.Moves {
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
			ids = append(ids, lm.MoveID)
			break
		}
	}
	return ids
}
