package tui_test

import (
	"testing"

	"github.com/Graphixa/Pokedex/internal/tui"
	"github.com/Graphixa/Pokedex/internal/tui/components"
)

func TestFooterSplitFixedWidthRight(t *testing.T) {
	left := components.FooterSplit(false, 80, "")
	if len(left) < 40 {
		t.Fatalf("footer too short: %q", left)
	}
	withBar := components.FooterSplit(false, 80, "Building image cache [████░░░░] 50%")
	if withBar == left {
		t.Fatal("expected different footer with progress")
	}
}

func TestFormatNativeCacheBar(t *testing.T) {
	got := tui.FormatNativeCacheBarForTest(5, 10)
	if got == "" || !contains(got, "Building image cache") {
		t.Fatalf("bar=%q", got)
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
