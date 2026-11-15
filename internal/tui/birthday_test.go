package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/tui"
)

// applyNoDrain updates without running Cmds — birthdayTick would loop forever in drainCmds.
func applyNoDrain(t *testing.T, m tui.Model, msgs ...tea.Msg) tui.Model {
	t.Helper()
	var model tea.Model = m
	for _, msg := range msgs {
		next, _ := model.Update(msg)
		model = next
	}
	return model.(tui.Model)
}

func TestDeclanEnterOpensBirthday(t *testing.T) {
	m := tui.NewModel(tui.Options{
		Index:         fixtureIndex(),
		Details:       &fakeDetails{},
		Prefill:       "declan",
		DisableSprite: true,
	})
	m = applyNoDrain(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})

	m = applyNoDrain(t, m, key("enter"))
	if m.BirthdayVisible() {
		t.Fatal("1st enter must not open birthday")
	}
	if m.DeclanEnterCount() != 1 {
		t.Fatalf("count = %d want 1", m.DeclanEnterCount())
	}

	m = applyNoDrain(t, m, key("enter"))
	if m.BirthdayVisible() || m.DeclanEnterCount() != 2 {
		t.Fatalf("2nd enter: visible=%v count=%d", m.BirthdayVisible(), m.DeclanEnterCount())
	}

	m = applyNoDrain(t, m, key("enter"))
	if !m.BirthdayVisible() {
		t.Fatal("3rd enter must open birthday")
	}
	if m.DeclanEnterCount() != 0 {
		t.Fatalf("count should reset after open, got %d", m.DeclanEnterCount())
	}
	view := m.View().Content
	if !strings.ContainsRune(view, '\u2588') && !strings.Contains(view, "Declan") {
		t.Fatalf("birthday view missing Declan banner; len=%d", len(view))
	}

	m = applyNoDrain(t, m, key("esc"))
	if m.BirthdayVisible() {
		t.Fatal("esc must close birthday")
	}
}

func TestDeclanEnterCountResetsOnQueryChange(t *testing.T) {
	m := tui.NewModel(tui.Options{
		Index:         fixtureIndex(),
		Details:       &fakeDetails{},
		Prefill:       "declan",
		DisableSprite: true,
	})
	m = applyNoDrain(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = applyNoDrain(t, m, key("enter"), key("enter"))
	if m.DeclanEnterCount() != 2 {
		t.Fatalf("count = %d want 2", m.DeclanEnterCount())
	}

	m = applyNoDrain(t, m, key("x")) // leaves "declan"
	if m.DeclanEnterCount() != 0 {
		t.Fatalf("count should reset when query changes, got %d", m.DeclanEnterCount())
	}
}

func trimForErr(s string) string {
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
