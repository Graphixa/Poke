// Diagnostic: encode a tiny sixel image and print it.
//
// Run this inside Windows Terminal (not Cursor/VS Code integrated terminal):
//
//	go run ./scripts/diag_sixel.go
//
// You should see a small orange square. If you see raw escape garbage instead,
// that terminal host does not render Sixel.
package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"os"
	"strings"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
	spriteimage "github.com/Graphixa/Pokedex/internal/render/sprite/image"
	"github.com/mattn/go-sixel"
)

func main() {
	fmt.Fprintln(os.Stderr, "=== terminal env ===")
	for _, k := range []string{"WT_SESSION", "WT_PROFILE_ID", "TERM_PROGRAM", "TERM"} {
		fmt.Fprintf(os.Stderr, "%s=%q\n", k, os.Getenv(k))
	}

	r := spriteimage.New()
	fmt.Fprintf(os.Stderr, "detected protocol=%q available=%v\n", r.ProtocolName(), r.Available())

	img := image.NewRGBA(image.Rect(0, 0, 24, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 24; x++ {
			img.Set(x, y, color.RGBA{255, uint8(x * 10), 0, 255})
		}
	}

	var buf bytes.Buffer
	enc := sixel.NewEncoder(&buf)
	_ = enc.Encode(img)
	raw := buf.String()
	fmt.Fprintf(os.Stderr, "go-sixel escP=%d len=%d\n", strings.Count(raw, "\x1bP"), len(raw))

	frame, err := r.Render(context.Background(), img, sprite.RenderOptions{
		Width: 12, Height: 6, Source: sprite.SourcePixel,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "render error:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "renderer native=%v escP=%d len=%d\n",
		frame.Native, strings.Count(frame.Text, "\x1bP"), len(frame.Text))
	_ = os.WriteFile("sixel-smoke.out", []byte(frame.Text), 0o644)

	fmt.Fprintln(os.Stderr, "--- sixel to stdout (expect a small image in Windows Terminal) ---")
	_, _ = os.Stdout.WriteString(frame.Text)
	_, _ = os.Stdout.WriteString("\n")
	fmt.Fprintln(os.Stderr, "--- end ---")
	fmt.Fprintln(os.Stderr, "If that looked like garbage text, run this from Windows Terminal proper:")
	fmt.Fprintln(os.Stderr, "  wt.exe -d . go run ./scripts/diag_sixel.go")
}
