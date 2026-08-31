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
	// List ~58% / detail ~42%; aspect-corrected pad; chrome=8.
	cases := []struct {
		w, h       int
		sprite     bool
		verbose    bool
		onePane    bool
		tooSmall   bool
		spriteSize int
	}{
		{120, 52, true, true, false, false, 40},
		{120, 34, true, true, false, false, 40},
		{120, 22, false, true, false, false, 0}, // min bucket 24 does not fit body at h=22
		{120, 21, false, false, false, false, 0},
		{71, 26, true, false, false, false, 24},
		{70, 40, true, false, false, false, 24},
		{88, 40, true, true, false, false, 24},
		{51, 40, false, false, true, false, 0},
		{80, 17, false, false, true, false, 0},
		{39, 20, false, false, true, true, 0},
		{50, 11, false, false, true, true, 0},
	}
	for _, tc := range cases {
		m := tui.NewModel(tui.Options{Index: fixtureIndex(), Details: &fakeDetails{}})
		m = apply(t, m, tea.WindowSizeMsg{Width: tc.w, Height: tc.h})
		f := m.LayoutFlags()
		if f.ShowSprite != tc.sprite || f.VerboseFooter != tc.verbose || f.OnePane != tc.onePane || f.TooSmall != tc.tooSmall {
			t.Fatalf("%dx%d => sprite=%v verbose=%v one=%v small=%v; want sprite=%v verbose=%v one=%v small=%v",
				tc.w, tc.h, f.ShowSprite, f.VerboseFooter, f.OnePane, f.TooSmall,
				tc.sprite, tc.verbose, tc.onePane, tc.tooSmall)
		}
		if f.SpriteSize != tc.spriteSize {
			t.Fatalf("%dx%d SpriteSize=%d want %d", tc.w, tc.h, f.SpriteSize, tc.spriteSize)
		}
	}
}

func TestListFirstColumns(t *testing.T) {
	m := tui.NewModel(tui.Options{Index: fixtureIndex(), Details: &fakeDetails{}})
	m = apply(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	f := m.LayoutFlags()
	if !f.ShowSprite {
		t.Fatal("expected sprite")
	}
	// Detail ~42%, list ~58%.
	if f.RightWidth >= f.LeftWidth {
		t.Fatalf("list-first failed: left=%d right=%d", f.LeftWidth, f.RightWidth)
	}
	if f.LeftWidth < sprite.MinListWidth {
		t.Fatalf("list too narrow: %d", f.LeftWidth)
	}
	share := f.RightWidth * 100 / 120
	if share < 38 || share > 46 {
		t.Fatalf("right share %d%% (w=%d) want ~42%%", share, f.RightWidth)
	}
	listShare := f.LeftWidth * 100 / 120
	if listShare < 54 || listShare > 62 {
		t.Fatalf("list share %d%% (w=%d) want ~58%%", listShare, f.LeftWidth)
	}
	if f.LeftWidth+f.RightWidth+1 != 120 {
		t.Fatalf("widths must fill frame: left=%d right=%d", f.LeftWidth, f.RightWidth)
	}

	m = apply(t, m, tea.WindowSizeMsg{Width: 180, Height: 40})
	f = m.LayoutFlags()
	want := (180 * sprite.RightPanelSharePercent) / 100
	if f.RightWidth < want-2 || f.RightWidth > want+2 {
		t.Fatalf("wide right=%d want ~%d (42%%)", f.RightWidth, want)
	}
}

func TestSpriteShowsWhenBodyFits(t *testing.T) {
	m := tui.NewModel(tui.Options{Index: fixtureIndex(), Details: &fakeDetails{}})
	m = apply(t, m, tea.WindowSizeMsg{Width: 120, Height: sprite.MinTerminalHeight()})
	f := m.LayoutFlags()
	if !f.ShowSprite || f.SpriteSize < sprite.MinFrameSize {
		t.Fatalf("expected sprite at 120x%d, got show=%v size=%d", sprite.MinTerminalHeight(), f.ShowSprite, f.SpriteSize)
	}
	m = apply(t, m, tea.WindowSizeMsg{Width: 120, Height: sprite.MinTerminalHeight() - 1})
	f = m.LayoutFlags()
	if f.ShowSprite {
		t.Fatal("expected sprite hidden when body shorter than min outer")
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
		BrowseView: 0,
		ViewLabels: []string{"Sprite"},
		BodySize:   sprite.MaxFrameSize,
		LeftWidth:  70, RightWidth: 49,
		HeaderTypes: "Fire",
		HeaderGenus: "Lizard Pokémon",
		SpriteANSI:  "[sprite]",
	})
	for _, want := range []string{
		"Pokedex",
		"> c",
		"Charmander",
		"#004",
		"[sprite]",
		"Enter/i",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "[ Sprite ]") {
		t.Fatal("browse must not show text-view switcher")
	}
}

