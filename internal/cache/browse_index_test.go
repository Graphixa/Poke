package cache

import (
	"testing"

	"github.com/Graphixa/Poke/internal/domain"
)

func TestBrowseSummariesAndByID(t *testing.T) {
	entries := []domain.BrowseEntry{
		{ID: 25, Slug: "pikachu", Name: "Pikachu", Types: []string{"electric"}},
		{ID: 1, Slug: "bulbasaur", Name: "Bulbasaur", Types: []string{"grass"}},
	}
	sums := BrowseSummaries(entries)
	if len(sums) != 2 || sums[0].DisplayName != "Pikachu" || sums[0].SpritePath == "" {
		t.Fatalf("summaries = %+v", sums)
	}
	byID := BrowseByID(entries)
	if byID[25].Slug != "pikachu" || byID[1].Name != "Bulbasaur" {
		t.Fatalf("byID = %+v", byID)
	}
}
