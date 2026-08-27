// Package halfblock renders images as truecolor ANSI using the lower-half
// block glyph (▄), matching the approach used by github.com/danielgatis/imgcat.
//
// Each terminal cell encodes two source pixels: background = top, foreground
// (▄) = bottom. This is the universal fallback candidate vs Chafa mosaics.
package halfblock

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"strings"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"golang.org/x/image/draw"
)

// Renderer produces imgcat-style half-block ANSI frames.
type Renderer struct{}

// Name returns the logical renderer class.
func (Renderer) Name() string { return "halfblock" }

// Supported reports whether colour block rendering is available.
func (Renderer) Supported(caps sprite.Capabilities) bool {
	return caps.TrueColor || caps.ANSI256
}

// Render converts src into half-block ANSI art sized to opts.Width×opts.Height cells.
func (r Renderer) Render(ctx context.Context, src image.Image, opts sprite.RenderOptions) (sprite.Frame, error) {
	if err := ctx.Err(); err != nil {
		return sprite.Frame{}, err
	}
	if src == nil {
		return sprite.Frame{}, fmt.Errorf("halfblock: nil image")
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

	cropped := sprite.CropToContentSquare(src, threshold, 1)
	prepared := sprite.Prepare(cropped, bg, threshold)

	// Two vertical samples per cell (imgcat RESIZE_FACTOR_Y = 2).
	pxW := w
	pxH := h * 2
	scaled := image.NewRGBA(image.Rect(0, 0, pxW, pxH))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), prepared, prepared.Bounds(), draw.Over, nil)

	if err := ctx.Err(); err != nil {
		return sprite.Frame{}, err
	}

	ansi := encodeHalfBlocks(scaled, bg)
	if strings.TrimSpace(ansi) == "" {
		return sprite.Frame{}, fmt.Errorf("halfblock: empty output")
	}
	return sprite.Frame{Text: ansi, Backend: "halfblock"}, nil
}

func encodeHalfBlocks(img *image.RGBA, fallback color.RGBA) string {
	b := img.Bounds()
	maxY := b.Dy() - b.Dy()%2
	maxX := b.Dx()
	var out strings.Builder
	out.Grow(maxY/2*(maxX*28+8) + 8)

	for y := 0; y < maxY; y += 2 {
		for x := 0; x < maxX; x++ {
			tr, tg, tb, ta := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			br, bgc, bb, ba := img.At(b.Min.X+x, b.Min.Y+y+1).RGBA()
			if ta>>8 < 128 {
				// Transparent top → use theme bg so cells stay opaque in TUI.
				fmt.Fprintf(&out, "\x1b[48;2;%d;%d;%dm", fallback.R, fallback.G, fallback.B)
			} else {
				fmt.Fprintf(&out, "\x1b[48;2;%d;%d;%dm", tr>>8, tg>>8, tb>>8)
			}
			if ba>>8 < 128 {
				out.WriteString(" ")
			} else {
				fmt.Fprintf(&out, "\x1b[38;2;%d;%d;%dm▄", br>>8, bgc>>8, bb>>8)
			}
		}
		out.WriteString("\x1b[0m\n")
	}
	return out.String()
}
