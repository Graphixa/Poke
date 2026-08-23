// Package halfblock converts sprite PNGs into terminal half-block ANSI art.
package halfblock

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/png"
	"strings"
)

// DefaultBackground is used when compositing transparent sprite pixels.
var DefaultBackground = color.RGBA{R: 0, G: 0, B: 0, A: 255}

// Options control half-block conversion.
type Options struct {
	// Width and Height are the terminal cell dimensions of the output.
	Width  int
	Height int
	// Palette selects truecolor or 256-color ANSI sequences.
	Palette PaletteMode
	// Background is the panel colour under transparent pixels.
	Background color.RGBA
}

// ConvertPNG decodes PNG bytes and renders half-block ANSI art.
func ConvertPNG(pngData []byte, opts Options) (string, error) {
	img, err := DecodePNG(pngData)
	if err != nil {
		return "", err
	}
	return Convert(img, opts), nil
}

// DecodePNG decodes and validates a PNG image.
func DecodePNG(pngData []byte) (image.Image, error) {
	if len(pngData) == 0 {
		return nil, fmt.Errorf("halfblock: empty image")
	}
	img, format, err := image.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, fmt.Errorf("halfblock: decode: %w", err)
	}
	if format != "png" {
		return nil, fmt.Errorf("halfblock: unsupported format %q", format)
	}
	return img, nil
}

// Convert renders img as half-block ANSI using nearest-neighbour scaling.
// Each terminal cell covers two vertical source pixels (▄).
func Convert(img image.Image, opts Options) string {
	if opts.Width < 1 {
		opts.Width = 1
	}
	if opts.Height < 1 {
		opts.Height = 1
	}
	if opts.Palette == "" {
		opts.Palette = PaletteTrueColor
	}
	bg := opts.Background
	if bg.A == 0 {
		bg = DefaultBackground
	}

	composited := compositeOnto(img, bg)
	// Target pixel grid is Width × (Height*2); odd source heights are handled
	// by sampling (missing rows read as background after bounds padding).
	targetW := opts.Width
	targetH := opts.Height * 2
	scaled := scaleNearest(composited, targetW, targetH)

	var b strings.Builder
	// Rough upper bound: cell * ~40 bytes of ANSI + glyph + newline.
	b.Grow(opts.Height * (opts.Width*48 + 1))

	for y := 0; y < opts.Height; y++ {
		if y > 0 {
			b.WriteByte('\n')
		}
		for x := 0; x < opts.Width; x++ {
			top := scaled.RGBAAt(x, y*2)
			bottom := scaled.RGBAAt(x, y*2+1)
			writeCell(&b, top, bottom, opts.Palette)
		}
		b.WriteString(resetSeq)
	}
	return b.String()
}

func writeCell(b *strings.Builder, top, bottom color.RGBA, mode PaletteMode) {
	// ▄ uses foreground for the lower pixel and background for the upper pixel.
	writeFG(b, bottom, mode)
	writeBG(b, top, mode)
	b.WriteRune('▄')
}

func compositeOnto(src image.Image, bg color.RGBA) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, &image.Uniform{C: bg}, image.Point{}, draw.Src)
	draw.Draw(dst, b, src, b.Min, draw.Over)
	return dst
}

func scaleNearest(src *image.RGBA, tw, th int) *image.RGBA {
	sb := src.Bounds()
	sw := sb.Dx()
	sh := sb.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, tw, th))
	if sw <= 0 || sh <= 0 {
		return dst
	}
	for y := 0; y < th; y++ {
		sy := sb.Min.Y + y*sh/th
		if sy >= sb.Max.Y {
			sy = sb.Max.Y - 1
		}
		for x := 0; x < tw; x++ {
			sx := sb.Min.X + x*sw/tw
			if sx >= sb.Max.X {
				sx = sb.Max.X - 1
			}
			dst.SetRGBA(x, y, src.RGBAAt(sx, sy))
		}
	}
	return dst
}
