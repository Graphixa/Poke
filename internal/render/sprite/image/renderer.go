// Package image renders sprites via native terminal graphics when available.
// Protocol details (Kitty, iTerm2, Sixel, …) stay inside this package.
package image

import (
	"bytes"
	"context"
	"fmt"
	stdimage "image"
	"image/color"
	_ "image/png"
	"os"
	"strings"
	"sync"

	termimg "github.com/blacktop/go-termimg"

	"github.com/Graphixa/Poke/internal/render/sprite"
)

// protocol is an internal native graphics backend identity.
type protocol string

const (
	protocolNone  protocol = ""
	protocolKitty protocol = "kitty"
	protocolITerm protocol = "iterm"
	protocolSixel protocol = "sixel"
)

// Renderer displays sprites as real images when the terminal supports it.
type Renderer struct {
	proto  protocol
	mu     sync.Mutex
	failed bool
}

var termimgInitOnce sync.Once

func initTermimg(p protocol) {
	termimgInitOnce.Do(func() {
		if p == protocolNone {
			return
		}
		val := "halfblocks"
		switch p {
		case protocolKitty:
			val = "kitty"
		case protocolITerm:
			val = "iterm2"
		case protocolSixel:
			val = "sixel"
		}
		_ = os.Setenv("TERMIMG_BYPASS_DETECTION", val)
		termimg.ClearFeatureCache()
	})
}

func renderKitty(src stdimage.Image, cellsW, cellsH int) (string, error) {
	return renderNative(src, cellsW, cellsH, termimg.Kitty)
}

func renderITerm(src stdimage.Image, cellsW, cellsH int) (string, error) {
	return renderNative(src, cellsW, cellsH, termimg.ITerm2)
}

func renderNative(src stdimage.Image, cellsW, cellsH int, proto termimg.Protocol) (string, error) {
	if src == nil {
		return "", fmt.Errorf("termimg: nil source")
	}
	if cellsW < 1 {
		cellsW = 1
	}
	if cellsH < 1 {
		cellsH = 1
	}
	out, err := termimg.New(src).
		Width(cellsW).
		Height(cellsH).
		Scale(termimg.ScaleFit).
		Protocol(proto).
		Render()
	if err != nil {
		return "", err
	}
	if out == "" {
		return "", fmt.Errorf("termimg: empty native render")
	}
	return out, nil
}

func clearAllNative() error {
	return termimg.ClearAll()
}

// New creates an Image renderer that auto-detects Kitty/iTerm/Sixel support from
// the environment. There is no user-facing protocol override.
func New() *Renderer {
	r := &Renderer{proto: detectProtocol()}
	switch r.proto {
	case protocolKitty, protocolITerm, protocolSixel:
		initTermimg(r.proto)
	}
	return r
}

// Name returns the logical renderer class.
func (r *Renderer) Name() string { return "image" }

// ProtocolName returns the active native protocol tag (kitty|iterm|sixel), or
// empty when none. Used for mem+disk native cache keys.
func (r *Renderer) ProtocolName() string {
	if r == nil {
		return ""
	}
	return string(r.proto)
}

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
	if envSixel() {
		return protocolSixel
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

// envSixel reports hosts known to support Sixel graphics.
// Kitty / Ghostty / iTerm are excluded (handled earlier in detectProtocol).
func envSixel() bool {
	if os.Getenv("WT_SESSION") != "" {
		return true
	}
	termProgram := strings.ToLower(os.Getenv("TERM_PROGRAM"))
	switch termProgram {
	case "windowsterminal":
		return true
	case "mintty":
		return true
	}
	// Some hosts expose WT without TERM_PROGRAM (or only WT_PROFILE_ID).
	if os.Getenv("WT_PROFILE_ID") != "" {
		return true
	}
	termName := strings.ToLower(os.Getenv("TERM"))
	return strings.Contains(termName, "sixel")
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
	if r.proto == protocolSixel {
		out, err := EncodeFromCells(src, w, h, opts)
		if err != nil {
			return sprite.Frame{}, err
		}
		if err := ctx.Err(); err != nil {
			return sprite.Frame{}, err
		}
		return sprite.Frame{Text: out, Backend: "image", Native: true}, nil
	}

	// Content crop + ScaleFit into the cell box (single scale path — do not
	// PrepareEncodeSrc then ScaleFill, which double-scales and fights WT).
	src = sprite.CropToContentSquare(src, threshold, 0)

	var (
		out string
		err error
	)
	switch r.proto {
	case protocolKitty:
		out, err = renderKitty(src, w, h)
	case protocolITerm:
		out, err = renderITerm(src, w, h)
	default:
		return sprite.Frame{}, fmt.Errorf("image: native backend unavailable")
	}
	if err != nil {
		return sprite.Frame{}, err
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
		Width: w, Height: h, Source: sprite.SourcePixel,
	})
}

// ClearBestEffort deletes prior native placements for Kitty/iTerm.
// Sixel is a no-op: Bubble Tea alt-screen redraw replaces the frame, and
// sixel precise-clear can wipe the whole UI.
func ClearBestEffort() {
	proto := detectProtocol()
	if proto == protocolNone || proto == protocolSixel {
		return
	}
	defer func() { _ = recover() }()
	_ = clearAllNative()
}

// ClearNativeRegion returns a scoped clear sequence for a WxH cell region.
// Sixel fills the region with theme background; Kitty/iTerm returns empty (caller uses spaces).
func ClearNativeRegion(cellsW, cellsH int, bg color.RGBA) string {
	if cellsW < 1 || cellsH < 1 {
		return ""
	}
	proto := detectProtocol()
	if proto != protocolSixel {
		return ""
	}
	seq, err := clearSixelRegion(cellsW, cellsH, bg)
	if err != nil {
		return ""
	}
	return seq
}

func clearSixelRegion(cellsW, cellsH int, bg color.RGBA) (string, error) {
	if bg.A == 0 {
		bg = sprite.DefaultBackground
	}
	pxW := cellsW * sprite.SixelCellPxW
	pxH := cellsH * sprite.SixelCellPxH
	img := stdimage.NewRGBA(stdimage.Rect(0, 0, pxW, pxH))
	for y := 0; y < pxH; y++ {
		for x := 0; x < pxW; x++ {
			img.Set(x, y, bg)
		}
	}
	return EncodeFromCells(img, cellsW, cellsH, sprite.RenderOptions{BackgroundColor: bg})
}
