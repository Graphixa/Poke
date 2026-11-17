package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Graphixa/Poke/internal/domain"
	"github.com/Graphixa/Poke/internal/textwidth"
	"github.com/Graphixa/Poke/internal/tui/components"
	"github.com/Graphixa/Poke/internal/tui/style"
)

// dossierPickerView paints G/F/M choice list as a centered popup over the dossier.
func (m Model) dossierPickerView(dossier string) string {
	w, h := m.width, m.height
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	footer := style.RenderCommandHelp([]key.Binding{
		featureBinding("up", "↑/↓", "choose"),
		featureBinding("enter", "enter", "select"),
		featureBinding("esc", "esc", "cancel"),
	}, w)
	bodyH := h - 1
	if bodyH < 8 {
		bodyH = h
		footer = ""
	}
	cardW := min(48, w-4)
	if cardW < 28 {
		cardW = min(w, 28)
	}
	innerW := cardW - 4
	if innerW < 12 {
		innerW = 12
	}
	body := m.overlayPickerLines(innerW)
	card := components.InnerCard(m.dossierPickerTitle(), cardW, len(body)+2, body)
	bg := lipgloss.NewStyle().Faint(true).Render(ansi.Strip(dossier))
	// Strip trailing footer from dossier so overlay uses the same body band.
	bgLines := strings.Split(strings.TrimRight(bg, "\n"), "\n")
	if len(bgLines) > bodyH {
		bgLines = bgLines[:bodyH]
	}
	bg = strings.Join(bgLines, "\n")
	merged := overlayCenterCard(bg, card, w, bodyH)
	if footer == "" {
		return merged
	}
	return merged + "\n" + textwidth.PadRight(textwidth.Truncate(footer, w), w)
}

func (m Model) dossierPickerTitle() string {
	switch {
	case m.gamePickerOpen:
		if m.dialogGameID > 0 {
			return "Game"
		}
		return "Choose game"
	case m.formPickerOpen:
		return "Choose form"
	case m.methodPickerOpen:
		return "Choose method"
	default:
		return "Choose"
	}
}

func (m Model) formPickerLines(width int) []string {
	if width < 24 {
		width = 24
	}
	var lines []string
	if m.formPickerFilter != "" {
		lines = append(lines, "Filter: "+m.formPickerFilter+"▌")
	} else {
		lines = append(lines, style.StatusMuted("Type to filter · ↑/↓ · Enter · Esc"))
	}
	lines = append(lines, "")
	if len(m.formPickerItems) == 0 {
		lines = append(lines, style.StatusMuted("No forms match."))
		return lines
	}
	const window = 12
	start := m.formPickerCursor - window/2
	if start < 0 {
		start = 0
	}
	end := start + window
	if end > len(m.formPickerItems) {
		end = len(m.formPickerItems)
		start = max(0, end-window)
	}
	for i := start; i < end; i++ {
		o := m.formPickerItems[i]
		label := o.Form.DisplayName
		if label == "" {
			label = o.Form.Slug
		}
		prefix := "  "
		switch o.Compat {
		case domain.FormCompatUnsupported:
			label += "  · unavailable"
		case domain.FormCompatUnknown:
			label += "  · unknown"
		}
		var line string
		if i == m.formPickerCursor {
			prefix = "> "
			line = lipgloss.NewStyle().Foreground(style.PrimaryColor()).Bold(true).Render(prefix + label)
		} else if o.Compat == domain.FormCompatUnsupported {
			line = style.StatusMuted(prefix + label)
		} else {
			line = prefix + label
		}
		lines = append(lines, textwidth.Truncate(line, width))
	}
	return lines
}

func (m *Model) handleFormPickerKey(msg string, runes []rune) {
	switch msg {
	case "esc":
		m.closeFormPicker(false)
	case "enter":
		m.closeFormPicker(true)
	case "up":
		if m.formPickerCursor > 0 {
			m.formPickerCursor--
		}
	case "down":
		if m.formPickerCursor < len(m.formPickerItems)-1 {
			m.formPickerCursor++
		}
	case "backspace", "ctrl+h":
		if m.formPickerFilter != "" {
			r := []rune(m.formPickerFilter)
			m.formPickerFilter = string(r[:len(r)-1])
			m.refreshFormPickerItems()
		} else {
			m.closeFormPicker(false)
		}
	default:
		if len(runes) == 1 {
			r := runes[0]
			if r >= 32 && r != 127 {
				m.formPickerFilter += string(r)
				m.refreshFormPickerItems()
			}
		}
	}
}

func (m Model) methodPickerLines(width int) []string {
	if width < 24 {
		width = 24
	}
	var lines []string
	lines = append(lines, style.StatusMuted("↑/↓ · Enter · Esc"), "")
	if len(m.methodPickerItems) == 0 {
		lines = append(lines, style.StatusMuted("No methods available."))
		return lines
	}
	for i, meth := range m.methodPickerItems {
		label := methodHeading(meth)
		prefix := "  "
		line := prefix + label
		if i == m.methodPickerCursor {
			line = lipgloss.NewStyle().Foreground(style.PrimaryColor()).Bold(true).Render("> " + label)
		}
		lines = append(lines, textwidth.Truncate(line, width))
	}
	return lines
}

func (m *Model) handleMethodPickerKey(msg string) {
	switch msg {
	case "esc", "backspace", "ctrl+h":
		m.closeMethodPicker(false)
	case "enter":
		m.closeMethodPicker(true)
	case "up":
		if m.methodPickerCursor > 0 {
			m.methodPickerCursor--
		}
	case "down":
		if m.methodPickerCursor < len(m.methodPickerItems)-1 {
			m.methodPickerCursor++
		}
	}
}

func (m Model) overlayPickerLines(width int) []string {
	switch {
	case m.gamePickerOpen:
		return m.gamePickerLines(width)
	case m.formPickerOpen:
		return m.formPickerLines(width)
	case m.methodPickerOpen:
		return m.methodPickerLines(width)
	default:
		return nil
	}
}
