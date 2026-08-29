package dataset

import (
	"fmt"
)

// Validate checks an in-memory builder before aggregate files are written.
func Validate(b *Builder, expected int) error {
	if b == nil {
		return fmt.Errorf("dataset: nil builder")
	}
	if expected < 1 {
		return fmt.Errorf("dataset: expected count %d", expected)
	}
	if len(b.Pokemon) != expected {
		return fmt.Errorf("dataset: pokemon %d != expected %d", len(b.Pokemon), expected)
	}
	if len(b.Species) != expected {
		return fmt.Errorf("dataset: species %d != expected %d", len(b.Species), expected)
	}
	if len(b.Encounters) != expected {
		return fmt.Errorf("dataset: encounters %d != expected %d", len(b.Encounters), expected)
	}
	for id, p := range b.Pokemon {
		if _, ok := b.Species[p.SpeciesID]; !ok && p.SpeciesID != 0 {
			if _, ok := b.Species[id]; !ok {
				return fmt.Errorf("dataset: pokemon %d missing species %d", id, p.SpeciesID)
			}
		}
		rows := b.Learnsets[id]
		for _, row := range rows {
			if _, ok := b.Moves[row.MoveID]; !ok {
				return fmt.Errorf("dataset: pokemon %d learnset references missing move %d", id, row.MoveID)
			}
		}
	}
	for id, sp := range b.Species {
		if dp := sp.DefaultPokemonID; dp != 0 {
			if _, ok := b.Pokemon[dp]; !ok {
				return fmt.Errorf("dataset: species %d default pokemon %d missing", id, dp)
			}
		}
		if chain := sp.EvolutionChainID; chain != 0 {
			if _, ok := b.Evolutions[chain]; !ok {
				return fmt.Errorf("dataset: species %d missing evolution chain %d", id, chain)
			}
		}
	}
	return nil
}
