package tui

import (
	"fmt"

	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Pokedex/internal/textwidth"
	"github.com/Graphixa/Pokedex/internal/tui/style"
)

// gamePickerLines renders the game list body for the centered picker popup.
func (m Model) gamePickerLines(width int) []string {
	if width < 24 {
		width = 24
	}
	var lines []string
	if m.gamePickerFilter != "" {
		lines = append(lines, "Filter: "+m.gamePickerFilter+"▌")
	} else {
		lines = append(lines, style.StatusMuted("Type to filter · ↑/↓ · Enter · Esc"))
	}
	lines = append(lines, "")
	if m.dialogCatalogErr != "" {
		lines = append(lines, style.StatusWarn(m.dialogCatalogErr))
		lines = append(lines, style.StatusMuted("Esc to close"))
		return lines
	}
	if len(m.gamePickerItems) == 0 {
		lines = append(lines, style.StatusMuted("No games match."))
		return lines
	}
	const window = 12
	start := m.gamePickerCursor - window/2
	if start < 0 {
		start = 0
	}
	end := start + window
	if end > len(m.gamePickerItems) {
		end = len(m.gamePickerItems)
		start = max(0, end-window)
	}
	for i := start; i < end; i++ {
		v := m.gamePickerItems[i]
		label := v.DisplayName
		if label == "" {
			label = v.Slug
		}
		if v.ID == 0 {
			label = "(All)"
		}
		prefix := "  "
		line := label
		if v.ID == m.gamePickerSuggest && m.dialogGameID == 0 && v.ID != 0 {
			line += "  · suggested"
		}
		if i == m.gamePickerCursor {
			prefix = "> "
			line = lipgloss.NewStyle().Foreground(style.PrimaryColor()).Bold(true).Render(prefix + line)
		} else if v.ID == m.dialogGameID || (v.ID == 0 && m.dialogGameID == 0) {
			line = lipgloss.NewStyle().Foreground(style.SecondaryColor()).Render(prefix + line)
		} else {
			line = prefix + line
		}
		lines = append(lines, textwidth.Truncate(line, width))
	}
	if end < len(m.gamePickerItems) {
		lines = append(lines, style.StatusMuted(fmt.Sprintf("… %d more", len(m.gamePickerItems)-end)))
	}
	return lines
}

func (m *Model) handleGamePickerKey(msg string, runes []rune) bool {
	switch msg {
	case "esc":
		m.dialogCatalogErr = ""
		m.closeGamePicker(false)
		return true
	case "enter":
		if m.dialogCatalogErr != "" || len(m.gamePickerItems) == 0 {
			return true
		}
		m.closeGamePicker(true)
		return true
	case "up":
		if m.gamePickerCursor > 0 {
			m.gamePickerCursor--
		}
		return true
	case "down":
		if m.gamePickerCursor < len(m.gamePickerItems)-1 {
			m.gamePickerCursor++
		}
		return true
	case "backspace":
		if m.gamePickerFilter != "" {
			r := []rune(m.gamePickerFilter)
			m.gamePickerFilter = string(r[:len(r)-1])
			m.filterGamePicker()
		}
		return true
	}
	if len(runes) == 1 {
		r := runes[0]
		if r >= 32 && r != 127 {
			m.gamePickerFilter += string(r)
			m.filterGamePicker()
			return true
		}
	}
	return true
}
