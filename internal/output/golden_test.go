package output_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/output"
)

var update = flag.Bool("update", false, "update golden files")

func TestGoldenSearchInfoStatsEvolution(t *testing.T) {
	t.Run("search", func(t *testing.T) {
		var buf bytes.Buffer
		err := output.SearchTable(&buf, []domain.SpeciesSummary{
			{ID: 4, DisplayName: "Charmander", Types: []string{"fire"}},
			{ID: 25, DisplayName: "Pikachu", Types: []string{"electric"}},
			{ID: 122, DisplayName: "Mr. Mime", Types: []string{"psychic", "fairy"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		assertGolden(t, "search.txt", buf.String())
	})

	t.Run("info", func(t *testing.T) {
		var buf bytes.Buffer
		p := domain.Pokemon{
			ID: 25, Slug: "pikachu", DisplayName: "Pikachu",
			HeightDecimeter: 4, WeightHectogram: 60, BaseExperience: 112,
			Types: []domain.TypeSlot{{Slot: 1, Name: "electric"}},
			Abilities: []domain.AbilitySlot{
				{Slot: 1, Name: "static"},
				{Slot: 3, Name: "lightning-rod", IsHidden: true},
			},
		}
		s := domain.Species{
			ID: 25, DisplayName: "Pikachu", Genus: "Mouse Pokémon",
			FlavorTextEntries: []domain.FlavorText{
				{Language: "en", Version: "red", VersionOrder: 1, Text: "When several of these Pokémon\ngather, their electricity could build and cause lightning storms."},
				{Language: "en", Version: "sword", VersionOrder: 40, Text: "Pikachu that can generate powerful electricity have cheek sacs that are extra soft and super stretchy."},
			},
		}
		if err := output.Info(&buf, p, s); err != nil {
			t.Fatal(err)
		}
		assertGolden(t, "info.txt", buf.String())
	})

	t.Run("stats", func(t *testing.T) {
		var buf bytes.Buffer
		p := domain.Pokemon{
			ID: 25, DisplayName: "Pikachu",
			Stats: domain.Stats{HP: 35, Attack: 55, Defense: 40, SpecialAttack: 50, SpecialDefense: 50, Speed: 90},
		}
		if err := output.Stats(&buf, p, "Pikachu"); err != nil {
			t.Fatal(err)
		}
		assertGolden(t, "stats.txt", buf.String())
	})

	t.Run("evolution", func(t *testing.T) {
		var buf bytes.Buffer
		lv16, lv36 := 16, 36
		tree := domain.EvolutionTree{
			Root: domain.EvolutionNode{
				SpeciesSlug: "charmander", DisplayName: "Charmander",
				Children: []domain.EvolutionBranch{{
					Conditions: []domain.EvolutionCondition{{Trigger: "level-up", MinimumLevel: &lv16}},
					Target: domain.EvolutionNode{
						SpeciesSlug: "charmeleon", DisplayName: "Charmeleon",
						Children: []domain.EvolutionBranch{{
							Conditions: []domain.EvolutionCondition{{Trigger: "level-up", MinimumLevel: &lv36}},
							Target:     domain.EvolutionNode{SpeciesSlug: "charizard", DisplayName: "Charizard"},
						}},
					},
				}},
			},
		}
		if err := output.Evolution(&buf, tree); err != nil {
			t.Fatal(err)
		}
		assertGolden(t, "evolution.txt", buf.String())
	})

	t.Run("evolution_branched", func(t *testing.T) {
		var buf bytes.Buffer
		tree := domain.EvolutionTree{
			Root: domain.EvolutionNode{
				SpeciesSlug: "eevee", DisplayName: "Eevee",
				Children: []domain.EvolutionBranch{
					{
						Conditions: []domain.EvolutionCondition{{Trigger: "use-item", Item: "water-stone"}},
						Target:     domain.EvolutionNode{SpeciesSlug: "vaporeon", DisplayName: "Vaporeon"},
					},
					{
						Conditions: []domain.EvolutionCondition{{Trigger: "use-item", Item: "thunder-stone"}},
						Target:     domain.EvolutionNode{SpeciesSlug: "jolteon", DisplayName: "Jolteon"},
					},
					{
						Conditions: []domain.EvolutionCondition{{Trigger: "use-item", Item: "fire-stone"}},
						Target:     domain.EvolutionNode{SpeciesSlug: "flareon", DisplayName: "Flareon"},
					},
				},
			},
		}
		if err := output.Evolution(&buf, tree); err != nil {
			t.Fatal(err)
		}
		assertGolden(t, "evolution_branched.txt", buf.String())
	})
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update)", path, err)
	}
	if got != string(want) {
		t.Fatalf("golden mismatch for %s\ngot:\n%s\nwant:\n%s", name, got, want)
	}
}
