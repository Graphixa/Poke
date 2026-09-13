package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/Graphixa/Pokedex/internal/domain"
)

func TestBrowseEntryUnmarshalsHeightWeight(t *testing.T) {
	const raw = `{"id":1,"slug":"bulbasaur","name":"Bulbasaur","types":["grass"],"hp":45,"attack":49,"defence":49,"sp_attack":65,"sp_defence":65,"speed":45,"height":7,"weight":69,"base_exp":64,"genus":"Seed Pokémon"}`
	var e domain.BrowseEntry
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		t.Fatal(err)
	}
	if !e.HasAbout() {
		t.Fatalf("HasAbout=false height=%d weight=%d", e.HeightDecimeter, e.WeightHectogram)
	}
	if e.HeightDecimeter != 7 || e.WeightHectogram != 69 || e.BaseExperience != 64 {
		t.Fatalf("got h=%d w=%d exp=%d", e.HeightDecimeter, e.WeightHectogram, e.BaseExperience)
	}
}
