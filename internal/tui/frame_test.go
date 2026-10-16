package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestRenderCombinedFrameTopRight(t *testing.T) {
	frame := RenderCombinedFrame("Title", 20, 24, 5, "L", "R", "tab art")
	top := strings.Split(frame, "\n")[0]
	want := 20 + 1 + 24
	if w := lipgloss.Width(top); w != want {
		t.Fatalf("top width=%d want %d (%q)", w, want, top)
	}
	tee := strings.Index(top, "┬")
	if tee < 0 {
		t.Fatalf("no tee: %q", top)
	}
	if !strings.Contains(top[tee:], "tab art") {
		t.Fatalf("label not in right segment: %q", top[tee:])
	}
}
