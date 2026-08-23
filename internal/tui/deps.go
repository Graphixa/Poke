package tui

import (
	"context"

	"github.com/Graphixa/Pokedex/internal/domain"
)

// LevelUpMove is a Moves-tab row (level + move metadata).
type LevelUpMove struct {
	Level int
	Move  domain.Move
}

// DetailSource loads selection and lazy-tab data for the browse TUI.
// Implementations must not expose transport DTOs.
type DetailSource interface {
	Load(ctx context.Context, speciesID int) (domain.Pokemon, domain.Species, error)
	Evolution(ctx context.Context, species domain.Species) (domain.EvolutionTree, error)
	Encounters(ctx context.Context, pokemonID int) ([]domain.Encounter, error)
	LevelUpMoves(ctx context.Context, pokemon domain.Pokemon) ([]LevelUpMove, error)
}

// SpriteRenderer produces half-block ANSI for a Pokémon sprite panel.
// Implementations must be safe to call from Bubble Tea commands and honour ctx.
type SpriteRenderer interface {
	Render(ctx context.Context, pokemon domain.Pokemon, cells int) (ansi string, usePlaceholder bool)
}
