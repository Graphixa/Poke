package bootstrap

import (
	"strings"
	"testing"

	"github.com/Graphixa/Pokedex/internal/tui/style"
)

func TestStageLabel(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"index":     "index",
		"enrich":    "types",
		"records":   "records",
		"evolution": "evo",
		"moves":     "moves",
		"dataset":   "ready",
		"artwork":   "art",
	}
	for stage, want := range cases {
		if got := stageLabel(Stage(stage)); got != want {
			t.Fatalf("stageLabel(%q)=%q want %q", stage, got, want)
		}
	}
	if got := stageLabel(""); got != "Working" {
		t.Fatalf("empty stage=%q want Working", got)
	}
	if got := stageLabel("unknown"); got != "unknown" {
		t.Fatalf("unknown stage must pass through, got %q", got)
	}
}

func TestProgressMsgUpdatesPercent(t *testing.T) {
	m := New(Deps{})
	updated, _ := m.Update(progressMsg{stage: "records", done: 5, total: 10, message: "Bulbasaur"})
	model := updated.(Model)
	if model.done != 5 || model.total != 10 {
		t.Fatalf("done/total = %d/%d", model.done, model.total)
	}
	if model.percent != 0.5 {
		t.Fatalf("percent=%v want 0.5", model.percent)
	}
	if model.stage != StageRecords {
		t.Fatalf("stage=%q want %q", model.stage, StageRecords)
	}
	if model.status != "Bulbasaur" {
		t.Fatalf("status=%q", model.status)
	}
}

func TestStageRailHighlightsCurrent(t *testing.T) {
	t.Parallel()
	pal := style.CurrentPalette()
	rail := stageRail(StageRecords, pal)
	if !strings.Contains(rail, "records") {
		t.Fatalf("rail missing records: %q", rail)
	}
	if stageIndex(StageRecords) != 2 {
		t.Fatalf("records index=%d want 2", stageIndex(StageRecords))
	}
	if stageIndex(StageArtwork) != len(bootstrapRail) {
		t.Fatalf("artwork index=%d", stageIndex(StageArtwork))
	}
	art := stageRail(StageArtwork, pal)
	if !strings.Contains(art, "art") {
		t.Fatalf("artwork rail missing art: %q", art)
	}
}

func TestViewShowsPokedexTitle(t *testing.T) {
	m := New(Deps{})
	m.width, m.height = 80, 24
	m.stage = StageRecords
	m.done, m.total = 389, 1025
	m.status = "Chimchar"
	got := m.View().Content
	if !strings.Contains(got, "Pokedex") {
		t.Fatalf("view missing Pokedex title:\n%s", got)
	}
	if strings.Contains(got, "Building Pokédex") {
		t.Fatalf("old headline still present:\n%s", got)
	}
	if !strings.Contains(got, "Chimchar") {
		t.Fatalf("view missing status:\n%s", got)
	}
}
