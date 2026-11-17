package tui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Poke/internal/domain"
	"github.com/Graphixa/Poke/internal/tui"
)

type fakeArtworkWarmer struct {
	ready map[int]bool
}

func (w *fakeArtworkWarmer) Prioritize(int) {}
func (w *fakeArtworkWarmer) SetNeighborsFromList([]domain.SpeciesSummary, int, int) {
}
func (w *fakeArtworkWarmer) Ready(id int) bool {
	if w == nil || w.ready == nil {
		return false
	}
	return w.ready[id]
}

func TestEnsureSpriteLoadUpgradesWhenArtworkReady(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{ansi: "HD"}
	w := &fakeArtworkWarmer{ready: map[int]bool{}}
	m := tui.NewModel(tui.Options{
		Index:         fixtureIndex(),
		Details:       details,
		Sprites:       sprites,
		ArtworkWarmer: w,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)

	id, _ := m.SelectedID()
	cells := m.CurrentSpriteCells()
	px := tui.SpriteMsgKindForTest(m.SpriteReqID(), m.SelectionGen(), id, cells, "PIXEL", "pixel", false)
	updated, _ = m.Update(px)
	m = updated.(tui.Model)
	if !m.HasSpriteANSI() {
		t.Fatal("expected pixel art")
	}

	before := m.SpriteReqID()
	_ = m.EnsureSelectionLoadsForTest()
	if m.SpriteReqID() != before {
		t.Fatal("pixel frame should stay until artwork Ready")
	}

	w.ready[id] = true
	_ = m.EnsureSelectionLoadsForTest()
	if m.SpriteReqID() <= before && !m.SelectedSpriteLoadingForTest() {
		t.Fatal("expected upgrade reload once artwork Ready")
	}
}
