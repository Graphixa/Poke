package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Poke/internal/domain"
	"github.com/Graphixa/Poke/internal/render/sprite"
	"github.com/Graphixa/Poke/internal/tui"
	"github.com/Graphixa/Poke/internal/tui/components"
	"github.com/Graphixa/Poke/internal/tui/views"
)

func TestLayoutBreakpoints(t *testing.T) {
	// Browse glance locks to bucket 16. Art cardH=12 → body needs ≥12 (term H − 5).
	cases := []struct {
		w, h       int
		sprite     bool
		verbose    bool
		onePane    bool
		tooSmall   bool
		spriteSize int
	}{
		{120, 52, true, true, false, false, 16},
		{120, 34, true, true, false, false, 16},
		{120, 21, true, false, false, false, 16},
		{120, 18, true, false, false, false, 16},
		{120, 17, false, false, true, false, 0},
		{88, 40, true, true, false, false, 16},
		{71, 40, true, false, false, false, 16},
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

func TestPinnedBucketStableWhenTaller(t *testing.T) {
	m := tui.NewModel(tui.Options{Index: fixtureIndex(), Details: &fakeDetails{}})
	m = apply(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	f1 := m.LayoutFlags()
	if f1.SpriteSize != 16 {
		t.Fatalf("SpriteSize=%d want 16", f1.SpriteSize)
	}
	m = apply(t, m, tea.WindowSizeMsg{Width: 120, Height: 80})
	f2 := m.LayoutFlags()
	if f2.SpriteSize != 16 {
		t.Fatalf("taller SpriteSize=%d want 16", f2.SpriteSize)
	}
	if f1.RightWidth != f2.RightWidth {
		t.Fatalf("right pane should stay fixed: %d vs %d", f1.RightWidth, f2.RightWidth)
	}
}

func TestListGrowsHorizontally(t *testing.T) {
	m := tui.NewModel(tui.Options{Index: fixtureIndex(), Details: &fakeDetails{}})
	m = apply(t, m, tea.WindowSizeMsg{Width: 100, Height: 40})
	f1 := m.LayoutFlags()
	m = apply(t, m, tea.WindowSizeMsg{Width: 140, Height: 40})
	f2 := m.LayoutFlags()
	if f1.RightWidth != f2.RightWidth {
		t.Fatalf("right pane must stay fixed: %d vs %d", f1.RightWidth, f2.RightWidth)
	}
	if f2.LeftWidth <= f1.LeftWidth {
		t.Fatalf("list should grow: left %d → %d", f1.LeftWidth, f2.LeftWidth)
	}
	wantRight := views.GlanceRightInnerWidth() + 1
	if f2.RightWidth != wantRight {
		t.Fatalf("RightWidth=%d want GlanceRightPanelWidth=%d", f2.RightWidth, wantRight)
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
	// List absorbs leftover; with fixed glance right pane, list is usually the wider column.
	if f.RightWidth != views.GlanceRightInnerWidth()+1 {
		t.Fatalf("RightWidth=%d want fixed glance %d", f.RightWidth, views.GlanceRightInnerWidth()+1)
	}

}

func TestSpriteShowsWhenBodyFits(t *testing.T) {
	m := tui.NewModel(tui.Options{Index: fixtureIndex(), Details: &fakeDetails{}})
	m = apply(t, m, tea.WindowSizeMsg{Width: 120, Height: 18})
	f := m.LayoutFlags()
	if !f.ShowSprite || f.SpriteSize < sprite.SmallestFrameBucket() {
		t.Fatalf("expected sprite at 120x18, got show=%v size=%d", f.ShowSprite, f.SpriteSize)
	}
	m = apply(t, m, tea.WindowSizeMsg{Width: 120, Height: 17})
	f = m.LayoutFlags()
	if f.ShowSprite {
		t.Fatal("expected sprite hidden in one-pane height")
	}
}

func TestBrowseSpritePadAndOrigin(t *testing.T) {
	rw := views.GlanceRightInnerWidth() + 1
	d := views.BrowseData{
		Width: 120, Height: 52,
		HasSelected: true,
		Selected:    domain.SpeciesSummary{ID: 4, DisplayName: "Charmander", Types: []string{"fire"}},
		ShowSprite:  true, SpriteNative: true,
		BrowseView: 0,
		BodySize:   views.GlanceBucket,
		LeftWidth:  120 - rw - 1, RightWidth: rw,
		HeaderTypes: "Fire",
	}
	row, col, artW, artH, ok := views.SpriteArtOrigin(d)
	if !ok {
		t.Fatal("expected origin")
	}
	if artW != 16 || artH != 8 {
		t.Fatalf("art %dx%d want 16x8", artW, artH)
	}
	// Pinned art card: frame top + header + card top border (pad=0).
	wantRow := 2 + 2 + 1 // 5
	wantCol := d.LeftWidth + 2 + 1
	if row != wantRow {
		t.Fatalf("art row=%d want %d (pinned under header)", row, wantRow)
	}
	if col != wantCol {
		t.Fatalf("art col=%d want %d", col, wantCol)
	}

	d.Height = 80
	row2, col2, artW2, artH2, ok2 := views.SpriteArtOrigin(d)
	if !ok2 || row2 != row || col2 != col || artW2 != artW || artH2 != artH {
		t.Fatalf("taller window moved art: %d,%d %dx%d vs %d,%d %dx%d",
			row, col, artW, artH, row2, col2, artW2, artH2)
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
		ViewLabels: []string{"Glance", "Art"},
		BodySize:   views.GlanceBucket,
		LeftWidth:  120 - (views.GlanceRightInnerWidth() + 1) - 1, RightWidth: views.GlanceRightInnerWidth() + 1,
		HeaderTypes:    "Fire",
		HeaderGenus:    "Lizard Pokémon",
		HasBrowseStats: true,
		Stats:          domain.Stats{HP: 39, Attack: 52, Defense: 43, SpecialAttack: 60, SpecialDefense: 50, Speed: 65},
		SpriteANSI:     "[sprite]",
		ImageState:     2,
		FooterHint:     "↑/↓ move • enter details • tab view • esc clear / quit",
	})
	for _, want := range []string{
		"Pokedex",
		"> c",
		"Charmander",
		"#004",
		"[sprite]",
		"Stats",
		"enter details",
		"tab view",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "[ Sprite ]") {
		t.Fatal("browse must not show text-view switcher")
	}
}

func TestBrowseListRowsNoTypes(t *testing.T) {
	rows := []components.ListRow{
		{Summary: domain.SpeciesSummary{ID: 1, DisplayName: "Bulbasaur", Types: []string{"grass", "poison"}}, Selected: true},
		{Summary: domain.SpeciesSummary{ID: 10, DisplayName: "Caterpie", Types: []string{"bug"}}, Comparing: true},
	}
	out := views.Browse(views.BrowseData{
		Width: 44, Height: 5,
		OnePane: true, ShowListPane: true,
		Rows: rows,
	})
	plain := stripANSI(out)
	for _, want := range []string{"Bulbasaur", "#001", "Caterpie ⇄", "#010"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("missing %q in list:\n%s", want, plain)
		}
	}
	assertDexRightAligned(t, strings.Split(plain, "\n")[1], "#001")
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
	}}, 12, 1, 0)
	if got, want := stripANSI(out), ">   Bulbasa…"; got != want {
		t.Fatalf("narrow list row = %q, want %q", got, want)
	}
}

