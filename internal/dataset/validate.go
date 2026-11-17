package dataset

import (
	"fmt"
	"sort"

	"github.com/Graphixa/Pokedex/internal/domain"
)

// Validate checks an in-memory builder before aggregate files are written.
// expected is the species/browse count. Pokemon may exceed that when varieties are stored.
func Validate(b *Builder, expected int, browse []domain.BrowseEntry) error {
	if b == nil {
		return fmt.Errorf("dataset: nil builder")
	}
	if expected < 1 {
		return fmt.Errorf("dataset: expected count %d", expected)
	}
	if len(b.Species) != expected {
		return fmt.Errorf("dataset: species %d != expected %d", len(b.Species), expected)
	}
	if len(browse) != expected {
		return fmt.Errorf("dataset: browse %d != expected %d", len(browse), expected)
	}
	if len(b.Pokemon) < expected {
		return fmt.Errorf("dataset: pokemon %d < expected %d", len(b.Pokemon), expected)
	}
	for _, entry := range browse {
		if entry.ID < 1 {
			return fmt.Errorf("dataset: browse entry has invalid id %d", entry.ID)
		}
		sp, ok := b.Species[entry.ID]
		if !ok {
			return fmt.Errorf("dataset: browse id %d missing species", entry.ID)
		}
		pokeID := sp.DefaultPokemonID
		if pokeID == 0 {
			pokeID = entry.ID
		}
		if _, ok := b.Pokemon[pokeID]; !ok {
			return fmt.Errorf("dataset: browse id %d missing default pokemon %d", entry.ID, pokeID)
		}
		if _, ok := b.Encounters[pokeID]; !ok {
			return fmt.Errorf("dataset: browse id %d missing encounters key %d", entry.ID, pokeID)
		}
	}
	for id, p := range b.Pokemon {
		if id < 1 || p.ID < 1 {
			return fmt.Errorf("dataset: invalid pokemon id %d", id)
		}
		speciesID := p.SpeciesID
		if speciesID == 0 {
			speciesID = id
		}
		if _, ok := b.Species[speciesID]; !ok {
			return fmt.Errorf("dataset: pokemon %d missing species %d", id, speciesID)
		}
		rows := b.Learnsets[id]
		for _, row := range rows {
			if row.MoveID < 1 {
				return fmt.Errorf("dataset: pokemon %d learnset has invalid move id", id)
			}
			if row.LearnMethod == "" {
				return fmt.Errorf("dataset: pokemon %d learnset missing learn_method", id)
			}
			if _, ok := b.Moves[row.MoveID]; !ok {
				return fmt.Errorf("dataset: pokemon %d learnset references missing move %d", id, row.MoveID)
			}
		}
	}
	for id, sp := range b.Species {
		if id < 1 || sp.ID < 1 {
			return fmt.Errorf("dataset: invalid species id %d", id)
		}
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
	for pokemonID, rows := range b.Learnsets {
		if _, ok := b.Pokemon[pokemonID]; !ok {
			return fmt.Errorf("dataset: learnset references missing pokemon %d", pokemonID)
		}
		sort.SliceStable(rows, func(i, j int) bool {
			if rows[i].VersionGroupID != rows[j].VersionGroupID {
				return rows[i].VersionGroupID < rows[j].VersionGroupID
			}
			if rows[i].LearnMethod != rows[j].LearnMethod {
				return rows[i].LearnMethod < rows[j].LearnMethod
			}
			if rows[i].LevelLearnedAt != rows[j].LevelLearnedAt {
				return rows[i].LevelLearnedAt < rows[j].LevelLearnedAt
			}
			return rows[i].MoveID < rows[j].MoveID
		})
		b.Learnsets[pokemonID] = rows
	}
	for id, f := range b.Forms {
		if id < 1 || f.ID < 1 {
			return fmt.Errorf("dataset: invalid form id %d", id)
		}
		if _, ok := b.Pokemon[f.PokemonID]; !ok {
			return fmt.Errorf("dataset: form %d missing pokemon %d", id, f.PokemonID)
		}
	}
	return nil
}
