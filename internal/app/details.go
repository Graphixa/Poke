package app

import (
	"context"
	"fmt"
	"sort"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/output"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

// PokemonDetail is the aggregated pokemon + species pair for info/stats.
type PokemonDetail struct {
	Pokemon domain.Pokemon
	Species domain.Species
}

// DetailService loads cache-backed Pokémon details without exposing API DTOs.
type DetailService struct {
	cache *cache.Service
}

func NewDetailService(cache *cache.Service) *DetailService {
	return &DetailService{cache: cache}
}

// LoadDossierCore returns species+pokemon for fast dialog paint.
func (d *DetailService) LoadDossierCore(ctx context.Context, speciesID int, slug string) (domain.Dossier, bool, error) {
	ctx = cache.WithInteractivePriority(ctx)
	res, err := d.cache.DossierCore(ctx, speciesID, slug)
	if err != nil {
		return domain.Dossier{}, false, err
	}
	return res.Dossier, res.Complete, nil
}

// LoadDossierExtras fills evolution, encounters, and moves after core.
func (d *DetailService) LoadDossierExtras(ctx context.Context, speciesID int, slug string, core domain.Dossier) (domain.Dossier, error) {
	ctx = cache.WithInteractivePriority(ctx)
	return d.cache.DossierExtras(ctx, speciesID, slug, core)
}

// LoadDossier returns the full per-species dossier (Enter/detail path).
func (d *DetailService) LoadDossier(ctx context.Context, speciesID int) (domain.Dossier, error) {
	return d.cache.Dossier(ctx, speciesID)
}

// WarmCore caches pokemon+species for browse headers without full dossier assemble.
func (d *DetailService) WarmCore(ctx context.Context, speciesID int) error {
	return d.cache.WarmCore(ctx, speciesID)
}

// LoadDossierWithSlug prefers a known slug for the on-disk dossier path.
func (d *DetailService) LoadDossierWithSlug(ctx context.Context, speciesID int, slug string) (domain.Dossier, error) {
	return d.cache.DossierWithSlug(ctx, speciesID, slug)
}

// Load fetches species then the default Pokémon variety.
// Preferring dossier when present keeps Enter/CLI paths coherent.
func (d *DetailService) Load(ctx context.Context, speciesID int) (PokemonDetail, error) {
	dossier, err := d.cache.Dossier(ctx, speciesID)
	if err == nil {
		return PokemonDetail{Pokemon: dossier.Pokemon, Species: dossier.Species}, nil
	}
	return d.loadSpeciesPokemon(ctx, speciesID)
}

// PokemonForSprite loads default-variety pokemon for sprite warm without assembling a dossier.
func (d *DetailService) PokemonForSprite(ctx context.Context, speciesID int) (domain.Pokemon, error) {
	// Prefer pokemon/{speciesID} first — national-dex default variety usually shares
	// that id, so we avoid a Species round-trip on the warm/artwork path.
	if p, err := d.cache.Pokemon(ctx, speciesID); err == nil && (p.Sprites.FrontDefault != "" || p.Sprites.OfficialArtwork != "") {
		stampArtworkCDN(&p)
		stampSpeciesSlug(ctx, d.cache, speciesID, &p)
		return p, nil
	}
	detail, err := d.loadSpeciesPokemon(ctx, speciesID)
	if err != nil {
		return domain.Pokemon{}, err
	}
	stampArtworkCDN(&detail.Pokemon)
	if detail.Pokemon.Slug == "" {
		detail.Pokemon.Slug = detail.Species.Slug
	}
	return detail.Pokemon, nil
}

func stampSpeciesSlug(ctx context.Context, svc *cache.Service, speciesID int, p *domain.Pokemon) {
	if p == nil || p.Slug != "" || speciesID < 1 {
		return
	}
	if sp, err := svc.Species(ctx, speciesID); err == nil && sp.Slug != "" {
		p.Slug = sp.Slug
	}
}

// stampArtworkCDN ensures interactive resolve prefers official artwork CDN even
// when the cached pokemon row only has FrontDefault.
func stampArtworkCDN(p *domain.Pokemon) {
	if p == nil || p.ID < 1 {
		return
	}
	if p.Sprites.OfficialArtwork == "" {
		p.Sprites.OfficialArtwork = sprite.CDNArtworkURL(p.ID)
	}
}

func (d *DetailService) loadSpeciesPokemon(ctx context.Context, speciesID int) (PokemonDetail, error) {
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
func (d *DetailService) LevelUpMoves(ctx context.Context, pokemon domain.Pokemon) ([]domain.LevelUpMove, error) {
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

	out := make([]domain.LevelUpMove, 0, len(rows))
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
		out = append(out, domain.LevelUpMove{Level: row.level, Move: move})
	}
	return out, nil
}
