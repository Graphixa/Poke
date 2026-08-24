package ascii

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"strings"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

// Ramp is the classic luminance character ramp (dark → light).
const Ramp = " .:-=+*#%@"

// Renderer produces classic ASCII luminance frames.
type Renderer struct{}

// Name returns the logical renderer class.
func (Renderer) Name() string { return "ascii" }

// Supported always returns true — ASCII is the last text fallback before placeholder.
func (Renderer) Supported(caps sprite.Capabilities) bool {
	_ = caps
	return true
}

// Render converts src into ASCII art prioritising silhouette recognition.
func (r Renderer) Render(ctx context.Context, src image.Image, opts sprite.RenderOptions) (sprite.Frame, error) {
	if err := ctx.Err(); err != nil {
		return sprite.Frame{}, err
	}
	if src == nil {
		return sprite.Frame{}, fmt.Errorf("ascii: nil image")
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

	// Compensate for roughly 2:1 cell aspect by sampling 2 vertical pixels per row.
	targetW := w
	targetH := h * 2
	prepared := sprite.Prepare(src, bg, threshold)
	scaled := sprite.Resize(prepared, targetW, targetH, opts.Source, opts.PreserveAR, bg)

	if err := ctx.Err(); err != nil {
		return sprite.Frame{}, err
	}

	ramp := []rune(Ramp)
	n := len(ramp)
	bgLum := luminance(bg)

	var b strings.Builder
	b.Grow(h * (w + 1))
	for y := 0; y < h; y++ {
		if y > 0 {
			b.WriteByte('\n')
		}
		for x := 0; x < w; x++ {
			top := scaled.RGBAAt(x, y*2)
			bottom := scaled.RGBAAt(x, y*2+1)
			lum := (luminance(top) + luminance(bottom)) / 2
			diff := lum - bgLum
			if diff < 0 {
				diff = -diff
			}
			if diff < 12 {
				b.WriteByte(' ')
				continue
			}
			// Map luminance onto ramp (darker ink → denser glyph).
			idx := (255 - lum) * (n - 1) / 255
			if idx < 0 {
				idx = 0
			}
			if idx >= n {
				idx = n - 1
			}
			b.WriteRune(ramp[idx])
		}
	}
	return sprite.Frame{Text: b.String(), Backend: "ascii"}, nil
}

func luminance(c color.RGBA) int {
	return (int(c.R)*299 + int(c.G)*587 + int(c.B)*114) / 1000
}
