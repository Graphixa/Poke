// Adapted from https://github.com/erik-adelbert/firework (MIT)

package shells

import (
	"image/color"
	"strconv"
	"strings"

	"github.com/Graphixa/Poke/internal/fireworks/helper"
	"github.com/Graphixa/Poke/internal/tui/style"
)

// UseThemeColors forces particle LUTs from style.CurrentPalette().
// false (default) = classic firework presets.
var UseThemeColors bool

var luts = [][]color.RGBA{
	{
		{R: 0xFF, G: 0x66, B: 0x4B, A: 0xFF}, // Red
		{R: 0x90, G: 0x38, B: 0x43, A: 0xFF}, // Pink
		{R: 0xFF, G: 0xE1, B: 0x7C, A: 0xFF}, // Yellow
		{R: 0xCE, G: 0x20, B: 0x29, A: 0xFF}, // Dark Red
	},
	{
		{R: 0xEB, G: 0x27, B: 0x9B, A: 0xFF}, // Magenta
		{R: 0xFA, G: 0xD8, B: 0x44, A: 0xFF}, // Yellow
		{R: 0xF2, G: 0x34, B: 0x48, A: 0xFF}, // Red
		{R: 0x3F, G: 0x34, B: 0xC8, A: 0xFF}, // Blue
		{R: 0xFF, G: 0x8B, B: 0x39, A: 0xFF}, // Orange
	},
	{
		{R: 0x98, G: 0xBA, B: 0xE3, A: 0xFF}, // Light Blue
		{R: 0x59, G: 0x81, B: 0xB1, A: 0xFF}, // Medium Blue
		{R: 0x36, G: 0x54, B: 0x75, A: 0xFF}, // Dark Blue
		{R: 0xF0, G: 0xF4, B: 0xFE, A: 0xFF}, // Very Light Blue
	},
	{
		{R: 0x22, G: 0x57, B: 0x7A, A: 0xFF}, // Navy Blue
		{R: 0x38, G: 0xA3, B: 0xA5, A: 0xFF}, // Teal
		{R: 0x57, G: 0xCC, B: 0x99, A: 0xFF}, // Light Green
		{R: 0x80, G: 0xED, B: 0x99, A: 0xFF}, // Medium Green
		{R: 0xC7, G: 0xF9, B: 0xCC, A: 0xFF}, // Very Light Green
	},
	{
		{R: 0xCD, G: 0xB4, B: 0xDB, A: 0xFF}, // Lavender
		{R: 0xFF, G: 0xC8, B: 0xDD, A: 0xFF}, // Pink
		{R: 0xFF, G: 0xAF, B: 0xCC, A: 0xFF}, // Light Pink
		{R: 0xBD, G: 0xE0, B: 0xFE, A: 0xFF}, // Light Blue
		{R: 0xA2, G: 0xD2, B: 0xFF, A: 0xFF}, // Medium Blue
	},
	{
		{R: 0x4F, G: 0x00, B: 0x0B, A: 0xFF}, // Dark Red
		{R: 0x72, G: 0x00, B: 0x26, A: 0xFF}, // Deep Red
		{R: 0xCE, G: 0x42, B: 0x57, A: 0xFF}, // Red
		{R: 0xFF, G: 0x7F, B: 0x51, A: 0xFF}, // Orange
		{R: 0xFF, G: 0x9B, B: 0x54, A: 0xFF}, // Light Orange
	},
	{
		{R: 0x00, G: 0x1D, B: 0x3D, A: 0xFF}, // Dark Blue
		{R: 0x00, G: 0x35, B: 0x66, A: 0xFF}, // Medium Blue
		{R: 0xFF, G: 0xC3, B: 0x00, A: 0xFF}, // Yellow
		{R: 0xFF, G: 0xD6, B: 0x0A, A: 0xFF}, // Light Yellow
	},
	{
		{R: 0x3D, G: 0x34, B: 0x8B, A: 0xFF}, // Dark Purple
		{R: 0x76, G: 0x78, B: 0xED, A: 0xFF}, // Medium Purple
		{R: 0xF7, G: 0xB8, B: 0x01, A: 0xFF}, // Yellow
		{R: 0xF1, G: 0x87, B: 0x01, A: 0xFF}, // Orange
		{R: 0xF3, G: 0x5B, B: 0x04, A: 0xFF}, // Red
	},
}

