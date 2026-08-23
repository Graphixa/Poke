package tui

// Tab is the active lower-right detail panel.
type Tab int

const (
	TabStats Tab = iota
	TabEvolution
	TabLocations
	TabMoves
	TabFlavour
	TabInfo
	tabCount
)

var tabLabels = [...]string{
	TabStats:     "Stats",
	TabEvolution: "Evolution",
	TabLocations: "Locations",
	TabMoves:     "Moves",
	TabFlavour:   "Flavour",
	TabInfo:      "Info",
}

var tabShortLabels = [...]string{
	TabStats:     "Sta",
	TabEvolution: "Evo",
	TabLocations: "Loc",
	TabMoves:     "Mov",
	TabFlavour:   "Fla",
	TabInfo:      "Inf",
}

func (t Tab) Label(short bool) string {
	if t < 0 || int(t) >= int(tabCount) {
		return ""
	}
	if short {
		return tabShortLabels[t]
	}
	return tabLabels[t]
}

func (t Tab) Next() Tab {
	return Tab((int(t) + 1) % int(tabCount))
}

func (t Tab) Prev() Tab {
	return Tab((int(t) + int(tabCount) - 1) % int(tabCount))
}

func (t Tab) NeedsLazyLoad() bool {
	switch t {
	case TabEvolution, TabLocations, TabMoves:
		return true
	default:
		return false
	}
}
