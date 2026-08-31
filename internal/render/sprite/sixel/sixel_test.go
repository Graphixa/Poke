package sixel_test

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
	spritesixel "github.com/Graphixa/Pokedex/internal/render/sprite/sixel"
)

func TestEncodeFromCellsSnapsHeightToSixelBands(t *testing.T) {
	const cellsW, cellsH = 40, 20
	pxW, pxH := sprite.PixelSize(cellsW, cellsH)
	if pxH%6 == 0 {
		t.Fatalf("fixture: PixelSize height %d already band-aligned", pxH)
	}
	wantH := pxH - (pxH % 6)

	src := image.NewRGBA(image.Rect(0, 0, 8, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 8; x++ {
			src.Set(x, y, color.RGBA{R: 0x80, G: 0x40, B: 0x20, A: 255})
		}
	}
	out, err := spritesixel.EncodeFromCells(src, cellsW, cellsH, sprite.RenderOptions{
		BackgroundColor: sprite.DefaultBackground,
	})
	if err != nil {
		t.Fatal(err)
	}
	raster := fmt.Sprintf(`"1;1;%d;%d`, pxW, wantH)
	if !strings.Contains(out, raster) {
		head := out
		if len(head) > 64 {
			head = head[:64]
		}
		t.Fatalf("raster header %q want substring %s", head, raster)
	}
}

func TestEncodeCropsHeightToSixelBands(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 12, 20)) // 20 % 6 == 2
	for y := 0; y < 20; y++ {
		for x := 0; x < 12; x++ {
			src.Set(x, y, color.RGBA{R: 0x10, G: 0x20, B: 0x30, A: 255})
		}
	}
	out, err := spritesixel.Encode(src)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, `"1;1;12;20`) {
		t.Fatal("encode must not keep leftover-band height 20")
	}
	if !strings.Contains(out, `"1;1;12;18`) {
		head := out
		if len(head) > 64 {
			head = head[:64]
		}
		t.Fatalf("raster header %q want height 18", head)
	}
}

func TestEncodeWithColors(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 8, 12))
	for y := 0; y < 12; y++ {
		for x := 0; x < 8; x++ {
			src.Set(x, y, color.RGBA{R: 0x80, G: 0x40, B: 0x20, A: 255})
		}
	}
	for _, n := range []int{64, 128, 256} {
		out, err := spritesixel.EncodeWithColors(src, n)
		if err != nil {
			t.Fatalf("colors=%d: %v", n, err)
		}
		if out == "" {
			t.Fatalf("colors=%d: empty output", n)
		}
	}
}

func TestMaxColorsIs256(t *testing.T) {
	if spritesixel.MaxColors != 256 {
		t.Fatalf("MaxColors=%d want 256", spritesixel.MaxColors)
	}
}
