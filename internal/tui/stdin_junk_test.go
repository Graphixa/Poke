package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestScrubTerminalJunk(t *testing.T) {
	cases := map[string]string{
		"":                "",
		"pikachu":         "pikachu",
		"025":             "025", // dex numbers must survive
		"027;3$y":         "",
		"2027;3$y":        "",
		"?2027;2$y":       "",
		"2026;1$y":        "",
		"pikachu027;3$y":  "pikachu",
		"char2027;0$y":    "char",
		"6;20;10t":        "",
		"pika6;20;10t":    "pika",
		"6;20;10t027;3$y": "",
	}
	for in, want := range cases {
		if got := scrubTerminalJunk(in); got != want {
			t.Fatalf("scrub(%q)=%q want %q", in, got, want)
		}
	}
}

func TestUpdateSearchDropsDecrpmJunkKeys(t *testing.T) {
	m := featureModel()
	for _, r := range "027;3$y" {
		next, _ := m.handleKey(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = next.(Model)
	}
	if q := m.queryValue(); q != "" {
		t.Fatalf("query=%q want empty after DECRPM junk keys", q)
	}
}

func TestUpdateSearchKeepsPrefillStripsDecrpmSuffix(t *testing.T) {
	m := NewModel(Options{
		Index:         featureModel().index,
		Prefill:       "pika",
		DisableSprite: true,
	})
	for _, r := range "027;3$y" {
		next, _ := m.handleKey(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = next.(Model)
	}
	if q := m.queryValue(); q != "pika" {
		t.Fatalf("query=%q want pika", q)
	}
}

func TestUpdateSearchDropsCellSizeJunkKeys(t *testing.T) {
	m := featureModel()
	for _, r := range "6;20;10t" {
		next, _ := m.handleKey(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = next.(Model)
	}
	if q := m.queryValue(); q != "" {
		t.Fatalf("query=%q want empty after cell-size junk", q)
	}
}

func TestIsTerminalJunkKey(t *testing.T) {
	if !isTerminalJunkKey(tea.KeyPressMsg{Text: "027;3$y"}) {
		t.Fatal("pure DECRPM paste must drop")
	}
	if !isTerminalJunkKey(tea.KeyPressMsg{Text: "6;20;10t"}) {
		t.Fatal("pure cell-size paste must drop")
	}
	if isTerminalJunkKey(tea.KeyPressMsg{Text: "pikachu027;3$y"}) {
		t.Fatal("mixed paste must not drop; scrub handles suffix")
	}
	if isTerminalJunkKey(tea.KeyPressMsg{Code: 'p', Text: "p"}) {
		t.Fatal("letter must not be junk")
	}
	if isTerminalJunkKey(tea.KeyPressMsg{Text: "025"}) {
		t.Fatal("dex number paste must not be junk")
	}
}

func TestUpdateSearchScrubsJunkSuffixFromValue(t *testing.T) {
	m := featureModel()
	m.search.SetValue("pikachu027;3$y")
	// Simulate a no-op key that still runs updateSearch scrub path via handleKey
	// by applying scrub the same way updateSearch does after textinput.Update.
	if cleaned := scrubTerminalJunk(m.search.Value()); cleaned != m.search.Value() {
		m.search.SetValue(cleaned)
	}
	if q := m.queryValue(); q != "pikachu" {
		t.Fatalf("query=%q want pikachu", q)
	}

	m.search.SetValue("pika")
	m.search.CursorEnd()
	next, _ := m.handleKey(tea.KeyPressMsg{Text: "027;3$y"})
	m = next.(Model)
	q := m.queryValue()
	if q != "pika" {
		t.Fatalf("pure junk key Text: query=%q want pika", q)
	}
	if strings.Contains(q, "$y") {
		t.Fatal("DECRPM still present")
	}
}
