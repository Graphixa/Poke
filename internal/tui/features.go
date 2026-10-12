package tui

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/Graphixa/Pokedex/internal/domain"
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
	// Stable partition preserves search relevance in Default mode, and avoids duplicates.
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if m.favourites[a.ID] != m.favourites[b.ID] {
			return m.favourites[a.ID]
		}
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

func (m Model) hasFavouriteDivider() bool {
	return len(m.filtered) > 1 && m.favourites[m.filtered[0].ID] && !m.favourites[m.filtered[len(m.filtered)-1].ID]
}

func (m Model) listHint() string {
	if m.featureNotice != "" {
		return m.featureNotice
	}

	collection := "All"
	if m.viewOptions.FavouritesOnly {
		collection = "Favourites"
	}
	typ := ""
	if m.viewOptions.Type != 0 {
		typ = " · " + filterTypes[m.viewOptions.Type]
	}
	hint := fmt.Sprintf("%s%s · %d", collection, typ, len(m.filtered))
	if m.viewOptions.Sort != 0 {
		hint += " · " + []string{"", "A–Z", "Speed↓", "Total↓"}[m.viewOptions.Sort]
	}
	if m.comparePin != nil && m.layout().OnePane {
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
	if msg.value {
		m.favourites[msg.id] = true
	} else {
		delete(m.favourites, msg.id)
	}
	m.featureNotice = ""
	m.refilter()
	if m.quitAfterFavourite {
		return m.requestQuit()
	}
	return m, m.syncSelectionLoads()
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
