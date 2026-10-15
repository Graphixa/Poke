package tui

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

func TestCompareStatLinesContentSized(t *testing.T) {
	labels := []string{"HP", "Attack", "Total"}
	lines := compareStatLines(labels, []int{39, 52, 309}, true, nil, 40)
	if len(lines) != 4 { // 3 stats + divider before Total
		t.Fatalf("want 4 lines, got %d %#v", len(lines), lines)
	}
	plain := make([]string, len(lines))
	for i, l := range lines {
		plain[i] = ansi.Strip(l)
		if textwidth.Width(l) != 40 {
			t.Fatalf("line %d width %d want 40: %q", i, textwidth.Width(l), plain[i])
		}
	}
	hp := plain[0]
	if !strings.HasPrefix(hp, "HP") || !strings.Contains(hp, "39") {
		t.Fatalf("HP row: %q", hp)
	}
	// Roomy budget: content hugs left (gap≤2); spare trails — not a budget-wide middle gap.
	trimmed := strings.TrimRight(hp, " ")
	if textwidth.Width(trimmed) > 6+2+3 { // Attack + maxGap + "309"
		t.Fatalf("label↔value gap inflated: trimmed width %d in %q", textwidth.Width(trimmed), hp)
	}
	if trimmed == hp {
		t.Fatalf("spare should trail after value cluster: %q", hp)
	}
	div := plain[2]
	if !strings.Contains(div, "─") || strings.ContainsAny(div, "0123456789") {
		t.Fatalf("divider should be dashes only: %q", div)
	}
	if !strings.HasPrefix(strings.TrimRight(div, " "), "──────") {
		t.Fatalf("divider should underline labels: %q", div)
	}

	withΔ := compareStatLines(labels, []int{58, 64, 405}, true, []string{"+19", "+12", "+96"}, 40)
	got := ansi.Strip(withΔ[0])
	if !strings.Contains(got, "58") || !strings.Contains(got, "+19") || strings.Contains(got, "…") {
		t.Fatalf("Δ column without truncating: %q", got)
	}
	idx := strings.Index(got, "58")
	rest := got[idx+2:]
	spaces := 0
	for _, r := range rest {
		if r != ' ' {
			break
		}
		spaces++
	}
	if spaces < 2 || !strings.HasPrefix(strings.TrimLeft(rest, " "), "+19") {
		t.Fatalf("want ≥2 spaces between value and Δ: %q", got)
	}
	if strings.TrimRight(got, " ") == got {
		t.Fatalf("roomy budget should keep trailing pad after Δ: %q", got)
	}
	dDiv := ansi.Strip(withΔ[2])
	trim := strings.TrimSpace(dDiv)
	if !strings.Contains(trim, "  ") || !strings.Contains(trim, "──") {
		t.Fatalf("Δ divider should break between value and Δ: %q", dDiv)
	}
}

func TestCompareStatColumnsAndArtOrigin(t *testing.T) {
	m := featureModel()
	m.layoutTh.DisableSprite = false
	m.width, m.height = 120, 40
	m.compareA = m.comparisonEntry(m.filtered[0])
	m.compareB = m.comparisonEntry(m.filtered[1])
	m.showCompare = true
	m.compareFocus = 1 // focused B art uses primary border (ANSI-heavier)
	rowA, colA, aw, ah, okA := m.compareArtOrigin(0)
	rowB, colB, _, _, okB := m.compareArtOrigin(1)
	if !okA || !okB || rowA != rowB || colB <= colA+aw || aw < 1 || ah < 1 {
		t.Fatalf("origins A=%d,%d B=%d,%d size=%dx%d ok=%v,%v", rowA, colA, rowB, colB, aw, ah, okA, okB)
	}
	g := m.compareLayoutGeom(m.compareFooterH())
	if !g.drawArt || g.coreW < 18 {
		t.Fatalf("expected side-by-side art+stats geom, drawArt=%v coreW=%d", g.drawArt, g.coreW)
	}
	view := ansi.Strip(m.compareView())
	if !strings.Contains(view, "╭─ Compare") {
		t.Fatalf("Compare should be the border title:\n%s", view)
	}
	if strings.Contains(view, "3…") || strings.Contains(view, "5…") || strings.Contains(view, "30…") {
		t.Fatalf("stats truncated despite room:\n%s", view)
	}
	for _, want := range []string{"39", "52", "309", "78", "84", "534"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing stat value %q:\n%s", want, view)
		}
	}
	if g.colWA+1+g.colWB != g.innerW {
		t.Fatalf("column split %d+%d+1 = %d want innerW %d", g.colWA, g.colWB, g.colWA+g.colWB+1, g.innerW)
	}
	artInset := g.padL + g.artOW + g.gap
	for _, row := range strings.Split(view, "\n") {
		if !strings.Contains(row, "HP") || !strings.Contains(row, "│") {
			continue
		}
		inner := strings.TrimPrefix(row, "│")
		parts := strings.SplitN(inner, "│", 2)
		if len(parts) != 2 {
			break
		}
		aIdx, bIdx := strings.Index(parts[0], "HP"), strings.Index(parts[1], "HP")
		if aIdx < artInset || bIdx < artInset || aIdx != bIdx {
			t.Fatalf("HP should sit beside art (inset≥%d), A=%d B=%d in %q", artInset, aIdx, bIdx, row)
		}
		break
	}
}

