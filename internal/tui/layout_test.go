package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/tui"
	"github.com/Graphixa/Pokedex/internal/tui/components"
	"github.com/Graphixa/Pokedex/internal/tui/views"
)

func TestLayoutBreakpoints(t *testing.T) {
	cases := []struct {
		w, h            int
		sprite, summary bool
		verbose, short  bool
		onePane         bool
		tooSmall        bool
		spriteSize      int
	}{
		{120, 52, true, true, true, false, false, false, 40},
		{120, 40, true, true, true, false, false, false, 28},
		{120, 30, true, true, true, false, false, false, 16},
		{109, 52, true, true, true, false, false, false, 40},
		{80, 28, true, false, true, false, false, false, 16},
		{79, 40, false, false, true, false, false, false, 0},
		{99, 52, true, true, true, false, false, false, 40},
		{87, 40, true, false, true, false, false, false, 28},
		{88, 24, false, false, true, false, false, false, 0},
		{75, 40, false, false, false, false, false, false, 0},
		{63, 40, false, false, false, true, false, false, 0},
		{51, 40, false, false, false, true, true, false, 0},
		{80, 17, false, false, false, false, true, false, 0},
		{39, 20, false, false, false, true, true, true, 0},
		{50, 11, false, false, false, true, true, true, 0},
	}
	for _, tc := range cases {
		m := tui.NewModel(tui.Options{Index: fixtureIndex(), Details: &fakeDetails{}})
		m = apply(t, m, tea.WindowSizeMsg{Width: tc.w, Height: tc.h})
		f := m.LayoutFlags()
		if f.ShowSprite != tc.sprite || f.ShowSummary != tc.summary || f.VerboseFooter != tc.verbose || f.ShortTabs != tc.short || f.OnePane != tc.onePane || f.TooSmall != tc.tooSmall {
			t.Fatalf("%dx%d => sprite=%v summary=%v verbose=%v short=%v one=%v small=%v; want sprite=%v summary=%v verbose=%v short=%v one=%v small=%v",
				tc.w, tc.h, f.ShowSprite, f.ShowSummary, f.VerboseFooter, f.ShortTabs, f.OnePane, f.TooSmall,
				tc.sprite, tc.summary, tc.verbose, tc.short, tc.onePane, tc.tooSmall)
		}
		if f.SpriteSize != tc.spriteSize {
			t.Fatalf("%dx%d SpriteSize=%d want %d", tc.w, tc.h, f.SpriteSize, tc.spriteSize)
		}
	}
}

func TestSpritePlaceholder(t *testing.T) {
	out := components.SpritePlaceholder("Charmander", 40, 40)
	if !strings.Contains(out, "[ Charmander ]") {
		t.Fatalf("placeholder = %q", out)
	}
}

func TestBrowseSnapshotRegions(t *testing.T) {
	rows := []components.ListRow{
		{Summary: domain.SpeciesSummary{ID: 4, DisplayName: "Charmander", Types: []string{"fire"}}, Selected: true},
		{Summary: domain.SpeciesSummary{ID: 5, DisplayName: "Charmeleon", Types: []string{"fire"}}},
	}
	out := views.Browse(views.BrowseData{
		Width: 120, Height: 52,
		Query: "c",
		Rows:  rows, ListOff: 0,
		HasSelected: true,
		Selected:    domain.SpeciesSummary{ID: 4, DisplayName: "Charmander", Types: []string{"fire"}},
		ShowSprite:  true, ShowSummary: true, VerboseFooter: true,
		SpriteSide:  40,
		ActiveTab: 1,
		TabLabels: []string{"Stats", "Evolution", "Locations", "Moves", "Flavour", "Info"},
		Info: components.MainInfo{
			Types: "Fire", Genus: "Lizard Pokémon",
			Height: "0.6 m", Weight: "8.5 kg",
			Ability: "Blaze", Hidden: "Solar Power", BaseExp: 62,
			ShowSummary: true, Summary: "Obviously prefers hot places.",
		},
		DetailTitle: "Evolution",
		DetailLines: []string{"Charmander ── Level 16 ──► Charmeleon"},
	})
	for _, want := range []string{
		"Pokedex — Browse Mode",
		"> c",
		"Charmander",
		"#004",
		"Evolution",
		"Type to filter",
		"[Stats]",
		"[Locations]",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	// Query is non-empty so placeholder text is replaced by the typed query.
	out2 := views.Browse(views.BrowseData{
		Width: 120, Height: 52, Query: "c",
		Rows: rows, HasSelected: true,
		Selected:   domain.SpeciesSummary{ID: 4, DisplayName: "Charmander"},
		ShowSprite: true, ShowSummary: true, VerboseFooter: true,
		SpriteSide:  40,
		TabLabels:   []string{"Stats", "Evolution", "Locations", "Moves", "Flavour", "Info"},
		Info:        components.MainInfo{Types: "Fire"},
		DetailTitle: "Stats",
		DetailLines: []string{"HP 39"},
	})
	if strings.Contains(out2, "Search Pokémon…") {
		t.Fatal("placeholder should hide when query set")
	}
	if !strings.Contains(out2, "> c") && !strings.Contains(out2, ">c") {
		// Search renders "> " + query
		if !strings.Contains(out2, "c") {
			t.Fatalf("query missing:\n%s", out2)
		}
	}
}
