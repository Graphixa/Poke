package cache

import (
	"github.com/Graphixa/Poke/internal/dataset"
	"github.com/Graphixa/Poke/internal/domain"
)

// referencedMoveIDs returns unique move IDs across all learn methods.
func referencedMoveIDs(pokemon domain.Pokemon) []int {
	return dataset.ReferencedMoveIDs(pokemon)
}
