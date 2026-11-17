package typeseed_test

import (
	"testing"

	"github.com/Graphixa/Poke/internal/cache/typeseed"
	"github.com/Graphixa/Poke/internal/domain"
)

func TestSeedLoadsAndLookup(t *testing.T) {
	if err := typeseed.LoadError(); err != nil {
		t.Fatal(err)
	}
	types := typeseed.Lookup(25)
	if len(types) != 1 || types[0] != "electric" {
		t.Fatalf("pikachu types=%v", types)
	}
	if !typeseed.Has(1) {
		t.Fatal("expected bulbasaur in seed")
	}
	if typeseed.Has(999999) {
		t.Fatal("unexpected id")
	}
}

func TestApplyFillsEmpty(t *testing.T) {
	index := []domain.SpeciesSummary{
		{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"},
		{ID: 6, Slug: "charizard", DisplayName: "Charizard", Types: []string{"fire", "flying"}},
		{ID: 999999, Slug: "future", DisplayName: "Future"},
	}
	out, missing := typeseed.Apply(index)
	if len(out[0].Types) != 1 || out[0].Types[0] != "electric" {
		t.Fatalf("pikachu=%v", out[0].Types)
	}
	if len(out[1].Types) != 2 {
		t.Fatalf("charizard should keep existing types: %v", out[1].Types)
	}
	if len(missing) != 1 || missing[0] != 999999 {
		t.Fatalf("missing=%v", missing)
	}
}
