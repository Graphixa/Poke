package chafa

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"sync"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

var (
	detectOnce sync.Once
	binPath    string
	binOK      bool
)

// Detect resolves the chafa binary once per process via LookPath.
func Detect() (path string, ok bool) {
	detectOnce.Do(func() {
		p, err := exec.LookPath("chafa")
		if err == nil && p != "" {
			binPath, binOK = p, true
		}
	})
	return binPath, binOK
}

func renderExec(ctx context.Context, bin string, src image.Image, opts sprite.RenderOptions) (sprite.Frame, error) {
	w, h := opts.Width, opts.Height
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	tmp, err := os.CreateTemp("", "poke-chafa-*.png")
	if err != nil {
		return sprite.Frame{}, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := png.Encode(tmp, src); err != nil {
		tmp.Close()
		return sprite.Frame{}, err
	}
	if err := tmp.Close(); err != nil {
		return sprite.Frame{}, err
	}

	args := []string{
		"--size", fmt.Sprintf("%dx%d", w, h),
		"--format", "symbols",
		"--symbols", "block",
		"--animate", "off",
		tmpName,
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return sprite.Frame{}, ctx.Err()
		}
		return sprite.Frame{}, fmt.Errorf("chafa: %w: %s", err, stderr.String())
	}
	out := stdout.Bytes()
	if len(bytes.TrimSpace(out)) == 0 {
		return sprite.Frame{}, fmt.Errorf("chafa: empty output")
	}
	return sprite.Frame{Text: string(out), Backend: "chafa"}, nil
}
