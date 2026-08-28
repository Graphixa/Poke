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
	"github.com/mattn/go-sixel"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

// protocol is an internal native graphics backend identity.
type protocol string

const (
	protocolNone  protocol = ""
	protocolKitty protocol = "kitty"
	protocolITerm protocol = "iterm"
	protocolSixel protocol = "sixel"
)

// Approximate monospace cell size used when converting cell dims → pixels
// for sixel encoding (CSI font-size query is out of scope).
const (
	sixelCellPxW = 8
	sixelCellPxH = 16
)

// Renderer displays sprites as real images when the terminal supports it.
type Renderer struct {
	proto  protocol
	mu     sync.Mutex
	failed bool
}

// New creates an Image renderer that auto-detects Kitty/iTerm/Sixel support from
// the environment. There is no user-facing protocol override.
func New() *Renderer {
	return &Renderer{proto: detectProtocol()}
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
	if strings.Contains(termName, "sixel") {
		return true
	}
	return false
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
		out, err := encodeSixel(src, w, h, opts)
		if err != nil {
			return sprite.Frame{}, err
		}
		if err := ctx.Err(); err != nil {
			return sprite.Frame{}, err
		}
		return sprite.Frame{Text: out, Backend: "image", Native: true}, nil
	}

	// Square official artwork: skip full-res content crop (termimg scales next).
	if b := src.Bounds(); b.Dx() != b.Dy() {
		src = sprite.CropToContentSquare(src, threshold, 1)
	}

	// go-termimg's Sixel path double-wraps mattn/go-sixel's already-complete DCS
	// sequence (ESC Pq + ESC P0;0;8q…ST), which Windows Terminal rejects.
	// Encode with go-sixel directly instead (handled above).

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

// encodeSixel builds a single valid DECSIXEL DCS via mattn/go-sixel.
func encodeSixel(src stdimage.Image, cellsW, cellsH int, opts sprite.RenderOptions) (string, error) {
	bg := opts.BackgroundColor
	if bg.A == 0 {
		bg = sprite.DefaultBackground
	}
	threshold := opts.AlphaThreshold
	if threshold == 0 {
		threshold = 16
	}
	pxW := cellsW * sixelCellPxW
	pxH := cellsH * sixelCellPxH
	source := opts.Source
	if source == "" {
		source = sprite.SourceArtwork
	}
	// Square artwork: downsample then Prepare; non-square: CropAndPrepare then Resize.
	prepared := sprite.PrepareEncodeSrc(src, pxW, pxH, source, bg, threshold, 1)

	var buf bytes.Buffer
	enc := sixel.NewEncoder(&buf)
	enc.Colors = 256
	if err := enc.Encode(prepared); err != nil {
		return "", fmt.Errorf("image: sixel encode: %w", err)
	}
	out := buf.String()
	if out == "" {
		return "", fmt.Errorf("image: empty sixel render")
	}
	// Sanity: exactly one DCS introducer (go-termimg used to emit two).
	if countESCPrefix(out, 'P') != 1 {
		return "", fmt.Errorf("image: malformed sixel sequence")
	}
	return out, nil
}

func countESCPrefix(s string, second byte) int {
	n := 0
	for i := 0; i+1 < len(s); i++ {
		if s[i] == 0x1b && s[i+1] == second {
			n++
		}
	}
	return n
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

func bypassDetection(proto protocol) (restore func()) {
	prev, had := os.LookupEnv("TERMIMG_BYPASS_DETECTION")
	val := "halfblocks"
	switch proto {
	case protocolKitty:
		val = "kitty"
	case protocolITerm:
		val = "iterm2"
	case protocolSixel:
		val = "sixel"
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

// ClearBestEffort deletes prior native placements for Kitty/iTerm.
// Sixel is a no-op: Bubble Tea alt-screen redraw replaces the frame, and
// sixel precise-clear can wipe the whole UI.
func ClearBestEffort() {
	proto := detectProtocol()
	if proto == protocolNone || proto == protocolSixel {
		return
	}
	defer func() { _ = recover() }()
	restore := bypassDetection(proto)
	defer restore()
	_ = termimg.ClearAll()
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
	pxW := cellsW * sixelCellPxW
	pxH := cellsH * sixelCellPxH
	img := stdimage.NewRGBA(stdimage.Rect(0, 0, pxW, pxH))
	for y := 0; y < pxH; y++ {
		for x := 0; x < pxW; x++ {
			img.Set(x, y, bg)
		}
	}
	return encodeSixel(img, cellsW, cellsH, sprite.RenderOptions{BackgroundColor: bg})
}
