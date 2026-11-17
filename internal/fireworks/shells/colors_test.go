package shells

import (
	"testing"

	"github.com/Graphixa/Poke/internal/tui/style"
)

func TestThemeLutUsesAccentShades(t *testing.T) {
	prev := UseThemeColors
	prevPal := style.CurrentPalette()
	t.Cleanup(func() {
		UseThemeColors = prev
		style.SetPalette(prevPal)
	})

	style.SetPalette(style.Palette{
		Primary:      "#ff0000",
		Secondary:    "#0000ff",
		Success:      "#00ff00",
		Warning:      "#ffff00",
		Error:        "#ff00ff",
		Components:   "#ffffff",
		Placeholders: "#000000",
		Base:         "#111111",
	})
	UseThemeColors = true

	lut := PickLut()
	if len(lut) != 4 {
		t.Fatalf("theme lut len = %d want 4 shades", len(lut))
	}
	// All shades same hue family: either red-ish or blue-ish, not mixed green/yellow.
	r0, b0 := lut[0].R, lut[0].B
	redFamily := r0 > b0
	for i, c := range lut {
		if redFamily {
			if c.R <= c.B {
				t.Fatalf("lut[%d]=%#v left red family", i, c)
			}
		} else if c.B <= c.R {
			t.Fatalf("lut[%d]=%#v left blue family", i, c)
		}
		if i > 0 {
			// Later entries are darker (lower max channel).
			prevBright := max(lut[i-1].R, lut[i-1].G, lut[i-1].B)
			curBright := max(c.R, c.G, c.B)
			if curBright > prevBright {
				t.Fatalf("shades not descending: [%d]=%d > [%d]=%d", i, curBright, i-1, prevBright)
			}
		}
	}

	UseThemeColors = false
	if len(PickLut()) == 0 {
		t.Fatal("classic lut empty")
	}
}

func TestParseHexRGBA(t *testing.T) {
	c, ok := parseHexRGBA("#cba6f7")
	if !ok || c.R != 0xcb || c.G != 0xa6 || c.B != 0xf7 {
		t.Fatalf("got %#v ok=%v", c, ok)
	}
	if _, ok := parseHexRGBA("bad"); ok {
		t.Fatal("expected reject")
	}
}
