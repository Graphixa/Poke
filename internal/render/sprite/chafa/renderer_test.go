package chafa_test

import (
	"context"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
	spritechafa "github.com/Graphixa/Pokedex/internal/render/sprite/chafa"
)

func TestChafaRenderer(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 40, G: 120, B: 200, A: 255})
		}
	}
	frame, err := (spritechafa.Renderer{}).Render(context.Background(), img, sprite.RenderOptions{
		Width:           40,
		Height:          40,
		PreserveAR:      true,
		ColorMode:       sprite.ColorTrueColor,
		BackgroundColor: color.RGBA{R: 0x31, G: 0x32, B: 0x44, A: 255},
		AlphaThreshold:  16,
		Source:          sprite.SourcePixel,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(frame.Text) == "" {
		t.Fatal("empty frame")
	}
	if frame.Backend != "chafa" {
		t.Fatalf("backend=%s", frame.Backend)
	}
}
