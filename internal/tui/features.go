package tui

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/textwidth"
)

type viewOptions struct {
	FavouritesOnly bool
	Type           int
	Sort           int
}

var filterTypes = []string{"All", "normal", "fire", "water", "electric", "grass", "ice", "fighting", "poison", "ground", "flying", "psychic", "bug", "rock", "ghost", "dragon", "dark", "steel", "fairy"}
var filterSorts = []string{"Default", "Name A–Z", "Speed (high first)", "Total (high first)"}

type compareEntry struct {
	Summary  domain.SpeciesSummary
	Stats    domain.Stats
	HasStats bool
}

type favouriteSavedMsg struct {
	id    int
	value bool
	err   error
}

func (m Model) overlayOpen() bool {
	return m.showHelp || m.showDialog || m.showFilters || m.showCompare || m.featureError != ""
}

func (m Model) filterSpecies(input []domain.SpeciesSummary, opts viewOptions) []domain.SpeciesSummary {
	out := make([]domain.SpeciesSummary, 0, len(input))
	for _, s := range input {
		if opts.FavouritesOnly && !m.favourites[s.ID] {
			continue
		}
		if opts.Type != 0 && !slices.Contains(s.Types, filterTypes[opts.Type]) {
			continue
		}
		out = append(out, s)
	}
	// Sort only — favourites stay in place. Quick-link duplicates are prepended in applyFavouritePins.
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch opts.Sort {
		case 1:
			an, bn := strings.ToLower(a.DisplayName), strings.ToLower(b.DisplayName)
			if an != bn {
				return an < bn
			}
		case 2, 3:
			ae, aok := m.browseEntries[a.ID]
			be, bok := m.browseEntries[b.ID]
			if aok != bok {
				return aok
			}
			av, bv := ae.Speed, be.Speed
			if opts.Sort == 3 {
				av, bv = ae.Stats().Total(), be.Stats().Total()
			}
			if av != bv {
				return av > bv
			}
		default:
			return false
		}
		return a.ID < b.ID
	})
	return out
}

// applyFavouritePins prepends quick-link copies of favourites (All collection only).
// Favourites-only stays unique. Sets m.favPinCount.
func (m *Model) applyFavouritePins(main []domain.SpeciesSummary) []domain.SpeciesSummary {
	if m.viewOptions.FavouritesOnly {
		m.favPinCount = 0
		return main
	}
	pins := make([]domain.SpeciesSummary, 0, len(m.favourites))
	for _, s := range main {
		if m.favourites[s.ID] {
			pins = append(pins, s)
		}
	}
	m.favPinCount = len(pins)
	if len(pins) == 0 {
		return main
	}
	out := make([]domain.SpeciesSummary, 0, len(pins)+len(main))
	out = append(out, pins...)
	return append(out, main...)
}

func (m Model) hasFavouriteDivider() bool {
	return m.favPinCount > 0 && len(m.filtered) > m.favPinCount
}

func (m Model) hasFavouriteLabel() bool {
	return m.favPinCount > 0
}

func (m Model) pinnedFavouriteCount() int {
	return m.favPinCount
}

func (m Model) matchedCount() int {
	n := len(m.filtered) - m.favPinCount
	if n < 0 {
		return 0
	}
	return n
}

// indexOfSpecies finds id in the pin block (preferPin) or the main list after pins.
func (m Model) indexOfSpecies(id int, preferPin bool) int {
	if preferPin {
		for i := 0; i < m.favPinCount && i < len(m.filtered); i++ {
			if m.filtered[i].ID == id {
				return i
			}
		}
		return -1
	}
	start := m.favPinCount
	if start > len(m.filtered) {
		start = 0
	}
	for i := start; i < len(m.filtered); i++ {
		if m.filtered[i].ID == id {
			return i
		}
	}
	return -1
}

func (m Model) collectionSize() int {
	if m.viewOptions.FavouritesOnly {
		return len(m.favourites)
	}
	if m.index != nil {
		return len(m.index.All())
	}
	return len(m.filtered)
}

// listBorderTitle is the left-pane top-edge title: base + [ matched/total ], narrowed as needed.
func (m Model) listBorderTitle(leftW int) string {
	base := "Pokedex"
	if m.comparePin != nil {
		base = "Pokedex · F3 vs " + m.comparePin.Summary.DisplayName
	}
	return fitListBorderTitle(base, m.matchedCount(), m.collectionSize(), leftW)
}

func fitListBorderTitle(base string, matched, total, leftW int) string {
	maxW := leftW - 4 // ╭─␠title␠… inside left segment
	if maxW < 3 {
		maxW = 3
	}
	spaced := fmt.Sprintf("[ %d/%d ]", matched, total)
	tight := fmt.Sprintf("[%d/%d]", matched, total)
	for _, c := range []string{base + " " + spaced, base + " " + tight, tight} {
		if textwidth.Width(c) <= maxW {
			return c
		}
	}
	return textwidth.Truncate(tight, maxW)
}

func (m Model) listHint() string {
	if m.featureNotice != "" {
		return m.featureNotice
	}
	// Two-pane: match count lives in the border title. One-pane has no title edge.
	if !m.layout().OnePane {
		return ""
	}
	hint := fmt.Sprintf("%d/%d", m.matchedCount(), m.collectionSize())
	if m.comparePin != nil {
		hint = "F3 vs " + m.comparePin.Summary.DisplayName + " · " + hint
	}
	return hint
}

func (m Model) requestQuit() (tea.Model, tea.Cmd) {
	if m.favouriteSaving {
		m.quitAfterFavourite = true
		m.featureNotice = "Saving before exit…"
		return m, nil
	}
	m.quitting = true
	m.glanceHoldID = 0
	return m, tea.Batch(m.wipeNativeGraphics(), tea.Quit)
}

