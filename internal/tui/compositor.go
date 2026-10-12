package tui

import (
	"io"
	"os"
	"sync"
)

// compositorWriter appends native graphics after Bubble Tea's cell output.
// Bubble Tea v2.0.9 flushes a bytes.Buffer through io.Copy: Buffer.WriteTo makes
// one Write for the complete frame. Do not infer frames from cursor-show or
// synchronized-output markers: hidden-cursor, non-sync frames have neither.
// Keep this contract in step with cursed_renderer.go when upgrading Bubble Tea.
// The same mutex serialises output and pending state, so a clear cannot be
// overtaken by a previously captured paint, or a DCS interleaved with cell bytes.
type compositorWriter struct {
	file    *os.File
	inner   io.Writer
	mu      sync.Mutex
	pending []byte
	oneshot bool
	wrote   bool
}

func newCompositorWriter(inner io.Writer) *compositorWriter {
	c := &compositorWriter{inner: inner}
	if f, ok := inner.(*os.File); ok {
		c.file = f
	}
	return c
}
func (c *compositorWriter) Fd() uintptr {
	if c != nil && c.file != nil {
		return c.file.Fd()
	}
	return ^(uintptr(0))
}
func (c *compositorWriter) Read(p []byte) (int, error) {
	if c != nil && c.file != nil {
		return c.file.Read(p)
	}
	return 0, io.EOF
}
func (c *compositorWriter) Close() error { return nil }
func (c *compositorWriter) SetNative(seq string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending, c.oneshot, c.wrote = []byte(seq), false, false
}
func (c *compositorWriter) SetNativeOnce(seq string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending, c.oneshot, c.wrote = []byte(seq), true, false
}
func (c *compositorWriter) ClearNative() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending, c.oneshot, c.wrote = nil, false, false
}
func (c *compositorWriter) FlushPending() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.wrote {
		_ = c.writePendingLocked()
	}
}
func (c *compositorWriter) PaintNow(seq string) {
	if c == nil || seq == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending, c.oneshot, c.wrote = []byte(seq), false, false
	_ = c.writePendingLocked()
}
func (c *compositorWriter) WriteDirect(seq string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending, c.oneshot, c.wrote = nil, false, false
	if seq != "" {
		_, _ = io.WriteString(c.inner, seq)
	}
}
func (c *compositorWriter) Write(p []byte) (int, error) {
	if c == nil {
		return 0, io.ErrClosedPipe
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	n, err := c.inner.Write(p)
	if err != nil {
		return n, err
	}
	if n != len(p) {
		return n, io.ErrShortWrite
	}
	if len(p) > 0 {
		err = c.writePendingLocked()
	}
	return n, err
}

// Caller holds mu through both the cell write and the native tail.
func (c *compositorWriter) writePendingLocked() error {
	if len(c.pending) == 0 {
		return nil
	}
	n, err := c.inner.Write(c.pending)
	if err == nil && n != len(c.pending) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return err
	}
	c.wrote = true
	if c.oneshot {
		c.pending, c.oneshot = nil, false
	}
	return nil
}
