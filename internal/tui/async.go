package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
)

type dossierCoreMsg struct {
	gen       int
	speciesID int
	slug      string
	dossier   domain.Dossier
	complete  bool
	err       error
}

type dossierExtrasMsg struct {
	gen       int
	speciesID int
	dossier   domain.Dossier
	err       error
}

type dossierMsg struct {
	gen       int
	speciesID int
	dossier   domain.Dossier
	err       error
}

type spriteMsg struct {
	reqID       int
	gen         int
	speciesID   int
	cells       int
	ansi        string
	placeholder bool
	native      bool
	assetKind   string
}

// spritePrefetchRadius is how many list rows above/below the cursor to warm
// via background render calls (engine/disk cache only).
const spritePrefetchRadius = 1

// spritePrefetchBurst limits how many new neighbor encodes start per selection.
const spritePrefetchBurst = 2

// warmHintMinInterval throttles footer "warming N/M" updates to reduce Sixel wipes.
const warmHintMinInterval = 250 * time.Millisecond

type spritePrefetchMsg struct {
	gen         int
	speciesID   int
	cells       int
	ansi        string
	placeholder bool
	native      bool
	assetKind   string
	pokemon     domain.Pokemon
}

func loadDossierCoreCmd(ctx context.Context, src DetailSource, gen, speciesID int, slug string) tea.Cmd {
	if src == nil {
		return nil
	}
	return func() tea.Msg {
		if err := ctx.Err(); err != nil {
			return dossierCoreMsg{gen: gen, speciesID: speciesID, slug: slug, err: err}
		}
		d, complete, err := src.LoadDossierCore(ctx, speciesID, slug)
		return dossierCoreMsg{gen: gen, speciesID: speciesID, slug: slug, dossier: d, complete: complete, err: err}
	}
}

func loadDossierExtrasCmd(ctx context.Context, src DetailSource, gen, speciesID int, slug string, core domain.Dossier) tea.Cmd {
	if src == nil {
		return nil
	}
	return func() tea.Msg {
		if err := ctx.Err(); err != nil {
			return dossierExtrasMsg{gen: gen, speciesID: speciesID, err: err}
		}
		d, err := src.LoadDossierExtras(ctx, speciesID, slug, core)
		return dossierExtrasMsg{gen: gen, speciesID: speciesID, dossier: d, err: err}
	}
}

func loadDossierCmd(ctx context.Context, src DetailSource, gen, speciesID int) tea.Cmd {
	if src == nil {
		return nil
	}
	return func() tea.Msg {
		d, complete, err := src.LoadDossierCore(ctx, speciesID, "")
		if err != nil || !complete {
			if err != nil {
				return dossierMsg{gen: gen, speciesID: speciesID, err: err}
			}
			extras, err := src.LoadDossierExtras(ctx, speciesID, "", d)
			if err != nil {
				return dossierMsg{gen: gen, speciesID: speciesID, err: err}
			}
			d = extras
		}
		return dossierMsg{gen: gen, speciesID: speciesID, dossier: d, err: err}
	}
}

func loadSpriteCmd(ctx context.Context, src SpriteRenderer, pokemonSrc PokemonSource, reqID, gen, speciesID, cells int, pokemon domain.Pokemon) tea.Cmd {
	if src == nil || cells < 1 {
		return func() tea.Msg {
			return spriteMsg{reqID: reqID, gen: gen, speciesID: speciesID, cells: cells, placeholder: true}
		}
	}
	return func() tea.Msg {
		if pokemonSrc != nil && pokemon.Sprites.FrontDefault == "" && pokemon.Sprites.OfficialArtwork == "" {
			if p, err := pokemonSrc.PokemonForSprite(ctx, speciesID); err == nil {
				pokemon = p
			}
		}
		res := src.RenderCold(ctx, pokemon, cells)
		return spriteMsg{
			reqID:       reqID,
			gen:         gen,
			speciesID:   speciesID,
			cells:       cells,
			ansi:        res.ANSI,
			placeholder: res.UsePlaceholder || res.ANSI == "",
			native:      res.Native && !res.UsePlaceholder && res.ANSI != "",
			assetKind:   res.AssetKind,
		}
	}
}

func loadSpritePrefetchCmd(ctx context.Context, src SpriteRenderer, pokemonSrc PokemonSource, gen, speciesID, cells int, pokemon domain.Pokemon) tea.Cmd {
	if src == nil || cells < 1 || speciesID < 1 {
		return nil
	}
	return func() tea.Msg {
		if err := ctx.Err(); err != nil {
			return spritePrefetchMsg{gen: gen, speciesID: speciesID, cells: cells, placeholder: true}
		}
		if pokemonSrc != nil && pokemon.Sprites.FrontDefault == "" && pokemon.Sprites.OfficialArtwork == "" {
			if p, err := pokemonSrc.PokemonForSprite(ctx, speciesID); err == nil {
				pokemon = p
			}
		}
		if err := ctx.Err(); err != nil {
			return spritePrefetchMsg{gen: gen, speciesID: speciesID, cells: cells, placeholder: true}
		}
		ansi, placeholder, native, kind := src.Render(ctx, pokemon, cells)
		return spritePrefetchMsg{
			gen:         gen,
			speciesID:   speciesID,
			cells:       cells,
			ansi:        ansi,
			placeholder: placeholder || ansi == "",
			native:      native && !placeholder && ansi != "",
			assetKind:   kind,
			pokemon:     pokemon,
		}
	}
}

type warmProgressMsg struct {
	Done      int
	Total     int
	Message   string
	SpeciesID int
	Stage     string
	Cells     int
	closed    bool
}

func listenWarmProgress(ch <-chan WarmProgress) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return warmProgressMsg{closed: true}
		}
		return warmProgressMsg{
			Done: p.Done, Total: p.Total, Message: p.Message,
			SpeciesID: p.SpeciesID, Stage: p.Stage, Cells: p.Cells,
		}
	}
}