func TestDialogOverlaySnapshot(t *testing.T) {
	out := views.Browse(views.BrowseData{
		Width: 120, Height: 40,
		HasSelected: true,
		Selected:    domain.SpeciesSummary{ID: 4, DisplayName: "Charmander"},
		ShowSprite:  true, VerboseFooter: true,
		BrowseView: 0,
		ViewLabels: []string{"Sprite"},
		BodySize:   sprite.MaxFrameSize,
		LeftWidth:  70, RightWidth: 49,
		ShowDialog:    true,
		DialogSection: 0,
		DialogLabels:  []string{"Summary", "Stats", "Evolution", "Moves", "Locations", "Abilities", "Flavour", "Breeding", "Training", "Forms", "Meta"},
		DialogLines:   []string{"Height  0.6 m"},
	})
	for _, want := range []string{"Charmander", "#004", "[ Summary ]", "Height", "Esc close"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestBrowseSpriteNoSwitcher(t *testing.T) {
	rows := []components.ListRow{
		{Summary: domain.SpeciesSummary{ID: 4, DisplayName: "Charmander", Types: []string{"fire"}}, Selected: true},
	}
	out := views.Browse(views.BrowseData{
		Width: 120, Height: 52,
		Rows: rows, HasSelected: true,
		Selected:   domain.SpeciesSummary{ID: 4, DisplayName: "Charmander", Types: []string{"fire"}},
		ShowSprite: true, VerboseFooter: true,
		BrowseView: 0,
		ViewLabels: []string{"Sprite"},
		BodySize:   sprite.MinFrameSize,
		LeftWidth:  70, RightWidth: 49,
		HeaderTypes: "Fire",
		SpriteANSI:  "X",
	})
	plain := stripANSI(out)
	if strings.Contains(plain, "[ Sprite ]") || strings.Contains(plain, "Summary ·") {
		t.Fatalf("unexpected browse switcher:\n%s", plain)
	}
	if !strings.Contains(plain, "Charmander") || !strings.Contains(plain, "X") {
		t.Fatalf("expected list+sprite browse:\n%s", plain)
	}
}

func TestDialogUsesFullOverlayWidth(t *testing.T) {
	long := strings.Repeat("flavour ", 20)
	out := views.Browse(views.BrowseData{
		Width: 120, Height: 40,
		HasSelected: true,
		Selected:    domain.SpeciesSummary{ID: 4, DisplayName: "Charmander"},
		ShowSprite:  true, VerboseFooter: true,
		BrowseView: 0,
		ViewLabels: []string{"Sprite"},
		BodySize:   sprite.MinFrameSize,
		LeftWidth:  70, RightWidth: 49,
		ShowDialog:    true,
		DialogSection: 0,
		DialogLabels:  []string{"Summary"},
		DialogLines:   []string{long},
	})
	plain := stripANSI(out)
	if !strings.Contains(plain, "flavour") {
		t.Fatal("dialog body missing")
	}
	if strings.Count(plain, "flavour") < 3 {
		t.Fatalf("expected wrapped dialog text, got:\n%s", plain)
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
				inEsc = false
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}
