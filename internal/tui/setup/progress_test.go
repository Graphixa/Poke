package setup

import (
	"strings"
	"testing"

	"github.com/Graphixa/Poke/internal/config"
)

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
