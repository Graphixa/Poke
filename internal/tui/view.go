package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/tui/components"
	"github.com/Graphixa/Pokedex/internal/tui/style"
	"github.com/Graphixa/Pokedex/internal/tui/views"
)

func init() {
	views.DefaultFrame = views.FrameRenderer{
		Calculate: func(termW, termH, headerH, footerH, margin, sep int) (int, int, int) {
			lay := CalculatePanelLayout(termW, termH, headerH, footerH, margin, sep)
			return lay.LeftWidth, lay.RightWidth, lay.PanelHeight
		},
		Combine: RenderCombinedFrame,
		Fill:    FillTerminalArea,
	}
}

func (m Model) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}
	content := views.Browse(m.browseData())
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m Model) browseData() views.BrowseData {
	lay := m.layout()

	rows := make([]components.ListRow, len(m.filtered))
	for i, s := range m.filtered {
		rows[i] = components.ListRow{Summary: s, Selected: i == m.cursor}
	}

	query := m.queryValue()
	search := m.search
	searchW := lay.leftWidth() - 4
	if searchW < 10 {
		searchW = 10
	}
	search.SetWidth(searchW)

	bodySide := lay.SpriteSize
	if bodySide < sprite.SmallestFrameBucket() {
		bodySide = sprite.SmallestFrameBucket()
	}

	data := views.BrowseData{
		Width:         m.width,
		Height:        m.height,
		Query:         query,
		SearchView:    search.View(),
		Rows:          rows,
		ListOff:       m.listOff,
		ListRows:      m.listVisibleRows(),
		ListTypes:     m.listTypes,
		ListNameCol:   nameColumnWidth(m.listColumn().MaxNameWidth),
		ShowSprite:    lay.ShowSprite,
		VerboseFooter: lay.VerboseFooter,
		OnePane:       lay.OnePane,
		TooSmall:      lay.TooSmall,
		ShowListPane:  query != "",
		BrowseView:    int(m.browseView),
		ViewLabels:    AllBrowseViewLabels(),
		BodySize:      bodySide,
		LeftWidth:     lay.leftWidth(),
		RightWidth:    lay.rightWidth(),
		ViewLines:     m.browseViewLines(),
		ShowHelp:      m.showHelp,
		HelpBody:      m.help.FullHelpView(m.keys.FullHelp()),
		ShowDialog:    m.showDialog,
		DialogSection: int(m.dialogSection),
		DialogLabels:  AllDialogSectionLabels(),
		DialogLines:   m.dialogContentLines(),
		DialogOff:     m.dialogVP.YOffset(),
	}
	if m.showDialog {
		data.FooterHint = "Tab section • ↑↓ scroll • Esc close"
	}

	rightID, _ := m.glanceDisplayID()
	if sum, ok := m.speciesSummary(rightID); ok {
		data.HasSelected = true
		data.Selected = sum
		data.HeaderTypes = style.TypeSummary(sum.Types)
		data.HeaderGenus = ""
		if st, ok := m.browseStats[rightID]; ok {
			data.HasBrowseStats = true
			data.Stats = st
		}
		if r := m.resources[rightID]; r != nil && r.HasCore {
			data.HeaderTypes = style.TypeSummary(pokemonTypeNames(r.Pokemon))
			data.HeaderGenus = r.Species.Genus
			data.Stats = r.Pokemon.Stats
			data.HasBrowseStats = true
		}
	}
	m.fillBrowseAbout(&data)
	// Art follows the list selection as soon as its frame is ready. Text hold
	// (name/stats/About) may lag until glanceCommitReady so About does not blank.
	if sel, ok := m.selected(); ok {
		if m.imageSpeciesID == sel.ID && !m.imagePlaceholder && m.imageANSI != "" {
			if m.imageNative {
				// Native reservation; Raw paint via scheduleNativeOverlay.
				data.SpriteNative = true
			} else {
				// Cell art (chafa/halfblock) is buffer truth; no Raw needed.
				data.SpriteANSI = m.imageANSI
			}
		}
	}
	if !m.showDialog && m.warmRight != "" && m.showsWarmHintInView() {
		data.WarmHint = m.warmRight
	}
	data.ImageState = int(m.imageState)
	return data
}

// glanceDisplayID is the species shown in the right-pane text (hold or selection).
func (m Model) glanceDisplayID() (id int, holding bool) {
	sel, ok := m.selected()
	if m.glanceHoldID > 0 && ok && m.glanceHoldID != sel.ID && !m.glanceCommitReady() {
		return m.glanceHoldID, true
	}
	if ok {
		return sel.ID, false
	}
	if m.glanceHoldID > 0 {
		return m.glanceHoldID, true
	}
	return 0, false
}

// glanceCommitReady is true when identity+stats+About can replace a held glance.
// Sprite readiness alone is not enough when the held species had About and the
// selection still lacks core — keep the previous About until the new one can
// fill it (or core has settled with an error).
func (m Model) glanceCommitReady() bool {
	sel, ok := m.selected()
	if !ok {
		return true
	}
	if !m.glanceSpriteCommitReady(sel.ID) {
		return false
	}
	return m.glanceAboutCommitReady(sel.ID)
}

