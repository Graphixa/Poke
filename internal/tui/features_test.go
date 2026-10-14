package tui

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/search"
	"github.com/Graphixa/Pokedex/internal/textwidth"
	"github.com/Graphixa/Pokedex/internal/tui/components"
	"github.com/charmbracelet/x/ansi"
)

func featureModel() Model {
	entries := []domain.BrowseEntry{
		{ID: 4, Slug: "charmander", Name: "Charmander", Types: []string{"fire"}, HP: 39, Attack: 52, Defence: 43, SpAttack: 60, SpDefence: 50, Speed: 65},
		{ID: 6, Slug: "charizard", Name: "Charizard", Types: []string{"fire", "flying"}, HP: 78, Attack: 84, Defence: 78, SpAttack: 109, SpDefence: 85, Speed: 100},
		{ID: 25, Slug: "pikachu", Name: "Pikachu", Types: []string{"electric"}, HP: 35, Attack: 55, Defence: 40, SpAttack: 50, SpDefence: 50, Speed: 90},
		{ID: 133, Slug: "eevee", Name: "Eevee", Types: []string{"normal"}, HP: 55, Attack: 55, Defence: 50, SpAttack: 45, SpDefence: 65, Speed: 55},
	}
	var summaries []domain.SpeciesSummary
	byID := map[int]domain.BrowseEntry{}
	for _, e := range entries {
		summaries = append(summaries, e.Summary())
		byID[e.ID] = e
	}
	m := NewModel(Options{Index: search.Build(summaries), BrowseEntries: byID, DisableSprite: true})
	m.width, m.height = 100, 30
	return m
}

func featureKey(m Model, code rune) Model {
	next, _ := m.handleKey(tea.KeyPressMsg{Code: code})
	return next.(Model)
}
func featureIDs(m Model) []int {
	var ids []int
	for _, s := range m.filtered {
		ids = append(ids, s.ID)
	}
	return ids
}

func TestViewOptionsApplyCancelAndIntersectSearch(t *testing.T) {
	m := featureModel()
	m.search.SetValue("char")
	m.refilter()
	m = featureKey(m, tea.KeyF2)
	m = featureKey(m, tea.KeyDown)
	m = featureKey(m, tea.KeyRight) // Normal excludes both search matches.
	if !strings.Contains(m.featureView(), "0 matches") {
		t.Fatal("draft count does not respect search")
	}
	if !reflect.DeepEqual(featureIDs(m), []int{4, 6}) {
		t.Fatal("draft mutated browse list")
	}
	m = featureKey(m, tea.KeyEscape)
	if m.viewOptions.Type != 0 || m.queryValue() != "char" {
		t.Fatal("cancel changed view/search")
	}
	m = featureKey(m, tea.KeyF2)
	m = featureKey(m, tea.KeyDown)
	m = featureKey(m, tea.KeyRight)
	m = featureKey(m, tea.KeyRight) // Fire.
	m = featureKey(m, tea.KeyEnter)
	if m.showFilters || m.viewOptions.Type != 2 || !reflect.DeepEqual(featureIDs(m), []int{4, 6}) {
		t.Fatal("apply failed")
	}
	m = featureKey(m, tea.KeyF2)
	m = featureKey(m, tea.KeyBackspace)
	m = featureKey(m, tea.KeyEnter)
	if m.viewOptions != (viewOptions{}) || m.queryValue() != "char" {
		t.Fatal("reset must preserve search")
	}
}

func TestFitListBorderTitle(t *testing.T) {
	if got := fitListBorderTitle("Pokedex", 12, 1025, 40); got != "Pokedex [ 12/1025 ]" {
		t.Fatalf("wide = %q", got)
	}
	if got := fitListBorderTitle("Pokedex", 12, 1025, 22); got != "Pokedex [12/1025]" && got != "[12/1025]" {
		// 22-4=18; "Pokedex [12/1025]" is 16 runes — prefer tight-with-base
		if textwidth.Width(got) > 18 {
			t.Fatalf("narrow too wide: %q", got)
		}
	}
	if got := fitListBorderTitle("Pokedex", 6, 6, 10); got != "[6/6]" {
		t.Fatalf("count-only = %q", got)
	}
}

