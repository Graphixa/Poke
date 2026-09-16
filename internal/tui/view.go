package tui

import (
	"strings"

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
		hintW := m.width
		if hintW < 10 {
			hintW = m.width
		}
		h := m.help
		h.SetWidth(max(1, hintW-1))
		data.FooterHint = h.ShortHelpView(m.keys.DialogShortHelp(m.dialogStacked()))
	}

	rightID, _ := m.glanceDisplayID()
	if sum, ok := m.speciesSummary(rightID); ok {
		data.HasSelected = true
		data.Selected = sum
		data.HeaderTypes = style.TypeSummary(sum.Types)
		data.HeaderGenus = ""
		if e, ok := m.browseEntries[rightID]; ok {
			data.HasBrowseStats = true
			data.Stats = e.Stats()
			if e.Genus != "" {
				data.HeaderGenus = e.Genus
			}
		}
		if r := m.resources[rightID]; r != nil && r.HasCore {
			data.HeaderTypes = style.TypeSummary(pokemonTypeNames(r.Pokemon))
			data.HeaderGenus = r.Species.Genus
			data.Stats = r.Pokemon.Stats
			data.HasBrowseStats = true
		}
	}
	m.fillBrowseAbilities(&data)
	// Art follows the list selection as soon as its frame is ready. Text hold
	// may lag until sprite commit so identity does not jump over a blank art box.
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
	if !m.showDialog {
		hintW := m.width
		if strings.TrimSpace(data.WarmHint) != "" {
			hintW = m.width - components.FooterRightWidth
		}
		if hintW < 10 {
			hintW = m.width
		}
		h := m.help
		h.SetWidth(max(1, hintW-1))
		data.FooterHint = h.ShortHelpView(m.keys.ShortHelp())
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

// glanceCommitReady is true when the selection's sprite can replace held text.
// Stats/abilities come from browse.json, so they do not gate commit.
func (m Model) glanceCommitReady() bool {
	sel, ok := m.selected()
	if !ok {
		return true
	}
	return m.glanceSpriteCommitReady(sel.ID)
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

// fillBrowseAbilities sets the glance Abilities card from browse.json (preferred),
// dossier core, or the last-good snapshot so cold scroll never blanks the card.
func (m Model) fillBrowseAbilities(data *views.BrowseData) {
	id, _ := m.glanceDisplayID()
	if abs := m.abilitiesFor(id); len(abs) > 0 {
		data.HasCore = true
		data.Abilities = abs
		if data.HeaderGenus == "" {
			if e, ok := m.browseEntries[id]; ok && e.Genus != "" {
				data.HeaderGenus = e.Genus
			} else if r := m.resources[id]; r != nil && r.HasCore {
				data.HeaderGenus = r.Species.Genus
			}
		}
		return
	}
	if m.glanceAbilitiesOK {
		data.HasCore = true
		data.Abilities = m.glanceAbilitiesSnap
	}
}

// abilitiesFor returns glance abilities for id from browse.json, else dossier core.
func (m Model) abilitiesFor(id int) []domain.AbilitySlot {
	if id < 1 {
		return nil
	}
	if e, ok := m.browseEntries[id]; ok && e.HasAbilities() {
		return e.Abilities
	}
	if r := m.resources[id]; r != nil && r.HasCore && len(r.Pokemon.Abilities) > 0 {
		return r.Pokemon.Abilities
	}
	return nil
}

// captureGlanceAbilities stores last-good abilities after Update (View is read-only).
func (m *Model) captureGlanceAbilities() {
	id, _ := m.glanceDisplayID()
	if abs := m.abilitiesFor(id); len(abs) > 0 {
		m.glanceAbilitiesSnap = abs
		m.glanceAbilitiesOK = true
	}
}

// maybeCommitGlanceHold clears the right-pane text hold once the selection sprite
// is ready. Sixel may already have painted while hold remained.
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
	m.markNativeOverlayDirty()
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

// showsWarmHintInView is false while native art is on screen in the Raw path
// (progress uses tea.Raw footer). Composite mode keeps the hint in View so
// cell flushes run; the compositor re-appends DCS after those writes.
func (m Model) showsWarmHintInView() bool {
	if m.composite {
		return true
	}
	if m.showHelp || m.showDialog || m.quitting {
		return true
	}
	return !m.nativeSpriteActive()
}

func pokemonTypeNames(p domain.Pokemon) []string {
	out := make([]string, 0, len(p.Types))
	for _, t := range p.Types {
		out = append(out, t.Name)
	}
	return out
}
