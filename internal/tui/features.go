package tui

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/output"
	"github.com/Graphixa/Pokedex/internal/textwidth"
	"github.com/Graphixa/Pokedex/internal/tui/style"
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

func (m Model) featureView() string {
	w, h := m.width, m.height
	if w < 40 || h < 18 {
		return lipgloss.Place(max(1, w), max(1, h), lipgloss.Center, lipgloss.Center, "Resize to 40 × 18\nEsc closes this view")
	}
	inner := min(w-6, 70)
	var lines []string
	if m.featureError != "" {
		lines = []string{"FAVOURITES", "", lipgloss.NewStyle().Width(inner).Render(m.featureError), "", "Enter / Esc close"}
	} else if m.showFilters {
		collection := "All Pokémon"
		if m.draftOptions.FavouritesOnly {
			collection = "Favourites only"
		}
		count := len(m.filterSpecies(m.index.Search(m.queryValue()), m.draftOptions))
		lines = []string{"VIEW OPTIONS", "", "  Collection   " + collection, "  Type         " + output.TitleFromSlug(filterTypes[m.draftOptions.Type]), "  Sort         " + filterSorts[m.draftOptions.Sort], "", fmt.Sprintf("%d matches · keeps your search", count), "", "↑/↓ choose   ←/→ change", "Enter apply  Esc cancel", "Backspace reset view options"}
		lines[2+m.filterRow] = "> " + strings.TrimPrefix(lines[2+m.filterRow], "  ")
	} else {
		a := *m.comparePin
		s, _ := m.selected()
		b := m.comparisonEntry(s)
		// A pin may have been made before its core arrived; use newly available data.
		if !a.HasStats {
			a = m.comparisonEntry(a.Summary)
		}
		col := (inner - 10) / 2
		row := func(label, left, right string) string {
			return textwidth.PadRight(label, 10) + textwidth.PadRight(textwidth.Truncate(left, col), col) + textwidth.PadRight(textwidth.Truncate(right, col), col)
		}
		lines = []string{"COMPARE · BASE STATS", row("", a.Summary.DisplayName, b.Summary.DisplayName), row("", output.FormatDex(a.Summary.ID), output.FormatDex(b.Summary.ID))}
		for i := 0; i < 2; i++ {
			at, bt := "", ""
			if i < len(a.Summary.Types) {
				at = output.TitleFromSlug(a.Summary.Types[i])
			}
			if i < len(b.Summary.Types) {
				bt = output.TitleFromSlug(b.Summary.Types[i])
			}
			lines = append(lines, row("", at, bt))
		}
		av := []int{a.Stats.HP, a.Stats.Attack, a.Stats.Defense, a.Stats.SpecialAttack, a.Stats.SpecialDefense, a.Stats.Speed, a.Stats.Total()}
		bv := []int{b.Stats.HP, b.Stats.Attack, b.Stats.Defense, b.Stats.SpecialAttack, b.Stats.SpecialDefense, b.Stats.Speed, b.Stats.Total()}
		for i, label := range []string{"HP", "Attack", "Defence", "Sp. Atk", "Sp. Def", "Speed", "Total"} {
			left, right := "—", "—"
			if a.HasStats {
				left = fmt.Sprint(av[i])
			}
			if b.HasStats {
				right = fmt.Sprint(bv[i])
			}
			if a.HasStats && b.HasStats {
				if av[i] > bv[i] {
					left += " +"
				}
				if bv[i] > av[i] {
					right += " +"
				}
			}
			lines = append(lines, row(label, left, right))
		}
		lines = append(lines, "+ higher stat · — unavailable", "Esc back · Backspace unpin")
	}
	// Keep long filesystem errors inside the terminal too.
	var clipped []string
	for _, line := range lines {
		clipped = append(clipped, strings.Split(line, "\n")...)
	}
	if len(clipped) > h-2 {
		clipped = append(clipped[:h-3], "… Enter / Esc close")
	}
	for i := range clipped {
		clipped[i] = textwidth.Truncate(clipped[i], inner)
	}
	clipped[0] = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(style.CurrentPalette().Primary)).Render(clipped[0])
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(style.BorderColor()).Width(inner).Render(strings.Join(clipped, "\n"))
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}
