// Package sixel encodes images as DECSIXEL DCS sequences (≤256 colours).
package sixel

import (
	"bytes"
	"fmt"
	stdimage "image"
	"strings"

	"github.com/mattn/go-sixel"

	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

// MaxColors is the sixel palette cap (DECSIXEL ≤256).
const MaxColors = 256

// Encode converts a prepared image to a single valid DECSIXEL DCS using the
// production default palette (64 colours).
func Encode(src stdimage.Image) (string, error) {
	return EncodeWithColors(src, config.DefaultSixelColors)
}

// EncodeWithColors encodes src with a configurable palette limit (1–256).
func EncodeWithColors(src stdimage.Image, colors int) (string, error) {
	if src == nil {
		return "", fmt.Errorf("sixel: nil source")
	}
	if colors < 1 || colors > MaxColors {
		return "", fmt.Errorf("sixel: invalid color count %d (want 1–256)", colors)
	}
	prepared := cropHeightToSixelBands(src)
	sprite.NoteSixelEncode()
	end := debuglog.Trace(debuglog.Render, "sixel.encode")
	defer end()
	var buf bytes.Buffer
	enc := sixel.NewEncoder(&buf)
	enc.Colors = colors
	if err := enc.Encode(prepared); err != nil {
		return "", fmt.Errorf("sixel: encode: %w", err)
	}
	out := buf.String()
	if out == "" {
		return "", fmt.Errorf("sixel: empty render")
	}
	// go-sixel hardcodes P1=0 (device default aspect). On Windows Terminal that
	// is often 2:1, so the bitmap paints ~2× tall and spills / gets cell-punched.
	// P1=7 forces 1:1 pixel aspect to match our PixelSize cell geometry.
	out = strings.Replace(out, "\x1bP0;0;8q", "\x1bP7;0;8q", 1)
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
	colors := opts.SixelColors
	if colors == 0 {
		colors = config.DefaultSixelColors
	}
	pxW, pxH := sprite.PixelSize(cellsW, cellsH)
	pxH = snapHeightToSixelBand(pxH)
	source := opts.Source
	if source == "" {
		source = sprite.SourceArtwork
	}
	// Crop pad 0: browse art cards are tight (1 cell inset only). Extra pixel
	// margin left the subject small inside the 24×12 reservation.
	prepared := sprite.PrepareEncodeSrc(src, pxW, pxH, source, bg, threshold, 0)
	return EncodeWithColors(prepared, colors)
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
