package bootstrap

import (
	"strings"
	"testing"
)

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

func TestViewShowsJokeAndCount(t *testing.T) {
	m := New(Deps{})
	m.width, m.height = 80, 24
	m.lines = []string{"Convincing Psyduck the headache is normal"}
	m.done, m.total = 389, 1025
	m.status = "Chimchar"
	got := m.View().Content
	if !strings.Contains(got, "Poke") {
		t.Fatalf("view missing title:\n%s", got)
	}
	if !strings.Contains(got, "Convincing Psyduck the headache is normal") {
		t.Fatalf("view missing headline:\n%s", got)
	}
	if strings.Contains(got, "Downloading Fetching") {
		t.Fatalf("stage label still present:\n%s", got)
	}
	if !strings.Contains(got, "389 of 1,025") || !strings.Contains(got, "Chimchar") {
		t.Fatalf("view missing count or status:\n%s", got)
	}
}
