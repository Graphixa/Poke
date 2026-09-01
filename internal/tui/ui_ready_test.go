package tui_test

import (
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/tui"
)

type deferredArtworkWarmer struct {
	started atomic.Bool
	ready   map[int]bool
}

func (w *deferredArtworkWarmer) Prioritize(int) {}
func (w *deferredArtworkWarmer) SetNeighborsFromList([]domain.SpeciesSummary, int, int) {
}
func (w *deferredArtworkWarmer) Ready(id int) bool {
	if w == nil || w.ready == nil {
		return false
	}
	return w.ready[id]
}
func (w *deferredArtworkWarmer) Start() { w.started.Store(true) }

func TestBackgroundWarmersStartAfterUIReady(t *testing.T) {
	w := &deferredArtworkWarmer{ready: map[int]bool{}}
	m := tui.NewModel(tui.Options{
		Index:         fixtureIndex(),
		Details:       &fakeDetails{},
		Sprites:       &fakeSprites{ansi: "ART"},
		ArtworkWarmer: w,
		OnUIReady:     w.Start,
	})
	if w.started.Load() {
		t.Fatal("background warmer must not start before first layout")
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	if !w.started.Load() {
		t.Fatal("background warmer must start exactly once after UI ready")
	}
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 121, Height: 52})
	m = updated.(tui.Model)
	if !w.started.Load() {
		t.Fatal("UI ready signal must remain latched")
	}
}

func TestCancellationBeforeUIReadyDoesNotStartWarmers(t *testing.T) {
	w := &deferredArtworkWarmer{ready: map[int]bool{}}
	_ = tui.NewModel(tui.Options{
		Index:         fixtureIndex(),
		Details:       &fakeDetails{},
		Sprites:       &fakeSprites{ansi: "ART"},
		ArtworkWarmer: w,
		OnUIReady:     w.Start,
	})
	if w.started.Load() {
		t.Fatal("cancel before UI ready must not start background warmers")
	}
}