func (m Model) glanceSpriteCommitReady(selID int) bool {
	if m.imageSpeciesID != selID {
		return false
	}
	switch m.imageState {
	case ImageReady:
		return !m.imagePlaceholder && m.imageANSI != ""
	case ImageUnavailable:
		return true
	default:
		return false
	}
}

// glanceAboutCommitReady is true when releasing text hold will not blank About.
// Uses the last-good About snapshot (not resources[glanceHoldID].HasCore) so
// cold hold-ID churn and LRU eviction cannot drop the preserved values.
func (m Model) glanceAboutCommitReady(selID int) bool {
	if m.glanceAboutFromID < 1 {
		return true
	}
	r := m.resources[selID]
	if r != nil && r.HasCore {
		return true
	}
	if r != nil && r.CoreErr != nil {
		// Settled failure: allow empty About rather than holding forever.
		return true
	}
	return false
}

// refreshGlanceAboutSnapshot stores About/genus from a species that has core.
func (m *Model) refreshGlanceAboutSnapshot(id int) {
	r := m.resources[id]
	if r == nil || !r.HasCore {
		return
	}
	m.glanceAboutFromID = id
	m.glanceAboutHeight = formatHeight(r.Pokemon.HeightDecimeter)
	m.glanceAboutWeight = formatWeight(r.Pokemon.WeightHectogram)
	m.glanceAboutBaseExp = r.Pokemon.BaseExperience
	m.glanceHeaderGenus = r.Species.Genus
}

// fillBrowseAbout sets HasCore/About from the selected species when it has core,
// otherwise from the last-good snapshot while cold-scrolling. CoreErr gives up
// (no snapshot fill) so a settled failure does not sticky-show a prior mon.
func (m Model) fillBrowseAbout(data *views.BrowseData) {
	sel, ok := m.selected()
	if !ok {
		return
	}
	r := m.resources[sel.ID]
	if r != nil && r.HasCore {
		data.HasCore = true
		data.AboutHeight = formatHeight(r.Pokemon.HeightDecimeter)
		data.AboutWeight = formatWeight(r.Pokemon.WeightHectogram)
		data.AboutBaseExp = r.Pokemon.BaseExperience
		if data.HeaderGenus == "" {
			data.HeaderGenus = r.Species.Genus
		}
		return
	}
	if r != nil && r.CoreErr != nil {
		return
	}
	if m.glanceAboutFromID < 1 {
		return
	}
	data.HasCore = true
	data.AboutHeight = m.glanceAboutHeight
	data.AboutWeight = m.glanceAboutWeight
	data.AboutBaseExp = m.glanceAboutBaseExp
	if data.HeaderGenus == "" {
		data.HeaderGenus = m.glanceHeaderGenus
	}
}

// maybeCommitGlanceHold clears the right-pane text hold once the selection is
// ready for identity+stats+About. Sixel may already have painted for the
// selection while hold remained (art is not gated on hold).
func (m *Model) maybeCommitGlanceHold() {
	if m.glanceHoldID == 0 {
		return
	}
	if m.showHelp || m.showDialog || m.quitting {
		m.glanceHoldID = 0
		return
	}
	if !m.glanceCommitReady() {
		return
	}
	m.glanceHoldID = 0
	m.markNativeOverlayDirtyFollowUp()
}

func (m Model) speciesSummary(id int) (domain.SpeciesSummary, bool) {
	if id < 1 {
		return domain.SpeciesSummary{}, false
	}
	for _, s := range m.filtered {
		if s.ID == id {
			return s, true
		}
	}
	if m.index != nil {
		for _, s := range m.index.All() {
			if s.ID == id {
				return s, true
			}
		}
	}
	return domain.SpeciesSummary{ID: id}, true
}

// nativeSpriteActive reports whether a native frame is shown for the selection
// without building browse view data (avoids recursion with showsWarmHintInView).
func (m Model) nativeSpriteActive() bool {
	if m.showHelp || m.showDialog || m.quitting {
		return false
	}
	sel, ok := m.selected()
	if !ok {
		return false
	}
	if m.imageSpeciesID != sel.ID || m.imagePlaceholder || !m.imageNative || m.imageANSI == "" {
		return false
	}
	lay := m.layout()
	return lay.ShowSprite && lay.SpriteSize >= sprite.SmallestFrameBucket()
}

// nativeGraphicsOnScreen is true when a native frame is expected on the art box —
// footer progress must use tea.Raw, not View.
func (m Model) nativeGraphicsOnScreen() bool {
	return m.nativeSpriteActive()
}

// showsWarmHintInView is false while native art is on screen (progress uses
// tea.Raw footer instead). Must not call browseData or nativeOverlayReady.
func (m Model) showsWarmHintInView() bool {
	if m.showHelp || m.showDialog || m.quitting {
		return true
	}
	if m.nativeGraphicsOnScreen() {
		return false
	}
	return true
}

func pokemonTypeNames(p domain.Pokemon) []string {
	out := make([]string, 0, len(p.Types))
	for _, t := range p.Types {
		out = append(out, t.Name)
	}
	return out
}