func TestFavouriteQuickLinkKeepsDexSelection(t *testing.T) {
	m := featureModel()
	m.selectSpeciesID(25)
	dexCursor := m.cursor
	var saved []int
	m.saveFavourites = func(ids []int) error { saved = ids; return nil }
	next, cmd := m.handleKey(tea.KeyPressMsg{Code: tea.KeyF4})
	m = next.(Model)
	if !m.favouriteSaving || m.favourites[25] {
		t.Fatal("must commit only after successful save")
	}
	next, _ = m.update(cmd())
	m = next.(Model)
	// Quick-link prepend + dex row kept: [25] + [4, 6, 25, 133]
	if !reflect.DeepEqual(saved, []int{25}) || !reflect.DeepEqual(featureIDs(m), []int{25, 4, 6, 25, 133}) {
		t.Fatalf("quick-link/save failed: ids=%v", featureIDs(m))
	}
	s, _ := m.selected()
	if s.ID != 25 || m.cursor == 0 || m.cursor != dexCursor+1 {
		t.Fatalf("F4 must keep dex-row selection, cursor=%d want %d", m.cursor, dexCursor+1)
	}
	rows := m.browseData().Rows
	if !rows[0].Favourite || rows[0].SectionLabel == "" || !rows[1].DividerBefore {
		t.Fatal("missing favourites label/divider")
	}
	if !rows[0].Favourite || !rows[m.cursor].Favourite {
		t.Fatal("★ must show on quick-link and dex row")
	}
	m.search.SetValue("char")
	m.refilter()
	if !reflect.DeepEqual(featureIDs(m), []int{4, 6}) || m.hasFavouriteDivider() {
		t.Fatal("favourites escaped search filter")
	}
	m.search.SetValue("")
	m.viewOptions.FavouritesOnly = true
	m.refilter()
	if !reflect.DeepEqual(featureIDs(m), []int{25}) || m.favPinCount != 0 {
		t.Fatal("favourites-only must be unique (no quick-link dupes)")
	}
	next, cmd = m.toggleFavourite()
	m = next.(Model)
	next, _ = m.update(cmd())
	m = next.(Model)
	if len(m.filtered) != 0 || !strings.Contains(m.View().Content, "No matches") {
		t.Fatal("removing last favourite should show empty results")
	}
}

func TestSearchFavouriteStarsWithoutQuickLinks(t *testing.T) {
	m := featureModel()
	m.search.SetValue("pika")
	m.refilter()
	m.selectSpeciesID(25)
	m.saveFavourites = func([]int) error { return nil }
	next, cmd := m.toggleFavourite()
	m = next.(Model)
	next, _ = m.update(cmd())
	m = next.(Model)
	if !m.favourites[25] || m.favPinCount != 0 || m.hasFavouriteDivider() {
		t.Fatal("search F4 must star in place without favourites block")
	}
	if !reflect.DeepEqual(featureIDs(m), []int{25}) {
		t.Fatalf("search results mutated: %v", featureIDs(m))
	}
	rows := m.browseData().Rows
	if len(rows) != 1 || !rows[0].Favourite || rows[0].SectionLabel != "" {
		t.Fatalf("want single starred row, got %+v", rows)
	}
	m.search.SetValue("")
	m.refilter()
	if m.favPinCount != 1 || !reflect.DeepEqual(featureIDs(m), []int{25, 4, 6, 25, 133}) {
		t.Fatalf("clearing search should restore quick-links: pin=%d ids=%v", m.favPinCount, featureIDs(m))
	}
}

