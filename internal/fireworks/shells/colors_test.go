package shells

import (
	"testing"

	"github.com/Graphixa/Pokedex/internal/tui/style"
)

func TestThemeLutUsesPalette(t *testing.T) {
	prev := UseThemeColors
	prevPal := style.CurrentPalette()
	t.Cleanup(func() {
		UseThemeColors = prev
		style.SetPalette(prevPal)
	})

	style.SetPalette(style.Palette{
		Primary:      "#112233",
		Secondary:    "#445566",
		Success:      "#778899",
		Warning:      "#aabbcc",
		Error:        "#ddeeff",
		Components:   "#010203",
		Placeholders: "#000000",
		Base:         "#111111",
	})
	UseThemeColors = true

	lut := PickLut()
	if len(lut) != 6 {
		t.Fatalf("theme lut len = %d want 6", len(lut))
	}
	want := [][3]uint8{
		{0x11, 0x22, 0x33},
		{0x44, 0x55, 0x66},
		{0x77, 0x88, 0x99},
		{0xaa, 0xbb, 0xcc},
		{0xdd, 0xee, 0xff},
		{0x01, 0x02, 0x03},
	}
	for i, c := range lut {
		if c.R != want[i][0] || c.G != want[i][1] || c.B != want[i][2] || c.A != 0xff {
			t.Fatalf("lut[%d] = %#v want RGB%v A=ff", i, c, want[i])
		}
	}

	UseThemeColors = false
	classic := PickLut()
	if len(classic) == 0 {
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
