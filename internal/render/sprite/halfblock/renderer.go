package halfblock

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"strings"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

// Renderer produces true-colour or ANSI-256 half-block frames.
type Renderer struct{}

// Name returns the logical renderer class.
func (Renderer) Name() string { return "halfblock" }

// Supported reports whether colour block rendering is available.
func (Renderer) Supported(caps sprite.Capabilities) bool {
	return caps.TrueColor || caps.ANSI256
}

// Render converts src into half-block ANSI art.
// Each cell uses ▀ with FG=top sample and BG=bottom sample.
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
	mode := resolveColor(opts.ColorMode)

	prepared := sprite.Prepare(src, bg, threshold)
	targetW := w
	targetH := h * 2
	scaled := sprite.Resize(prepared, targetW, targetH, opts.Source, opts.PreserveAR, bg)

	if err := ctx.Err(); err != nil {
		return sprite.Frame{}, err
	}

	var b strings.Builder
	b.Grow(h * (w*48 + 1))
	for y := 0; y < h; y++ {
		if y > 0 {
			b.WriteByte('\n')
		}
		for x := 0; x < w; x++ {
			top := scaled.RGBAAt(x, y*2)
			bottom := scaled.RGBAAt(x, y*2+1)
			writeCell(&b, top, bottom, mode)
		}
		b.WriteString(resetSeq)
	}
	return sprite.Frame{Text: b.String(), Backend: "halfblock"}, nil
}

func resolveColor(mode sprite.ColorMode) ColorMode {
	switch mode {
	case sprite.Color256:
		return Color256
	case sprite.ColorTrueColor, sprite.ColorAuto, "":
		return ColorTrueColor
	default:
		return ColorTrueColor
	}
}

func writeCell(b *strings.Builder, top, bottom color.RGBA, mode ColorMode) {
	// ▀: foreground = upper sample, background = lower sample.
	writeFG(b, top, mode)
	writeBG(b, bottom, mode)
	b.WriteRune('▀')
}

// Convert is a convenience for tests: render with default options.
func Convert(img image.Image, width, height int, mode ColorMode, bg color.RGBA) string {
	frame, err := (Renderer{}).Render(context.Background(), img, sprite.RenderOptions{
		Width:           width,
		Height:          height,
		PreserveAR:      false,
		ColorMode:       mode,
		BackgroundColor: bg,
		AlphaThreshold:  1,
		Source:          sprite.SourcePixel,
	})
	if err != nil {
		return ""
	}
	return frame.Text
}

// ConvertFit is Convert with aspect-preserving fit.
func ConvertFit(img image.Image, width, height int, mode ColorMode, bg color.RGBA) string {
	frame, err := (Renderer{}).Render(context.Background(), img, sprite.RenderOptions{
		Width:           width,
		Height:          height,
		PreserveAR:      true,
		ColorMode:       mode,
		BackgroundColor: bg,
		AlphaThreshold:  1,
		Source:          sprite.SourcePixel,
	})
	if err != nil {
		return ""
	}
	return frame.Text
}
