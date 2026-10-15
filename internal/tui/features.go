package tui

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/textwidth"
	"github.com/Graphixa/Pokedex/internal/tui/views"
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
// Skipped while searching (★ only on matching rows) and in Favourites-only (unique rows).
func (m *Model) applyFavouritePins(main []domain.SpeciesSummary) []domain.SpeciesSummary {
	if m.viewOptions.FavouritesOnly || m.queryValue() != "" {
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
	return fitListBorderTitle("Pokedex", m.matchedCount(), m.collectionSize(), leftW)
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
	return fmt.Sprintf("%d/%d", m.matchedCount(), m.collectionSize())
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
	m.compareA = m.comparisonEntry(m.comparePin.Summary)
	if !m.compareA.HasStats {
		m.compareA = *m.comparePin
	}
	m.compareB = m.comparisonEntry(s)
	m.compareFocus = 1
	*m.comparePin = m.compareA
	m.showCompare = true
	m.glanceHoldID = 0
	m.resetCompareArt()
	m.fillCompareArtFromCache(0)
	m.fillCompareArtFromCache(1)
	covered, wipe := m.coverNativeOverlay()
	if covered.compareArtANative || covered.compareArtBNative {
		covered.markNativeOverlayDirty()
	}
	return covered, tea.Batch(wipe, m.scheduleCompareArt())
}

func (m Model) compareFocused() compareEntry {
	if m.compareFocus == 0 {
		return m.compareA
	}
	return m.compareB
}

func (m *Model) swapCompareSides() {
	m.compareA, m.compareB = m.compareB, m.compareA
	m.compareArtA, m.compareArtB = m.compareArtB, m.compareArtA
	m.compareArtANative, m.compareArtBNative = m.compareArtBNative, m.compareArtANative
	m.compareArtAID, m.compareArtBID = m.compareArtBID, m.compareArtAID
	if m.comparePin != nil {
		*m.comparePin = m.compareA
	}
	// Focus stays on the same slot (left/right).
	m.markNativeOverlayDirty()
}

func (m *Model) syncCompareCursorToFocus() {
	id := m.compareFocused().Summary.ID
	if id < 1 {
		return
	}
	for i, s := range m.filtered {
		if s.ID == id {
			m.cursor = i
			m.ensureListVisible()
			return
		}
	}
}

func (m *Model) moveCompareFocused(delta int) bool {
	cur := m.compareFocused()
	otherID := m.compareA.Summary.ID
	if m.compareFocus == 0 {
		otherID = m.compareB.Summary.ID
	}
	start := -1
	for i, s := range m.filtered {
		if s.ID == cur.Summary.ID {
			start = i
			break
		}
	}
	if start < 0 {
		start = m.cursor
	}
	for i := start + delta; i >= 0 && i < len(m.filtered); i += delta {
		if m.filtered[i].ID == otherID {
			continue
		}
		e := m.comparisonEntry(m.filtered[i])
		side := m.compareFocus
		if side == 0 {
			m.compareA = e
			m.compareArtA, m.compareArtANative, m.compareArtAID = "", false, 0
			if m.comparePin != nil {
				*m.comparePin = e
			}
		} else {
			m.compareB = e
			m.compareArtB, m.compareArtBNative, m.compareArtBID = "", false, 0
		}
		m.cursor = i
		m.ensureListVisible()
		// Drop the old sixel immediately; async load will repaint this side.
		m.wipeCompareSideRect(side)
		m.markNativeOverlayDirty()
		return true
	}
	return false
}

func (m *Model) resetCompareArt() {
	m.compareArtA, m.compareArtB = "", ""
	m.compareArtANative, m.compareArtBNative = false, false
	m.compareArtAID, m.compareArtBID = 0, 0
	m.compareArtCells = views.GlanceBucket
}

func (m Model) compareGenus(id int) string {
	if e, ok := m.browseEntries[id]; ok && e.Genus != "" {
		return e.Genus
	}
	if r := m.resources[id]; r != nil && r.HasCore && r.Species.Genus != "" {
		return r.Species.Genus
	}
	return ""
}

func (m Model) pokemonForCompare(id int) (domain.Pokemon, bool) {
	if id < 1 {
		return domain.Pokemon{}, false
	}
	if r := m.resources[id]; r != nil && r.HasCore {
		return r.Pokemon, true
	}
	if m.pokemonSrc != nil {
		p, err := m.pokemonSrc.PokemonForSprite(m.ctx, id)
		if err == nil {
			return p, true
		}
	}
	if e, ok := m.browseEntries[id]; ok {
		return domain.Pokemon{ID: id, Slug: e.Slug, DisplayName: e.Name}, true
	}
	return domain.Pokemon{ID: id}, true
}

// compareNativeRenderer prefers Kitty/iTerm/Sixel frames painted via CUP overlay.
type compareNativeRenderer interface {
	NativeWarmRender(ctx context.Context, pokemon domain.Pokemon, cells int) sprite.RenderResult
}

// compareCellRenderer is the chafa/halfblock fallback when native is unavailable.
type compareCellRenderer interface {
	WarmRender(ctx context.Context, pokemon domain.Pokemon, cells int) sprite.RenderResult
}

func (m *Model) fillCompareArtFromCache(side int) {
	e := m.compareA
	if side == 1 {
		e = m.compareB
	}
	cells := m.compareArtCells
	if cells < sprite.SmallestFrameBucket() {
		cells = views.GlanceBucket
		m.compareArtCells = cells
	}
	p, ok := m.pokemonForCompare(e.Summary.ID)
	if !ok || m.sprites == nil {
		return
	}
	res, hit := m.sprites.LookupCached(m.ctx, p, cells)
	if !hit || res.UsePlaceholder || res.ANSI == "" {
		return
	}
	if side == 0 {
		m.compareArtA, m.compareArtANative, m.compareArtAID = res.ANSI, res.Native, e.Summary.ID
	} else {
		m.compareArtB, m.compareArtBNative, m.compareArtBID = res.ANSI, res.Native, e.Summary.ID
	}
}

func (m Model) scheduleCompareArt() tea.Cmd {
	return tea.Batch(m.loadCompareArtSide(0), m.loadCompareArtSide(1))
}

func (m Model) loadCompareArtSide(side int) tea.Cmd {
	e := m.compareA
	if side == 1 {
		e = m.compareB
	}
	id := e.Summary.ID
	cells := m.compareArtCells
	if cells < sprite.SmallestFrameBucket() {
		cells = views.GlanceBucket
	}
	if id < 1 || m.sprites == nil {
		return nil
	}
	if side == 0 && m.compareArtAID == id && m.compareArtA != "" {
		return nil
	}
	if side == 1 && m.compareArtBID == id && m.compareArtB != "" {
		return nil
	}
	sprites := m.sprites
	ctx := m.ctx
	p, ok := m.pokemonForCompare(id)
	if !ok {
		return nil
	}
	return func() tea.Msg {
		// Prefer native — dual columns reserve cells; CUP paints sixel into boxes.
		if nr, ok := sprites.(compareNativeRenderer); ok {
			res := nr.NativeWarmRender(ctx, p, cells)
			if !res.UsePlaceholder && res.ANSI != "" && res.Native {
				return compareArtMsg{side: side, speciesID: id, cells: cells, ansi: res.ANSI, native: true}
			}
		}
		if res, hit := sprites.LookupCached(ctx, p, cells); hit && !res.UsePlaceholder && res.ANSI != "" {
			return compareArtMsg{side: side, speciesID: id, cells: cells, ansi: res.ANSI, native: res.Native}
		}
		if wr, ok := sprites.(compareCellRenderer); ok {
			res := wr.WarmRender(ctx, p, cells)
			if !res.UsePlaceholder && res.ANSI != "" && !res.Native {
				return compareArtMsg{side: side, speciesID: id, cells: cells, ansi: res.ANSI}
			}
		}
		res := sprites.RenderCold(ctx, p, cells)
		if res.UsePlaceholder || res.ANSI == "" {
			return compareArtMsg{side: side, speciesID: id, cells: cells}
		}
		return compareArtMsg{side: side, speciesID: id, cells: cells, ansi: res.ANSI, native: res.Native}
	}
}

type compareArtMsg struct {
	side      int
	speciesID int
	cells     int
	ansi      string
	native    bool
}

func (m Model) finishCompareArt(msg compareArtMsg) (tea.Model, tea.Cmd) {
	if !m.showCompare || msg.speciesID < 1 {
		return m, nil
	}
	want := m.compareA.Summary.ID
	if msg.side == 1 {
		want = m.compareB.Summary.ID
	}
	if msg.speciesID != want {
		return m, nil
	}
	if msg.side == 0 {
		m.compareArtA, m.compareArtANative, m.compareArtAID = msg.ansi, msg.native, msg.speciesID
	} else {
		m.compareArtB, m.compareArtBNative, m.compareArtBID = msg.ansi, msg.native, msg.speciesID
	}
	m.compareArtCells = msg.cells
	if msg.native && msg.ansi != "" {
		m.markNativeOverlayDirty()
	}
	return m, nil
}

func (m *Model) closeFeature() {
	if m.showCompare && m.compareNativePainted {
		// Erase dual sixels before browse takes over — browse wipe won't hit these CUPs.
		m.forceCompareNativeWipe()
	}
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
		switch k {
		case "tab", "shift+tab":
			m.compareFocus = 1 - m.compareFocus
			m.syncCompareCursorToFocus()
		case "up":
			if m.moveCompareFocused(-1) {
				return m, tea.Batch(m.syncSelectionLoads(), m.loadCompareArtSide(m.compareFocus))
			}
		case "down":
			if m.moveCompareFocused(1) {
				return m, tea.Batch(m.syncSelectionLoads(), m.loadCompareArtSide(m.compareFocus))
			}
		case " ", "space":
			m.swapCompareSides()
			m.syncCompareCursorToFocus()
		}
		return m, nil
	}
	// Filter wizard: Collection → Type → Sort.
	switch k {
	case "up":
		m.nudgeFilterChoice(-1)
	case "down":
		m.nudgeFilterChoice(1)
	case "left", "shift+tab":
		if m.filterStep > 0 {
			m.filterStep--
		}
	case "right", "tab":
		if m.filterStep < 2 {
			m.filterStep++
		}
	case "backspace":
		m.draftOptions = viewOptions{}
	case " ", "space":
		return m.applyFilterDraft()
	case "enter":
		if m.filterStep < 2 {
			m.filterStep++
			return m, nil
		}
		return m.applyFilterDraft()
	}
	return m, nil
}

func (m *Model) nudgeFilterChoice(delta int) {
	switch m.filterStep {
	case 0:
		m.draftOptions.FavouritesOnly = !m.draftOptions.FavouritesOnly
	case 1:
		n := len(filterTypes)
		m.draftOptions.Type = (m.draftOptions.Type + delta + n) % n
	case 2:
		n := len(filterSorts)
		m.draftOptions.Sort = (m.draftOptions.Sort + delta + n) % n
	}
}

func (m Model) applyFilterDraft() (tea.Model, tea.Cmd) {
	m.viewOptions = m.draftOptions
	m.featureNotice = ""
	m.closeFeature()
	m.refilter()
	return m, m.syncSelectionLoads()
}
