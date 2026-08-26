package image_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	spriteimage "github.com/Graphixa/Pokedex/internal/render/sprite/image"
)

func clearNativeEnv(t *testing.T) {
	t.Helper()
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("GHOSTTY_RESOURCES_DIR", "")
	t.Setenv("WT_SESSION", "")
	t.Setenv("TERM", "xterm")
}

func TestNativeCapableEnv(t *testing.T) {
	clearNativeEnv(t)
	if spriteimage.NativeCapable() {
		t.Fatal("without hints should be false")
	}
	t.Setenv("KITTY_WINDOW_ID", "1")
	if !spriteimage.NativeCapable() {
		t.Fatal("expected kitty via KITTY_WINDOW_ID")
	}
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("TERM_PROGRAM", "iTerm.app")
	if !spriteimage.NativeCapable() {
		t.Fatal("expected iterm")
	}
}

func TestNativeCapableSixelWTSession(t *testing.T) {
	clearNativeEnv(t)
	t.Setenv("WT_SESSION", "abc-123")
	r := spriteimage.New()
	if !r.Available() {
		t.Fatal("expected available via WT_SESSION")
	}
	if got := r.ProtocolName(); got != "sixel" {
		t.Fatalf("ProtocolName = %q, want sixel", got)
	}
}

func TestNativeCapableSixelWindowsTerminal(t *testing.T) {
	clearNativeEnv(t)
	t.Setenv("TERM_PROGRAM", "WindowsTerminal")
	r := spriteimage.New()
	if !r.Available() {
		t.Fatal("expected available via WindowsTerminal")
	}
	if got := r.ProtocolName(); got != "sixel" {
		t.Fatalf("ProtocolName = %q, want sixel", got)
	}
}

func TestKittyWinsOverSixel(t *testing.T) {
	clearNativeEnv(t)
	t.Setenv("WT_SESSION", "abc")
	t.Setenv("KITTY_WINDOW_ID", "1")
	r := spriteimage.New()
	if got := r.ProtocolName(); got != "kitty" {
		t.Fatalf("ProtocolName = %q, want kitty when both hinted", got)
	}
}

func TestBackendDisable(t *testing.T) {
	var b *spriteimage.Renderer
	if b.Available() {
		t.Fatal("nil available")
	}
	os.Setenv("KITTY_WINDOW_ID", "1")
	t.Cleanup(func() { os.Unsetenv("KITTY_WINDOW_ID") })
	b = spriteimage.New()
	if !b.Available() {
		t.Fatal("expected available")
	}
	b.Disable()
	if b.Available() {
		t.Fatal("expected disabled")
	}
}

func TestRenderPNGBad(t *testing.T) {
	os.Setenv("KITTY_WINDOW_ID", "1")
	t.Cleanup(func() { os.Unsetenv("KITTY_WINDOW_ID") })
	b := spriteimage.New()
	_, err := b.RenderPNG([]byte("not-png"), 8, 8)
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestRenderPNGBypassRestored(t *testing.T) {
	os.Setenv("KITTY_WINDOW_ID", "1")
	t.Cleanup(func() { os.Unsetenv("KITTY_WINDOW_ID") })
	prev := "halfblocks"
	os.Setenv("TERMIMG_BYPASS_DETECTION", prev)
	t.Cleanup(func() { os.Unsetenv("TERMIMG_BYPASS_DETECTION") })

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	b := spriteimage.New()
	// May fail in CI without a real terminal protocol handler; still must restore env.
	_, _ = b.RenderPNG(buf.Bytes(), 4, 4)
	if got := os.Getenv("TERMIMG_BYPASS_DETECTION"); got != prev {
		t.Fatalf("bypass env = %q, want %q", got, prev)
	}
}

func TestSixelBypassRestored(t *testing.T) {
	clearNativeEnv(t)
	t.Setenv("WT_SESSION", "test")
	prev := "halfblocks"
	os.Setenv("TERMIMG_BYPASS_DETECTION", prev)
	t.Cleanup(func() { os.Unsetenv("TERMIMG_BYPASS_DETECTION") })

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	b := spriteimage.New()
	_, _ = b.RenderPNG(buf.Bytes(), 4, 4)
	if got := os.Getenv("TERMIMG_BYPASS_DETECTION"); got != prev {
		t.Fatalf("bypass env = %q, want %q", got, prev)
	}
}

func TestSixelEncodeSingleDCS(t *testing.T) {
	clearNativeEnv(t)
	t.Setenv("WT_SESSION", "test")

	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetRGBA(x, y, color.RGBA{255, 80, 0, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	b := spriteimage.New()
	frame, err := b.RenderPNG(buf.Bytes(), 8, 4)
	if err != nil {
		t.Fatalf("RenderPNG: %v", err)
	}
	if !frame.Native || frame.Text == "" {
		t.Fatal("expected native sixel frame")
	}
	if frame.Text[0] != 0x1b || frame.Text[1] != 'P' {
		t.Fatalf("expected DCS start ESC P, got %q", frame.Text[:min(8, len(frame.Text))])
	}
	escP := 0
	for i := 0; i+1 < len(frame.Text); i++ {
		if frame.Text[i] == 0x1b && frame.Text[i+1] == 'P' {
			escP++
		}
	}
	if escP != 1 {
		t.Fatalf("ESC P count = %d, want 1 (go-termimg double-wrap bug)", escP)
	}
}
