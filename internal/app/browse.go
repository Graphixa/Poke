package app

import (
	"context"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	spritechafa "github.com/Graphixa/Pokedex/internal/render/sprite/chafa"
	spriteimage "github.com/Graphixa/Pokedex/internal/render/sprite/image"
	"github.com/Graphixa/Pokedex/internal/render/sprite/placeholder"
	"github.com/Graphixa/Pokedex/internal/search"
	"github.com/Graphixa/Pokedex/internal/tui"
	"github.com/Graphixa/Pokedex/internal/tui/style"
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
	*sprite.Engine
}

func (t tuiSprites) Render(ctx context.Context, pokemon domain.Pokemon, cells int) (string, bool) {
	if t.Engine == nil {
		return "", true
	}
	res := t.Engine.Render(ctx, pokemon, cells)
	return res.ANSI, res.UsePlaceholder
}

func (t tuiSprites) ClearNative() {
	if t.Engine != nil {
		t.Engine.ClearNative()
	}
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

	img := spriteimage.New()
	caps := sprite.DetectCapabilities(img.Available())
	colorMode := sprite.ColorMode(svc.Config.Render.Color)
	if colorMode == "" || colorMode == sprite.ColorAuto {
		if caps.TrueColor {
			colorMode = sprite.ColorTrueColor
		} else {
			colorMode = sprite.Color256
		}
	}
	sourceKind := sprite.SpriteSource(svc.Config.Render.SpriteSource)
	if sourceKind == "" {
		sourceKind = sprite.SourcePixel
	}

	engine := &sprite.Engine{
		Source: &sprite.SourceResolver{
			Store: svc.Cache,
			API:   svc.API,
		},
		Modes: sprite.NewModeResolver(
			img,
			spritechafa.Renderer{},
			placeholder.Renderer{},
		),
		Artefacts:     svc.Cache,
		Caps:          caps,
		Preference:    sprite.RenderMode(svc.Config.EffectiveSpriteMode()),
		Color:         colorMode,
		SourceKind:    sourceKind,
		Background:    style.BaseRGBA(),
		ClearNativeFn: spriteimage.ClearBestEffort,
		OnNativeFail:  img.Disable,
	}

	return tui.Run(ctx, tui.Options{
		Index:              search.Build(index),
		Details:            tuiDetails{NewDetailService(svc.Cache)},
		Sprites:            tuiSprites{engine},
		Selected:           req.Selected,
		Prefill:            req.Prefill,
		Context:            ctx,
		MinWidthForSprite:  svc.Config.TUI.MinWidthForSprite,
		MinHeightForSprite: svc.Config.TUI.MinHeightForSprite,
		DisableSprite:      !svc.Config.TUI.ShowSprite || svc.Config.EffectiveSpriteMode() == "off",
	})
}
