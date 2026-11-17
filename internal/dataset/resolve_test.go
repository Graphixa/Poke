package dataset_test

import (
	"strings"
	"testing"

	"github.com/Graphixa/Poke/internal/dataset"
	"github.com/Graphixa/Poke/internal/domain"
)

func TestSelectableFormsDefaultLabel(t *testing.T) {
	t.Parallel()
	sp := dataset.DatasetSpecies{
		ID: 19, Slug: "rattata", DisplayName: "Rattata", DefaultPokemonID: 19,
		Varieties: []dataset.DatasetVariety{
			{PokemonID: 19, Slug: "rattata", IsDefault: true},
			{PokemonID: 10091, Slug: "rattata-alola", IsDefault: false},
		},
	}
	pokemon := map[int]dataset.DatasetPokemon{
		19:    {ID: 19, DisplayName: "Rattata"},
		10091: {ID: 10091, DisplayName: "Rattata"},
	}
	choices := dataset.SelectableForms(sp, nil, pokemon)
	if len(choices) != 2 {
		t.Fatalf("choices=%d", len(choices))
	}
	if !choices[0].IsDefault || choices[0].DisplayName != "Default" {
		t.Fatalf("default=%+v", choices[0])
	}
	if choices[1].DisplayName != "Alola" {
		t.Fatalf("alt label=%q want Alola", choices[1].DisplayName)
	}
}

func TestClassifyFormGameIntroOrder(t *testing.T) {
	t.Parallel()
	versions := map[int]dataset.DatasetVersion{
		12: {ID: 12, Slug: "pearl", VersionGroupID: 8, Order: 800}, // diamond-pearl
		27: {ID: 27, Slug: "sun", VersionGroupID: 17, Order: 1700}, // sun-moon
	}
	groups := map[int]dataset.DatasetVersionGroup{
		8:  {ID: 8, Slug: "diamond-pearl", Order: 800},
		17: {ID: 17, Slug: "sun-moon", Order: 1700},
	}
	forms := map[int]dataset.DatasetForm{
		10091: {ID: 10091, PokemonID: 10091, Slug: "rattata-alola", IsDefault: true, IntroVersionGroupID: 17},
	}
	alola := domain.SelectableForm{PokemonID: 10091, Slug: "rattata-alola", IsDefault: false}
	if got := dataset.ClassifyFormGame(alola, 12, versions, groups, forms); got != domain.FormCompatUnsupported {
		t.Fatalf("pearl vs alola=%v want unsupported", got)
	}
	if got := dataset.ClassifyFormGame(alola, 27, versions, groups, forms); got != domain.FormCompatUnknown {
		t.Fatalf("sun vs alola=%v want unknown (not assumed supported)", got)
	}
	def := domain.SelectableForm{PokemonID: 19, IsDefault: true}
	if got := dataset.ClassifyFormGame(def, 12, versions, groups, forms); got != domain.FormCompatSupported {
		t.Fatalf("default=%v", got)
	}
}

func TestResolveFallsBackOnlyWhenUnsupported(t *testing.T) {
	t.Parallel()
	sp := dataset.DatasetSpecies{
		ID: 19, Slug: "rattata", DisplayName: "Rattata", DefaultPokemonID: 19,
		Varieties: []dataset.DatasetVariety{
			{PokemonID: 19, Slug: "rattata", IsDefault: true},
			{PokemonID: 10091, Slug: "rattata-alola", IsDefault: false},
		},
	}
	in := dataset.ResolveInput{
		Species:       sp,
		PokemonID:     10091,
		GameVersionID: 12,
		Versions: map[int]dataset.DatasetVersion{
			12: {ID: 12, Slug: "pearl", DisplayName: "Pearl", VersionGroupID: 8, Order: 800},
		},
		VersionGroups: map[int]dataset.DatasetVersionGroup{
			8:  {ID: 8, Order: 800},
			17: {ID: 17, Order: 1700},
		},
		Forms: map[int]dataset.DatasetForm{
			10091: {ID: 10091, PokemonID: 10091, IsDefault: true, IntroVersionGroupID: 17},
		},
		Pokemon: map[int]dataset.DatasetPokemon{19: {ID: 19}, 10091: {ID: 10091}},
	}
	got := dataset.ResolveDossierContext(in)
	if !got.FellBackToDefault || got.PokemonID != 19 || got.FormID != 0 {
		t.Fatalf("fallback=%+v", got)
	}
	if !strings.Contains(got.Notice, "Pearl") || !strings.Contains(got.Notice, "Default") {
		t.Fatalf("notice=%q", got.Notice)
	}

	// Missing intro → Unknown, no fallback.
	in.Forms = map[int]dataset.DatasetForm{10091: {ID: 10091, PokemonID: 10091, IsDefault: true}}
	got = dataset.ResolveDossierContext(in)
	if got.FellBackToDefault || got.PokemonID != 10091 || got.Compat != domain.FormCompatUnknown {
		t.Fatalf("unknown must not fallback: %+v", got)
	}
}

