package bootstrap

import (
	"testing"
)

func TestStageLabel(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"index":     "Fetching species index",
		"records":   "Building Pokédex records",
		"evolution": "Fetching evolution chains",
		"moves":     "Fetching moves",
		"dataset":   "Activating dataset",
	}
	for stage, want := range cases {
		if got := stageLabel(stage); got != want {
			t.Fatalf("stageLabel(%q)=%q want %q", stage, got, want)
		}
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
	if model.stageTitle != "Building Pokédex records" {
		t.Fatalf("stageTitle=%q", model.stageTitle)
	}
}
