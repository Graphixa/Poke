package app

import (
	"context"
	"fmt"
	"sort"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/output"
)

// detailCache is the subset of cache.Service used for inline detail aggregation.
type detailCache interface {
	Pokemon(context.Context, int) (domain.Pokemon, error)
	Species(context.Context, int) (domain.Species, error)
	EvolutionChain(context.Context, int) (domain.EvolutionTree, error)
	Encounters(context.Context, int) ([]domain.Encounter, error)
	Move(context.Context, int) (domain.Move, error)
}

// PokemonDetail is the aggregated pokemon + species pair for info/stats.
type PokemonDetail struct {
	Pokemon domain.Pokemon
	Species domain.Species
}

// LevelUpMove is a level-up learnset row for the Moves tab.
type LevelUpMove struct {
	Level int
	Move  domain.Move
}

// DetailService loads cache-backed Pokémon details without exposing API DTOs.
type DetailService struct {
	cache detailCache
}

func NewDetailService(cache detailCache) *DetailService {
	return &DetailService{cache: cache}
}

// Load fetches species then the default Pokémon variety.
func (d *DetailService) Load(ctx context.Context, speciesID int) (PokemonDetail, error) {
	species, err := d.cache.Species(ctx, speciesID)
	if err != nil {
		return PokemonDetail{}, fmt.Errorf("species %d: %w", speciesID, err)
	}
	pokemonID := species.DefaultPokemonID
	if pokemonID == 0 {
		pokemonID = speciesID
	}
	pokemon, err := d.cache.Pokemon(ctx, pokemonID)
	if err != nil {
		return PokemonDetail{}, fmt.Errorf("pokemon %d: %w", pokemonID, err)
	}
	if species.DisplayName != "" {
		pokemon.DisplayName = species.DisplayName
	}
	return PokemonDetail{Pokemon: pokemon, Species: species}, nil
}

// Evolution loads the evolution chain for a species (lazy detail path).
func (d *DetailService) Evolution(ctx context.Context, species domain.Species) (domain.EvolutionTree, error) {
	if species.EvolutionChainID == 0 {
		return domain.EvolutionTree{}, fmt.Errorf("evolution data unavailable for %s", species.DisplayName)
	}
	tree, err := d.cache.EvolutionChain(ctx, species.EvolutionChainID)
	if err != nil {
		return domain.EvolutionTree{}, fmt.Errorf("evolution chain %d: %w", species.EvolutionChainID, err)
	}
	return tree, nil
}

// Encounters loads encounter rows for the default Pokémon variety.
func (d *DetailService) Encounters(ctx context.Context, pokemonID int) ([]domain.Encounter, error) {
	enc, err := d.cache.Encounters(ctx, pokemonID)
	if err != nil {
		return nil, fmt.Errorf("encounters %d: %w", pokemonID, err)
	}
	return enc, nil
}

// LevelUpMoves returns level-up moves from the latest version group with data.
func (d *DetailService) LevelUpMoves(ctx context.Context, pokemon domain.Pokemon) ([]LevelUpMove, error) {
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
		return nil, nil
	}

	type pending struct {
		level  int
		moveID int
		slug   string
	}
	var rows []pending
	for _, lm := range pokemon.Moves {
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
			rows = append(rows, pending{level: learn.Level, moveID: lm.MoveID, slug: lm.Slug})
			break
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].level != rows[j].level {
			return rows[i].level < rows[j].level
		}
		return rows[i].slug < rows[j].slug
	})

	out := make([]LevelUpMove, 0, len(rows))
	for _, row := range rows {
		move, err := d.cache.Move(ctx, row.moveID)
		if err != nil {
			move = domain.Move{
				ID:          row.moveID,
				Slug:        row.slug,
				DisplayName: output.TitleFromSlug(row.slug),
			}
		}
		if move.DisplayName == "" {
			move.DisplayName = output.TitleFromSlug(move.Slug)
		}
		out = append(out, LevelUpMove{Level: row.level, Move: move})
	}
	return out, nil
}
