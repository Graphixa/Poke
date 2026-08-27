package tui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/tui"
	"github.com/Graphixa/Pokedex/internal/tui/views"
)

func TestNativeOverlayDebounceDoesNotStarve(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{ansi: "ART"}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)

	id, ok := m.SelectedID()
	if !ok {
		t.Fatal("expected selection")
	}
	cells := m.CurrentSpriteCells()
	msg := tui.SpriteMsgNativeForTest(tui.SpriteMsgForTest(
		m.SpriteReqID(), m.SelectionGen(), id, cells,
		"\x1bP0;0;8q#1~~~\x1b\\", false,
	))
	updated, cmd := m.Update(msg)
	m = updated.(tui.Model)
	if cmd == nil {
		t.Fatal("expected overlay schedule cmd after native sprite load")
	}
	if !m.NativePaintPendingForTest() {
		t.Fatal("expected nativePaintPending after schedule")
	}

	// Flood updates (warm progress style). Overlay must stay pending through follow-up window.
	for i := 0; i < 20; i++ {
		updated, _ = m.Update(tui.WarmProgressMsgForTest(i, 100))
		m = updated.(tui.Model)
	}
	if !m.NativePaintPendingForTest() {
		t.Fatal("overlay pending flag cleared by update flood — paint would starve")
	}
}

func TestNativeOverlayClearsOnSelectionWithoutArt(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{ansi: "ART"}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	id, _ := m.SelectedID()
	cells := m.CurrentSpriteCells()
	msg := tui.SpriteMsgNativeForTest(tui.SpriteMsgForTest(
		m.SpriteReqID(), m.SelectionGen(), id, cells,
		"\x1bP0;0;8q#1~~~\x1b\\", false,
	))
	updated, _ = m.Update(msg)
	m = updated.(tui.Model)
	// Complete follow-up so paint state is committed.
	updated, _ = m.Update(tui.NativeOverlayFollowUpForTest())
	m = updated.(tui.Model)
	if m.NativePaintANSIForTest() == "" {
		t.Fatal("expected painted native ANSI tracked")
	}

	updated, cmd := m.Update(key("down"))
	m = updated.(tui.Model)
	if m.NativePaintANSIForTest() != "" {
		t.Fatal("selection change without art must clear tracked native paint")
	}
	if cmd == nil {
		t.Fatal("expected clear/load cmds on selection change")
	}
}

func TestNativeOverlayNotPerpetualLoop(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{ansi: "ART"}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	id, _ := m.SelectedID()
	cells := m.CurrentSpriteCells()
	msg := tui.SpriteMsgNativeForTest(tui.SpriteMsgForTest(
		m.SpriteReqID(), m.SelectionGen(), id, cells,
		"\x1bP0;0;8q#1~~~\x1b\\", false,
	))
	updated, _ = m.Update(msg)
	m = updated.(tui.Model)
	if !m.NativePaintPendingForTest() {
		t.Fatal("expected pending after dirty paint")
	}
	// Follow-up re-paint clears pending — no perpetual 100ms loop.
	updated, cmd := m.Update(tui.NativeOverlayFollowUpForTest())
	m = updated.(tui.Model)
	if m.NativePaintPendingForTest() {
		t.Fatal("follow-up should clear nativePaintPending")
	}
	_ = cmd
}

func TestSpriteArtOriginTwoPane(t *testing.T) {
	d := views.BrowseData{
		Width: 120, Height: 52,
		ShowSprite: true, SpriteNative: true,
		LeftWidth: 40, RightWidth: 79,
		BodySize:    40,
		HasSelected: true,
		Selected:    domain.SpeciesSummary{ID: 25, DisplayName: "Pikachu"},
	}
	row, col, artW, artH, ok := views.SpriteArtOrigin(d)
	if !ok {
		t.Fatal("expected origin")
	}
	if row < 2 || col < 2 {
		t.Fatalf("origin row=%d col=%d looks invalid", row, col)
	}
	if artW < 1 || artH < 1 {
		t.Fatalf("art size %dx%d", artW, artH)
	}
	t.Logf("origin row=%d col=%d %dx%d", row, col, artW, artH)
}

func TestClearCellRectCoversBox(t *testing.T) {
	seq := tui.ClearCellRectForTest(5, 10, 3, 2)
	if seq == "" {
		t.Fatal("expected clear sequence")
	}
	// Two CUP+spaces rows at (5,10) and (6,10).
	if got := countSubstr(seq, "\x1b[5;10H"); got != 1 {
		t.Fatalf("row5 cups=%d", got)
	}
	if got := countSubstr(seq, "\x1b[6;10H"); got != 1 {
		t.Fatalf("row6 cups=%d", got)
	}
	if !containsSpaces(seq, 3) {
		t.Fatal("expected space run of width 3")
	}
}

func TestWarmHintHiddenWhileNativeSpriteVisible(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{ansi: "ART"}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)

	id, _ := m.SelectedID()
	cells := m.CurrentSpriteCells()
	msg := tui.SpriteMsgNativeForTest(tui.SpriteMsgForTest(
		m.SpriteReqID(), m.SelectionGen(), id, cells,
		"\x1bP0;0;8q#1~~~\x1b\\", false,
	))
	updated, _ = m.Update(msg)
	m = updated.(tui.Model)

	updated, _ = m.Update(tui.WarmProgressMsgForTest(50, 100))
	m = updated.(tui.Model)
	if m.BrowseWarmHintForTest() != "" {
		t.Fatalf("warm hint must be omitted from view while native sprite shows, got %q", m.BrowseWarmHintForTest())
	}
}

func TestDialogCloseRepaintsNativeOverlay(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{ansi: "ART"}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)

	id, _ := m.SelectedID()
	cells := m.CurrentSpriteCells()
	msg := tui.SpriteMsgNativeForTest(tui.SpriteMsgForTest(
		m.SpriteReqID(), m.SelectionGen(), id, cells,
		"\x1bP0;0;8q#1~~~\x1b\\", false,
	))
	updated, _ = m.Update(msg)
	m = updated.(tui.Model)

	updated, _ = m.Update(key("enter"))
	m = updated.(tui.Model)
	if !m.DialogVisible() {
		t.Fatal("expected dialog")
	}
	updated, _ = m.Update(key("esc"))
	m = updated.(tui.Model)
	if m.DialogVisible() {
		t.Fatal("expected dialog closed")
	}
	if !m.NativePaintPendingForTest() && !m.NativeOverlayDirtyForTest() {
		t.Fatal("expected native overlay dirty/pending after dialog close")
	}
}

func countSubstr(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}

func containsSpaces(s string, n int) bool {
	run := make([]byte, n)
	for i := range run {
		run[i] = ' '
	}
	return countSubstr(s, string(run)) > 0
}
