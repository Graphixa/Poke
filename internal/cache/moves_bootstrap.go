package cache

import (
	"github.com/Graphixa/Pokedex/internal/dataset"
	"github.com/Graphixa/Pokedex/internal/domain"
)

// referencedMoveIDs returns unique move IDs across all learn methods.
func referencedMoveIDs(pokemon domain.Pokemon) []int {
	return dataset.ReferencedMoveIDs(pokemon)
}
