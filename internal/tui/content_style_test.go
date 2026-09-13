package tui

import (
	"strings"
	"testing"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/tui/components"
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

	want := primaryText("Hidden") + "  Solar Power"
	if line != want {
		t.Fatalf("ability line:\n got %q\nwant %q", line, want)
	}
	requireANSI(t, primaryText("Hidden"))
	if strings.Contains(line, primaryText("Solar Power")) {
		t.Fatalf("ability name should remain plain: %q", line)
	}
}

func TestGlanceAbilityGridStableSlots(t *testing.T) {
	// Wartortle-like: slot 1 + hidden only — Slot 2 shows placeholder; Hidden is full width.
	lines := components.GlanceAbilityGrid([]domain.AbilitySlot{
		{Slot: 1, Name: "torrent"},
		{Slot: 3, Name: "rain-dish", IsHidden: true},
	}, 48)
	if len(lines) != 2 {
		t.Fatalf("want 2 rows, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "Torrent") || !strings.Contains(lines[0], "Slot 2") {
		t.Fatalf("row1 should keep Slot 1 + Slot 2 cells: %q", lines[0])
	}
	if strings.Contains(lines[0], "Rain Dish") {
		t.Fatalf("hidden must not sit on row1: %q", lines[0])
	}
	if !strings.Contains(lines[1], "Hidden") || !strings.Contains(lines[1], "Rain Dish") {
		t.Fatalf("row2 should be full-width Hidden: %q", lines[1])
	}
	if strings.Contains(lines[1], "(hidden)") {
		t.Fatalf("must not suffix (hidden) when label is Hidden: %q", lines[1])
	}
	full := components.GlanceAbilityGrid([]domain.AbilitySlot{
		{Slot: 1, Name: "poison-point"},
		{Slot: 2, Name: "rivalry"},
		{Slot: 3, Name: "sheer-force", IsHidden: true},
	}, 48)
	s1a := strings.Index(lines[0], "Slot 1")
	s1b := strings.Index(full[0], "Slot 1")
	s2a := strings.Index(lines[0], "Slot 2")
	s2b := strings.Index(full[0], "Slot 2")
	if s1a < 0 || s1a != s1b || s2a < 0 || s2a != s2b {
		t.Fatalf("slot columns shifted: empty s1=%d s2=%d full s1=%d s2=%d", s1a, s2a, s1b, s2b)
	}
	if !strings.Contains(full[1], "Sheer Force") || strings.Contains(full[1], "Slot 3") {
		t.Fatalf("row2 should be Hidden name only: %q", full[1])
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