func TestUnstarFromQuickLinkAdvancesCursor(t *testing.T) {
	m := featureModel()
	m.favourites[25] = true
	m.favourites[4] = true
	m.favourites[6] = true
	m.saveFavourites = func([]int) error { return nil }
	m.refilter()
	// Pins in main order: 4, 6, 25 then main…
	if !reflect.DeepEqual(featureIDs(m)[:3], []int{4, 6, 25}) {
		t.Fatalf("pins=%v", featureIDs(m)[:3])
	}
	m.cursor = 0 // quick-link Charmander
	next, cmd := m.toggleFavourite()
	m = next.(Model)
	next, _ = m.update(cmd())
	m = next.(Model)
	s, _ := m.selected()
	if s.ID != 6 || m.cursor != 0 {
		t.Fatalf("unstar mid-pin should land on next pin, got id=%d cursor=%d", s.ID, m.cursor)
	}
	// Unstar last remaining pin while on last quick-link → first dex row.
	m.favourites = map[int]bool{25: true}
	m.refilter()
	m.cursor = 0
	next, cmd = m.toggleFavourite()
	m = next.(Model)
	next, _ = m.update(cmd())
	m = next.(Model)
	s, _ = m.selected()
	if m.favPinCount != 0 || s.ID != 4 || m.cursor != 0 {
		t.Fatalf("last unstar should land on first dex row, id=%d cursor=%d pin=%d", s.ID, m.cursor, m.favPinCount)
	}
}

func TestFavouriteSaveFailureAndGracefulExit(t *testing.T) {
	m := featureModel()
	m.saveFavourites = func([]int) error { return errors.New("disk full") }
	next, cmd := m.toggleFavourite()
	m = next.(Model)
	m = featureKey(m, tea.KeyF4) // Ignore repeat while the one write is outstanding.
	next, _ = m.requestQuit()
	m = next.(Model)
	if m.quitting || !m.quitAfterFavourite {
		t.Fatal("quit raced outstanding save")
	}
	next, _ = m.update(cmd())
	m = next.(Model)
	if len(m.favourites) != 0 || m.quitting || !strings.Contains(m.featureView(), "disk full") {
		t.Fatal("save failure was hidden or applied")
	}
	m = featureKey(m, tea.KeyEscape)
	m.saveFavourites = func([]int) error { return nil }
	next, cmd = m.toggleFavourite()
	m = next.(Model)
	next, _ = m.requestQuit()
	m = next.(Model)
	next, _ = m.update(cmd())
	m = next.(Model)
	if !m.quitting || !m.favourites[4] {
		t.Fatal("successful pending save did not complete before exit")
	}
}

func TestUnreadableFavouritesCannotBeOverwritten(t *testing.T) {
	m := featureModel()
	m.favouriteLoadError = "bad JSON"
	m.saveFavourites = func([]int) error { t.Fatal("must not save after load error"); return nil }
	m = featureKey(m, tea.KeyF4)
	if !strings.Contains(m.featureView(), "preserved") {
		t.Fatal("missing recovery explanation")
	}
}

func TestFavouriteDividerDoesNotHideSelectionWhenScrolling(t *testing.T) {
	m := featureModel()
	var all []domain.SpeciesSummary
	for i := 1; i <= 50; i++ {
		all = append(all, domain.SpeciesSummary{ID: i, DisplayName: fmt.Sprintf("Species %02d", i)})
	}
	m.index = search.Build(all)
	m.favourites[1] = true
	m.favourites[2] = true
	m.height = 18
	m.refilter()
	if m.favPinCount != 2 || len(m.filtered) != 52 {
		t.Fatalf("want 2 quick-links + 50 dex rows, pin=%d len=%d", m.favPinCount, len(m.filtered))
	}
	for i := range m.filtered {
		m.cursor = i
		m.ensureListVisible()
		d := m.browseData()
		extra := 0
		if m.hasFavouriteLabel() {
			extra++
		}
		if m.hasFavouriteDivider() {
			extra++
		}
		list := components.List(d.Rows[d.ListOff:], 28, m.listVisibleRows()+extra, 0)
		mark := ">   "
		if m.favourites[m.filtered[i].ID] {
			mark = "> ★ "
		}
		if !strings.Contains(ansi.Strip(list), mark+m.filtered[i].DisplayName) {
			t.Fatalf("selection %d not visible at offset %d\n%s", i, m.listOff, ansi.Strip(list))
		}
	}
}

