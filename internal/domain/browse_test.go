package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/Graphixa/Poke/internal/domain"
)

func TestBrowseEntryUnmarshalsAbilities(t *testing.T) {
	const raw = `{"id":1,"slug":"bulbasaur","name":"Bulbasaur","types":["grass"],"hp":45,"attack":49,"defence":49,"sp_attack":65,"sp_defence":65,"speed":45,"height":7,"weight":69,"base_exp":64,"genus":"Seed Pokémon","abilities":[{"name":"overgrow"},{"name":"chlorophyll","is_hidden":true}]}`
	var e domain.BrowseEntry
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		t.Fatal(err)
	}
	if !e.HasAbilities() {
		t.Fatal("HasAbilities=false")
	}
	if len(e.Abilities) != 2 || e.Abilities[0].Name != "overgrow" || !e.Abilities[1].IsHidden {
		t.Fatalf("abilities=%+v", e.Abilities)
	}
}

func TestBrowseEntryFromRecord(t *testing.T) {
	sum := domain.SpeciesSummary{ID: 4, Slug: "charmander", DisplayName: "Charmander"}
	poke := domain.Pokemon{
		Slug: "charmander", DisplayName: "Charmander",
		Types:           []domain.TypeSlot{{Slot: 1, Name: "fire"}},
		Stats:           domain.Stats{HP: 39, Speed: 65},
		HeightDecimeter: 6, BaseExperience: 62,
		Abilities: []domain.AbilitySlot{{Slot: 1, Name: "blaze"}},
	}
	sp := domain.Species{Genus: "Lizard Pokémon"}
	e := domain.BrowseEntryFromRecord(sum, poke, sp)
	if e.HP != 39 || e.Speed != 65 || e.Genus != sp.Genus || !e.HasAbilities() {
		t.Fatalf("entry=%+v", e)
	}
	if len(e.Types) != 1 || e.Types[0] != "fire" {
		t.Fatalf("types=%v", e.Types)
	}
}

func TestBrowseEntryWithoutAbilities(t *testing.T) {
	const raw = `{"id":1,"slug":"bulbasaur","name":"Bulbasaur","height":7,"weight":69}`
	var e domain.BrowseEntry
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		t.Fatal(err)
	}
	if e.HasAbilities() {
		t.Fatal("old browse.json without abilities must not claim HasAbilities")
	}
}

func TestPokemonSpeciesFromBrowse(t *testing.T) {
	e := domain.BrowseEntry{
		ID: 4, Slug: "charmander", Name: "Charmander", Types: []string{"fire"},
		HP: 39, Speed: 65, HeightDecimeter: 6, WeightHectogram: 85,
		Genus: "Lizard Pokémon", Abilities: []domain.AbilitySlot{{Name: "blaze"}},
	}
	p := domain.PokemonFromBrowse(e)
	if p.ID != 4 || p.DisplayName != "Charmander" || p.Stats.HP != 39 || len(p.Types) != 1 || p.Types[0].Name != "fire" {
		t.Fatalf("pokemon=%+v", p)
	}
	s := domain.SpeciesFromBrowse(e)
	if s.ID != 4 || s.Genus != "Lizard Pokémon" || s.DefaultPokemonID != 4 {
		t.Fatalf("species=%+v", s)
	}
}