func TestCompareMediumWidthKeepsDeltaValues(t *testing.T) {
	m := featureModel()
	m.layoutTh.DisableSprite = false
	m.compareA = m.comparisonEntry(m.filtered[0])
	m.compareB = m.comparisonEntry(m.filtered[1])
	m.showCompare = true
	// Medium widths used to set coreW=full budget then bolt Δ on → "58…" with empty gap.
	for _, width := range []int{100, 110, 120} {
		m.width, m.height = width, 40
		g := m.compareLayoutGeom(m.compareFooterH())
		if !g.drawArt {
			continue
		}
		view := ansi.Strip(m.compareView())
		for _, want := range []string{"39", "309", "78", "534"} {
			if !strings.Contains(view, want) {
				t.Fatalf("width %d missing %q:\n%s", width, want, view)
			}
		}
		if strings.Contains(view, "78…") || strings.Contains(view, "53…") || strings.Contains(view, "39…") {
			t.Fatalf("width %d truncated values despite room:\n%s", width, view)
		}
	}
}

func TestCompareColumnSplitFillsInnerWidth(t *testing.T) {
	m := featureModel()
	m.layoutTh.DisableSprite = false
	for _, width := range []int{80, 100, 120, 121, 140} {
		m.width, m.height = width, 40
		g := m.compareLayoutGeom(m.compareFooterH())
		if g.colWA+1+g.colWB != g.innerW {
			t.Fatalf("width %d: split %d + 1 + %d = %d want %d", width, g.colWA, g.colWB, g.colWA+1+g.colWB, g.innerW)
		}
		if g.colWB < g.colWA {
			t.Fatalf("width %d: B should be ≥ A, got A=%d B=%d", width, g.colWA, g.colWB)
		}
	}

	m.compareA = m.comparisonEntry(m.filtered[0])
	m.compareB = m.comparisonEntry(m.filtered[1])
	m.showCompare = true
	m.width, m.height = 120, 40
	g := m.compareLayoutGeom(m.compareFooterH())
	if g.colWB > g.colWA+1 {
		t.Fatalf("50/50 split: B should be A or A+1, got A=%d B=%d", g.colWA, g.colWB)
	}

	// Body rows must fill innerW exactly — spare/missing columns show up as uneven halves.
	m.width, m.height = 121, 40
	m.compareFocus = 1
	// Fat ANSI art used to inflate half A and Truncate ate B's values ("7…", "8…").
	fat := strings.Repeat(lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Render("█"), 40)
	fatArt := strings.Repeat(fat+"\n", 8)
	m.compareArtA, m.compareArtAID, m.compareArtANative = fatArt, m.compareA.Summary.ID, false
	m.compareArtB, m.compareArtBID, m.compareArtBNative = fatArt, m.compareB.Summary.ID, false
	g = m.compareLayoutGeom(m.compareFooterH())
	view := m.compareView()
	for i, row := range strings.Split(view, "\n") {
		if !strings.HasPrefix(row, "│") || !strings.HasSuffix(ansi.Strip(row), "│") {
			continue
		}
		if strings.Contains(row, "tab focus") {
			break
		}
		plain := ansi.Strip(row)
		if !strings.HasPrefix(plain, "│") || !strings.HasSuffix(plain, "│") {
			continue
		}
		if w := textwidth.Width(row) - 2; w != g.innerW {
			t.Fatalf("row %d width %d want innerW %d\n%s", i, w, g.innerW, plain)
		}
		if strings.Contains(plain, "…") && (strings.Contains(plain, "HP") || strings.Contains(plain, "Total")) {
			t.Fatalf("stats truncated despite fitted halves:\n%s", plain)
		}
	}
	plainView := ansi.Strip(view)
	for _, want := range []string{"78", "84", "534", "39", "52", "309"} {
		if !strings.Contains(plainView, want) {
			t.Fatalf("missing value %q after fat-art layout:\n%s", want, plainView)
		}
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
		t.Fatalf("missing in-column titles:\n%s", view)
	}
	if strings.Count(view, "HP") < 2 || strings.Count(view, "Speed") < 2 {
		t.Fatalf("stat labels must appear on both sides:\n%s", view)
	}
	if !strings.Contains(view, "╭─ Compare") {
		t.Fatalf("Compare should be the border title:\n%s", view)
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
