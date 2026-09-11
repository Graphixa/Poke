package sprite_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

func TestCropAndPrepareCompositesBackground(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	// Wide opaque strip → square side 8 with transparent bands that become bg.
	for y := 4; y < 8; y++ {
		for x := 4; x < 12; x++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	bg := color.RGBA{0, 0, 255, 255}
	out := sprite.CropAndPrepare(img, bg, 16, 0)
	if out.Bounds().Dx() != 8 || out.Bounds().Dy() != 8 {
		t.Fatalf("got %v want 8x8", out.Bounds())
	}
	// Top row of the square is outside the 4-tall strip → background.
	r, g, b, a := out.At(0, 0).RGBA()
	if a>>8 != 255 || b>>8 != 255 || r>>8 != 0 || g>>8 != 0 {
		t.Fatalf("expected blue bg at corner, got rgba=%d,%d,%d,%d", r>>8, g>>8, b>>8, a>>8)
	}
}

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
	img := image.NewRGBA(image.Rect(0, 0, 20, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 20; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 10, G: 20, B: 30, A: 255})
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

func TestPrepareEncodeSrcCropsSquareArtwork(t *testing.T) {
	// Square canvas with a centred subject — crop must fill the target, not
	// keep the transparent margin (that made browse sprites look tiny).
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 16; y < 48; y++ {
		for x := 16; x < 48; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	bg := color.RGBA{0, 0, 255, 255}
	out := sprite.PrepareEncodeSrc(img, 32, 32, sprite.SourceArtwork, bg, 16, 0)
	if out.Bounds().Dx() != 32 || out.Bounds().Dy() != 32 {
		t.Fatalf("got %v want 32x32", out.Bounds())
	}
	r, g, b, a := out.At(0, 0).RGBA()
	if a>>8 != 255 || r>>8 != 255 || g>>8 != 0 || b>>8 != 0 {
		t.Fatalf("expected subject at corner after crop, got rgba=%d,%d,%d,%d", r>>8, g>>8, b>>8, a>>8)
	}
}

func TestPrepareEncodeSrcArtworkFitsSquareTarget(t *testing.T) {
	// Square opaque subject into square target: fit fills corners (no letterbox).
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 255, G: 80, B: 40, A: 255})
		}
	}
	bg := color.RGBA{0, 0, 255, 255}
	out := sprite.PrepareEncodeSrc(img, 240, 240, sprite.SourceArtwork, bg, 16, 0)
	if out.Bounds().Dx() != 240 || out.Bounds().Dy() != 240 {
		t.Fatalf("got %v want 240x240", out.Bounds())
	}
	r, g, b, a := out.At(0, 0).RGBA()
	if a>>8 != 255 || r>>8 != 255 || b>>8 == 255 {
		t.Fatalf("expected subject at corner after fit, got rgba=%d,%d,%d,%d", r>>8, g>>8, b>>8, a>>8)
	}
}

func TestPrepareEncodeSrcArtworkLetterboxesWide(t *testing.T) {
	// Wide opaque strip into a square target: fit letterboxes (no cover-crop).
	img := image.NewRGBA(image.Rect(0, 0, 40, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 40; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 255, G: 80, B: 40, A: 255})
		}
	}
	bg := color.RGBA{0, 0, 255, 255}
	out := sprite.PrepareEncodeSrc(img, 16, 16, sprite.SourceArtwork, bg, 16, 0)
	if out.Bounds().Dx() != 16 || out.Bounds().Dy() != 16 {
		t.Fatalf("got %v want 16x16", out.Bounds())
	}
	r, g, b, a := out.At(8, 0).RGBA()
	if a>>8 != 255 || b>>8 != 255 || r>>8 != 0 {
		t.Fatalf("expected letterbox bg on the short axis, got rgba=%d,%d,%d,%d", r>>8, g>>8, b>>8, a>>8)
	}
}

func TestPrepareEncodeSrcPixelStillFits(t *testing.T) {
	// Pixel sprites keep fit/letterbox (no squash).
	img := image.NewRGBA(image.Rect(0, 0, 20, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 20; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 255, G: 80, B: 40, A: 255})
		}
	}
	bg := color.RGBA{0, 0, 255, 255}
	out := sprite.PrepareEncodeSrc(img, 16, 16, sprite.SourcePixel, bg, 16, 0)
	if out.Bounds().Dx() != 16 || out.Bounds().Dy() != 16 {
		t.Fatalf("got %v want 16x16", out.Bounds())
	}
	r, g, b, a := out.At(8, 0).RGBA()
	if a>>8 != 255 || b>>8 != 255 || r>>8 != 0 {
		t.Fatalf("expected letterbox bg on the short axis, got rgba=%d,%d,%d,%d", r>>8, g>>8, b>>8, a>>8)
	}
	r, g, b, a = out.At(8, 8).RGBA()
	if a>>8 != 255 || r>>8 != 255 {
		t.Fatalf("expected subject in the centre, got rgba=%d,%d,%d,%d", r>>8, g>>8, b>>8, a>>8)
	}
}

func TestPrepareEncodeSrcNonSquareStillCrops(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for y := 4; y < 12; y++ {
		for x := 10; x < 30; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 255, G: 80, B: 40, A: 255})
		}
	}
	bg := color.RGBA{A: 255}
	out := sprite.PrepareEncodeSrc(img, 16, 16, sprite.SourcePixel, bg, 16, 0)
	if out.Bounds().Dx() != 16 || out.Bounds().Dy() != 16 {
		t.Fatalf("got %v want 16x16", out.Bounds())
	}
}
