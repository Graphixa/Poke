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
	withBar := components.FooterSplit(false, 80, "Artwork ██████░░░░ 5/10")
	if withBar == left {
		t.Fatal("expected different footer with artwork progress")
	}
}

func TestFormatNativeCacheBarHidden(t *testing.T) {
	if got := tui.FormatNativeCacheBarForTest(36, 5, 10); got != "" {
		t.Fatalf("native warm must not show progress bar, got %q", got)
	}
}

func TestImageCacheFooterHiddenForNativeWarm(t *testing.T) {
	if got := tui.FormatNativeCacheBarForTest(40, 0, 1025); got != "" {
		t.Fatalf("expected hidden bar, got %q", got)
	}
}

func TestImageCacheFooterHiddenWhenComplete(t *testing.T) {
	if got := tui.FormatNativeCacheBarForTest(40, 8, 8); got != "" {
		t.Fatalf("expected hidden bar, got %q", got)
	}
}

func TestNativeCacheBarFitsFooterWidth(t *testing.T) {
	got := tui.FormatNativeCacheBarForTest(40, 432, 1025)
	if got != "" {
		t.Fatalf("native bar must be hidden, got %q", got)
	}
	if w := lipgloss.Width(got); w > components.FooterRightWidth {
		t.Fatalf("bar width=%d max=%d", w, components.FooterRightWidth)
	}
}