func TestSortMissingStatsAndFavouritePriority(t *testing.T) {
	m := featureModel()
	m.viewOptions.Sort = 2
	m.refilter()
	if !reflect.DeepEqual(featureIDs(m), []int{6, 25, 4, 133}) {
		t.Fatal("speed ordering")
	}
	delete(m.browseEntries, 6)
	m.refilter()
	if !reflect.DeepEqual(featureIDs(m), []int{25, 4, 133, 6}) {
		t.Fatal("missing stats should come last")
	}
	m.favourites[133] = true
	m.refilter()
	// Quick-link Eevee + full speed order (Eevee still in place).
	if !reflect.DeepEqual(featureIDs(m), []int{133, 25, 4, 133, 6}) {
		t.Fatalf("favourites quick-link: %v", featureIDs(m))
	}
}

func TestComparePinShowsNameAdjacentMark(t *testing.T) {
	m := featureModel()
	m.width, m.height = 100, 30
	m = featureKey(m, tea.KeyF3) // pin Charmander
	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "Charmander ⇄") {
		t.Fatalf("pinned row should show name-adjacent ⇄:\n%s", view)
	}
	if strings.Contains(view, "F3 vs") {
		t.Fatalf("title/hint must not carry F3 vs:\n%s", view)
	}
	if strings.Contains(view, "Pokedex ·") && strings.Contains(view, "vs") {
		t.Fatalf("list title must stay plain Pokedex:\n%s", view)
	}
}

func TestComparePinCloseUnpinAndMissingStats(t *testing.T) {
	m := featureModel()
	m.search.SetValue("char")
	m.refilter()
	m = featureKey(m, tea.KeyF3)
	m = featureKey(m, tea.KeyDown)
	m = featureKey(m, tea.KeyF3)
	view := ansi.Strip(m.featureView())
	for _, want := range []string{"Charmander #004", "Charizard #006", "HP", "309", "534", "+225", "Compare"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q: %s", want, view)
		}
	}
	// Mirrored labels on both sides of the rule.
	if strings.Count(view, "Attack") < 2 || strings.Count(view, "│") < 5 {
		t.Fatalf("expected mirrored stat labels / vertical rule:\n%s", view)
	}
	if !m.showCompare || m.comparePin.Summary.ID != 4 {
		t.Fatal("pin/compare failed")
	}
	m = featureKey(m, tea.KeyEscape)
	s, _ := m.selected()
	if m.showCompare || s.ID != 6 || m.queryValue() != "char" || m.comparePin == nil {
		t.Fatal("close lost browse state")
	}
	delete(m.browseEntries, 6)
	m = featureKey(m, tea.KeyF3)
	if !strings.Contains(m.featureView(), "—") {
		t.Fatal("missing stats shown as zero")
	}
	view = ansi.Strip(m.featureView())
	if strings.Contains(view, "Δ = B") || strings.Contains(view, "unpin") {
		t.Fatalf("legend/unpin must be gone:\n%s", view)
	}
	m = featureKey(m, tea.KeyEscape)
	for m.cursor > 0 {
		m = featureKey(m, tea.KeyUp)
	}
	m = featureKey(m, tea.KeyF3) // same-species F3 unpins from browse
	if m.comparePin != nil || m.showCompare {
		t.Fatal("same-species F3 must unpin")
	}
}

func TestFeatureViewsFitSmallTerminals(t *testing.T) {
	for _, size := range [][2]int{{20, 6}, {40, 12}, {40, 18}, {52, 18}, {80, 24}, {120, 32}} {
		for _, compare := range []bool{false, true} {
			m := featureModel()
			m.width, m.height = size[0], size[1]
			if compare {
				m = featureKey(m, tea.KeyF3)
				m = featureKey(m, tea.KeyDown)
				m = featureKey(m, tea.KeyF3)
			} else {
				m = featureKey(m, tea.KeyF2)
			}
			view := m.View().Content
			if size[0] >= 40 && size[1] >= 18 {
				// Compare packs help on one line; esc may truncate below ~52 cols.
				needEsc := !compare || size[0] >= 52
				if needEsc && !strings.Contains(view, "esc") {
					t.Fatal("close action missing")
				}
				if compare && !strings.Contains(view, "Total") {
					t.Fatal("stats clipped")
				}
			}
			lines := strings.Split(view, "\n")
			if len(lines) > m.height {
				t.Fatalf("too tall %v: %d", size, len(lines))
			}
			for _, line := range lines {
				if textwidth.Width(line) > m.width {
					t.Fatalf("too wide %v: %q", size, line)
				}
			}
		}
	}
}

