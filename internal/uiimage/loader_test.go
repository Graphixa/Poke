package uiimage_test

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Graphixa/Pokedex/internal/uiimage"
)

func TestFromImageAndGenerateANSIBlockOpts(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	data, err := uiimage.FromImage(img)
	if err != nil {
		t.Fatal(err)
	}
	if data.Width != 8 || data.Height != 8 || len(data.Pixels) != 8*8*4 {
		t.Fatalf("unexpected image data: %+v len=%d", data, len(data.Pixels))
	}

	for _, q := range []uiimage.QualityPreset{
		uiimage.QualityMosaic,
		uiimage.QualityMosaicFast,
	} {
		got := uiimage.GenerateANSIBlockOpts(data, uiimage.RenderOpts{
			Width: 16, Height: 16, Quality: q,
			Background: color.RGBA{R: 0x31, G: 0x32, B: 0x44, A: 255},
		})
		if strings.TrimSpace(got) == "" {
			t.Fatalf("empty output for quality %d", q)
		}
	}
}

func TestLoadPNGAndExtractBuffer(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	path := filepath.Join(t.TempDir(), "tiny.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	data, err := uiimage.LoadPNGAndExtractBuffer(path)
	if err != nil {
		t.Fatal(err)
	}
	if data.Width != 4 || data.Height != 4 {
		t.Fatalf("got %dx%d", data.Width, data.Height)
	}
}
