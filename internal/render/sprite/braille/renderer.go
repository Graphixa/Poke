package braille

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"strings"
	"unicode/utf8"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

// Renderer produces monochrome Braille (2×4) silhouette frames.
type Renderer struct{}

// Name returns the logical renderer class.
func (Renderer) Name() string { return "braille" }

// Supported reports whether Braille output is available.
func (Renderer) Supported(caps sprite.Capabilities) bool {
	return caps.Braille && caps.Unicode
}

// Render converts src into a Braille art frame focused on silhouette.
func (r Renderer) Render(ctx context.Context, src image.Image, opts sprite.RenderOptions) (sprite.Frame, error) {
	if err := ctx.Err(); err != nil {
		return sprite.Frame{}, err
	}
	if src == nil {
		return sprite.Frame{}, fmt.Errorf("braille: nil image")
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

	// Each Braille cell covers 2×4 source samples.
	targetW := w * 2
	targetH := h * 4
	prepared := sprite.Prepare(src, bg, threshold)
	scaled := sprite.Resize(prepared, targetW, targetH, opts.Source, opts.PreserveAR, bg)

	if err := ctx.Err(); err != nil {
		return sprite.Frame{}, err
	}

	var b strings.Builder
	b.Grow(h * (w*3 + 1))
	for row := 0; row < h; row++ {
		if row > 0 {
			b.WriteByte('\n')
		}
		for col := 0; col < w; col++ {
			var bits int
			for dy := 0; dy < 4; dy++ {
				for dx := 0; dx < 2; dx++ {
					c := scaled.RGBAAt(col*2+dx, row*4+dy)
					if isInk(c, bg) {
						bits |= brailleBit(dx, dy)
					}
				}
			}
			if bits == 0 {
				b.WriteByte(' ')
			} else {
				var buf [utf8.UTFMax]byte
				n := utf8.EncodeRune(buf[:], rune(0x2800+bits))
				b.Write(buf[:n])
			}
		}
	}
	return sprite.Frame{Text: b.String(), Backend: "braille"}, nil
}

// Braille dot bit mapping (Unicode Braille patterns):
//
//	(0,0)=0x01 (1,0)=0x08
//	(0,1)=0x02 (1,1)=0x10
//	(0,2)=0x04 (1,2)=0x20
//	(0,3)=0x40 (1,3)=0x80
func brailleBit(dx, dy int) int {
	switch {
	case dx == 0 && dy == 0:
		return 0x01
	case dx == 0 && dy == 1:
		return 0x02
	case dx == 0 && dy == 2:
		return 0x04
	case dx == 1 && dy == 0:
		return 0x08
	case dx == 1 && dy == 1:
		return 0x10
	case dx == 1 && dy == 2:
		return 0x20
	case dx == 0 && dy == 3:
		return 0x40
	case dx == 1 && dy == 3:
		return 0x80
	}
	return 0
}

func isInk(c, bg color.RGBA) bool {
	lum := luminance(c.R, c.G, c.B)
	bgLum := luminance(bg.R, bg.G, bg.B)
	diff := lum - bgLum
	if diff < 0 {
		diff = -diff
	}
	return diff > 40
}

func luminance(r, g, b uint8) int {
	return (int(r)*299 + int(g)*587 + int(b)*114) / 1000
}