func TestDialogOverlaySnapshot(t *testing.T) {
	labels := []string{"Overview", "Stats", "Evolution", "Moves", "Encounters", "Descriptions", "Biology"}
	out := views.Browse(views.BrowseData{
		Width: 120, Height: 40,
		HasSelected: true,
		Selected:    domain.SpeciesSummary{ID: 4, DisplayName: "Charmander"},
		ShowSprite:  true, VerboseFooter: true,
		BrowseView: 0,
		ViewLabels: []string{"Glance", "Art"},
		BodySize:   sprite.MaxFrameSize,
		LeftWidth:  70, RightWidth: 49,
		ShowDialog:    true,
		DialogSection: 0,
		DialogLabels:  labels,
		DialogLines:   []string{"Fire · Lizard Pokémon", "Height    0.6 m"},
		FooterHint:    "←/→ section • ↑/↓ scroll • esc close",
	})
	plain := stripANSI(out)
	for _, want := range []string{"Charmander", "#004", "Overview", "Height", "←/→ section", "esc close"} {
		if !strings.Contains(out, want) && !strings.Contains(plain, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(plain, "Pokedex\n") && strings.Contains(plain, "> Bulbasaur") {
		t.Fatalf("full-screen dossier must not show browse list underneath:\n%s", plain)
	}
	if strings.Count(plain, "esc close") != 1 {
		t.Fatalf("dialog must render one command footer:\n%s", out)
	}
	if strings.Contains(plain, "[ Overview ]") || strings.Contains(plain, "● Overview") {
		t.Fatal("dialog must use padded chips, not brackets/rail")
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

func TestDossierFrameFillsTerminal(t *testing.T) {
	labels := []string{"Overview", "Stats", "Evolution", "Moves", "Encounters", "Descriptions", "Biology"}
	out := stripANSI(components.DossierFrame(
		"Charmander #004",
		0,
		labels,
		[]string{"Height  0.6 m"},
		0,
		120,
		40,
		" ←/→ section • esc close",
	))
	lines := strings.Split(out, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected card + footer:\n%s", out)
	}
	if !strings.HasPrefix(lines[0], "╭─") || !strings.Contains(lines[0], "Charmander") {
		t.Fatalf("expected full-frame title border:\n%s", lines[0])
	}
	if !strings.Contains(out, "Overview") || strings.Contains(out, "[ Overview ]") {
		t.Fatalf("expected padded chip header (no brackets):\n%s", out)
	}
	// No dimmed browse peek: first content row should start at the card edge.
	if strings.Contains(lines[0], "bbbb") {
		t.Fatalf("dossier must not composite over browse:\n%s", out)
	}
	footer := lines[len(lines)-1]
	if !strings.Contains(footer, "esc close") {
		t.Fatalf("footer should be on dossier frame: %q", footer)
	}
}

func TestDossierBodySharesSectionInset(t *testing.T) {
	labels := []string{"Overview", "Stats", "Evolution", "Moves", "Encounters", "Descriptions", "Biology"}
	out := stripANSI(components.DossierFrame(
		"Venusaur #003",
		3,
		labels,
		[]string{"Move List"},
		0,
		120,
		40,
		"",
	))
	var chipLine, bodyLine string
	for _, line := range strings.Split(out, "\n") {
		if chipLine == "" && strings.Contains(line, "Overview") && strings.Contains(line, "Moves") {
			chipLine = line
		}
		if bodyLine == "" && strings.Contains(line, "Move List") {
			bodyLine = line
		}
	}
	if chipLine == "" || bodyLine == "" {
		t.Fatalf("missing chip/body lines:\n%s", out)
	}
	chipCol := strings.Index(chipLine, "Overview")
	bodyCol := strings.Index(bodyLine, "Move List")
	// Body shares the chip *label* column (inside chip H-pad), not the chip box edge.
	if bodyCol < 3 {
		t.Fatalf("body should use chip-label inset, Move List at col %d in %q", bodyCol, bodyLine)
	}
	if chipCol != bodyCol {
		t.Fatalf("body should line up under chip labels: Overview@%d Move List@%d\nchip: %q\nbody: %q", chipCol, bodyCol, chipLine, bodyLine)
	}
}

func TestDialogNarrowUsesCompactPager(t *testing.T) {
	labels := []string{"Overview", "Stats", "Evolution", "Moves", "Encounters", "Descriptions", "Biology"}
	out := stripANSI(components.DossierFrame(
		"Charmander #004",
		3,
		labels,
		[]string{"Lvl Move Type Cat Pow Acc PP"},
		0,
		70,
		30,
		"",
	))
	if !strings.Contains(out, "‹  4/7  Moves  ›") {
		t.Fatalf("expected compact pager when full switcher does not fit:\n%s", out)
	}
	if strings.Contains(out, "● Overview") {
		t.Fatalf("must not render the vertical rail:\n%s", out)
	}
}

func TestSwitcherChipStableWidth(t *testing.T) {
	labels := []string{"Overview", "Stats", "Moves"}
	a := components.ViewSwitcher(0, labels, 0)
	b := components.ViewSwitcher(1, labels, 0)
	if wa, wb := lipgloss.Width(a), lipgloss.Width(b); wa != wb {
		t.Fatalf("selection jumped width: active0=%d %q active1=%d %q", wa, stripANSI(a), wb, stripANSI(b))
	}
	if strings.Contains(stripANSI(a), "[") {
		t.Fatalf("brackets must not appear: %q", stripANSI(a))
	}
}

func assertDexRightAligned(t *testing.T, row, dex string) {
	t.Helper()
	trimmed := strings.TrimRight(row, " ")
	if !strings.HasSuffix(trimmed, dex) {
		t.Fatalf("id %q should be right-aligned in %q", dex, row)
	}
	if pad := len(row) - len(trimmed); pad != 1 {
		t.Fatalf("want 1 column of padding after %s, got %d in %q", dex, pad, row)
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
