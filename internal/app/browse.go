package app

import (
	"context"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/halfblock"
	"github.com/Graphixa/Pokedex/internal/render/sprites"
	"github.com/Graphixa/Pokedex/internal/search"
	"github.com/Graphixa/Pokedex/internal/tui"
)

type tuiDetails struct {
	*DetailService
}

func (t tuiDetails) Load(ctx context.Context, speciesID int) (domain.Pokemon, domain.Species, error) {
	d, err := t.DetailService.Load(ctx, speciesID)
	if err != nil {
		return domain.Pokemon{}, domain.Species{}, err
	}
	return d.Pokemon, d.Species, nil
}

func (t tuiDetails) LevelUpMoves(ctx context.Context, pokemon domain.Pokemon) ([]tui.LevelUpMove, error) {
	rows, err := t.DetailService.LevelUpMoves(ctx, pokemon)
	if err != nil {
		return nil, err
	}
	out := make([]tui.LevelUpMove, len(rows))
	for i, row := range rows {
		out[i] = tui.LevelUpMove{Level: row.Level, Move: row.Move}
	}
	return out, nil
}

type tuiSprites struct {
	*sprites.Renderer
}

func (t tuiSprites) Render(ctx context.Context, pokemon domain.Pokemon, cells int) (string, bool) {
	if t.Renderer == nil {
		return "", true
	}
	res := t.Renderer.Render(ctx, pokemon, cells)
	return res.ANSI, res.UsePlaceholder
}

func (h *Handler) openBrowse(ctx context.Context, req BrowseRequest) error {
	svc, err := h.Services()
	if err != nil {
		return err
	}
	ApplyTheme(h.Paths, svc.Config)
	index, _, err := svc.Cache.EnsureIndex(ctx, nil)
	if err != nil {
		return err
	}
	renderer := &sprites.Renderer{
		Resolver: &sprites.Resolver{
			Store: svc.Cache,
			API:   svc.API,
		},
		Artefacts: svc.Cache,
		Enabled:   svc.Config.Render.Halfblock,
		Palette:   halfblock.DetectPalette(),
	}
	return tui.Run(ctx, tui.Options{
		Index:    search.Build(index),
		Details:  tuiDetails{NewDetailService(svc.Cache)},
		Sprites:  tuiSprites{renderer},
		Selected: req.Selected,
		Prefill:  req.Prefill,
		Context:  ctx,
	})
}
