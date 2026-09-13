package tui

import (
	"bytes"
	"io"
	"os"
	"sync"
)

// compositorWriter forwards Bubble Tea cell output, then appends a pending
// native graphics sequence (CUP+DCS) once per completed frame. io.Copy may
// split a frame across several Write calls — tailing every Write injects DCS
// mid-stream and corrupts the terminal (list column garbage).
//
// When wrapping *os.File it implements charmbracelet/x/term.File (Read/Write/
// Close/Fd) so Bubble Tea still detects the console and enables VT mode.
type compositorWriter struct {
	file  *os.File // non-nil when inner is a console file
	inner io.Writer

	mu      sync.Mutex
	pending []byte
	oneshot bool // clear: append once then drop; paint: re-append every frame
}

// End-of-frame markers from Bubble Tea's cursed renderer (sync output or
// cursor-show fallback when mode 2026 is unavailable).
var (
	syncOutputEnd = []byte("\x1b[?2026l")
	cursorShow    = []byte("\x1b[?25h")
)

func newCompositorWriter(inner io.Writer) *compositorWriter {
	c := &compositorWriter{inner: inner}
	if f, ok := inner.(*os.File); ok {
		c.file = f
	}
	return c
}

// Fd implements term.File so BT can enable VT processing / read size.
func (c *compositorWriter) Fd() uintptr {
	if c != nil && c.file != nil {
		return c.file.Fd()
	}
	return ^(uintptr(0))
}

// Read implements term.File (unused for output; required for the interface).
func (c *compositorWriter) Read(p []byte) (int, error) {
	if c != nil && c.file != nil {
		return c.file.Read(p)
	}
	return 0, io.EOF
}

// Close implements term.File. Never closes the underlying console.
func (c *compositorWriter) Close() error { return nil }

// SetNative stores a sequence appended once after each completed cell frame.
func (c *compositorWriter) SetNative(seq string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.pending = []byte(seq)
	c.oneshot = false
	c.mu.Unlock()
}

// SetNativeOnce appends seq after the next completed frame only (geometry wipe).
func (c *compositorWriter) SetNativeOnce(seq string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.pending = []byte(seq)
	c.oneshot = true
	c.mu.Unlock()
}

// ClearNative drops any pending native sequence.
func (c *compositorWriter) ClearNative() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.pending = nil
	c.oneshot = false
	c.mu.Unlock()
}

func frameComplete(p []byte) bool {
	return bytes.Contains(p, syncOutputEnd) || bytes.Contains(p, cursorShow)
}

func (c *compositorWriter) Write(p []byte) (int, error) {
	if c == nil {
		return 0, io.ErrClosedPipe
	}
	if _, err := c.inner.Write(p); err != nil {
		return 0, err
	}
	if frameComplete(p) {
		c.writePendingTail()
	}
	return len(p), nil
}

func (c *compositorWriter) writePendingTail() {
	c.mu.Lock()
	seq := c.pending
	oneshot := c.oneshot
	if oneshot {
		c.pending = nil
		c.oneshot = false
	}
	c.mu.Unlock()
	if len(seq) == 0 {
		return
	}
	_, _ = c.inner.Write(seq)
}
