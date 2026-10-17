package dataset_test

import (
	"testing"

	"github.com/Graphixa/Pokedex/internal/dataset"
	"github.com/Graphixa/Pokedex/internal/domain"
)

func TestVersionGroupIDForVersion(t *testing.T) {
	t.Parallel()
	versions := map[int]dataset.DatasetVersion{
		1:  {ID: 1, Slug: "red", VersionGroupID: 1, Order: 100},
		2:  {ID: 2, Slug: "blue", VersionGroupID: 1, Order: 101},
		40: {ID: 40, Slug: "scarlet", VersionGroupID: 25, Order: 2500},
	}
	vg, ok := dataset.VersionGroupIDForVersion(versions, 40)
	if !ok || vg != 25 {
		t.Fatalf("vg=%d ok=%v", vg, ok)
	}
}

func TestNewestVersionWithCoverage(t *testing.T) {
	t.Parallel()
	versions := map[int]dataset.DatasetVersion{
		1:  {ID: 1, Slug: "red", Order: 100},
		2:  {ID: 2, Slug: "yellow", Order: 200},
		40: {ID: 40, Slug: "scarlet", Order: 2500},
	}
	got, ok := dataset.NewestVersionWithCoverage(versions, nil, map[string]struct{}{"red": {}, "yellow": {}})
	if !ok || got.Slug != "yellow" {
		t.Fatalf("got=%+v ok=%v want yellow", got, ok)
	}
	got, ok = dataset.NewestVersionWithCoverage(versions, map[int]struct{}{40: {}}, nil)
	if !ok || got.Slug != "scarlet" {
		t.Fatalf("got=%+v ok=%v want scarlet", got, ok)
	}
}

func TestEncountersAndFlavorExactVersion(t *testing.T) {
	t.Parallel()
	enc := []dataset.DatasetEncounter{
		{Version: "red", VersionID: 1, Area: "route-1"},
		{Version: "blue", VersionID: 2, Area: "route-2"},
	}
	got := dataset.EncountersForVersion(enc, "red", 0)
	if len(got) != 1 || got[0].Area != "route-1" {
		t.Fatalf("enc=%+v", got)
	}
	flavors := []dataset.DatasetFlavorText{
		{Version: "red", VersionID: 1, Text: "a"},
		{Version: "yellow", VersionID: 3, Text: "b"},
	}
	f := dataset.FlavorForVersion(flavors, "", 3)
	if len(f) != 1 || f[0].Text != "b" {
		t.Fatalf("flavor=%+v", f)
	}
}

func TestSelectableFormsVarietiesAndCosmetic(t *testing.T) {
	t.Parallel()
	sp := dataset.DatasetSpecies{
		ID: 25, Slug: "pikachu", DisplayName: "Pikachu", DefaultPokemonID: 25,
		Varieties: []dataset.DatasetVariety{
			{PokemonID: 25, Slug: "pikachu", IsDefault: true},
			{PokemonID: 10080, Slug: "pikachu-partner", IsDefault: false},
		},
	}
	pokemon := map[int]dataset.DatasetPokemon{
		25:    {ID: 25, DisplayName: "Pikachu"},
		10080: {ID: 10080, DisplayName: "Partner Pikachu"},
	}
	forms := map[int]dataset.DatasetForm{
		10041: {ID: 10041, PokemonID: 25, Slug: "pikachu-cosplay", FormName: "cosplay", DisplayName: "Cosplay Pikachu", FormOrder: 2},
		1:     {ID: 1, PokemonID: 25, Slug: "pikachu", IsDefault: true, FormName: ""},
	}
	choices := dataset.SelectableForms(sp, forms, pokemon)
	if len(choices) != 3 {
		t.Fatalf("choices=%d %+v", len(choices), choices)
	}
	if choices[0].PokemonID != 25 || !choices[0].IsDefault || choices[0].DisplayName != "Default" {
		t.Fatalf("first=%+v", choices[0])
	}
	if choices[1].PokemonID != 10080 || choices[1].DisplayName != "Partner" {
		t.Fatalf("second=%+v", choices[1])
	}
	if choices[2].FormID != 10041 || choices[2].DisplayName != "Cosplay" {
		t.Fatalf("cosmetic=%+v", choices[2])
	}
}

func TestHabitatNullPreserved(t *testing.T) {
	t.Parallel()
	compact := dataset.SpeciesFromDomain(domain.Species{ID: 1, Slug: "bulbasaur", DisplayName: "Bulbasaur"})
	if compact.Habitat != nil {
		t.Fatalf("habitat should be null, got %q", *compact.Habitat)
	}
	back := dataset.SpeciesToDomain(compact)
	if back.Habitat != "" {
		t.Fatalf("habitat=%q", back.Habitat)
	}
	h := "forest"
	with := dataset.DatasetSpecies{ID: 1, Habitat: &h}
	if dataset.SpeciesToDomain(with).Habitat != "forest" {
		t.Fatal("habitat round-trip failed")
	}
}
