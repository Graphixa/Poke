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

func TestNativeCapableEnv(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("GHOSTTY_RESOURCES_DIR", "")
	t.Setenv("TERM", "xterm")
	if spriteimage.NativeCapable("off") {
		t.Fatal("off should be false")
	}
	if spriteimage.NativeCapable("nope") {
		t.Fatal("unknown should be false")
	}
	if spriteimage.NativeCapable("auto") {
		t.Fatal("auto without hints should be false")
	}
	t.Setenv("KITTY_WINDOW_ID", "1")
	if !spriteimage.NativeCapable("auto") {
		t.Fatal("expected kitty via KITTY_WINDOW_ID")
	}
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("TERM_PROGRAM", "iTerm.app")
	if !spriteimage.NativeCapable("auto") {
		t.Fatal("expected iterm")
	}
}

func TestBackendDisable(t *testing.T) {
	var b *spriteimage.Renderer
	if b.Available() {
		t.Fatal("nil available")
	}
	os.Setenv("KITTY_WINDOW_ID", "1")
	t.Cleanup(func() { os.Unsetenv("KITTY_WINDOW_ID") })
	b = spriteimage.New("auto")
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
	b := spriteimage.New("auto")
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
	b := spriteimage.New("auto")
	// May fail in CI without a real terminal protocol handler; still must restore env.
	_, _ = b.RenderPNG(buf.Bytes(), 4, 4)
	if got := os.Getenv("TERMIMG_BYPASS_DETECTION"); got != prev {
		t.Fatalf("bypass env = %q, want %q", got, prev)
	}
}
