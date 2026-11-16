package bootstrap

import (
	"strings"
	"testing"
)

func TestDownloadLabel(t *testing.T) {
	t.Parallel()
	cases := map[Stage]string{
		StageIndex:     "Downloading Generating index",
		StageEnrich:    "Downloading Configuring types",
		StageRecords:   "Downloading Fetching records",
		StageEvolution: "Downloading Building evolutions",
		StageMoves:     "Downloading Fetching moves",
		StageDataset:   "Downloading Finalizing dataset",
		StageArtwork:   "Downloading Artwork",
		"":             "Downloading…",
	}
	for stage, want := range cases {
		if got := downloadLabel(stage); got != want {
			t.Fatalf("downloadLabel(%q)=%q want %q", stage, got, want)
		}
	}
	if got := downloadLabel("unknown"); got != "Downloading unknown" {
		t.Fatalf("unknown stage=%q", got)
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

func TestViewShowsTitleAndPhase(t *testing.T) {
	m := New(Deps{})
	m.width, m.height = 80, 24
	m.stage = StageRecords
	m.done, m.total = 389, 1025
	m.status = "Chimchar"
	got := m.View().Content
	if !strings.Contains(got, "Poke - The terminal based Pokedex") {
		t.Fatalf("view missing title:\n%s", got)
	}
	if !strings.Contains(got, "Downloading Fetching records") {
		t.Fatalf("view missing phase line:\n%s", got)
	}
	if strings.Contains(got, "index ·") || strings.Contains(got, "records ·") {
		t.Fatalf("old stage rail still present:\n%s", got)
	}
	if !strings.Contains(got, "Chimchar") {
		t.Fatalf("view missing status:\n%s", got)
	}
}
