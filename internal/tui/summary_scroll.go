package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/tui/components"
)

type summaryTickMsg struct{}

const summaryTickEvery = 1800 * time.Millisecond

func tickSummary() tea.Cmd {
	return tea.Tick(summaryTickEvery, func(time.Time) tea.Msg {
		return summaryTickMsg{}
	})
}

func (m *Model) advanceSummaryScroll() {
	if !m.layout().ShowSummary || m.showHelp || m.showInfo {
		return
	}
	sel, ok := m.selected()
	if !ok {
		m.summaryVP.GotoTop()
		return
	}
	r := m.resources[sel.ID]
	if r == nil || !r.HasCore {
		return
	}
	text := latestSummary(r.Species)
	if text == "" {
		m.summaryVP.GotoTop()
		return
	}
	sumW := m.summaryColumnWidth()
	bandH := m.identityBandHeight()
	// Match renderIdentityBand: summary uses leftover rows under header+info+blank.
	bodyH := bandH - identityInfoLines
	if bodyH < 3 {
		bodyH = 3
	}
	wrapped := components.SummaryWrappedLines(text, sumW)
	m.summaryVP.SetWidth(sumW)
	m.summaryVP.SetHeight(bodyH)
	m.summaryVP.SetContentLines(wrapped)
	maxOff := len(wrapped) - bodyH
	if maxOff <= 0 {
		m.summaryVP.GotoTop()
		return
	}
	next := m.summaryVP.YOffset() + 1
	if next > maxOff {
		next = 0
	}
	m.summaryVP.SetYOffset(next)
}

// identityInfoLines is header + Type/Species/Height/Ability/Hidden/Base Exp + blank gap.
const identityInfoLines = 8

func (m Model) identityBandHeight() int {
	side := m.currentSpriteSide()
	// Keep room for stacked info + Summary under it beside the sprite.
	minText := identityInfoLines + 3
	if side < minText {
		return minText
	}
	return side
}

func (m Model) summaryColumnWidth() int {
	lay := m.layout()
	right := lay.rightWidth() - 1 // frame inner
	if right < 24 {
		right = 24
	}
	if !lay.ShowSprite {
		return right
	}
	side := m.currentSpriteSide()
	sepW := 3
	infoW := right - side - sepW
	if infoW < 16 {
		return right
	}
	return infoW
}
