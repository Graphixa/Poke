package sprites

import (
	"context"
	"image/color"

	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/halfblock"
)

// ArtefactCache stores derived half-block ANSI strings.
type ArtefactCache interface {
	Halfblock(ctx context.Context, key string) (string, error)
	StoreHalfblock(ctx context.Context, key, ansi string) error
}

// Renderer resolves sprites and converts them to half-block ANSI asynchronously-safe.
type Renderer struct {
	Resolver   *Resolver
	Artefacts  ArtefactCache
	Background color.RGBA
	Enabled    bool
	Palette    halfblock.PaletteMode
}

// RenderResult is either ANSI cell-art or a placeholder request.
type RenderResult struct {
	ANSI           string
	UsePlaceholder bool
	CacheKey       string
}

// Render converts front_default for the given cell size. It never panics;
// missing/invalid sprites and conversion failures yield UsePlaceholder.
func (r *Renderer) Render(ctx context.Context, pokemon domain.Pokemon, cells int) RenderResult {
	if r == nil || !r.Enabled || cells < 1 {
		return RenderResult{UsePlaceholder: true}
	}
	if err := ctx.Err(); err != nil {
		return RenderResult{UsePlaceholder: true}
	}
	res := r.Resolver.Resolve(ctx, pokemon)
	if res.UsePlaceholder || len(res.PNG) == 0 {
		return RenderResult{UsePlaceholder: true}
	}
	if err := ctx.Err(); err != nil {
		return RenderResult{UsePlaceholder: true}
	}

	bg := r.Background
	if bg.A == 0 {
		bg = halfblock.DefaultBackground
	}
	palette := r.Palette
	if palette == "" {
		palette = halfblock.PaletteTrueColor
	}
	opts := halfblock.Options{
		Width:      cells,
		Height:     cells,
		Palette:    palette,
		Background: bg,
	}
	key := halfblock.CacheKey(res.ContentHash, opts)

	if r.Artefacts != nil {
		if cached, err := r.Artefacts.Halfblock(ctx, key); err == nil && cached != "" {
			debuglog.Renderf("halfblock cache hit key=%s cells=%d", key, cells)
			return RenderResult{ANSI: cached, CacheKey: key}
		}
	}

	ansi, err := halfblock.ConvertPNG(res.PNG, opts)
	if err != nil || ansi == "" {
		debuglog.Renderf("halfblock convert fail pokemon=%d cells=%d err=%v", pokemon.ID, cells, err)
		return RenderResult{UsePlaceholder: true}
	}
	if err := ctx.Err(); err != nil {
		debuglog.Renderf("halfblock cancelled pokemon=%d", pokemon.ID)
		return RenderResult{UsePlaceholder: true}
	}
	if r.Artefacts != nil {
		_ = r.Artefacts.StoreHalfblock(ctx, key, ansi)
	}
	debuglog.Renderf("halfblock render pokemon=%d cells=%d key=%s", pokemon.ID, cells, key)
	return RenderResult{ANSI: ansi, CacheKey: key}
}
