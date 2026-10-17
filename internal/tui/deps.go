package tui

import (
	"context"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

// DetailSource loads dossier data for the Enter/detail overlay and browse glance About.
// Browse highlight may call LoadDossierCore only; extras load on Enter.
type DetailSource interface {
	LoadDossierCore(ctx context.Context, speciesID int, slug string) (domain.Dossier, bool, error)
	LoadDossierExtras(ctx context.Context, speciesID int, slug string, core domain.Dossier) (domain.Dossier, error)
}

// CatalogSource supplies dataset catalogs for dossier form/game controls.
type CatalogSource interface {
	CatalogVersions() ([]domain.Version, error)
	VersionGroupIDForVersion(versionID int) (int, bool)
	SelectableFormsForSpecies(speciesID int) ([]domain.SelectableForm, error)
	FormOptionsForGame(speciesID, gameVersionID int) ([]domain.FormOption, error)
	ResolveDossierContext(speciesID, pokemonID, formID, gameVersionID int) (domain.ResolvedDossierContext, error)
	LevelUpMovesForGroup(pokemonID, versionGroupID int) ([]domain.LevelUpMove, error)
	MovesForGroup(pokemonID, versionGroupID int) ([]domain.LevelUpMove, error)
	PokemonByID(ctx context.Context, pokemonID int) (domain.Pokemon, error)
	TypeChart() (map[string]domain.TypeInfo, error)
	AbilityByID(id int) (domain.Ability, bool)
	AbilityBySlug(slug string) (domain.Ability, bool)
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
