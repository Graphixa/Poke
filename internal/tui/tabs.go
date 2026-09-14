package tui

// BrowseView selects the browse right-pane layout (Glance vs large Art).
type BrowseView int

const (
	BrowseGlance BrowseView = iota
	BrowseArt
	browseViewCount
)

var browseViewLabels = [...]string{
	BrowseGlance: "Glance",
	BrowseArt:    "Art",
}

func (v BrowseView) Label() string {
	if v < 0 || int(v) >= int(browseViewCount) {
		return ""
	}
	return browseViewLabels[v]
}

func (v BrowseView) Next() BrowseView {
	return BrowseView((int(v) + 1) % int(browseViewCount))
}

func (v BrowseView) Prev() BrowseView {
	return BrowseView((int(v) + int(browseViewCount) - 1) % int(browseViewCount))
}

// DialogSection is a detail section inside the Enter overlay (full dossier).
type DialogSection int

const (
	DialogSummary DialogSection = iota
	DialogStats
	DialogEvolution
	DialogMoves
	DialogLocations
	DialogAbilities
	DialogFlavour
	DialogBreeding
	DialogTraining
	DialogForms
	DialogMeta
	dialogSectionCount
)

var dialogSectionLabels = [...]string{
	DialogSummary:   "Summary",
	DialogStats:     "Stats",
	DialogEvolution: "Evolution",
	DialogMoves:     "Moves",
	DialogLocations: "Locations",
	DialogAbilities: "Abilities",
	DialogFlavour:   "Flavour",
	DialogBreeding:  "Breeding",
	DialogTraining:  "Training",
	DialogForms:     "Forms",
	DialogMeta:      "Meta",
}

func (s DialogSection) Label() string {
	if s < 0 || int(s) >= int(dialogSectionCount) {
		return ""
	}
	return dialogSectionLabels[s]
}

func (s DialogSection) Next() DialogSection {
	return DialogSection((int(s) + 1) % int(dialogSectionCount))
}

func (s DialogSection) Prev() DialogSection {
	return DialogSection((int(s) + int(dialogSectionCount) - 1) % int(dialogSectionCount))
}

// NeedsLazyLoad is true when the section historically needed an extra fetch.
// With dossiers, data arrives with LoadDossier; this stays for compatibility.
func (s DialogSection) NeedsLazyLoad() bool {
	return false
}

// AllDialogSectionLabels returns labels in cycle order for the dialog switcher.
func AllDialogSectionLabels() []string {
	out := make([]string, int(dialogSectionCount))
	for i := DialogSection(0); i < dialogSectionCount; i++ {
		out[i] = i.Label()
	}
	return out
}

// AllBrowseViewLabels returns labels in cycle order for the browse switcher.
func AllBrowseViewLabels() []string {
	out := make([]string, int(browseViewCount))
	for i := BrowseView(0); i < browseViewCount; i++ {
		out[i] = i.Label()
	}
	return out
}
