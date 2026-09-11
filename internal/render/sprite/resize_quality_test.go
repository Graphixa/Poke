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

func TestAlgorithmVersionBumpedForNativeKey(t *testing.T) {
	if sprite.AlgorithmVersion != "v15" {
		t.Fatalf("AlgorithmVersion=%s want v15 (10x20 cell px + artwork fit encode)", sprite.AlgorithmVersion)
	}
}

func TestPixelSizeMatchesCellDefaults(t *testing.T) {
	if sprite.SixelCellPxW != 10 || sprite.SixelCellPxH != 20 {
		t.Fatalf("cell px %dx%d want 10x20", sprite.SixelCellPxW, sprite.SixelCellPxH)
	}
	w, h := sprite.PixelSize(24, 12)
	if w != 240 || h != 240 {
		t.Fatalf("PixelSize(24,12)=%dx%d want 240x240", w, h)
	}
}

func TestNativeCacheKeyRoundTrip(t *testing.T) {
	bg := color.RGBA{R: 0x31, G: 0x32, B: 0x44, A: 255}
	key := sprite.NativeCacheKey("named-25-pikachu-artwork", "sixel", 40, 20, bg, 64)
	pxW, pxH := sprite.PixelSize(40, 20)
	wantPrefix := fmt.Sprintf("named-25-pikachu-artwork-image-sixel-%dx%d-313244-c64-", pxW, pxH)
	if !strings.HasPrefix(key, wantPrefix) || !strings.HasSuffix(key, sprite.AlgorithmVersion) {
		t.Fatalf("key=%q want prefix %q", key, wantPrefix)
	}
	hash, proto, w, h, bgHash, ver, palette, err := sprite.ParseNativeCacheKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if hash != "named-25-pikachu-artwork" || proto != "sixel" || w != pxW || h != pxH || bgHash != "313244" || ver != sprite.AlgorithmVersion || palette != 64 {
		t.Fatalf("parsed hash=%s proto=%s %dx%d bg=%s palette=%d ver=%s", hash, proto, w, h, bgHash, palette, ver)
	}
}

func TestNativeCacheKeyPaletteIdentity(t *testing.T) {
	bg := sprite.DefaultBackground
	hash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	k64 := sprite.NativeCacheKey(hash, "sixel", 32, 16, bg, 64)
	k128 := sprite.NativeCacheKey(hash, "sixel", 32, 16, bg, 128)
	k256 := sprite.NativeCacheKey(hash, "sixel", 32, 16, bg, 256)
	if k64 == k128 || k128 == k256 || k64 == k256 {
		t.Fatal("palette must change native cache key")
	}
	k32 := sprite.NativeCacheKey(hash, "sixel", 32, 16, bg, 64)
	k40 := sprite.NativeCacheKey(hash, "sixel", 40, 20, bg, 64)
	if k32 == k40 {
		t.Fatal("bucket must change native cache key")
	}
	bg2 := color.RGBA{R: 1, G: 2, B: 3, A: 255}
	kBg := sprite.NativeCacheKey(hash, "sixel", 32, 16, bg2, 64)
	if k64 == kBg {
		t.Fatal("background must change native cache key")
	}
}

func TestParseNativeCacheKeyRejectsEscape(t *testing.T) {
	if _, _, _, _, _, _, _, err := sprite.ParseNativeCacheKey("../escape-image-8x8-000000-v10"); err == nil {
		t.Fatal("expected reject")
	}
	if _, _, _, _, _, _, _, err := sprite.ParseNativeCacheKey(`bad\path-image-8x8-000000-v10`); err == nil {
		t.Fatal("expected reject")
	}
}

func TestParseNativeCacheKeyRejectsLegacyFormat(t *testing.T) {
	// Pre-v13 keys without palette tag must be rejected.
	legacy := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef-image-sixel-256x128-313244-v12"
	if _, _, _, _, _, _, _, err := sprite.ParseNativeCacheKey(legacy); err == nil {
		t.Fatal("expected reject of legacy key without palette tag")
	}
}
