package sprite

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/png"
)

// DefaultBackground is used when compositing transparent sprite pixels.
var DefaultBackground = color.RGBA{R: 0, G: 0, B: 0, A: 255}

// Prepare converts src to RGBA, applies alpha thresholding, and composites
// translucent pixels onto bg. Transparent regions become bg (not white).
func Prepare(src image.Image, bg color.RGBA, alphaThreshold uint8) *image.RGBA {
	if bg.A == 0 {
		bg = DefaultBackground
	}
	b := src.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, &image.Uniform{C: bg}, image.Point{}, draw.Src)

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := src.At(x, y).RGBA()
			a8 := uint8(a >> 8)
			if a8 < alphaThreshold {
				continue
			}
			srcC := color.RGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(bl >> 8),
				A: a8,
			}
			if a8 == 255 {
				dst.SetRGBA(x, y, srcC)
				continue
			}
			dst.SetRGBA(x, y, blendOver(bg, srcC))
		}
	}
	return dst
}

func blendOver(dst, src color.RGBA) color.RGBA {
	as := float64(src.A) / 255
	ad := 1 - as
	return color.RGBA{
		R: uint8(float64(src.R)*as + float64(dst.R)*ad + 0.5),
		G: uint8(float64(src.G)*as + float64(dst.G)*ad + 0.5),
		B: uint8(float64(src.B)*as + float64(dst.B)*ad + 0.5),
		A: 255,
	}
}

// DecodeImage decodes image bytes (PNG via registered decoder).
func DecodeImage(data []byte) (image.Image, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("sprite: empty image")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("sprite: decode: %w", err)
	}
	return img, nil
}
