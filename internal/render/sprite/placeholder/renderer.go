package placeholder

import (
	"context"
	"image"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/Graphixa/Poke/internal/render/sprite"
)

// Renderer always succeeds, producing a centred [ Name ] label.
type Renderer struct{}

// Name returns the logical renderer class.
func (Renderer) Name() string { return "placeholder" }

// Supported always returns true — the final fallback must never fail.
func (Renderer) Supported(caps sprite.Capabilities) bool {
	_ = caps
	return true
}

// Render returns a centred placeholder; src may be nil.
func (r Renderer) Render(_ context.Context, _ image.Image, opts sprite.RenderOptions) (sprite.Frame, error) {
	w, h := opts.Width, opts.Height
	if w < 1 {
		w = sprite.FrameWidth
	}
	if h < 1 {
		h = sprite.FrameHeight
	}
	name := opts.Label
	if name == "" {
		name = "???"
	}
	label := "[ " + name + " ]"
	if lipgloss.Width(label) > w {
		label = truncate(label, w)
	}
	return sprite.Frame{Text: centre(label, w, h), Backend: "placeholder"}, nil
}

func centre(label string, w, h int) string {
	lw := lipgloss.Width(label)
	padLeft := (w - lw) / 2
	if padLeft < 0 {
		padLeft = 0
	}
	padRight := w - lw - padLeft
	if padRight < 0 {
		padRight = 0
	}
	line := strings.Repeat(" ", padLeft) + label + strings.Repeat(" ", padRight)
	row := (h - 1) / 2
	if row < 0 {
		row = 0
	}
	lines := make([]string, h)
	blank := strings.Repeat(" ", w)
	for i := 0; i < h; i++ {
		if i == row {
			lines[i] = line
		} else {
			lines[i] = blank
		}
	}
	return strings.Join(lines, "\n")
}

func truncate(s string, max int) string {
	if max < 1 {
		return ""
	}
	if lipgloss.Width(s) <= max {
		return s
	}
	var b strings.Builder
	width := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if width+rw > max {
			break
		}
		b.WriteRune(r)
		width += rw
	}
	return b.String()
}
