package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Pokedex/internal/tui/components"
)

// patchWarmFooterCmd writes the warm status into the footer right segment via
// tea.Raw so Sixel art in the panel above is not wiped by a full View flush.
func patchWarmFooterCmd(m Model) tea.Cmd {
	if m.warmRight == "" || m.width < 1 || m.height < 1 {
		return nil
	}
	text := padWarmFooterRight(m.warmRight)
	row := m.height
	col := m.width - components.FooterRightWidth + 1
	if col < 1 {
		col = 1
	}
	seq := decSaveCursor + fmt.Sprintf("\x1b[%d;%dH%s", row, col, text) + decRestoreCursor
	return tea.Raw(seq)
}

func padWarmFooterRight(s string) string {
	w := lipgloss.Width(s)
	target := components.FooterRightWidth
	if w >= target {
		return s
	}
	return strings.Repeat(" ", target-w) + s
}

func clearWarmFooterRawCmd(m Model) tea.Cmd {
	if m.width < 1 || m.height < 1 {
		return nil
	}
	row := m.height
	col := m.width - components.FooterRightWidth + 1
	if col < 1 {
		col = 1
	}
	blank := strings.Repeat(" ", components.FooterRightWidth)
	seq := decSaveCursor + fmt.Sprintf("\x1b[%d;%dH%s", row, col, blank) + decRestoreCursor
	return tea.Raw(seq)
}

func (m Model) warmFooterPatchIfNeeded() tea.Cmd {
	if !m.nativeGraphicsOnScreen() {
		return nil
	}
	if m.effectiveWarmFooterRight() == "" {
		if m.warmRight != "" {
			return clearWarmFooterRawCmd(m)
		}
		return nil
	}
	return patchWarmFooterCmd(m)
}
