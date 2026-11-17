package style

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Poke/internal/textwidth"
)

// FontGet-style key chip: fixed ANSI grey bg (dark 236) for consistent terminals.
const commandKeyBg = "236"

func commandKeyStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(SecondaryColor()).
		Background(lipgloss.Color(commandKeyBg)).
		Bold(true).
		Padding(0, 1)
}

func commandDescStyle() lipgloss.Style {
	mu.RLock()
	p := active
	mu.RUnlock()
	return lipgloss.NewStyle().Foreground(fg(p.Components))
}

// RenderKeyWithDescription renders a FontGet-style shortcut: chip key + desc.
func RenderKeyWithDescription(keyLabel, description string) string {
	return commandKeyStyle().Render(keyLabel) + " " + commandDescStyle().Render(description)
}

// RenderCommandHelp joins enabled bindings as key chips, truncated to width.
func RenderCommandHelp(bindings []key.Binding, width int) string {
	parts := make([]string, 0, len(bindings))
	seen := map[string]struct{}{}
	for _, b := range bindings {
		if !b.Enabled() {
			continue
		}
		h := b.Help()
		if h.Key == "" {
			continue
		}
		// Dedupe ↑/↓-style shared help keys from paired bindings.
		if _, ok := seen[h.Key]; ok {
			continue
		}
		seen[h.Key] = struct{}{}
		parts = append(parts, RenderKeyWithDescription(h.Key, h.Desc))
	}
	line := strings.Join(parts, "  ")
	if width > 0 {
		line = textwidth.Truncate(line, width)
	}
	return line
}
