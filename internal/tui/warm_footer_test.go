package tui_test

import (
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Pokedex/internal/tui"
	"github.com/Graphixa/Pokedex/internal/tui/components"
)

func TestFooterSplitFixedWidthRight(t *testing.T) {
	left := components.FooterSplit(false, 80, "")
	if len(left) < 40 {
		t.Fatalf("footer too short: %q", left)
	}
	withBar := components.FooterSplit(false, 80, "Image Cache ██████░░░░ 5/10")
	if withBar == left {
		t.Fatal("expected different footer with progress")
	}
}

func TestFormatNativeCacheBar(t *testing.T) {
	got := tui.FormatNativeCacheBarForTest(36, 5, 10)
	if got == "" || !contains(got, "Image Cache") {
		t.Fatalf("bar=%q", got)
	}
	if !contains(got, "(36)") {
		t.Fatalf("expected size bucket in bar=%q", got)
	}
	if !contains(got, "5/10") {
		t.Fatalf("expected count in bar=%q", got)
	}
}

func TestNativeCacheBarFitsFooterWidth(t *testing.T) {
	got := tui.FormatNativeCacheBarForTest(40, 432, 1025)
	if w := lipgloss.Width(got); w > components.FooterRightWidth {
		t.Fatalf("bar width=%d max=%d bar=%q", w, components.FooterRightWidth, got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexSubstring(s, sub) >= 0)
}

func indexSubstring(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
