package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

type summaryTickMsg struct{}

const summaryTickEvery = 1800 * time.Millisecond

func tickSummary() tea.Cmd {
	return tea.Tick(summaryTickEvery, func(time.Time) tea.Msg {
		return summaryTickMsg{}
	})
}

// advanceSummaryScroll is a no-op under the sprite-first layout (browse Summary
// is a static glance body; long flavour lives in the detail dialog).
func (m *Model) advanceSummaryScroll() {}
