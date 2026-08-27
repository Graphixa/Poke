// Package chafa renders sprites via Chafa mosaic Unicode/ANSI output.
package chafa

import (
	"context"
	"fmt"
	"image"
	"strings"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/uiimage"
	chafago "github.com/ploMP4/chafa-go"
)

// Samples per cell for early resize. Chafa's internal grain is 8×8; an 8×8
// isotropic grid keeps octant/sextant detail while preserving 1:1 subject crops.
const samplesPerCell = 8

// Renderer produces Chafa mosaic (sextant/octant) ANSI frames for the TUI.
type Renderer struct{}

// Name returns the logical renderer class.
func (Renderer) Name() string { return "chafa" }

// Supported reports whether colour block rendering is available.
func (Renderer) Supported(caps sprite.Capabilities) bool {
	return caps.TrueColor || caps.ANSI256
}

// Render converts src into Chafa mosaic ANSI art.
//
// Speed path:
//  1. Composite transparency onto the theme background
//  2. Early-resize with golang.org/x/image/draw to an octant-aligned pixel grid
//  3. Convert with QualityMosaicFast (mosaic glyphs, interactive work factor)
func (r Renderer) Render(ctx context.Context, src image.Image, opts sprite.RenderOptions) (sprite.Frame, error) {
	if err := ctx.Err(); err != nil {
		return sprite.Frame{}, err
	}
	if src == nil {
		return sprite.Frame{}, fmt.Errorf("chafa: nil image")
	}
	w, h := opts.Width, opts.Height
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	bg := opts.BackgroundColor
	if bg.A == 0 {
		bg = sprite.DefaultBackground
	}
	threshold := opts.AlphaThreshold
	if threshold == 0 {
		threshold = 16
	}

	// Sample isotropically. opts.Width×Height should already be cell-aspect
	// corrected (e.g. 32×16) so Stretch into that grid is visually square.
	sampleSide := max(w, h) * samplesPerCell
	quality := uiimage.QualityMosaicFast
	if opts.Quality == sprite.QualityPrewarm {
		quality = uiimage.QualityMosaic
		// Mild integer upscale before mosaic conversion for prewarm fidelity.
		if opts.Source == sprite.SourcePixel && sampleSide < 256 {
			sampleSide *= 2
		}
	}
	// Square artwork: downsample then Prepare; non-square: CropAndPrepare then Resize.
	scaled := sprite.PrepareEncodeSrc(src, sampleSide, sampleSide, opts.Source, bg, threshold, 1)

	if err := ctx.Err(); err != nil {
		return sprite.Frame{}, err
	}

	data, err := uiimage.FromImage(scaled)
	if err != nil {
		return sprite.Frame{}, fmt.Errorf("chafa: %w", err)
	}

	// Stretch fills the cell grid. With Height = Width/CellAspectY and ~1:2
	// cell pixels, the canvas pixel box is square — no vertical stretch.
	ansi := uiimage.GenerateANSIBlockOpts(data, uiimage.RenderOpts{
		Width:      w,
		Height:     h,
		Mode:       canvasMode(opts.ColorMode),
		Quality:    quality,
		Stretch:    true,
		Background: bg,
	})
	if strings.TrimSpace(ansi) == "" {
		return sprite.Frame{}, fmt.Errorf("chafa: empty output")
	}
	return sprite.Frame{Text: ansi, Backend: "chafa"}, nil
}

func canvasMode(mode sprite.ColorMode) chafago.CanvasMode {
	switch mode {
	case sprite.Color256:
		return chafago.CHAFA_CANVAS_MODE_INDEXED_256
	default:
		return chafago.CHAFA_CANVAS_MODE_TRUECOLOR
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
