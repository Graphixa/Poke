package halfblock_test

import (
	"context"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/render/sprite/halfblock"
)

func TestCompositeTransparency(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.SetRGBA(0, 0, color.RGBA{0, 0, 0, 0})
	img.SetRGBA(1, 0, color.RGBA{255, 0, 0, 255})
	img.SetRGBA(0, 1, color.RGBA{0, 255, 0, 128})
	img.SetRGBA(1, 1, color.RGBA{0, 0, 255, 255})

	bg := color.RGBA{R: 10, G: 20, B: 30, A: 255}
	out := halfblock.Convert(img, 2, 1, halfblock.ColorTrueColor, bg)
	norm := halfblock.NormalizeANSI(out)

	if !strings.Contains(norm, "ESC[38;2;10;20;30m") && !strings.Contains(norm, "ESC[48;2;10;20;30m") {
		// Transparent top-left composites to bg as top (FG) of first cell with ▀.
		t.Fatalf("expected background composite in output:\n%s", norm)
	}
	if !strings.Contains(norm, "ESC[38;2;255;0;0m") && !strings.Contains(norm, "ESC[48;2;255;0;0m") {
		t.Fatalf("expected opaque red:\n%s", norm)
	}
}

func TestHalfBlockGlyph(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 2))
	img.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	img.SetRGBA(0, 1, color.RGBA{0, 255, 0, 255})
	out := halfblock.Convert(img, 1, 1, halfblock.ColorTrueColor, color.RGBA{A: 255})
	if !strings.Contains(out, "▀") {
		t.Fatalf("missing ▀ glyph: %s", halfblock.NormalizeANSI(out))
	}
}

func TestFitPreservesAspect(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			img.SetRGBA(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	bg := color.RGBA{R: 1, G: 2, B: 3, A: 255}
	out := halfblock.ConvertFit(img, 4, 4, halfblock.ColorTrueColor, bg)
	norm := halfblock.NormalizeANSI(out)
	if !strings.Contains(norm, "ESC[38;2;1;2;3m") && !strings.Contains(norm, "ESC[48;2;1;2;3m") {
		t.Fatalf("expected letterbox background:\n%s", norm)
	}
}

func TestANSI256(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 2))
	img.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	img.SetRGBA(0, 1, color.RGBA{0, 0, 255, 255})
	truec := halfblock.Convert(img, 1, 1, halfblock.ColorTrueColor, color.RGBA{A: 255})
	pal256 := halfblock.Convert(img, 1, 1, halfblock.Color256, color.RGBA{A: 255})
	if strings.Contains(pal256, "38;2;") || strings.Contains(pal256, "48;2;") {
		t.Fatalf("256 mode leaked truecolor: %s", halfblock.NormalizeANSI(pal256))
	}
	if !strings.Contains(pal256, "38;5;") {
		t.Fatalf("256 mode missing indexed colours: %s", halfblock.NormalizeANSI(pal256))
	}
	if !strings.Contains(truec, "38;2;") {
		t.Fatalf("truecolor missing: %s", halfblock.NormalizeANSI(truec))
	}
}

func TestRendererInterface(t *testing.T) {
	var r sprite.Renderer = halfblock.Renderer{}
	caps := sprite.Capabilities{TrueColor: true, ANSI256: true}
	if !r.Supported(caps) {
		t.Fatal("expected supported")
	}
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	frame, err := r.Render(context.Background(), img, sprite.RenderOptions{
		Width: 2, Height: 1, PreserveAR: true, ColorMode: sprite.ColorTrueColor,
		BackgroundColor: color.RGBA{A: 255}, Source: sprite.SourcePixel,
	})
	if err != nil || frame.Backend != "halfblock" || frame.Text == "" {
		t.Fatalf("frame=%+v err=%v", frame, err)
	}
}
