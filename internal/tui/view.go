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

	rightID, hold := m.glanceDisplayID()
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
			data.HasCore = true
			data.HeaderTypes = style.TypeSummary(pokemonTypeNames(r.Pokemon))
			data.HeaderGenus = r.Species.Genus
			data.Stats = r.Pokemon.Stats
			data.HasBrowseStats = true
			data.AboutHeight = formatHeight(r.Pokemon.HeightDecimeter)
			data.AboutWeight = formatWeight(r.Pokemon.WeightHectogram)
			data.AboutBaseExp = r.Pokemon.BaseExperience
		}
		if !hold && m.imageSpeciesID == rightID && !m.imagePlaceholder && m.imageANSI != "" {
			if m.imageNative {
				data.SpriteNative = true
			} else {
				data.SpriteANSI = m.imageANSI
			}
		}
	}
	if !m.showDialog && m.warmRight != "" && m.showsWarmHintInView() {
		data.WarmHint = m.warmRight
	}
	data.ImageState = int(m.imageState)
	if hold || m.nativeStickyPaintOK() {
		// Empty native reservation while held/sticky sixel stays on screen.
		data.SpriteNative = true
		if hold {
			// Stable reservation so list-only View churn does not imply loading UI.
			data.ImageState = int(ImageReady)
		}
	}
	return data
}

// glanceDisplayID is the species shown in the right pane (hold or selection).
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

// glanceCommitReady is true when the selected species can replace a held glance.
func (m Model) glanceCommitReady() bool {
	sel, ok := m.selected()
	if !ok {
		return true
	}
	if m.imageSpeciesID != sel.ID {
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

// maybeCommitGlanceHold clears the right-pane hold once the selection is ready.
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

// nativeGraphicsOnScreen is true when a native frame (current, hold, or sticky)
// is expected on the art box — footer progress must use tea.Raw, not View.
func (m Model) nativeGraphicsOnScreen() bool {
	if m.glanceHoldID > 0 {
		return true
	}
	return m.nativeSpriteActive() || m.nativeStickyPaintOK()
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