func TestFeatureOverlaysSuppressNativePainting(t *testing.T) {
	for _, overlay := range []string{"filter", "compare", "error"} {
		t.Run(overlay, func(t *testing.T) {
			m := featureModel()
			m.layoutTh.DisableSprite = false
			m.width, m.height = 120, 52
			m.imageNative = true
			m.imageANSI = "\x1bP0;0;8q#1~~~\x1b\\"
			m.imageSpeciesID = 4
			m.imageCells = m.currentSpriteCells()
			m.nativeOverlayDirty = true
			if !m.nativeOverlayReady() {
				t.Fatal("fixture must be paintable before opening overlay")
			}
			switch overlay {
			case "filter":
				m.showFilters = true
			case "compare":
				m.showCompare = true
			case "error":
				m.featureError = "disk full"
			}
			if m.nativeOverlayReady() || m.nativeSpriteActive() {
				t.Fatal("native paint enabled over overlay")
			}
			next, cmd := m.scheduleNativeOverlay()
			if cmd != nil || !next.nativeOverlayDirty {
				t.Fatal("overlay should defer native repaint")
			}
			next.closeFeature()
			if !next.nativeOverlayReady() || !next.nativeOverlayDirty {
				t.Fatal("closing must restore native art")
			}
		})
	}
}

func TestBrowseHelpOrder(t *testing.T) {
	var labels []string
	for _, b := range defaultKeyMap().ShortHelp() {
		labels = append(labels, b.Help().Key)
	}
	if !reflect.DeepEqual(labels, []string{"enter", "tab", "F2", "F3", "F4", "esc"}) {
		t.Fatal(labels)
	}
}

func TestCompareNavigationSkipsPinAndPreservesSearch(t *testing.T) {
	m := featureModel()
	m = featureKey(m, tea.KeyF3)
	m = featureKey(m, tea.KeyDown)
	m = featureKey(m, tea.KeyF3)
	if m.compareFocus != 1 || m.compareA.Summary.ID != 4 || m.compareB.Summary.ID != 6 {
		t.Fatalf("open state A=%d B=%d focus=%d", m.compareA.Summary.ID, m.compareB.Summary.ID, m.compareFocus)
	}
	m = featureKey(m, tea.KeyDown)
	if m.compareB.Summary.ID != 25 || !m.showCompare {
		t.Fatalf("focused B did not advance, B=%d", m.compareB.Summary.ID)
	}
	m = featureKey(m, tea.KeyUp)
	m = featureKey(m, tea.KeyUp)
	if m.compareB.Summary.ID != 6 {
		t.Fatalf("must not compare A against itself, B=%d", m.compareB.Summary.ID)
	}
	m = featureKey(m, tea.KeyEscape)
	if m.showCompare || m.comparePin.Summary.ID != 4 {
		t.Fatal("back lost pin")
	}
}

func TestFormatCompareDelta(t *testing.T) {
	if got := ansi.Strip(formatCompareDelta(0)); got != "0" {
		t.Fatalf("tie = %q", got)
	}
	if got := ansi.Strip(formatCompareDelta(19)); got != "+19" {
		t.Fatalf("pos = %q", got)
	}
	if got := ansi.Strip(formatCompareDelta(-19)); got != "-19" {
		t.Fatalf("neg = %q", got)
	}
}

