package halfblock_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/Graphixa/Pokedex/internal/render/halfblock"
)

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestCompositeTransparency(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	// Fully transparent top-left; opaque red top-right; semi-transparent green bottom.
	img.SetRGBA(0, 0, color.RGBA{0, 0, 0, 0})
	img.SetRGBA(1, 0, color.RGBA{255, 0, 0, 255})
	img.SetRGBA(0, 1, color.RGBA{0, 255, 0, 128})
	img.SetRGBA(1, 1, color.RGBA{0, 0, 255, 255})

	bg := color.RGBA{R: 10, G: 20, B: 30, A: 255}
	out := halfblock.Convert(img, halfblock.Options{
		Width: 2, Height: 1, Palette: halfblock.PaletteTrueColor, Background: bg,
	})
	norm := halfblock.NormalizeANSI(out)

	// Top-left transparent → background as BG of first cell.
	if !strings.Contains(norm, "ESC[48;2;10;20;30m") {
		t.Fatalf("expected background composite in output:\n%s", norm)
	}
	// Opaque red should appear as BG of second cell (top row).
	if !strings.Contains(norm, "ESC[48;2;255;0;0m") {
		t.Fatalf("expected opaque red:\n%s", norm)
	}
	// No pure black halo from uncomposited alpha.
	if strings.Contains(norm, "ESC[48;2;0;0;0m") {
		t.Fatalf("unexpected black bg (missing composite):\n%s", norm)
	}
}

func TestOddHeightPadsBottom(t *testing.T) {
	// 2×1 source scales to 1 cell wide × 1 cell tall (2 px). Odd heights after
	// decode are exercised via a 3-pixel-tall image scaled to even target.
	img := image.NewRGBA(image.Rect(0, 0, 1, 3))
	img.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	img.SetRGBA(0, 1, color.RGBA{0, 255, 0, 255})
	img.SetRGBA(0, 2, color.RGBA{0, 0, 255, 255})

	out := halfblock.Convert(img, halfblock.Options{
		Width: 1, Height: 2, Palette: halfblock.PaletteTrueColor,
		Background: color.RGBA{A: 255},
	})
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		t.Fatalf("height lines = %d, want 2", len(lines))
	}
	for i, line := range lines {
		if !strings.Contains(line, "▄") {
			t.Fatalf("line %d missing half-block glyph: %q", i, halfblock.NormalizeANSI(line))
		}
	}
}

func TestNearestNeighbourScaleAspect(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			if x < 2 {
				img.SetRGBA(x, y, color.RGBA{255, 0, 0, 255})
			} else {
				img.SetRGBA(x, y, color.RGBA{0, 0, 255, 255})
			}
		}
	}
	out := halfblock.Convert(img, halfblock.Options{
		Width: 4, Height: 2, Palette: halfblock.PaletteTrueColor,
		Background: color.RGBA{A: 255},
	})
	norm := halfblock.NormalizeANSI(out)
	lines := strings.Split(norm, "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d", len(lines))
	}
	// Left half should stay red-dominated; right half blue.
	left := lines[0][:len(lines[0])/2]
	right := lines[0][len(lines[0])/2:]
	if !strings.Contains(left, "255;0;0") {
		t.Fatalf("left half missing red: %s", left)
	}
	if !strings.Contains(right, "0;0;255") {
		t.Fatalf("right half missing blue: %s", right)
	}
}

func TestPalette256Fallback(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 2))
	img.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	img.SetRGBA(0, 1, color.RGBA{0, 0, 255, 255})

	truec := halfblock.Convert(img, halfblock.Options{
		Width: 1, Height: 1, Palette: halfblock.PaletteTrueColor, Background: color.RGBA{A: 255},
	})
	pal256 := halfblock.Convert(img, halfblock.Options{
		Width: 1, Height: 1, Palette: halfblock.Palette256, Background: color.RGBA{A: 255},
	})
	if strings.Contains(pal256, "38;2;") || strings.Contains(pal256, "48;2;") {
		t.Fatalf("256 mode leaked truecolor: %s", halfblock.NormalizeANSI(pal256))
	}
	if !strings.Contains(pal256, "38;5;") || !strings.Contains(pal256, "48;5;") {
		t.Fatalf("256 mode missing indexed colours: %s", halfblock.NormalizeANSI(pal256))
	}
	if !strings.Contains(truec, "38;2;") {
		t.Fatalf("truecolor missing: %s", halfblock.NormalizeANSI(truec))
	}
}

func TestConvertPNGBadImage(t *testing.T) {
	_, err := halfblock.ConvertPNG([]byte("not-a-png"), halfblock.Options{Width: 2, Height: 2})
	if err == nil {
		t.Fatal("expected error for bad image")
	}
}

func TestGoldenANSINormalized(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	img.SetRGBA(1, 0, color.RGBA{0, 255, 0, 255})
	img.SetRGBA(0, 1, color.RGBA{0, 0, 255, 255})
	img.SetRGBA(1, 1, color.RGBA{255, 255, 0, 255})

	out := halfblock.Convert(img, halfblock.Options{
		Width: 2, Height: 1, Palette: halfblock.PaletteTrueColor,
		Background: color.RGBA{A: 255},
	})
	got := halfblock.NormalizeANSI(out)
	want := "ESC[38;2;0;0;255mESC[48;2;255;0;0m▄ESC[38;2;255;255;0mESC[48;2;0;255;0m▄ESC[0m"
	if got != want {
		t.Fatalf("golden mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestCacheKeyDeterministic(t *testing.T) {
	opts := halfblock.Options{
		Width: 16, Height: 16, Palette: halfblock.PaletteTrueColor,
		Background: color.RGBA{R: 1, G: 2, B: 3, A: 255},
	}
	a := halfblock.CacheKey("abc", opts)
	b := halfblock.CacheKey("abc", opts)
	if a != b {
		t.Fatalf("keys differ: %q vs %q", a, b)
	}
	if a != "abc-16x16-truecolor-010203" {
		t.Fatalf("key = %q", a)
	}
	opts.Palette = halfblock.Palette256
	c := halfblock.CacheKey("abc", opts)
	if c == a {
		t.Fatal("palette change must invalidate key")
	}
	opts.Width = 8
	d := halfblock.CacheKey("abc", opts)
	if d == c {
		t.Fatal("dimension change must invalidate key")
	}
	hash, w, h, pal, bg, err := halfblock.ParseCacheKey(a)
	if err != nil || hash != "abc" || w != 16 || h != 16 || pal != halfblock.PaletteTrueColor || bg != "010203" {
		t.Fatalf("parse = %s %d %d %s %s err=%v", hash, w, h, pal, bg, err)
	}
}

func TestEncodeRoundTrip(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.SetRGBA(0, 0, color.RGBA{100, 100, 100, 255})
	data := encodePNG(t, img)
	out, err := halfblock.ConvertPNG(data, halfblock.Options{Width: 1, Height: 1, Palette: halfblock.Palette256})
	if err != nil || !strings.Contains(out, "▄") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}
