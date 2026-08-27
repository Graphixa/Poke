package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
)

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
}

func loadDossierCmd(ctx context.Context, src DetailSource, gen, speciesID int) tea.Cmd {
	if src == nil {
		return nil
	}
	return func() tea.Msg {
		d, err := src.LoadDossier(ctx, speciesID)
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
		ansi, placeholder, native := src.Render(ctx, pokemon, cells)
		return spriteMsg{
			reqID:       reqID,
			gen:         gen,
			speciesID:   speciesID,
			cells:       cells,
			ansi:        ansi,
			placeholder: placeholder || ansi == "",
			native:      native && !placeholder && ansi != "",
		}
	}
}

type warmProgressMsg struct {
	Done    int
	Total   int
	Message string
	closed  bool
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
		return warmProgressMsg{Done: p.Done, Total: p.Total, Message: p.Message}
	}
}
