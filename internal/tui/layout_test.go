package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/tui"
	"github.com/Graphixa/Pokedex/internal/tui/components"
	"github.com/Graphixa/Pokedex/internal/tui/style"
	"github.com/Graphixa/Pokedex/internal/tui/views"
)

func TestLayoutBreakpoints(t *testing.T) {
	// List ~25–30% without types; sprite pane takes the rest. Chrome=8.
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
		{70, 40, true, false, false, false, 32},
		{88, 40, true, true, false, false, 40},
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
	if f.LeftWidth+f.RightWidth+1 != 120 {
		t.Fatalf("widths must fill frame: left=%d right=%d", f.LeftWidth, f.RightWidth)
	}
	listShare := f.LeftWidth * 100 / 120
	if listShare < 24 || listShare > 32 {
		t.Fatalf("name-only list share %d%% (w=%d) want ~25–30%%", listShare, f.LeftWidth)
	}
	if f.LeftWidth >= f.RightWidth {
		t.Fatalf("name-only list should be narrower than sprite pane: left=%d right=%d", f.LeftWidth, f.RightWidth)
	}

	m = tui.NewModel(tui.Options{Index: fixtureIndex(), Details: &fakeDetails{}, ListTypes: true})
	m = apply(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	f = m.LayoutFlags()
	listShare = f.LeftWidth * 100 / 120
	if listShare < 45 || listShare > 55 {
		t.Fatalf("typed list share %d%% (w=%d) want ~50%%", listShare, f.LeftWidth)
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

func TestBrowseListRowsShowColoredTypes(t *testing.T) {
	rows := []components.ListRow{
		{Summary: domain.SpeciesSummary{ID: 1, DisplayName: "Bulbasaur", Types: []string{"grass", "poison"}}, Selected: true},
		{Summary: domain.SpeciesSummary{ID: 10, DisplayName: "Caterpie", Types: []string{"bug"}}},
	}
	out := views.Browse(views.BrowseData{
		Width: 44, Height: 5,
		OnePane: true, ShowListPane: true,
		Rows: rows, ListTypes: true,
	})
	plain := stripANSI(out)
	lines := strings.Split(plain, "\n")
	if !strings.Contains(lines[1], "> Bulbasaur") || !strings.Contains(lines[1], "#001") || !strings.Contains(lines[1], "Grass/Poison") {
		t.Fatalf("selected list row = %q", lines[1])
	}
	nameAt := strings.Index(lines[1], "Bulbasaur")
	dexAt := strings.Index(lines[1], "#001")
	if nameAt < 0 || dexAt < 0 || dexAt-nameAt-len("Bulbasaur") > 6 {
		t.Fatalf("dex should sit next to the name, row=%q", lines[1])
	}
	if !strings.Contains(lines[2], "Caterpie") || !strings.Contains(lines[2], "#010") || !strings.Contains(lines[2], "Bug") {
		t.Fatalf("second list row = %q", lines[2])
	}
	for _, want := range []string{
		style.TypeLabel("grass"),
		style.TypeLabel("poison"),
		style.TypeLabel("bug"),
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing coloured type label %q in:\n%q", want, out)
		}
	}
}

func TestBrowseListRowsHideTypesByDefault(t *testing.T) {
	rows := []components.ListRow{
		{Summary: domain.SpeciesSummary{ID: 1, DisplayName: "Bulbasaur", Types: []string{"grass", "poison"}}, Selected: true},
		{Summary: domain.SpeciesSummary{ID: 10, DisplayName: "Caterpie", Types: []string{"bug"}}},
	}
	out := views.Browse(views.BrowseData{
		Width: 44, Height: 5,
		OnePane: true, ShowListPane: true,
		Rows: rows,
	})
	plain := stripANSI(out)
	for _, want := range []string{"Bulbasaur", "#001", "Caterpie", "#010"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("missing %q in list:\n%s", want, plain)
		}
	}
	nameAt := strings.Index(plain, "Bulbasaur")
	dexAt := strings.Index(plain, "#001")
	if nameAt < 0 || dexAt < 0 || dexAt-nameAt-len("Bulbasaur") > 6 {
		t.Fatalf("dex should sit next to the name:\n%s", plain)
	}
	for _, unwanted := range []string{"Grass", "Poison", "Bug"} {
		if strings.Contains(plain, unwanted) {
			t.Fatalf("list unexpectedly contains type %q:\n%s", unwanted, plain)
		}
	}
}

func TestBrowseListVeryNarrowPrioritizesSelectedName(t *testing.T) {
	out := components.List([]components.ListRow{{
		Summary:  domain.SpeciesSummary{ID: 1, DisplayName: "Bulbasaur", Types: []string{"grass", "poison"}},
		Selected: true,
	}}, 12, 1, false, 0, nil)
	if got, want := stripANSI(out), "> Bulbasaur "; got != want {
		t.Fatalf("narrow list row = %q, want %q", got, want)
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
	for _, want := range []string{"Pokedex", "Charmander", "#004", "> Summary", "Height", "Tab section", "Esc close"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Enter/i") {
		t.Fatalf("dialog must replace the browse footer:\n%s", out)
	}
	if strings.Count(out, "Esc close") != 1 {
		t.Fatalf("dialog must render one command footer:\n%s", out)
	}
	if strings.Contains(stripANSI(out), "[ Summary ]") {
		t.Fatal("dialog must use the vertical rail, not the horizontal switcher")
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

func TestDialogInsetLeavesBrowseVisible(t *testing.T) {
	background := strings.TrimSuffix(strings.Repeat(strings.Repeat("b", 120)+"\n", 40), "\n")
	rendered := components.OverlayDialog(
		background,
		"Charmander #004",
		0,
		[]string{"Summary", "Stats", "Evolution", "Moves"},
		[]string{"Height  0.6 m"},
		0,
		120,
		40,
	)
	out := stripANSI(rendered)
	lines := strings.Split(out, "\n")
	if len(lines) < 4 || !strings.Contains(lines[2], "bbbbbbb╭") {
		t.Fatalf("expected 7-column and 2-row browse peek around card:\n%s", out)
	}
	if !strings.Contains(out, "> Summary") || !strings.Contains(out, "  Stats") {
		t.Fatalf("expected vertical section rail:\n%s", out)
	}

	const left, top, cardW, cardH = 7, 2, 106, 36
	assertBlankDialogRow(t, lines[top+1], left, cardW)
	assertBlankDialogRow(t, lines[top+cardH-2], left, cardW)
	firstContent := []rune(lines[top+2])
	split := -1
	for i := left + 1; i < left+cardW-1; i++ {
		if firstContent[i] == '│' {
			split = i
			break
		}
	}
	if split < 0 {
		t.Fatalf("dialog content row lacks rail divider: %q", lines[top+2])
	}
	if firstContent[left+1] != ' ' || firstContent[split-1] != ' ' || firstContent[split+1] != ' ' {
		t.Fatalf("dialog content row lacks explicit side/divider padding: %q", lines[top+2])
	}
	for _, line := range lines[top+2 : top+cardH-2] {
		cells := []rune(line)
		if cells[left] != '│' || cells[left+1] != ' ' || cells[left+cardW-2] != ' ' || cells[left+cardW-1] != '│' {
			t.Fatalf("dialog content row lacks one-column inner padding: %q", line)
		}
	}
}

func assertBlankDialogRow(t *testing.T, line string, left, cardW int) {
	t.Helper()
	cells := []rune(line)
	if len(cells) < left+cardW || cells[left] != '│' || cells[left+cardW-1] != '│' {
		t.Fatalf("malformed dialog padding row: %q", line)
	}
	for _, cell := range cells[left+1 : left+cardW-1] {
		if cell != ' ' {
			t.Fatalf("expected blank dialog padding row: %q", line)
		}
	}
}

func TestDialogNarrowUsesStackedPager(t *testing.T) {
	out := stripANSI(components.OverlayDialog(
		"",
		"Charmander #004",
		3,
		[]string{"Summary", "Stats", "Evolution", "Moves", "Locations", "Abilities", "Flavour", "Breeding", "Training", "Forms", "Meta"},
		[]string{"Lvl Move Type Cat Pow Acc"},
		0,
		80,
		30,
	))
	if !strings.Contains(out, "‹  4/11  Moves  ›") {
		t.Fatalf("expected stacked pager at narrow width:\n%s", out)
	}
	if strings.Contains(out, "> Moves") {
		t.Fatalf("narrow dialog must not render the rail:\n%s", out)
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
