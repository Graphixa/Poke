package cache_test

import (
	"testing"

	"github.com/Graphixa/Poke/internal/cache"
	"github.com/Graphixa/Poke/internal/domain"
)

func TestNeedsTypeEnrichmentFalseWhenSeeded(t *testing.T) {
	index := []domain.SpeciesSummary{
		{ID: 1, Slug: "bulbasaur", DisplayName: "Bulbasaur"},
		{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"},
		{ID: 150, Slug: "mewtwo", DisplayName: "Mewtwo"},
	}
	if cache.NeedsTypeEnrichment(index) {
		t.Fatal("seeded national-dex IDs should not need enrich")
	}
}

func TestNeedsTypeEnrichmentTrueForUnknownID(t *testing.T) {
	index := []domain.SpeciesSummary{
		{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"},
		{ID: 999999, Slug: "future", DisplayName: "Future"},
	}
	if !cache.NeedsTypeEnrichment(index) {
		t.Fatal("unknown ID without types should need enrich")
	}
}