func TestCompareStatColumnsAndArtOrigin(t *testing.T) {
	coreA := ansi.Strip(compareStatCore("HP", 80, true, 18))
	coreB := ansi.Strip(compareStatCore("HP", 60, true, 18))
	if !strings.HasPrefix(coreA, "HP") || !strings.HasSuffix(strings.TrimRight(coreA, " "), "80") {
		t.Fatalf("label L / value R: %q", coreA)
	}
	if textwidth.Width(coreA) != textwidth.Width(coreB) {
		t.Fatalf("cores must match width: %q (%d) vs %q (%d)", coreA, textwidth.Width(coreA), coreB, textwidth.Width(coreB))
	}

	m := featureModel()
	m.layoutTh.DisableSprite = false
	m.width, m.height = 120, 40
	m.compareA = m.comparisonEntry(m.filtered[0])
	m.compareB = m.comparisonEntry(m.filtered[1])
	m.showCompare = true
	rowA, colA, aw, ah, okA := m.compareArtOrigin(0)
	rowB, colB, _, _, okB := m.compareArtOrigin(1)
	if !okA || !okB || rowA != rowB || colB <= colA+aw || aw < 1 || ah < 1 {
		t.Fatalf("origins A=%d,%d B=%d,%d size=%dx%d ok=%v,%v", rowA, colA, rowB, colB, aw, ah, okA, okB)
	}
	g := m.compareLayoutGeom(m.compareFooterH())
	coreW := g.artOW
	if coreW < 18 {
		coreW = 18
	}
	padCore := (g.colW - coreW) / 2
	if padCore < 1 {
		t.Fatalf("stats should be inset under art, padCore=%d colW=%d coreW=%d", padCore, g.colW, coreW)
	}
	view := ansi.Strip(m.compareView())
	for _, row := range strings.Split(view, "\n") {
		if !strings.Contains(row, "HP") || !strings.Contains(row, "│") {
			continue
		}
		inner := strings.TrimPrefix(row, "│")
		if idx := strings.Index(inner, "HP"); idx >= 0 && idx < padCore {
			t.Fatalf("HP should sit in centred core band (pad=%d), got idx=%d in %q", padCore, idx, row)
		}
		// B core starts after rule; Δ may follow but both HP labels share the same inset.
		parts := strings.SplitN(inner, "│", 2)
		if len(parts) == 2 {
			aIdx, bIdx := strings.Index(parts[0], "HP"), strings.Index(parts[1], "HP")
			if aIdx != bIdx {
				t.Fatalf("asymmetric HP inset A=%d B=%d in %q", aIdx, bIdx, row)
			}
		}
		break
	}
}

func TestCompareTabSyncsListCursor(t *testing.T) {
	m := featureModel()
	m = featureKey(m, tea.KeyF3)      // pin 4
	m = featureKey(m, tea.KeyDown)    // 6
	m = featureKey(m, tea.KeyF3)      // compare, focus B=6
	m = featureKey(m, tea.KeyDown)    // B → 25
	if m.compareB.Summary.ID != 25 || m.cursor != 2 {
		t.Fatalf("B advance B=%d cursor=%d", m.compareB.Summary.ID, m.cursor)
	}
	m = featureKey(m, tea.KeyTab) // focus A=4
	if m.compareFocus != 0 || m.filtered[m.cursor].ID != 4 {
		t.Fatalf("tab must sync cursor to A, focus=%d cursorID=%d", m.compareFocus, m.filtered[m.cursor].ID)
	}
	m = featureKey(m, tea.KeyDown)
	if m.compareA.Summary.ID != 6 || m.filtered[m.cursor].ID != 6 {
		t.Fatalf("↑/↓ on A should change A, A=%d cursorID=%d", m.compareA.Summary.ID, m.filtered[m.cursor].ID)
	}
}

func TestCompareNativeClearUsesArtHOnly(t *testing.T) {
	m := featureModel()
	m.layoutTh.DisableSprite = false
	m.width, m.height = 120, 40
	row, col, artW, artH, ok := m.compareArtOrigin(0)
	if !ok || artH < 1 {
		t.Fatal("need art origin")
	}
	seq := m.compareNativeClearSeq()
	artOnly := clearCellRect(row, col, artW, artH)
	withSlack := clearCellRect(row, col, artW, artH+sixelClearSlackRows)
	if !strings.Contains(seq, artOnly) {
		t.Fatal("compare clear must erase artH rows")
	}
	if withSlack != artOnly && strings.Contains(seq, withSlack) {
		t.Fatal("compare clear must not use sixelClearSlackRows (bleeds under card)")
	}
}

