package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/tui"
	"github.com/Graphixa/Pokedex/internal/tui/components"
	"github.com/Graphixa/Pokedex/internal/tui/views"
)

func TestLayoutBreakpoints(t *testing.T) {
	// Fixed MaxFrameSize×(MaxFrameSize/2) art (+ pad) when terminal clears sprite gates.
	cases := []struct {
		w, h            int
		sprite, summary bool
		verbose         bool
		onePane         bool
		tooSmall        bool
		spriteSize      int
	}{
		{120, 52, true, true, true, false, false, 32},
		{120, 50, true, true, true, false, false, 32},
		{140, 56, true, true, true, false, false, 32},
		{120, 40, true, true, true, false, false, 32}, // height gate default 40
		{120, 30, false, true, true, false, false, 0},
		{109, 52, true, true, true, false, false, 32},
		{100, 50, true, true, true, false, false, 32},
		{100, 40, true, true, true, false, false, 32},
		{99, 52, false, true, true, false, false, 0}, // width < 100
		{88, 40, false, true, true, false, false, 0},
		{88, 24, false, false, true, false, false, 0},
		{80, 28, false, false, true, false, false, 0},
		{79, 40, false, false, true, false, false, 0},
		{75, 40, false, false, false, false, false, 0},
		{63, 40, false, false, false, false, false, 0},
		{51, 40, false, false, false, true, false, 0},
		{80, 17, false, false, false, true, false, 0},
		{39, 20, false, false, false, true, true, 0},
		{50, 11, false, false, false, true, true, 0},
	}
	for _, tc := range cases {
		m := tui.NewModel(tui.Options{Index: fixtureIndex(), Details: &fakeDetails{}})
		m = apply(t, m, tea.WindowSizeMsg{Width: tc.w, Height: tc.h})
		f := m.LayoutFlags()
		if f.ShowSprite != tc.sprite || f.ShowSummary != tc.summary || f.VerboseFooter != tc.verbose || f.OnePane != tc.onePane || f.TooSmall != tc.tooSmall {
			t.Fatalf("%dx%d => sprite=%v summary=%v verbose=%v one=%v small=%v; want sprite=%v summary=%v verbose=%v one=%v small=%v",
				tc.w, tc.h, f.ShowSprite, f.ShowSummary, f.VerboseFooter, f.OnePane, f.TooSmall,
				tc.sprite, tc.summary, tc.verbose, tc.onePane, tc.tooSmall)
		}
		if f.SpriteSize != tc.spriteSize {
			t.Fatalf("%dx%d SpriteSize=%d want %d", tc.w, tc.h, f.SpriteSize, tc.spriteSize)
		}
	}
}

func TestSpritePlaceholder(t *testing.T) {
	out := components.SpritePlaceholder("Charmander", sprite.FrameWidth, sprite.FrameHeight)
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
		ShowSprite:  true, VerboseFooter: true,
		BrowseView:  0,
		ViewLabels:  []string{"Sprite", "Summary", "Stats", "Evolution"},
		BodySize:    sprite.FrameOuterWidth(),
		HeaderTypes: "Fire",
		HeaderGenus: "Lizard Pokémon",
		SpriteANSI:  "[sprite]",
	})
	for _, want := range []string{
		"Pokedex",
		"> c",
		"Charmander",
		"#004",
		"[ Sprite ]",
		"Summary",
		"Stats",
		"Evolution",
		"Tab view",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}

	out2 := views.Browse(views.BrowseData{
		Width: 120, Height: 52, Query: "c",
		Rows: rows, HasSelected: true,
		Selected:   domain.SpeciesSummary{ID: 4, DisplayName: "Charmander"},
		ShowSprite: true, VerboseFooter: true,
		BrowseView: 1,
		ViewLabels: []string{"Sprite", "Summary", "Stats", "Evolution"},
		BodySize:   sprite.FrameOuterWidth(),
		ViewLines:  []string{"Height  0.6 m", "Ability Blaze"},
		HeaderTypes: "Fire",
	})
	if strings.Contains(out2, "Search Pokémon…") {
		t.Fatal("placeholder should hide when query set")
	}
	if !strings.Contains(out2, "[ Summary ]") {
		t.Fatalf("expected Summary active:\n%s", out2)
	}
	if !strings.Contains(out2, "Height") {
		t.Fatalf("expected summary body:\n%s", out2)
	}
}

func TestDialogOverlaySnapshot(t *testing.T) {
	out := views.Browse(views.BrowseData{
		Width: 120, Height: 40,
		HasSelected: true,
		Selected:    domain.SpeciesSummary{ID: 4, DisplayName: "Charmander"},
		ShowSprite:  true, VerboseFooter: true,
		BrowseView:  0,
		ViewLabels:  []string{"Sprite", "Summary", "Stats", "Evolution"},
		BodySize:    sprite.FrameOuterWidth(),
		ShowDialog:  true,
		DialogSection: 0,
		DialogLabels:  []string{"Moves", "Locations", "Abilities", "Flavour", "Breeding", "Training", "Forms", "Meta"},
		DialogLines:   []string{"Lv 1  Scratch"},
	})
	for _, want := range []string{"Charmander", "#004", "[ Moves ]", "Scratch", "Esc close"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}
