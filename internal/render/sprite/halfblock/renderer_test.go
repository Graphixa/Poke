package halfblock_test

import (
	"context"
	"image"
	"image/color"
	"testing"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/render/sprite/halfblock"
)

func TestHalfblockRender(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 200, B: 0, A: 255})
		}
	}
	frame, err := (halfblock.Renderer{}).Render(context.Background(), img, sprite.RenderOptions{
		Width: 16, Height: 8,
		BackgroundColor: color.RGBA{A: 255},
		ColorMode:       sprite.ColorTrueColor,
		Source:          sprite.SourceArtwork,
	})
	if err != nil {
		t.Fatal(err)
	}
	if frame.Backend != "halfblock" || frame.Text == "" {
		t.Fatalf("bad frame %+v", frame)
	}
	if !containsRune(frame.Text, '▄') {
		t.Fatal("expected lower half block glyph")
	}
}

func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}
