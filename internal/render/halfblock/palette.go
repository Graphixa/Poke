package halfblock

import (
	"fmt"
	"image/color"
	"os"
	"strings"
)

// PaletteMode selects the ANSI colour encoding.
type PaletteMode string

const (
	PaletteTrueColor PaletteMode = "truecolor"
	Palette256       PaletteMode = "256"
)

const resetSeq = "\x1b[0m"

// DetectPalette chooses truecolor when COLORTERM advertises it, otherwise 256.
func DetectPalette() PaletteMode {
	ct := strings.ToLower(os.Getenv("COLORTERM"))
	if ct == "truecolor" || ct == "24bit" {
		return PaletteTrueColor
	}
	term := strings.ToLower(os.Getenv("TERM"))
	if strings.Contains(term, "truecolor") || term == "xterm-kitty" || term == "wezterm" {
		return PaletteTrueColor
	}
	return Palette256
}

func writeFG(b *strings.Builder, c color.RGBA, mode PaletteMode) {
	switch mode {
	case Palette256:
		fmt.Fprintf(b, "\x1b[38;5;%dm", to256(c))
	default:
		fmt.Fprintf(b, "\x1b[38;2;%d;%d;%dm", c.R, c.G, c.B)
	}
}

func writeBG(b *strings.Builder, c color.RGBA, mode PaletteMode) {
	switch mode {
	case Palette256:
		fmt.Fprintf(b, "\x1b[48;5;%dm", to256(c))
	default:
		fmt.Fprintf(b, "\x1b[48;2;%d;%d;%dm", c.R, c.G, c.B)
	}
}

// to256 maps an RGB colour to the nearest xterm 256-colour index.
func to256(c color.RGBA) int {
	// Try the 6x6x6 colour cube (indices 16–231) and grayscale (232–255).
	cube := func(v uint8) int {
		switch {
		case v < 48:
			return 0
		case v < 115:
			return 1
		default:
			return (int(v) - 35) / 40
		}
	}
	ri, gi, bi := cube(c.R), cube(c.G), cube(c.B)
	cubeIdx := 16 + 36*ri + 6*gi + bi
	cubeColor := color.RGBA{
		R: cubeLevel(ri),
		G: cubeLevel(gi),
		B: cubeLevel(bi),
		A: 255,
	}

	grayIdx := 232
	if avg := (int(c.R) + int(c.G) + int(c.B)) / 3; avg > 8 {
		grayIdx = 232 + (avg-8)/10
		if grayIdx > 255 {
			grayIdx = 255
		}
	}
	grayLevel := uint8(8 + (grayIdx-232)*10)
	grayColor := color.RGBA{R: grayLevel, G: grayLevel, B: grayLevel, A: 255}

	if colourDist(c, grayColor) < colourDist(c, cubeColor) {
		return grayIdx
	}
	return cubeIdx
}

func cubeLevel(i int) uint8 {
	if i <= 0 {
		return 0
	}
	return uint8(55 + (i-1)*40)
}

func colourDist(a, b color.RGBA) int {
	dr := int(a.R) - int(b.R)
	dg := int(a.G) - int(b.G)
	db := int(a.B) - int(b.B)
	return dr*dr + dg*dg + db*db
}

// NormalizeANSI replaces ESC bytes so golden files stay readable and stable.
func NormalizeANSI(s string) string {
	return strings.ReplaceAll(s, "\x1b", "ESC")
}
