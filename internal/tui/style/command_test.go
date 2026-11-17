package style_test

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"

	"github.com/Graphixa/Poke/internal/tui/style"
)

func TestRenderKeyWithDescriptionChip(t *testing.T) {
	got := style.RenderKeyWithDescription("enter", "details")
	if !strings.Contains(got, "enter") || !strings.Contains(got, "details") {
		t.Fatalf("got %q", got)
	}
	// Chip uses ANSI 256 bg 236.
	if !strings.Contains(got, "48;5;236") {
		t.Fatalf("expected CommandKey bg 236, got %q", got)
	}
}

func TestRenderCommandHelpDedupesAndTruncates(t *testing.T) {
	bindings := []key.Binding{
		key.NewBinding(key.WithKeys("up"), key.WithHelp("↑/↓", "move")),
		key.NewBinding(key.WithKeys("down"), key.WithHelp("↑/↓", "move")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "details")),
	}
	got := style.RenderCommandHelp(bindings, 0)
	if strings.Count(got, "↑/↓") != 1 {
		t.Fatalf("dedupe failed: %q", got)
	}
	short := style.RenderCommandHelp(bindings, 8)
	if !strings.Contains(short, "…") {
		t.Fatalf("expected truncate: %q", short)
	}
}
