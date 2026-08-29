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

// ContentBounds returns the tight rectangle of pixels with alpha ≥ threshold.
// If nothing is opaque, it returns src.Bounds().
func ContentBounds(src image.Image, alphaThreshold uint8) image.Rectangle {
	b := src.Bounds()
	minX, minY := b.Max.X, b.Max.Y
	maxX, maxY := b.Min.X, b.Min.Y
	found := false
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := src.At(x, y).RGBA()
			if uint8(a>>8) < alphaThreshold {
				continue
			}
			found = true
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x+1 > maxX {
				maxX = x + 1
			}
			if y+1 > maxY {
				maxY = y + 1
			}
		}
	}
	if !found {
		return b
	}
	return image.Rect(minX, minY, maxX, maxY)
}

// CropAndPrepare trims transparent margins to a 1:1 crop and composites onto bg
// in one pass (single bounds scan + single write pass).
func CropAndPrepare(src image.Image, bg color.RGBA, alphaThreshold uint8, pad int) *image.RGBA {
	if src == nil {
		out := image.NewRGBA(image.Rect(0, 0, 1, 1))
		draw.Draw(out, out.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)
		return out
	}
	if bg.A == 0 {
		bg = DefaultBackground
	}
	if pad < 0 {
		pad = 0
	}
	b := src.Bounds()
	content := ContentBounds(src, alphaThreshold)
	content = content.Inset(-pad).Intersect(b)
	if content.Empty() {
		content = b
	}

	side := content.Dx()
	if content.Dy() > side {
		side = content.Dy()
	}
	if side < 1 {
		side = 1
	}
	cx := (content.Min.X + content.Max.X) / 2
	cy := (content.Min.Y + content.Max.Y) / 2
	sq := image.Rect(cx-side/2, cy-side/2, cx-side/2+side, cy-side/2+side)

	dst := image.NewRGBA(image.Rect(0, 0, side, side))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)

	for y := 0; y < side; y++ {
		sy := sq.Min.Y + y
		for x := 0; x < side; x++ {
			sx := sq.Min.X + x
			if !image.Pt(sx, sy).In(b) {
				continue
			}
			r, g, bl, a := src.At(sx, sy).RGBA()
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

// PrepareEncodeSrc prepares src for terminal encode at tw×th.
// Square (or near-square) artwork: skip ContentBounds; resize then Prepare at target.
// Non-square / pixel sprites: keep CropAndPrepare then Resize (current behavior).
func PrepareEncodeSrc(src image.Image, tw, th int, source SpriteSource, bg color.RGBA, threshold uint8, pad int) *image.RGBA {
	if tw < 1 {
		tw = 1
	}
	if th < 1 {
		th = 1
	}
	if bg.A == 0 {
		bg = DefaultBackground
	}
	if threshold == 0 {
		threshold = 16
	}
	if src == nil {
		out := image.NewRGBA(image.Rect(0, 0, tw, th))
		draw.Draw(out, out.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)
		return out
	}
	b := src.Bounds()
	if b.Dx() == b.Dy() && b.Dx() > 0 {
		// Official artwork is square (e.g. 475×475): downsample first, composite small.
		rgba := asRGBA(src)
		resized := Resize(rgba, tw, th, source, true, bg)
		return Prepare(resized, bg, threshold)
	}
	prepared := CropAndPrepare(src, bg, threshold, pad)
	return Resize(prepared, tw, th, source, true, bg)
}

// asRGBA copies src into an RGBA buffer without alpha compositing or cropping.
func asRGBA(src image.Image) *image.RGBA {
	if rgba, ok := src.(*image.RGBA); ok {
		return rgba
	}
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst
}

// CropToContentSquare trims transparent margins, then returns a 1:1 crop centred
// on the opaque subject. pad adds a few pixels of margin before squaring.
// Transparent pixels outside the subject stay transparent (Prepare composites later).
func CropToContentSquare(src image.Image, alphaThreshold uint8, pad int) *image.RGBA {
	if src == nil {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}
	if pad < 0 {
		pad = 0
	}
	b := src.Bounds()
	content := ContentBounds(src, alphaThreshold)
	content = content.Inset(-pad).Intersect(b)
	if content.Empty() {
		content = b
	}

	side := content.Dx()
	if content.Dy() > side {
		side = content.Dy()
	}
	if side < 1 {
		side = 1
	}

	cx := (content.Min.X + content.Max.X) / 2
	cy := (content.Min.Y + content.Max.Y) / 2
	sq := image.Rect(cx-side/2, cy-side/2, cx-side/2+side, cy-side/2+side)

	dst := image.NewRGBA(image.Rect(0, 0, side, side))
	draw.Draw(dst, dst.Bounds(), src, sq.Min, draw.Src)
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
	notePNGDecode()
	if len(data) == 0 {
		return nil, fmt.Errorf("sprite: empty image")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("sprite: decode: %w", err)
	}
	return img, nil
}