func TestCompareNativeReadyPaintsNotBrowse(t *testing.T) {
	m := featureModel()
	m.layoutTh.DisableSprite = false
	m.width, m.height = 120, 40
	m.showCompare = true
	m.compareA = m.comparisonEntry(m.filtered[0])
	m.compareB = m.comparisonEntry(m.filtered[1])
	m.compareArtA = "\x1bP0;0;8q#1~~~\x1b\\"
	m.compareArtB = "\x1bP0;0;8q#2~~~\x1b\\"
	m.compareArtANative, m.compareArtBNative = true, true
	m.compareArtAID, m.compareArtBID = m.compareA.Summary.ID, m.compareB.Summary.ID
	m.compareArtCells = 16
	m.nativeOverlayDirty = true
	if m.nativeOverlayReady() {
		t.Fatal("browse native must stay suppressed")
	}
	if !m.compareNativeReady() {
		t.Fatal("compare dual sixel should be ready")
	}
	next, cmd := m.scheduleNativeOverlay()
	if cmd == nil || next.nativeOverlayDirty {
		t.Fatal("compare should emit dual paint and clear dirty")
	}
	if !next.compareNativePainted {
		t.Fatal("compare paint flag")
	}
}

func TestCompareTabSwapMirroredStats(t *testing.T) {
	m := featureModel()
	m = featureKey(m, tea.KeyF3)
	m = featureKey(m, tea.KeyDown)
	m = featureKey(m, tea.KeyF3)
	m = featureKey(m, tea.KeyTab)
	if m.compareFocus != 0 {
		t.Fatal("tab should focus A")
	}
	view := ansi.Strip(m.featureView())
	if !strings.Contains(view, "Charmander #004") || !strings.Contains(view, "Charizard #006") {
		t.Fatalf("missing dual border titles:\n%s", view)
	}
	if strings.Count(view, "HP") < 2 || strings.Count(view, "Speed") < 2 {
		t.Fatalf("stat labels must appear on both sides:\n%s", view)
	}
	if !strings.HasPrefix(strings.TrimLeft(view, "\n"), " Compare") && !strings.Contains(view, "\n Compare\n") {
		if !strings.Contains(view, " Compare\n╭") {
			t.Fatalf("Compare title should sit outside the border:\n%s", view)
		}
	}
	m = featureKey(m, ' ')
	if m.compareA.Summary.ID != 6 || m.compareB.Summary.ID != 4 || m.compareFocus != 0 {
		t.Fatalf("space swap failed A=%d B=%d focus=%d", m.compareA.Summary.ID, m.compareB.Summary.ID, m.compareFocus)
	}
}

func TestCompareDossierBrowseRestoresNativeAfterEveryCellFrame(t *testing.T) {
	m := featureModel()
	m.layoutTh.DisableSprite = false
	m.width, m.height = 120, 32
	m = featureKey(m, tea.KeyF3)
	m = featureKey(m, tea.KeyDown)
	m.imageNative = true
	m.imageANSI = "\x1bP0;0;8q#1~~~\x1b\\"
	m.imageSpeciesID = 6
	m.imageCells = m.currentSpriteCells()
	m.nativeOverlayDirty = true
	var buf bytes.Buffer
	m.composite = true
	m.compositor = newCompositorWriter(&buf)
	step := func(code rune, wantArt bool) {
		t.Helper()
		next, _ := m.Update(tea.KeyPressMsg{Code: code})
		m = next.(Model)
		buf.Reset()
		// Windows Terminal may have neither sync-output nor cursor-show markers.
		_, err := m.compositor.Write([]byte(m.View().Content))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(buf.String(), m.imageANSI) != wantArt {
			t.Fatalf("key %d: native tail present=%v, want %v", code, strings.HasSuffix(buf.String(), decRestoreCursor), wantArt)
		}
		if !wantArt && strings.Contains(buf.String(), decSaveCursor) {
			t.Fatal("native clear erased covering screen")
		}
		if wantArt && !strings.Contains(buf.String(), m.imageANSI) {
			t.Fatal("art payload missing")
		}
	}
	// Prime browse paint, then exercise the exact reported sequence repeatedly.
	m, _ = m.scheduleNativeOverlay()
	for i := 0; i < 3; i++ {
		step(tea.KeyF3, false)
		step(tea.KeyEscape, true)
		step(tea.KeyEnter, false)
		step(tea.KeyEscape, true)
		buf.Reset()
		_, _ = m.compositor.Write([]byte("later cell update"))
		if !strings.Contains(buf.String(), m.imageANSI) {
			t.Fatal("later cells erased native image")
		}
	}
}
