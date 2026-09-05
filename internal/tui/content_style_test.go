package tui

import (
	"strings"
	"testing"

	"github.com/Graphixa/Pokedex/internal/domain"
)

func TestEvolutionStylesNamesOnly(t *testing.T) {
	level := 16
	tree := domain.EvolutionTree{Root: domain.EvolutionNode{
		DisplayName: "Bulbasaur",
		Children: []domain.EvolutionBranch{{
			Conditions: []domain.EvolutionCondition{{MinimumLevel: &level}},
			Target:     domain.EvolutionNode{DisplayName: "Ivysaur"},
		}},
	}}

	line := formatEvolutionLines(tree)[0]
	want := primaryText("Bulbasaur") + " ── Level 16 ──► " + primaryText("Ivysaur")
	if line != want {
		t.Fatalf("evolution line:\n got %q\nwant %q", line, want)
	}
	requireANSI(t, primaryText("Bulbasaur"))
	if strings.Contains(line, primaryText("Level 16")) {
		t.Fatalf("condition should remain plain: %q", line)
	}
}

func TestMoveHeaderIsStyledButRowsArePlain(t *testing.T) {
	power, accuracy := 40, 100
	lines := formatMoveLines([]LevelUpMove{{
		Level: 1,
		Move: domain.Move{
			DisplayName: "Ember",
			Type:        "fire",
			DamageClass: "special",
			Power:       &power,
			Accuracy:    &accuracy,
		},
	}})

	header := "Lvl   Move             Type       Cat        Pow   Acc"
	if lines[0] != primaryText(header) {
		t.Fatalf("header not styled with primary: %q", lines[0])
	}
	requireANSI(t, lines[0])
	if strings.Contains(lines[1], "\x1b[") {
		t.Fatalf("move row should remain plain: %q", lines[1])
	}
}

func TestAbilityStylesSlotOnly(t *testing.T) {
	line := formatAbilityDialogLines(domain.Pokemon{Abilities: []domain.AbilitySlot{{
		Slot:     3,
		Name:     "solar-power",
		IsHidden: true,
	}}})[0]

	want := primaryText("Slot 3") + "  Solar Power  (hidden)"
	if line != want {
		t.Fatalf("ability line:\n got %q\nwant %q", line, want)
	}
	requireANSI(t, primaryText("Slot 3"))
	if strings.Contains(line, primaryText("Solar Power")) {
		t.Fatalf("ability name should remain plain: %q", line)
	}
}

func TestFlavourStylesVersionOnly(t *testing.T) {
	lines := formatFlavourLines(domain.Species{FlavorTextEntries: []domain.FlavorText{{
		Language:     "en",
		Version:      "lets-go-eevee",
		VersionOrder: 1,
		Text:         "Tough fights could excite this Pokémon.",
	}}})

	if lines[0] != primaryText("Lets Go Eevee") {
		t.Fatalf("version heading not styled: %q", lines[0])
	}
	requireANSI(t, lines[0])
	if strings.Contains(lines[1], "\x1b[") {
		t.Fatalf("flavour paragraph should remain plain: %q", lines[1])
	}
}

func TestLocationsStyleVersionOnly(t *testing.T) {
	lines := formatEncounterLines([]domain.Encounter{{
		Version:      "omega-ruby",
		Area:         "route-101",
		Method:       "walk",
		MinimumLevel: 2,
		MaximumLevel: 3,
		Chance:       20,
	}})

	if lines[0] != primaryText("Omega Ruby") {
		t.Fatalf("location version heading not styled: %q", lines[0])
	}
	if strings.Contains(lines[1], "\x1b[") {
		t.Fatalf("location data should remain plain: %q", lines[1])
	}
}

func TestStatsStyleLabelsOnly(t *testing.T) {
	lines := formatStatsBars(domain.Pokemon{Stats: domain.Stats{HP: 58}})

	if !strings.HasPrefix(lines[0], primaryText("HP      ")) {
		t.Fatalf("stat label not styled: %q", lines[0])
	}
	if strings.Contains(strings.TrimPrefix(lines[0], primaryText("HP      ")), "\x1b[") {
		t.Fatalf("stat bar and value should remain plain: %q", lines[0])
	}
}

func requireANSI(t *testing.T, s string) {
	t.Helper()
	if !strings.Contains(s, "\x1b[") {
		t.Fatalf("expected ANSI styling in %q", s)
	}
}
