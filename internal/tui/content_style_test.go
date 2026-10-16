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
		SpeciesSlug: "bulbasaur",
		Children: []domain.EvolutionBranch{{
			Conditions: []domain.EvolutionCondition{{MinimumLevel: &level}},
			Target:     domain.EvolutionNode{DisplayName: "Ivysaur", SpeciesSlug: "ivysaur"},
		}},
	}}

	line := formatEvolutionLines(tree, "", "")[0]
	want := primaryText("Bulbasaur") + " ── Level 16 ──► " + primaryText("Ivysaur")
	if line != want {
		t.Fatalf("evolution line:\n got %q\nwant %q", line, want)
	}
	requireANSI(t, primaryText("Bulbasaur"))
	if strings.Contains(line, primaryText("Level 16")) {
		t.Fatalf("condition should remain plain: %q", line)
	}
}

func TestEvolutionBoldsCurrentSpecies(t *testing.T) {
	level := 16
	tree := domain.EvolutionTree{Root: domain.EvolutionNode{
		DisplayName: "Charmander",
		SpeciesSlug: "charmander",
		Children: []domain.EvolutionBranch{{
			Conditions: []domain.EvolutionCondition{{MinimumLevel: &level}},
			Target:     domain.EvolutionNode{DisplayName: "Charmeleon", SpeciesSlug: "charmeleon"},
		}},
	}}
	line := formatEvolutionLines(tree, "charmander", "Charmander")[0]
	if !strings.Contains(line, boldPrimary("Charmander")) {
		t.Fatalf("current species should be bold: %q", line)
	}
	if strings.Contains(line, boldPrimary("Charmeleon")) {
		t.Fatalf("other species should not be bold: %q", line)
	}
	if strings.Contains(formatEvolutionLines(tree, "charmander", "Charmander")[0], "Trigger source") {
		t.Fatal("evolution must omit Trigger source")
	}
}

func TestMoveHeaderIncludesPP(t *testing.T) {
	power, accuracy := 40, 100
	lines := formatMoveLines([]domain.LevelUpMove{{
		Level: 1,
		Move: domain.Move{
			DisplayName: "Ember",
			Type:        "fire",
			DamageClass: "special",
			Power:       &power,
			Accuracy:    &accuracy,
			PP:          25,
		},
	}}, 80)

	wantHeader := primaryText("Lvl   Move             Type       Cat        Pow   Acc   PP")
	if lines[0] != wantHeader {
		t.Fatalf("header not styled with primary:\n got %q\nwant %q", lines[0], wantHeader)
	}
	requireANSI(t, lines[0])
	if strings.Contains(lines[1], "\x1b[") {
		t.Fatalf("move row should remain plain: %q", lines[1])
	}
	if !strings.Contains(lines[1], "25") {
		t.Fatalf("move row should include PP: %q", lines[1])
	}
}

func TestMoveColumnsDropAccThenPPWhenNarrow(t *testing.T) {
	power, accuracy := 40, 100
	row := []domain.LevelUpMove{{
		Level: 1,
		Move: domain.Move{
			DisplayName: "Ember",
			Type:        "fire",
			DamageClass: "special",
			Power:       &power,
			Accuracy:    &accuracy,
			PP:          25,
		},
	}}
	mid := formatMoveLines(row, 55)
	if strings.Contains(mid[0], "Acc") || !strings.Contains(mid[0], "PP") {
		t.Fatalf("mid width should drop Acc keep PP: %q", mid[0])
	}
	narrow := formatMoveLines(row, 40)
	if strings.Contains(narrow[0], "Acc") || strings.Contains(narrow[0], "PP") {
		t.Fatalf("narrow width should drop Acc and PP: %q", narrow[0])
	}
}

func TestOverviewAndBiologyLabels(t *testing.T) {
	p := domain.Pokemon{
		DisplayName:     "Charmander",
		HeightDecimeter: 6,
		WeightHectogram: 85,
		BaseExperience:  62,
		Types:           []domain.TypeSlot{{Name: "fire"}},
		Abilities: []domain.AbilitySlot{
			{Slot: 1, Name: "blaze"},
			{Slot: 3, Name: "solar-power", IsHidden: true},
		},
	}
	s := domain.Species{
		Genus:         "Lizard Pokémon",
		Generation:    "generation-iv",
		CaptureRate:   45,
		GenderRate:    1,
		GrowthRate:    "medium-slow",
		BaseHappiness: 50,
		Habitat:       "mountain",
		Shape:         "upright",
		Color:         "red",
		Varieties:     []domain.Variety{{Slug: "charmander", IsDefault: true}},
		FlavorTextEntries: []domain.FlavorText{{
			Language: "en", Version: "red", VersionOrder: 1,
			Text: "Prefers hot places.",
		}},
	}
	overview := formatOverviewLines(p, s, 60)
	joined := strings.Join(overview, "\n")
	if !strings.HasPrefix(overview[0], "Prefers hot places.") {
		t.Fatalf("blurb should lead Overview:\n%s", joined)
	}
	if !strings.Contains(joined, primaryText("Type      ")) || !strings.Contains(joined, "Lizard Pokémon") {
		t.Fatalf("overview missing Type/Category:\n%s", joined)
	}
	if strings.Contains(joined, "Blaze") || strings.Contains(joined, "Ability") {
		t.Fatalf("overview must not list abilities:\n%s", joined)
	}
	if !strings.Contains(joined, primaryText("Generation")) || !strings.Contains(joined, "IV") {
		t.Fatalf("overview Generation should be labelled Roman IV:\n%s", joined)
	}
	if strings.Contains(joined, "Iv") && !strings.Contains(joined, "IV") {
		t.Fatalf("generation must not title-case to Iv:\n%s", joined)
	}

	bio := formatBiologyLines(s)
	bioJoined := strings.Join(bio, "\n")
	for _, want := range []string{"Gender", "Growth", "Habitat", "Forms", "(default)"} {
		if !strings.Contains(bioJoined, want) {
			t.Fatalf("biology missing %q:\n%s", want, bioJoined)
		}
	}
	if strings.Contains(bioJoined, "Egg") {
		t.Fatalf("biology must omit egg groups until domain has them:\n%s", bioJoined)
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
		Conditions:   []string{"time-morning"},
	}})

	if lines[0] != primaryText("Omega Ruby") {
		t.Fatalf("location version heading not styled: %q", lines[0])
	}
	if strings.Contains(lines[1], "\x1b[") {
		t.Fatalf("location data should remain plain: %q", lines[1])
	}
	if len(lines) < 3 || !strings.Contains(lines[2], "Morning") {
		t.Fatalf("conditions should indent on next line: %v", lines)
	}
}

func TestStatsStyleLabelsOnly(t *testing.T) {
	lines := formatStatsBars(domain.Pokemon{Stats: domain.Stats{HP: 58, Attack: 64, Defense: 58, SpecialAttack: 80, SpecialDefense: 65, Speed: 90}})

	if !strings.HasPrefix(lines[0], primaryText("HP      ")) {
		t.Fatalf("stat label not styled: %q", lines[0])
	}
	if strings.Contains(strings.TrimPrefix(lines[0], primaryText("HP      ")), "\x1b[") {
		t.Fatalf("stat bar and value should remain plain: %q", lines[0])
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "Total") || !strings.Contains(joined, "415") {
		t.Fatalf("stats should include Total sum:\n%s", joined)
	}
}

func requireANSI(t *testing.T, s string) {
	t.Helper()
	if !strings.Contains(s, "\x1b[") {
		t.Fatalf("expected ANSI styling in %q", s)
	}
}
