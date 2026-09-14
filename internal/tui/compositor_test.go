package tui

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

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

func TestCompositorWriterSplitFrameTailsOnce(t *testing.T) {
	var buf bytes.Buffer
	c := newCompositorWriter(&buf)
	c.SetNative("DCS")
	if _, err := c.Write([]byte("PART1")); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "DCS") {
		t.Fatalf("must not tail mid-frame: %q", buf.String())
	}
	if _, err := c.Write(append([]byte("PART2"), syncOutputEnd...)); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "PART1PART2\x1b[?2026lDCS" {
		t.Fatalf("got %q", got)
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

func TestCompositorWriterNoTailBeforeFrameEnd(t *testing.T) {
	var buf bytes.Buffer
	c := newCompositorWriter(&buf)
	c.SetNative("DCS")
	if _, err := c.Write([]byte("CELLS")); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "CELLS" {
		t.Fatalf("got %q", buf.String())
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
