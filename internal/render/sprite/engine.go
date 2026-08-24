package sprite

import (
	"context"
	"image"
	"image/color"

	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/domain"
)

// ArtefactCache stores derived text-renderer ANSI strings.
type ArtefactCache interface {
	Halfblock(ctx context.Context, key string) (string, error)
	StoreHalfblock(ctx context.Context, key, ansi string) error
}

// ClearNativeFunc clears prior native image placements (best-effort).
type ClearNativeFunc func()

// Engine orchestrates PNG resolution, mode selection, rendering, and caching.
type Engine struct {
	Source     *SourceResolver
	Modes      *ModeResolver
	Artefacts  ArtefactCache
	Caps       Capabilities
	Preference RenderMode
	Color      ColorMode
	SourceKind SpriteSource
	Background color.RGBA
	// FrameW/FrameH override the fixed frame; zero uses FrameWidth/FrameHeight.
	FrameW int
	FrameH int
	// ClearNativeFn clears native placements; optional.
	ClearNativeFn ClearNativeFunc
	// OnNativeFail is called when native image rendering fails (session fallback).
	OnNativeFail func()
}

// RenderResult is either ANSI cell-art or a placeholder request.
type RenderResult struct {
	ANSI           string
	UsePlaceholder bool
	CacheKey       string
	Backend        string
}

// ClearNative clears prior native image placements (best-effort).
func (e *Engine) ClearNative() {
	if e != nil && e.ClearNativeFn != nil {
		e.ClearNativeFn()
	}
}

// Render converts front_default into the fixed sprite frame. cells is accepted
// for TUI compatibility but the fixed (bucketed) frame size is used.
func (e *Engine) Render(ctx context.Context, pokemon domain.Pokemon, cells int) RenderResult {
	if e == nil {
		return RenderResult{UsePlaceholder: true}
	}
	if err := ctx.Err(); err != nil {
		return RenderResult{UsePlaceholder: true}
	}
	_ = cells
	fw, fh := e.frameSize()
	label := displayName(pokemon)

	if e.Preference == ModeOff {
		return e.placeholderResult(label, fw, fh)
	}

	if e.Source == nil {
		return e.placeholderResult(label, fw, fh)
	}
	res := e.Source.Resolve(ctx, pokemon)
	if res.UsePlaceholder || len(res.PNG) == 0 {
		return e.placeholderResult(label, fw, fh)
	}
	if err := ctx.Err(); err != nil {
		return RenderResult{UsePlaceholder: true}
	}

	img, err := DecodeImage(res.PNG)
	if err != nil {
		debuglog.Renderf("sprite decode fail pokemon=%d err=%v", pokemon.ID, err)
		return e.placeholderResult(label, fw, fh)
	}

	opts := e.renderOptions(label, fw, fh, res.ContentHash)
	return e.renderChain(ctx, img, opts, pokemon.ID)
}

func (e *Engine) renderChain(ctx context.Context, img image.Image, opts RenderOptions, pokemonID int) RenderResult {
	if e.Modes == nil || len(e.Modes.Renderers) == 0 {
		return e.placeholderResult(opts.Label, opts.Width, opts.Height)
	}

	start := e.Modes.Resolve(e.Caps, e.Preference)
	started := false
	for _, rend := range e.Modes.Renderers {
		if !started {
			if start == nil || rend.Name() != start.Name() {
				continue
			}
			started = true
		}
		if !rend.Supported(e.Caps) {
			continue
		}
		if err := ctx.Err(); err != nil {
			return RenderResult{UsePlaceholder: true}
		}

		// Cache text backends (not native image).
		key := ""
		if rend.Name() != "image" && rend.Name() != "placeholder" && opts.ContentHash != "" {
			color := effectiveColor(opts.ColorMode, e.Caps)
			key = CacheKey(opts.ContentHash, opts.Source, rend.Name(), opts.Width, opts.Height, color)
			if e.Artefacts != nil {
				if cached, err := e.Artefacts.Halfblock(ctx, key); err == nil && cached != "" {
					debuglog.Renderf("%s cache hit key=%s frame=%dx%d", rend.Name(), key, opts.Width, opts.Height)
					return RenderResult{ANSI: cached, CacheKey: key, Backend: rend.Name()}
				}
			}
			opts.ColorMode = color
		}

		frame, err := rend.Render(ctx, img, opts)
		if err != nil || frame.IsEmpty() {
			debuglog.Renderf("%s render fail pokemon=%d err=%v; falling back", rend.Name(), pokemonID, err)
			if rend.Name() == "image" && e.OnNativeFail != nil {
				e.OnNativeFail()
			}
			continue
		}
		if err := ctx.Err(); err != nil {
			return RenderResult{UsePlaceholder: true}
		}
		if key != "" && e.Artefacts != nil && !frame.Native {
			_ = e.Artefacts.StoreHalfblock(ctx, key, frame.Text)
		}
		debuglog.Renderf("%s render pokemon=%d frame=%dx%d key=%s", rend.Name(), pokemonID, opts.Width, opts.Height, key)
		if frame.Backend == "placeholder" {
			return RenderResult{ANSI: frame.Text, UsePlaceholder: true, Backend: "placeholder"}
		}
		return RenderResult{ANSI: frame.Text, CacheKey: key, Backend: frame.Backend}
	}
	return e.placeholderResult(opts.Label, opts.Width, opts.Height)
}

func (e *Engine) placeholderResult(label string, w, h int) RenderResult {
	if e.Modes != nil {
		for _, rend := range e.Modes.Renderers {
			if rend.Name() != "placeholder" {
				continue
			}
			frame, err := rend.Render(context.Background(), nil, RenderOptions{
				Width: w, Height: h, Label: label,
			})
			if err == nil && frame.Text != "" {
				return RenderResult{ANSI: frame.Text, UsePlaceholder: true, Backend: "placeholder"}
			}
		}
	}
	return RenderResult{UsePlaceholder: true, Backend: "placeholder"}
}

func (e *Engine) renderOptions(label string, fw, fh int, hash string) RenderOptions {
	bg := e.Background
	if bg.A == 0 {
		bg = DefaultBackground
	}
	source := e.SourceKind
	if source == "" {
		source = SourcePixel
	}
	color := e.Color
	if color == "" {
		color = ColorAuto
	}
	return RenderOptions{
		Width:           fw,
		Height:          fh,
		PreserveAR:      true,
		ColorMode:       color,
		Background:      BackgroundTheme,
		BackgroundColor: bg,
		AlphaThreshold:  16,
		Source:          source,
		Label:           label,
		ContentHash:     hash,
	}
}

func (e *Engine) frameSize() (w, h int) {
	w, h = e.FrameW, e.FrameH
	if w < 1 {
		w = FrameWidth
	}
	if h < 1 {
		h = FrameHeight
	}
	return BucketSize(w), BucketSize(h)
}

func effectiveColor(mode ColorMode, caps Capabilities) ColorMode {
	switch mode {
	case ColorTrueColor:
		if caps.TrueColor {
			return ColorTrueColor
		}
		return Color256
	case Color256:
		return Color256
	case ColorMono:
		return ColorMono
	default: // auto
		if caps.TrueColor {
			return ColorTrueColor
		}
		return Color256
	}
}

func displayName(p domain.Pokemon) string {
	if p.DisplayName != "" {
		return p.DisplayName
	}
	if p.Slug != "" {
		return p.Slug
	}
	return "???"
}