func TestMissingLearnsetDoesNotAffectCompat(t *testing.T) {
	t.Parallel()
	// Empty learnset must not be consulted — classify stays Unknown without intro.
	choice := domain.SelectableForm{PokemonID: 25, IsDefault: false, Slug: "pikachu-belle"}
	got := dataset.ClassifyFormGame(choice, 40, map[int]dataset.DatasetVersion{
		40: {ID: 40, VersionGroupID: 25, Order: 2500},
	}, map[int]dataset.DatasetVersionGroup{25: {ID: 25, Order: 2500}}, nil)
	if got != domain.FormCompatUnknown {
		t.Fatalf("got=%v", got)
	}
	if dataset.LearnsetPokemonID(choice) != 25 {
		t.Fatal("learnset id must stay on choice pokemon")
	}
}

func TestLearnMethodsForGroupOrder(t *testing.T) {
	t.Parallel()
	rows := []dataset.DatasetLearn{
		{PokemonID: 1, MoveID: 1, VersionGroupID: 1, LearnMethod: "tutor"},
		{PokemonID: 1, MoveID: 2, VersionGroupID: 1, LearnMethod: "level-up"},
		{PokemonID: 1, MoveID: 3, VersionGroupID: 1, LearnMethod: "machine"},
		{PokemonID: 1, MoveID: 4, VersionGroupID: 2, LearnMethod: "egg"},
	}
	got := dataset.LearnMethodsForGroup(rows, 1)
	want := []string{"level-up", "machine", "tutor"}
	if len(got) != len(want) {
		t.Fatalf("got=%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got=%v want=%v", got, want)
		}
	}
}

func TestFormOptionsMarksUnsupported(t *testing.T) {
	t.Parallel()
	sp := dataset.DatasetSpecies{
		ID: 6, Slug: "charizard", DisplayName: "Charizard", DefaultPokemonID: 6,
		Varieties: []dataset.DatasetVariety{{PokemonID: 6, Slug: "charizard", IsDefault: true}},
	}
	in := dataset.ResolveInput{
		Species:       sp,
		GameVersionID: 1,
		Versions: map[int]dataset.DatasetVersion{
			1: {ID: 1, Slug: "red", DisplayName: "Red", VersionGroupID: 1, Order: 100},
		},
		VersionGroups: map[int]dataset.DatasetVersionGroup{
			1:  {ID: 1, Order: 100},
			15: {ID: 15, Order: 1500}, // xy
		},
		Forms: map[int]dataset.DatasetForm{
			10033: {ID: 10033, PokemonID: 6, FormName: "mega-x", Slug: "charizard-mega-x", FormOrder: 2, IntroVersionGroupID: 15},
		},
		Pokemon: map[int]dataset.DatasetPokemon{6: {ID: 6, DisplayName: "Charizard"}},
	}
	opts := dataset.FormOptionsForGame(in)
	if len(opts) < 2 {
		t.Fatalf("opts=%+v", opts)
	}
	var mega domain.FormOption
	for _, o := range opts {
		if o.Form.FormID == 10033 {
			mega = o
		}
	}
	if mega.Compat != domain.FormCompatUnsupported {
		t.Fatalf("mega in red=%v", mega.Compat)
	}
	if mega.Form.DisplayName != "Mega X" {
		t.Fatalf("label=%q", mega.Form.DisplayName)
	}
}

func TestVersionAndVersionGroupLookupsDistinct(t *testing.T) {
	t.Parallel()
	versions := map[int]dataset.DatasetVersion{
		12: {ID: 12, Slug: "pearl", VersionGroupID: 8},
		13: {ID: 13, Slug: "diamond", VersionGroupID: 8},
	}
	vg, ok := dataset.VersionGroupIDForVersion(versions, 12)
	if !ok || vg != 8 {
		t.Fatalf("vg=%d ok=%v", vg, ok)
	}
	if _, ok := dataset.VersionGroupIDForVersion(versions, 999); ok {
		t.Fatal("missing version must not invent a group")
	}
}
