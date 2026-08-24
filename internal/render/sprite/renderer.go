package sprite

import (
	"context"
	"image"
)

// Renderer converts a prepared image into a terminal Frame.
// Terminal protocol details must not leak through this interface.
type Renderer interface {
	Name() string
	Supported(caps Capabilities) bool
	Render(ctx context.Context, src image.Image, opts RenderOptions) (Frame, error)
}

// Frame is the render output consumed by the TUI sprite panel.
type Frame struct {
	Text    string
	Backend string // logical class: image, halfblock, braille, ascii, placeholder
	// Native is true when Text embeds native graphics sequences.
	Native bool
}

// IsEmpty reports whether the frame has no drawable content.
func (f Frame) IsEmpty() bool {
	return f.Text == ""
}
