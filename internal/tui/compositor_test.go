package tui

import (
	"bytes"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

var syncOutputEnd = []byte("\x1b[?2026l")

func frameEnd() []byte { return append([]byte("CELLS"), syncOutputEnd...) }

func TestCompositorWriterCellsThenDCS(t *testing.T) {
	var buf bytes.Buffer
	c := newCompositorWriter(&buf)
	c.SetNative("DCS")
	if _, err := c.Write(frameEnd()); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "CELLS\x1b[?2026lDCS" {
		t.Fatalf("got %q want cells then sync end then DCS", got)
	}
	buf.Reset()
	if _, err := c.Write(frameEnd()); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "CELLS\x1b[?2026lDCS" {
		t.Fatalf("persistent pending per frame: got %q", got)
	}
}

// Match the actual upstream flush path: io.Copy from bytes.Buffer calls WriteTo.
func TestCompositorWriterHiddenCursorFrame(t *testing.T) {
	var buf bytes.Buffer
	c := newCompositorWriter(&buf)
	c.SetNative("DCS")
	for _, frame := range []string{"BROWSE", "RESTORED BROWSE", "NEXT SPECIES"} {
		buf.Reset()
		if _, err := io.Copy(c, bytes.NewBufferString(frame)); err != nil {
			t.Fatal(err)
		}
		if got := buf.String(); got != frame+"DCS" {
			t.Fatalf("unmarked frame lost art: %q", got)
		}
	}
}

func TestCompositorWriterClearOnce(t *testing.T) {
	var buf bytes.Buffer
	c := newCompositorWriter(&buf)
	c.SetNativeOnce("CLR")
	if _, err := c.Write(frameEnd()); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "CELLS\x1b[?2026lCLR" {
		t.Fatalf("got %q want ACLR", got)
	}
	buf.Reset()
	if _, err := c.Write(frameEnd()); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "CELLS\x1b[?2026l" {
		t.Fatalf("oneshot must not repeat: got %q", got)
	}
}

func TestCompositorWriterClearNative(t *testing.T) {
	var buf bytes.Buffer
	c := newCompositorWriter(&buf)
	c.SetNative("DCS")
	c.ClearNative()
	if _, err := c.Write(frameEnd()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "DCS") {
		t.Fatalf("ClearNative left DCS: %q", buf.String())
	}
}

func TestCompositorWriterFlushPendingKeepsPending(t *testing.T) {
	var buf bytes.Buffer
	c := newCompositorWriter(&buf)
	c.SetNative("DCS")
	c.FlushPending()
	if got := buf.String(); got != "DCS" {
		t.Fatalf("FlushPending write: got %q", got)
	}
	buf.Reset()
	if _, err := c.Write(frameEnd()); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "CELLS\x1b[?2026lDCS" {
		t.Fatalf("pending must survive FlushPending for re-assert: got %q", got)
	}
}

func TestCompositorWriterFlushSkipsAfterFrameTail(t *testing.T) {
	var buf bytes.Buffer
	c := newCompositorWriter(&buf)
	c.SetNative("DCS")
	if _, err := c.Write(frameEnd()); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	c.FlushPending()
	if got := buf.String(); got != "" {
		t.Fatalf("flush after frame tail must not double-write sixel: got %q", got)
	}
	// A new SetNative re-arms the flush (viewEquals skip path).
	c.SetNative("DCS2")
	c.FlushPending()
	if got := buf.String(); got != "DCS2" {
		t.Fatalf("new SetNative must flush: got %q", got)
	}
}

func TestCompositorWriterPaintNowKeepsPending(t *testing.T) {
	var buf bytes.Buffer
	c := newCompositorWriter(&buf)
	c.PaintNow("DCS")
	if got := buf.String(); got != "DCS" {
		t.Fatalf("PaintNow write: got %q", got)
	}
	buf.Reset()
	if _, err := c.Write(frameEnd()); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "CELLS\x1b[?2026lDCS" {
		t.Fatalf("pending must survive PaintNow for re-assert: got %q", got)
	}
}

func TestCompositorWriterWriteDirectClearsPending(t *testing.T) {
	var buf bytes.Buffer
	c := newCompositorWriter(&buf)
	c.SetNative("DCS")
	c.WriteDirect("CLR")
	if got := buf.String(); got != "CLR" {
		t.Fatalf("got %q want CLR", got)
	}
	buf.Reset()
	if _, err := c.Write(frameEnd()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "DCS") {
		t.Fatalf("WriteDirect must drop pending: %q", buf.String())
	}
}

func TestCompositorWriterForwardsStdoutFd(t *testing.T) {
	c := newCompositorWriter(os.Stdout)
	if c.Fd() != os.Stdout.Fd() {
		t.Fatalf("Fd=%v want stdout %v", c.Fd(), os.Stdout.Fd())
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCompositorOneShotFlushCannotEraseLaterFrames(t *testing.T) {
	var buf bytes.Buffer
	c := newCompositorWriter(&buf)
	c.SetNativeOnce("CLEAR")
	c.FlushPending()
	c.FlushPending()
	_, _ = c.Write(append([]byte("DOSSIER"), syncOutputEnd...))
	if got := buf.String(); got != "CLEARDOSSIER\x1b[?2026l" {
		t.Fatalf("stale clear: %q", got)
	}
}

// A paint and cell write must never enter the terminal writer concurrently.
// This writer makes any overlap observable without relying on bytes.Buffer races.
type overlapWriter struct {
	active  atomic.Int32
	overlap atomic.Bool
}

func (w *overlapWriter) Write(p []byte) (int, error) {
	if w.active.Add(1) != 1 {
		w.overlap.Store(true)
	}
	runtime.Gosched()
	w.active.Add(-1)
	return len(p), nil
}
func TestCompositorSerialisesOutput(t *testing.T) {
	w := &overlapWriter{}
	c := newCompositorWriter(w)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c.SetNative("DCS")
				c.FlushPending()
				_, _ = c.Write([]byte("CELLS"))
				c.WriteDirect("CLEAR")
			}
		}()
	}
	wg.Wait()
	if w.overlap.Load() {
		t.Fatal("cell and native output overlapped")
	}
}
