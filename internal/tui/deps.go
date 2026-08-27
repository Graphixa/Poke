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

// DetailSource loads dossier data for the Enter/detail overlay.
// Browse highlight must not call LoadDossier — only Enter/i does.
type DetailSource interface {
	LoadDossier(ctx context.Context, speciesID int) (domain.Dossier, error)
}

// PokemonSource loads default-variety Pokémon (including sprite URLs) for art.
type PokemonSource interface {
	PokemonForSprite(ctx context.Context, speciesID int) (domain.Pokemon, error)
}

// SpriteRenderer produces terminal sprite art for a Pokémon panel.
// Implementations must be safe to call from Bubble Tea commands and honour ctx.
type SpriteRenderer interface {
	Render(ctx context.Context, pokemon domain.Pokemon, cells int) (ansi string, usePlaceholder, native bool, assetKind string)
	// ClearNative clears prior native graphics placements (Kitty/iTerm/Sixel). May be a no-op.
	ClearNative()
	// InvalidateSpecies drops in-memory encode caches for a species (artwork pop-in).
	InvalidateSpecies(speciesID int)
}
