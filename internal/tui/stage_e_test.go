package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/Graphixa/Poke/internal/domain"
)

// Stage E regression coverage for dossier presentation paths (no live TUI).

func TestStageE_NoAlternateFormsHidesFormContext(t *testing.T) {
	m := NewModel(Options{})
	m.dialogForms = []domain.SelectableForm{{PokemonID: 1, DisplayName: "Default", IsDefault: true}}
	m.dialogGameLabel = "Pearl"
	m.dialogGameID = 12
	joined := strings.Join(m.dialogContextLines(80), "\n")
	if m.dialogHasForms() {
		t.Fatal("single form must not report HasForms")
	}
	// Form/Game/Method are top-border chips, not body context.
	for _, ban := range []string{"Form:", "Game:", "Method:"} {
		if strings.Contains(joined, ban) {
			t.Fatalf("body must hide %s: %q", ban, joined)
		}
	}
}

func TestStageE_RegionalTypingInOverview(t *testing.T) {
	alolan := domain.Pokemon{
		DisplayName: "Rattata",
		Types:       []domain.TypeSlot{{Name: "dark"}, {Name: "normal"}},
		Abilities:   []domain.AbilitySlot{{Slot: 1, Name: "gluttony"}},
	}
	s := domain.Species{Genus: "Mouse Pokémon", Generation: "generation-vii"}
	joined := strings.Join(formatOverviewLines(alolan, s, 60, nil, 0, ""), "\n")
	if !strings.Contains(joined, "Dark") || !strings.Contains(joined, "Normal") {
		t.Fatalf("regional typing missing:\n%s", joined)
	}
}

func TestStageE_BranchedEvolutionNoStageFinal(t *testing.T) {
	lv := 16
	tree := domain.EvolutionTree{Root: domain.EvolutionNode{
		DisplayName: "Eevee", SpeciesSlug: "eevee",
		Children: []domain.EvolutionBranch{
			{Conditions: []domain.EvolutionCondition{{MinimumLevel: &lv, Trigger: "level-up"}},
				Target: domain.EvolutionNode{DisplayName: "Vaporeon", SpeciesSlug: "vaporeon"}},
			{Conditions: []domain.EvolutionCondition{{Trigger: "use-item", Item: "thunder-stone"}},
				Target: domain.EvolutionNode{DisplayName: "Jolteon", SpeciesSlug: "jolteon"}},
		},
	}}
	lines := formatEvolutionLines(tree, "eevee", "Eevee")
	joined := strings.Join(lines, "\n")
	if len(lines) < 2 {
		t.Fatalf("want both branches:\n%s", joined)
	}
	if strings.Contains(joined, "Stage:") || strings.Contains(joined, "Final evolution") {
		t.Fatalf("must not invent Stage/Final:\n%s", joined)
	}
}

func TestStageE_MissingEncountersAndDescriptions(t *testing.T) {
	enc := formatEncounterLinesForGame(nil, 40, "scarlet", 80)
	if enc[0] != "No records available for this selection." {
		t.Fatalf("enc=%q", enc[0])
	}
	desc := formatFlavourLinesForGame(domain.Species{}, 40, "scarlet", "Scarlet")
	if desc[0] != "No records available for this selection." {
		t.Fatalf("desc=%q", desc[0])
	}
	noGame := formatEncounterLinesForGame(nil, 0, "", 80)
	if noGame[0] != "No wild encounters recorded." {
		t.Fatalf("All games empty=%q", noGame[0])
	}
}

func TestStageE_LearnsetDiffersByGameLabel(t *testing.T) {
	pow := 40
	sv := formatMoveLines([]domain.LevelUpMove{
		{Level: 0, Method: "machine", Move: domain.Move{DisplayName: "Tera Blast", Type: "normal", DamageClass: "special", Power: &pow}},
	}, 80)
	if !strings.HasPrefix(statusLine("Game: Red", "Row 1 of 1", 80), "Game: Red") {
		t.Fatal("red game status missing")
	}
	svJoined := strings.Join(sv, "\n")
	if !strings.HasPrefix(statusLine("Game: Scarlet", "Row 1 of 1", 80), "Game: Scarlet") || !strings.Contains(svJoined, "Tera Blast") {
		t.Fatalf("scarlet learnset missing:\n%s", svJoined)
	}
}

func TestStageE_RapidFormAndGameClearsMoveDetail(t *testing.T) {
	cat := stubCatalog{
		forms: []domain.SelectableForm{
			{Key: "pokemon:25", PokemonID: 25, DisplayName: "Default", IsDefault: true},
			{Key: "pokemon:10080", PokemonID: 10080, DisplayName: "Partner"},
		},
		pokemon: map[int]domain.Pokemon{25: {ID: 25}, 10080: {ID: 10080}},
		versions: []domain.Version{
			{ID: 1, Slug: "red", DisplayName: "Red", VersionGroupID: 1, Order: 1},
			{ID: 40, Slug: "scarlet", DisplayName: "Scarlet", VersionGroupID: 25, Order: 40},
		},
		vg: map[int]int{1: 1, 40: 25},
	}
	m := NewModel(Options{Catalog: cat, Context: context.Background()})
	m.catalog = cat
	m.dialogSpeciesID = 25
	m.dialogForms = cat.forms
	m.dialogFormIdx = 0
	m.dialogSection = DialogMoves
	m.moveDetailOpen = true
	m.moveFocusIdx = 3

	m.selectDialogForm(1)
	if m.moveDetailOpen {
		t.Fatal("form change must exit move detail")
	}
	if m.dialogPokemon.ID != 10080 {
		t.Fatalf("form pokemon=%d", m.dialogPokemon.ID)
	}

	m.moveDetailOpen = true
	m.dialogGameID = 1
	m.dialogGameLabel = "Red"
	m.openGamePicker()
	if m.moveDetailOpen {
		t.Fatal("opening game picker should clear move detail")
	}
}

func TestStageE_OverviewBlurbFollowsGame(t *testing.T) {
	s := domain.Species{
		Genus: "Seed Pokémon",
		FlavorTextEntries: []domain.FlavorText{
			{Language: "en", Version: "red", VersionOrder: 1, Text: "Old entry."},
			{Language: "en", Version: "scarlet", VersionOrder: 99, Text: "Newest reference."},
		},
	}
	p := domain.Pokemon{Types: []domain.TypeSlot{{Name: "grass"}}}
	newest := strings.Join(formatOverviewLines(p, s, 60, nil, 0, ""), "\n")
	if !strings.Contains(newest, "Newest reference.") {
		t.Fatalf("no game selected uses newest entry:\n%s", newest)
	}
	red := strings.Join(formatOverviewLines(p, s, 60, nil, 0, "red"), "\n")
	if !strings.Contains(red, "Old entry.") || strings.Contains(red, "Newest reference.") {
		t.Fatalf("selected game should drive overview blurb:\n%s", red)
	}
}
