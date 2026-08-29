package dataset_test

import (
	"testing"

	"github.com/Graphixa/Pokedex/internal/dataset"
	"github.com/Graphixa/Pokedex/internal/domain"
)

func TestSpeciesFromDomainTrimsMultilingualPayload(t *testing.T) {
	src := domain.Species{
		ID:          25,
		Slug:        "pikachu",
		DisplayName: "Pikachu",
		Genus:       "Mouse Pokémon",
		Names: []domain.LocalizedName{
			{Language: "ja", Value: "ピカチュウ"},
			{Language: "en", Value: "Pikachu"},
		},
		Genera: []domain.LocalizedText{
			{Language: "ja", Value: "ねずみポケモン"},
			{Language: "en", Value: "Mouse Pokémon"},
		},
		FlavorTextEntries: []domain.FlavorText{
			{Language: "ja", Version: "red", VersionOrder: 1, Text: "日本語"},
			{Language: "en", Version: "red", VersionOrder: 1, VersionGroup: "red-blue", VersionGroupOrder: 1, Text: "English red"},
			{Language: "en", Version: "yellow", VersionOrder: 2, Text: "English yellow"},
		},
		Varieties: []domain.Variety{{PokemonID: 25, Slug: "pikachu", IsDefault: true}},
	}
	compact := dataset.SpeciesFromDomain(src)
	if len(compact.FlavorTexts) != 2 {
		t.Fatalf("flavor texts=%d want 2 english entries", len(compact.FlavorTexts))
	}
	if compact.FlavorTexts[0].Version != "red" || compact.FlavorTexts[1].Version != "yellow" {
		t.Fatalf("compact flavor versions=%+v", compact.FlavorTexts)
	}
	back := dataset.SpeciesToDomain(compact)
	if len(back.Names) != 0 || len(back.Genera) != 0 {
		t.Fatalf("round-trip must not restore multilingual arrays: names=%d genera=%d", len(back.Names), len(back.Genera))
	}
	if len(back.FlavorTextEntries) != 2 {
		t.Fatalf("flavor entries=%d want 2", len(back.FlavorTextEntries))
	}
	for _, f := range back.FlavorTextEntries {
		if f.Language != "en" {
			t.Fatalf("flavor language=%q want en", f.Language)
		}
	}
	if back.Genus != "Mouse Pokémon" || back.DisplayName != "Pikachu" {
		t.Fatalf("identity fields lost: %+v", back)
	}
	if len(back.Varieties) != 1 || !back.Varieties[0].IsDefault {
		t.Fatalf("varieties=%+v", back.Varieties)
	}
}

func TestPokemonRoundTrip(t *testing.T) {
	src := domain.Pokemon{
		ID: 25, Slug: "pikachu", DisplayName: "Pikachu", SpeciesID: 25,
		HeightDecimeter: 4, WeightHectogram: 60, BaseExperience: 112,
		Types:     []domain.TypeSlot{{Slot: 1, Name: "electric"}},
		Abilities: []domain.AbilitySlot{{Slot: 1, Name: "static"}},
		Stats:     domain.Stats{HP: 35, Speed: 90},
		Moves: []domain.LearnedMove{{
			MoveID: 1, Slug: "thunder-shock",
			Learnings: []domain.MoveLearning{{Method: "level-up", Level: 1}},
		}},
		Sprites: domain.Sprites{
			FrontDefault:    "https://example/pixel.png",
			HomeFront:       "https://example/home.png",
			OfficialArtwork: "https://example/art.png",
		},
	}
	compact := dataset.PokemonFromDomain(src)
	got := dataset.PokemonToDomain(compact, src.Moves)
	if got.ID != src.ID || got.SpeciesID != src.SpeciesID || got.BaseExperience != src.BaseExperience {
		t.Fatalf("identity mismatch: %+v", got)
	}
	if got.Sprites.OfficialArtwork != src.Sprites.OfficialArtwork || got.Sprites.HomeFront != src.Sprites.HomeFront {
		t.Fatalf("sprites=%+v", got.Sprites)
	}
	learnset := dataset.LearnsetFromPokemon(src)
	if len(learnset) != 1 || learnset[0].MoveID != 1 || learnset[0].Level != 1 {
		t.Fatalf("learnset=%+v", learnset)
	}
}

func TestMoveRoundTripDropsNames(t *testing.T) {
	src := domain.Move{
		ID: 1, Slug: "pound", DisplayName: "Pound", PP: 35,
		Names: []domain.LocalizedName{{Language: "ja", Value: "はたく"}},
	}
	compact := dataset.MoveFromDomain(src)
	got := dataset.MoveToDomain(compact)
	if got.ID != src.ID || got.PP != src.PP {
		t.Fatalf("move=%+v", got)
	}
	if len(got.Names) != 0 {
		t.Fatalf("names should stay empty after round-trip")
	}
}

func TestEvolutionRoundTrip(t *testing.T) {
	level := 10
	src := domain.EvolutionTree{
		Root: domain.EvolutionNode{
			SpeciesID: 25, SpeciesSlug: "pikachu", DisplayName: "Pikachu",
			Children: []domain.EvolutionBranch{{
				Conditions: []domain.EvolutionCondition{{Trigger: "level-up", MinimumLevel: &level}},
				Target:     domain.EvolutionNode{SpeciesID: 26, SpeciesSlug: "raichu", DisplayName: "Raichu"},
			}},
		},
	}
	got := dataset.EvolutionToDomain(dataset.EvolutionFromDomain(src))
	if got.Root.SpeciesID != 25 || len(got.Root.Children) != 1 {
		t.Fatalf("evolution=%+v", got)
	}
	if got.Root.Children[0].Target.SpeciesID != 26 {
		t.Fatalf("target=%+v", got.Root.Children[0].Target)
	}
}

func TestSchemaVersion(t *testing.T) {
	if dataset.SchemaVersion != 3 {
		t.Fatalf("schema version=%d want 3", dataset.SchemaVersion)
	}
}
