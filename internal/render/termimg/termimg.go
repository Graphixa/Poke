// Package termimg isolates github.com/blacktop/go-termimg behind a narrow
// adapter. Detection and environment workarounds run once per process.
package termimg

import (
	"fmt"
	stdimage "image"
	"os"
	"sync"

	termimg "github.com/blacktop/go-termimg"
)

// Protocol is an application-owned native graphics backend.
type Protocol int

const (
	ProtocolNone Protocol = iota
	ProtocolKitty
	ProtocolITerm
	ProtocolSixel
)

var (
	initOnce sync.Once
	active   Protocol
)

// Init applies the one-time TERMIMG_BYPASS_DETECTION workaround for the
// session-selected protocol. Subsequent calls are no-ops.
func Init(p Protocol) {
	initOnce.Do(func() {
		active = p
		if p == ProtocolNone {
			return
		}
		val := "halfblocks"
		switch p {
		case ProtocolKitty:
			val = "kitty"
		case ProtocolITerm:
			val = "iterm2"
		case ProtocolSixel:
			val = "sixel"
		}
		_ = os.Setenv("TERMIMG_BYPASS_DETECTION", val)
		termimg.ClearFeatureCache()
	})
}

// RenderKitty encodes src with the Kitty protocol, fitting the cell box.
func RenderKitty(src stdimage.Image, cellsW, cellsH int) (string, error) {
	return renderNative(src, cellsW, cellsH, termimg.Kitty)
}

// RenderITerm encodes src with the iTerm2 protocol, fitting the cell box.
func RenderITerm(src stdimage.Image, cellsW, cellsH int) (string, error) {
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

// ClearAll deletes prior native placements (Kitty/iTerm).
func ClearAll() error {
	return termimg.ClearAll()
}