func (m Model) toggleFavourite() (tea.Model, tea.Cmd) {
	if m.favouriteLoadError != "" {
		m.featureError = "Favourites could not be loaded. The file has been preserved.\n\n" + m.favouriteLoadError
		return m.coverNativeOverlay()
	}
	if m.favouriteSaving {
		return m, nil
	}
	sel, ok := m.selected()
	if !ok {
		return m, nil
	}
	value := !m.favourites[sel.ID]
	ids := make([]int, 0, len(m.favourites)+1)
	for id, on := range m.favourites {
		if on && id != sel.ID {
			ids = append(ids, id)
		}
	}
	if value {
		ids = append(ids, sel.ID)
	}
	slices.Sort(ids)
	save := m.saveFavourites
	m.favouriteSaving = true
	m.featureNotice = "Saving favourite…"
	return m, func() tea.Msg {
		var err error
		if save != nil {
			err = save(ids)
		}
		return favouriteSavedMsg{id: sel.ID, value: value, err: err}
	}
}

func (m Model) finishFavouriteSave(msg favouriteSavedMsg) (tea.Model, tea.Cmd) {
	m.favouriteSaving = false
	if msg.err != nil {
		m.quitAfterFavourite = false
		m.featureNotice = "Save failed; F4 retries"
		m.featureError = "Favourite was not changed. Close this message, then press F4 to retry.\n\n" + msg.err.Error()
		return m.coverNativeOverlay()
	}
	// Unstar from quick-link block: advance to the next pin, or first dex row if none left.
	advancePin := false
	nextPinID := 0
	if !msg.value && m.favPinCount > 0 && m.cursor < m.favPinCount {
		advancePin = true
		if m.cursor+1 < m.favPinCount {
			nextPinID = m.filtered[m.cursor+1].ID
		}
	}
	if msg.value {
		m.favourites[msg.id] = true
	} else {
		delete(m.favourites, msg.id)
	}
	m.featureNotice = ""
	m.refilter()
	if advancePin {
		m.cursorAfterUnstarPin(nextPinID)
	}
	if m.quitAfterFavourite {
		return m.requestQuit()
	}
	return m, m.syncSelectionLoads()
}

// cursorAfterUnstarPin lands on nextPinID in the quick-link block, or the first main-list row.
func (m *Model) cursorAfterUnstarPin(nextPinID int) {
	if len(m.filtered) == 0 {
		m.cursor = 0
		m.listOff = 0
		return
	}
	if nextPinID != 0 {
		if idx := m.indexOfSpecies(nextPinID, true); idx >= 0 {
			m.cursor = idx
			m.ensureListVisible()
			return
		}
	}
	// Last quick-link removed (or next pin missing) → first dex row.
	m.cursor = m.favPinCount
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	m.ensureListVisible()
}

func (m Model) comparisonEntry(s domain.SpeciesSummary) compareEntry {
	e := compareEntry{Summary: s}
	if b, ok := m.browseEntries[s.ID]; ok {
		e.Stats = b.Stats()
		e.HasStats = true
	} else if r := m.resources[s.ID]; r != nil && r.HasCore {
		e.Stats = r.Pokemon.Stats
		e.HasStats = true
	}
	return e
}

func (m Model) pinOrCompare() (tea.Model, tea.Cmd) {
	s, ok := m.selected()
	if !ok {
		return m, nil
	}
	m.featureNotice = ""
	if m.comparePin == nil {
		e := m.comparisonEntry(s)
		m.comparePin = &e
		return m, nil
	}
	if m.comparePin.Summary.ID == s.ID {
		m.comparePin = nil
		return m, nil
	}
	m.showCompare = true
	m.glanceHoldID = 0
	return m.coverNativeOverlay()
}

func (m *Model) closeFeature() {
	m.showFilters = false
	m.showCompare = false
	m.featureError = ""
	m.invalidateNativePaintGeometry()
	m.markNativeOverlayDirty()
}

func (m Model) handleFeatureKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if m.featureError != "" {
		if k == "esc" || k == "enter" {
			m.closeFeature()
		}
		return m, nil
	}
	if k == "esc" {
		m.closeFeature()
		return m, nil
	}
	if m.showCompare {
		if k == "up" || k == "down" {
			delta := 1
			if k == "up" {
				delta = -1
			}
			for i := m.cursor + delta; i >= 0 && i < len(m.filtered); i += delta {
				if m.comparePin != nil && m.filtered[i].ID == m.comparePin.Summary.ID {
					continue
				}
				m.cursor = i
				m.ensureListVisible()
				return m, m.syncSelectionLoads()
			}
		}
		if k == "backspace" {
			m.comparePin = nil
			m.closeFeature()
		}
		return m, nil
	}
	switch k {
	case "up", "shift+tab":
		m.filterRow = (m.filterRow + 2) % 3
	case "down", "tab":
		m.filterRow = (m.filterRow + 1) % 3
	case "left", "right", "space":
		delta := 1
		if k == "left" {
			delta = -1
		}
		switch m.filterRow {
		case 0:
			m.draftOptions.FavouritesOnly = !m.draftOptions.FavouritesOnly
		case 1:
			m.draftOptions.Type = (m.draftOptions.Type + delta + len(filterTypes)) % len(filterTypes)
		case 2:
			m.draftOptions.Sort = (m.draftOptions.Sort + delta + len(filterSorts)) % len(filterSorts)
		}
	case "backspace":
		m.draftOptions = viewOptions{}
	case "enter":
		m.viewOptions = m.draftOptions
		m.featureNotice = ""
		m.closeFeature()
		m.refilter()
		return m, m.syncSelectionLoads()
	}
	return m, nil
}
