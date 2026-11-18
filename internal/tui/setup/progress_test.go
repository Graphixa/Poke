package setup

import (
	"strings"
	"testing"

	"github.com/Graphixa/Poke/internal/config"
)

func TestSpinnerSitsAboveTheJoke(t *testing.T) {
	m := New(Deps{Config: config.Default(), Ask: true})
	m.width, m.height = 80, 24
	if strings.Contains(m.View().Content, "·") {
		t.Fatal("spinner is on the theme step")
	}
	m.step = stepWait
	view := m.View().Content
	jokeAt := strings.Index(view, m.headline())
	dotAt := strings.Index(view, "·")
	if jokeAt < 0 || dotAt < 0 || dotAt > jokeAt || !strings.Contains(view, "●") {
		t.Fatalf("spinner should be above the joke:\n%s", view)
	}
	next, cmd := m.Update(spinTick{})
	if cmd == nil {
		t.Fatal("spinner stopped")
	}
	if next.(Model).spin == m.spin {
		t.Fatal("frame did not advance")
	}
}

func TestStageLineReplacesTheLastSentence(t *testing.T) {
	m := New(Deps{Config: config.Default(), Ask: false})
	m.width, m.height = 80, 24
	next, _ := m.Update(progressMsg{stage: "dataset", done: 1, total: 1})
	m = next.(Model)
	next, _ = m.Update(progressMsg{stage: "form", done: 1484, total: 1579})
	view := next.(Model).View().Content
	if !strings.Contains(view, "Investigating Pokémon forms") {
		t.Fatalf("form line missing:\n%s", view)
	}
	if strings.Contains(view, "Game catalog ready") || strings.Contains(view, " of ") {
		t.Fatalf("old status still on screen:\n%s", view)
	}
}