var glitterLuts = [][]color.RGBA{
	{
		{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
		{R: 0xF2, G: 0xF6, B: 0xFF, A: 0xFF},
		{R: 0xE8, G: 0xEE, B: 0xFF, A: 0xFF},
		{R: 0xFF, G: 0xF6, B: 0xD8, A: 0xFF},
	},
	{
		{R: 0xFF, G: 0xFF, B: 0xF2, A: 0xFF},
		{R: 0xFF, G: 0xF2, B: 0xC9, A: 0xFF},
		{R: 0xF6, G: 0xFB, B: 0xFF, A: 0xFF},
		{R: 0xD8, G: 0xE8, B: 0xFF, A: 0xFF},
	},
	{
		{R: 0xFF, G: 0xF8, B: 0xE5, A: 0xFF},
		{R: 0xFF, G: 0xEC, B: 0xBE, A: 0xFF},
		{R: 0xE7, G: 0xF3, B: 0xFF, A: 0xFF},
		{R: 0xC8, G: 0xDD, B: 0xFF, A: 0xFF},
	},
}

type LUT []color.RGBA

func (l LUT) PickColor() color.RGBA {
	c, _ := helper.Pick(l)
	return c
}

func PickLut() LUT {
	if UseThemeColors {
		return themeLut()
	}
	l, _ := helper.Pick(luts)
	return l
}

func PickGlitterLut() LUT {
	if UseThemeColors {
		return themeLut()
	}
	l, _ := helper.Pick(glitterLuts)
	return l
}

var kamuroLuts = [][]color.RGBA{
	{
		{R: 0xF2, G: 0xE9, B: 0xBE, A: 0xFF}, // Cream
		{R: 0xE2, G: 0xC4, B: 0x88, A: 0xFF}, // Tan
		{R: 0x95, G: 0xCA, B: 0xB0, A: 0xFF}, // Mint
		{R: 0x1A, G: 0x40, B: 0x7E, A: 0xFF}, // Navy
	},
}

func PickKamuroLut() LUT {
	if UseThemeColors {
		return themeLut()
	}
	l, _ := helper.Pick(kamuroLuts)
	return l
}

func themeLut() LUT {
	// One accent per firework (primary or secondary), then darker shades.
	// Gradient() further dims as particles fade — no rainbow LUT.
	p := style.CurrentPalette()
	accents := make([]color.RGBA, 0, 2)
	for _, hex := range []string{p.Primary, p.Secondary} {
		if c, ok := parseHexRGBA(hex); ok {
			accents = append(accents, c)
		}
	}
	if len(accents) == 0 {
		l, _ := helper.Pick(luts)
		return l
	}
	base, _ := helper.Pick(accents)
	return LUT{
		scaleColor(base, 1.00),
		scaleColor(base, 0.80),
		scaleColor(base, 0.60),
		scaleColor(base, 0.40),
	}
}

func parseHexRGBA(hex string) (color.RGBA, bool) {
	s := strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(s) != 6 {
		return color.RGBA{}, false
	}
	n, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return color.RGBA{}, false
	}
	return color.RGBA{
		R: uint8(n >> 16),
		G: uint8(n >> 8),
		B: uint8(n),
		A: 0xFF,
	}, true
}

func scaleChannel(v uint8, gain float64) uint8 {
	s := float64(v) * gain
	return uint8(min(max(s, 0), 255))
}

func scaleColor(c color.RGBA, gain float64) color.RGBA {
	return color.RGBA{
		R: scaleChannel(c.R, gain),
		G: scaleChannel(c.G, gain),
		B: scaleChannel(c.B, gain),
		A: scaleChannel(c.A, gain),
	}
}
