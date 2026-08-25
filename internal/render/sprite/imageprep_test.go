package sprite_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

func TestCropToContentSquareTrimsMargins(t *testing.T) {
	// 40×40 canvas with a 10×10 opaque block offset in the centre-ish area.
	img := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for y := 12; y < 22; y++ {
		for x := 14; x < 24; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 255, G: 80, B: 40, A: 255})
		}
	}

	out := sprite.CropToContentSquare(img, 16, 0)
	if out.Bounds().Dx() != 10 || out.Bounds().Dy() != 10 {
		t.Fatalf("got %dx%d want 10x10", out.Bounds().Dx(), out.Bounds().Dy())
	}
	// Corner of crop should be opaque subject, not empty.
	if _, _, _, a := out.At(0, 0).RGBA(); uint8(a>>8) < 16 {
		t.Fatal("expected opaque subject at crop origin")
	}
}

func TestCropToContentSquarePadsToOneToOne(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 30, 30))
	for y := 5; y < 25; y++ { // 20 tall
		for x := 10; x < 20; x++ { // 10 wide
			img.SetRGBA(x, y, color.RGBA{R: 10, G: 200, B: 10, A: 255})
		}
	}
	out := sprite.CropToContentSquare(img, 16, 0)
	if out.Bounds().Dx() != out.Bounds().Dy() {
		t.Fatalf("not square: %dx%d", out.Bounds().Dx(), out.Bounds().Dy())
	}
	if out.Bounds().Dx() != 20 {
		t.Fatalf("side=%d want 20", out.Bounds().Dx())
	}
}

func TestContentBoundsEmptyFallsBack(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	b := sprite.ContentBounds(img, 16)
	if !b.Eq(img.Bounds()) {
		t.Fatalf("bounds=%v want %v", b, img.Bounds())
	}
}
