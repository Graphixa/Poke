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

	// Flood updates (warm progress style). Overlay must stay pending.
	for i := 0; i < 20; i++ {
		updated, _ = m.Update(tui.WarmProgressMsgForTest(i, 100))
		m = updated.(tui.Model)
	}
	if !m.NativePaintPendingForTest() {
		t.Fatal("overlay pending flag cleared by update flood — paint would starve")
	}
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
	row, col, ok := views.SpriteArtOrigin(d)
	if !ok {
		t.Fatal("expected origin")
	}
	if row < 2 || col < 2 {
		t.Fatalf("origin row=%d col=%d looks invalid", row, col)
	}
	t.Logf("origin row=%d col=%d", row, col)
}
