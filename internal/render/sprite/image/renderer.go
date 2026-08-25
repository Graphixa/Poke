// Package image renders sprites via native terminal graphics when available.
// Protocol details (Kitty, iTerm2, …) stay inside this package.
package image

import (
	"bytes"
	"context"
	"fmt"
	stdimage "image"
	_ "image/png"
	"os"
	"strings"
	"sync"

	termimg "github.com/blacktop/go-termimg"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

// protocol is an internal native graphics backend identity.
type protocol string

const (
	protocolNone  protocol = ""
	protocolKitty protocol = "kitty"
	protocolITerm protocol = "iterm"
)

// Renderer displays sprites as real images when the terminal supports it.
type Renderer struct {
	proto  protocol
	mu     sync.Mutex
	failed bool
}

// New creates an Image renderer that auto-detects Kitty/iTerm support from the
// environment. There is no user-facing protocol override.
func New() *Renderer {
	return &Renderer{proto: detectProtocol()}
}

// Name returns the logical renderer class.
func (r *Renderer) Name() string { return "image" }

// Supported reports whether native image output is available this session.
func (r *Renderer) Supported(caps sprite.Capabilities) bool {
	if r == nil || r.proto == protocolNone {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return !r.failed && caps.NativeImage
}

// Available is true when native rendering can be attempted.
func (r *Renderer) Available() bool {
	if r == nil || r.proto == protocolNone {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return !r.failed
}

// Disable turns off native rendering for the rest of the session.
func (r *Renderer) Disable() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.failed = true
	r.mu.Unlock()
}

// NativeCapable reports whether env heuristics suggest a native protocol.
func NativeCapable() bool {
	return detectProtocol() != protocolNone
}

func detectProtocol() protocol {
	if envKitty() {
		return protocolKitty
	}
	if envITerm() {
		return protocolITerm
	}
	return protocolNone
}

func envKitty() bool {
	termProgram := os.Getenv("TERM_PROGRAM")
	termName := strings.ToLower(os.Getenv("TERM"))
	switch termProgram {
	case "Apple_Terminal", "Terminal":
		return false
	case "ghostty", "WezTerm", "rio", "kitty":
		return true
	}
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return true
	}
	if os.Getenv("GHOSTTY_RESOURCES_DIR") != "" {
		return true
	}
	if strings.Contains(termName, "kitty") {
		return true
	}
	return false
}

func envITerm() bool {
	return os.Getenv("TERM_PROGRAM") == "iTerm.app"
}

// Render encodes the image with the selected native protocol.
func (r *Renderer) Render(ctx context.Context, src stdimage.Image, opts sprite.RenderOptions) (sprite.Frame, error) {
	if err := ctx.Err(); err != nil {
		return sprite.Frame{}, err
	}
	if !r.Available() {
		return sprite.Frame{}, fmt.Errorf("image: native backend unavailable")
	}
	w, h := opts.Width, opts.Height
	if w < 1 {
		w = sprite.FrameWidth
	}
	if h < 1 {
		h = sprite.FrameHeight
	}
	if src == nil {
		return sprite.Frame{}, fmt.Errorf("image: nil source")
	}

	threshold := opts.AlphaThreshold
	if threshold == 0 {
		threshold = 16
	}
	src = sprite.CropToContentSquare(src, threshold, 1)

	restore := bypassDetection(r.proto)
	defer restore()

	proto := termimg.Kitty
	if r.proto == protocolITerm {
		proto = termimg.ITerm2
	}

	out, err := termimg.New(src).
		Width(w).
		Height(h).
		Scale(termimg.ScaleFit).
		Protocol(proto).
		Render()
	if err != nil {
		return sprite.Frame{}, err
	}
	if out == "" {
		return sprite.Frame{}, fmt.Errorf("image: empty native render")
	}
	if err := ctx.Err(); err != nil {
		return sprite.Frame{}, err
	}
	return sprite.Frame{Text: out, Backend: "image", Native: true}, nil
}

// RenderPNG is a convenience used by the engine when PNG bytes are already loaded.
func (r *Renderer) RenderPNG(pngData []byte, w, h int) (sprite.Frame, error) {
	img, format, err := stdimage.Decode(bytes.NewReader(pngData))
	if err != nil {
		return sprite.Frame{}, fmt.Errorf("image: decode: %w", err)
	}
	if format != "png" {
		return sprite.Frame{}, fmt.Errorf("image: unsupported format %q", format)
	}
	return r.Render(context.Background(), img, sprite.RenderOptions{
		Width: w, Height: h, PreserveAR: true, Source: sprite.SourcePixel,
	})
}

func bypassDetection(proto protocol) (restore func()) {
	prev, had := os.LookupEnv("TERMIMG_BYPASS_DETECTION")
	val := "halfblocks"
	switch proto {
	case protocolKitty:
		val = "kitty"
	case protocolITerm:
		val = "iterm2"
	}
	_ = os.Setenv("TERMIMG_BYPASS_DETECTION", val)
	termimg.ClearFeatureCache()
	return func() {
		if had {
			_ = os.Setenv("TERMIMG_BYPASS_DETECTION", prev)
		} else {
			_ = os.Unsetenv("TERMIMG_BYPASS_DETECTION")
		}
		termimg.ClearFeatureCache()
	}
}

// ClearBestEffort deletes prior native placements. Safe no-op otherwise.
func ClearBestEffort() {
	defer func() { _ = recover() }()
	restore := bypassDetection(protocolKitty)
	defer restore()
	_ = termimg.ClearAll()
}
