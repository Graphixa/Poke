package tui

import (
	"context"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

// LevelUpMove is a Moves-tab row (level + move metadata).
type LevelUpMove struct {
	Level int
	Move  domain.Move
}

// DetailSource loads dossier data for the Enter/detail overlay.
// Browse highlight must not call LoadDossier — only Enter/i does.
type DetailSource interface {
	LoadDossierCore(ctx context.Context, speciesID int, slug string) (domain.Dossier, bool, error)
	LoadDossierExtras(ctx context.Context, speciesID int, slug string, core domain.Dossier) (domain.Dossier, error)
}

// PokemonSource loads default-variety Pokémon (including sprite URLs) for art.
type PokemonSource interface {
	PokemonForSprite(ctx context.Context, speciesID int) (domain.Pokemon, error)
}

// DossierWarmer pre-assembles dossiers or core pairs in the background.
type DossierWarmer interface {
	Prioritize(speciesID int)
	SetNeighborsFromList(list []domain.SpeciesSummary, cursor, radius int)
}

// SpriteRenderer produces terminal sprite art for a Pokémon panel.
// Implementations must be safe to call from Bubble Tea commands and honour ctx.
type SpriteRenderer interface {
	// LookupCached is a cheap mem/disk artefact probe. It must not HTTP, decode PNG, or encode.
	LookupCached(ctx context.Context, pokemon domain.Pokemon, cells int) (sprite.RenderResult, bool)
	Render(ctx context.Context, pokemon domain.Pokemon, cells int) (ansi string, usePlaceholder, native bool, assetKind string)
	// RenderCold encodes a frame. Expensive; call only after LookupCached misses.
	RenderCold(ctx context.Context, pokemon domain.Pokemon, cells int) sprite.RenderResult
	// ClearNative clears prior native graphics placements (Kitty/iTerm/Sixel). May be a no-op.
	ClearNative()
	// ClearNativeRegion emits a scoped native clear for a WxH cell art box.
	ClearNativeRegion(cellsW, cellsH int) string
	// InvalidateSpecies drops in-memory encode caches for a species (artwork pop-in).
	InvalidateSpecies(speciesID int)
	// PersistFrame enqueues an async native gzip write (off the display path).
	PersistFrame(ctx context.Context, cacheKey, frame string) error
}
