package tui

import (
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

func TestFavouritePersistencePreservesSelectionAndNoDuplicates(t *testing.T) {
	m := featureModel()
	m.selectSpeciesID(25)
	var saved []int
	m.saveFavourites = func(ids []int) error { saved = ids; return nil }
	next, cmd := m.handleKey(tea.KeyPressMsg{Code: tea.KeyF4})
	m = next.(Model)
	if !m.favouriteSaving || m.favourites[25] {
		t.Fatal("must commit only after successful save")
	}
	next, _ = m.update(cmd())
	m = next.(Model)
	if !reflect.DeepEqual(saved, []int{25}) || !reflect.DeepEqual(featureIDs(m), []int{25, 4, 6, 133}) {
		t.Fatal("pin/save ordering failed")
	}
	s, _ := m.selected()
	if s.ID != 25 || m.cursor != 0 {
		t.Fatal("favourite changed selected species")
	}
	rows := m.browseData().Rows
	if !rows[0].Favourite || !rows[1].DividerBefore {
		t.Fatal("missing group marker/divider")
	}
	m.search.SetValue("char")
	m.refilter()
	if !reflect.DeepEqual(featureIDs(m), []int{4, 6}) || m.hasFavouriteDivider() {
		t.Fatal("favourites escaped search filter")
	}
	m.search.SetValue("")
	m.viewOptions.FavouritesOnly = true
	m.refilter()
	if !reflect.DeepEqual(featureIDs(m), []int{25}) {
		t.Fatal("favourites collection failed")
	}
	next, cmd = m.toggleFavourite()
	m = next.(Model)
	next, _ = m.update(cmd())
	m = next.(Model)
	if len(m.filtered) != 0 || !strings.Contains(m.View().Content, "No matches") {
		t.Fatal("removing last favourite should show empty results")
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
	for i := 0; i < len(all); i++ {
		m.cursor = i
		m.ensureListVisible()
		d := m.browseData()
		list := components.List(d.Rows[d.ListOff:], 28, m.listVisibleRows()+1, false, 0, nil)
		if !strings.Contains(ansi.Strip(list), fmt.Sprintf(">%sSpecies %02d", map[bool]string{true: "★", false: " "}[i < 2], i+1)) {
			t.Fatalf("selection %d not visible at offset %d", i, m.listOff)
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
	if !reflect.DeepEqual(featureIDs(m), []int{133, 25, 4, 6}) {
		t.Fatal("favourites should stay grouped")
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
	for _, want := range []string{"Charmander", "Charizard", "309", "534 +", "BASE STATS"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q: %s", want, view)
		}
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
	m = featureKey(m, tea.KeyBackspace)
	if m.comparePin != nil || m.showCompare {
		t.Fatal("unpin failed")
	}
	m = featureKey(m, tea.KeyF3)
	m = featureKey(m, tea.KeyF3)
	if m.comparePin != nil || m.showCompare {
		t.Fatal("same-species F3 must unpin")
	}
}

func TestFeatureViewsFitSmallTerminals(t *testing.T) {
	for _, size := range [][2]int{{40, 12}, {40, 18}, {52, 18}, {80, 24}} {
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
