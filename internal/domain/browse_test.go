package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/Graphixa/Pokedex/internal/domain"
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
