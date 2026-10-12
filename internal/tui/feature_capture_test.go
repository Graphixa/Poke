package tui

import (
	"os"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestCaptureFeatures(t *testing.T) {
	if os.Getenv("POKE_CAPTURE") == "" {
		t.Skip("manual capture")
	}
	m := featureModel()
	m.width, m.height = 100, 24
	m = featureKey(m, tea.KeyF2)
	if err := os.WriteFile("../../docs/filter-preview.txt", []byte(ansi.Strip(m.View().Content)), 0600); err != nil {
		t.Fatal(err)
	}
	m = featureKey(m, tea.KeyEscape)
	m = featureKey(m, tea.KeyF3)
	m = featureKey(m, tea.KeyDown)
	m = featureKey(m, tea.KeyF3)
	if err := os.WriteFile("../../docs/compare-preview.txt", []byte(ansi.Strip(m.View().Content)), 0600); err != nil {
		t.Fatal(err)
	}
}
