package sprite_test

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

func TestResizeArtworkInteractiveProducesTargetSize(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 200, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 200; x++ {
			src.SetRGBA(x, y, color.RGBA{R: 200, G: 40, B: 40, A: 255})
		}
	}
	bg := color.RGBA{A: 255}
	out := sprite.ResizeWithQuality(src, 64, 64, sprite.SourceArtwork, sprite.QualityInteractive, true, bg)
	if out.Bounds().Dx() != 64 || out.Bounds().Dy() != 64 {
		t.Fatalf("got %v", out.Bounds())
	}
	prewarm := sprite.ResizeWithQuality(src, 64, 64, sprite.SourceArtwork, sprite.QualityPrewarm, true, bg)
	if prewarm.Bounds().Dx() != 64 {
		t.Fatalf("prewarm size %v", prewarm.Bounds())
	}
}

func TestAlgorithmVersionBumpedForPrep(t *testing.T) {
	if sprite.AlgorithmVersion != "v12" {
		t.Fatalf("AlgorithmVersion=%s want v12 (sixel band-aligned encode height)", sprite.AlgorithmVersion)
	}
}

func TestNativeCacheKeyRoundTrip(t *testing.T) {
	bg := color.RGBA{R: 0x31, G: 0x32, B: 0x44, A: 255}
	key := sprite.NativeCacheKey("named-25-pikachu-artwork", "sixel", 40, 20, bg)
	pxW, pxH := sprite.PixelSize(40, 20)
	wantPrefix := fmt.Sprintf("named-25-pikachu-artwork-image-sixel-%dx%d-313244-", pxW, pxH)
	if !strings.HasPrefix(key, wantPrefix) || !strings.HasSuffix(key, sprite.AlgorithmVersion) {
		t.Fatalf("key=%q want prefix %q", key, wantPrefix)
	}
	hash, proto, w, h, bgHash, ver, err := sprite.ParseNativeCacheKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if hash != "named-25-pikachu-artwork" || proto != "sixel" || w != pxW || h != pxH || bgHash != "313244" || ver != sprite.AlgorithmVersion {
		t.Fatalf("parsed hash=%s proto=%s %dx%d bg=%s ver=%s", hash, proto, w, h, bgHash, ver)
	}
}

func TestParseNativeCacheKeyRejectsEscape(t *testing.T) {
	if _, _, _, _, _, _, err := sprite.ParseNativeCacheKey("../escape-image-8x8-000000-v10"); err == nil {
		t.Fatal("expected reject")
	}
	if _, _, _, _, _, _, err := sprite.ParseNativeCacheKey(`bad\path-image-8x8-000000-v10`); err == nil {
		t.Fatal("expected reject")
	}
}
