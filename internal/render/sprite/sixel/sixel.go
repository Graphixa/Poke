// Package sixel encodes images as DECSIXEL DCS sequences (≤256 colours).
package sixel

import (
	"bytes"
	"fmt"
	stdimage "image"

	"github.com/mattn/go-sixel"

	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

// MaxColors is the sixel palette cap (DECSIXEL ≤256).
const MaxColors = 256

// Encode converts a prepared image to a single valid DECSIXEL DCS.
// Height is snapped down to a multiple of 6 so the encoder does not emit a
// leftover band below the raster (6px sixel bands).
func Encode(src stdimage.Image) (string, error) {
	if src == nil {
		return "", fmt.Errorf("sixel: nil source")
	}
	src = cropHeightToSixelBands(src)
	sprite.NoteSixelEncode()
	end := debuglog.Trace(debuglog.Render, "sixel.encode")
	defer end()
	var buf bytes.Buffer
	enc := sixel.NewEncoder(&buf)
	enc.Colors = MaxColors
	if err := enc.Encode(src); err != nil {
		return "", fmt.Errorf("sixel: encode: %w", err)
	}
	out := buf.String()
	if out == "" {
		return "", fmt.Errorf("sixel: empty render")
	}
	if countESCPrefix(out, 'P') != 1 {
		return "", fmt.Errorf("sixel: malformed sequence")
	}
	return out, nil
}

// EncodeFromCells prepares src to the sixel pixel target and encodes it.
func EncodeFromCells(src stdimage.Image, cellsW, cellsH int, opts sprite.RenderOptions) (string, error) {
	bg := opts.BackgroundColor
	if bg.A == 0 {
		bg = sprite.DefaultBackground
	}
	threshold := opts.AlphaThreshold
	if threshold == 0 {
		threshold = 16
	}
	pxW, pxH := sprite.PixelSize(cellsW, cellsH)
	pxH = snapHeightToSixelBand(pxH)
	source := opts.Source
	if source == "" {
		source = sprite.SourceArtwork
	}
	prepared := sprite.PrepareEncodeSrc(src, pxW, pxH, source, bg, threshold, 1)
	return Encode(prepared)
}

// snapHeightToSixelBand rounds pxH down to a multiple of 6 (one sixel band).
func snapHeightToSixelBand(pxH int) int {
	if pxH < 1 {
		return 1
	}
	if rem := pxH % 6; rem != 0 {
		pxH -= rem
		if pxH < 6 {
			pxH = 6
		}
	}
	return pxH
}

func cropHeightToSixelBands(src stdimage.Image) stdimage.Image {
	b := src.Bounds()
	h := b.Dy()
	want := h - (h % 6)
	if want == h || want < 6 {
		return src
	}
	r := stdimage.Rect(b.Min.X, b.Min.Y, b.Max.X, b.Min.Y+want)
	if s, ok := src.(interface {
		SubImage(stdimage.Rectangle) stdimage.Image
	}); ok {
		return s.SubImage(r)
	}
	return src
}

func countESCPrefix(s string, second byte) int {
	n := 0
	for i := 0; i+1 < len(s); i++ {
		if s[i] == 0x1b && s[i+1] == second {
			n++
		}
	}
	return n
}
