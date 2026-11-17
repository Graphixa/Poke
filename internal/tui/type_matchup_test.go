package tui

import (
	"strings"
	"testing"

	"github.com/Graphixa/Poke/internal/domain"
)

func TestTypeMatchupsFire(t *testing.T) {
	chart := map[string]domain.TypeInfo{
		"fire": {
			Slug:   "fire",
			Half:   []string{"fire", "grass", "ice", "bug", "steel", "fairy"},
			Double: []string{"water", "ground", "rock"},
		},
		"water":  {Slug: "water"},
		"ground": {Slug: "ground"},
		"rock":   {Slug: "rock"},
		"grass":  {Slug: "grass"},
		"normal": {Slug: "normal"},
	}
	weak, resist, immune := typeMatchups([]string{"fire"}, chart)
	joined := strings.Join(weak, ",")
	for _, want := range []string{"Water", "Ground", "Rock"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("weak missing %s: %v", want, weak)
		}
	}
	if !strings.Contains(strings.Join(resist, ","), "Grass") {
		t.Fatalf("resist missing Grass: %v", resist)
	}
	if len(immune) != 0 {
		t.Fatalf("fire should have no immunities: %v", immune)
	}
}

func TestMovesSingleMethodTable(t *testing.T) {
	pow := 40
	rows := []domain.LevelUpMove{
		{Level: 1, Method: "level-up", Move: domain.Move{DisplayName: "Ember", Type: "fire", DamageClass: "special", Power: &pow}},
		{Level: 7, Method: "level-up", Move: domain.Move{DisplayName: "Fire Fang", Type: "fire", DamageClass: "physical", Power: &pow}},
	}
	lines := formatMoveLines(rows, 80)
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"Lvl", "Ember", "Fire Fang"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("moves missing %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "Move attributes: current.") || strings.Contains(joined, "Learnset: Pearl") {
		t.Fatalf("learnset belongs on status line, not sticky:\n%s", joined)
	}
	if strings.Contains(joined, "Machine") {
		t.Fatalf("single-method table must not list other methods:\n%s", joined)
	}
	sticky := moveStickyLines(rows, 80, "level-up")
	if len(sticky) != 1 || !strings.Contains(stripANSIForFocus(sticky[0]), "Lvl") {
		t.Fatalf("sticky=%v", sticky)
	}
	hint := statusLine("Learnset: Pearl", "Row 1 of 2", 80)
	if !strings.Contains(hint, "Learnset: Pearl") || !strings.Contains(hint, "Row 1 of 2") {
		t.Fatalf("status line=%q", hint)
	}
}
