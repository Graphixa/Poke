package search_test

import (
	"testing"

	"github.com/Graphixa/Poke/internal/domain"
	"github.com/Graphixa/Poke/internal/search"
)

func TestNormalizeMrMimeVariants(t *testing.T) {
	want := "mr-mime"
	for _, in := range []string{"mr. mime", "Mr. Mime", "mr-mime", "MR MIME", "mr mime", "  mr..mime  "} {
		if got := search.Normalize(in); got != want {
			t.Fatalf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRankPriority(t *testing.T) {
	tests := []struct {
		query, slug string
		want        int
	}{
		{"pikachu", "pikachu", search.RankExact},
		{"pika", "pikachu", search.RankStartsWith},
		{"chu", "pikachu", search.RankContains},
		{"mime", "mr-mime", search.RankWordPrefix},
		{"pcm", "pikachu", search.RankNone},
		{"pku", "pikachu", search.RankSubsequence},
		{"zzzz", "pikachu", search.RankNone},
	}
	for _, tt := range tests {
		if got := search.Rank(tt.query, tt.slug, ""); got != tt.want {
			t.Fatalf("Rank(%q, %q) = %d, want %d", tt.query, tt.slug, got, tt.want)
		}
	}
}

func fixtureIndex() *search.Index {
	return search.Build([]domain.SpeciesSummary{
		{ID: 4, Slug: "charmander", DisplayName: "Charmander", Types: []string{"fire"}},
		{ID: 5, Slug: "charmeleon", DisplayName: "Charmeleon", Types: []string{"fire"}},
		{ID: 6, Slug: "charizard", DisplayName: "Charizard", Types: []string{"fire", "flying"}},
		{ID: 25, Slug: "pikachu", DisplayName: "Pikachu", Types: []string{"electric"}},
		{ID: 122, Slug: "mr-mime", DisplayName: "Mr. Mime", Types: []string{"psychic", "fairy"}},
		{ID: 133, Slug: "eevee", DisplayName: "Eevee", Types: []string{"normal"}},
		{ID: 152, Slug: "chikorita", DisplayName: "Chikorita", Types: []string{"grass"}},
	})
}

func TestSearchMrMimeQueries(t *testing.T) {
	idx := fixtureIndex()
	for _, q := range []string{"mr. mime", "mr-mime", "mr mime", "Mr. Mime"} {
		hits := idx.Search(q)
		if len(hits) == 0 || hits[0].Slug != "mr-mime" {
			t.Fatalf("Search(%q) = %#v, want mr-mime first", q, hits)
		}
	}
}

func TestSearchStartsWithBeforeContains(t *testing.T) {
	idx := fixtureIndex()
	hits := idx.Search("char")
	if len(hits) < 3 {
		t.Fatalf("expected char* hits, got %#v", hits)
	}
	// starts-with: charmander, charmeleon, charizard — all starts-with "char"
	// chikorita is subsequence or contains? "chikorita" contains "chi" not "char".
	// "char" as subsequence of chikorita: c-h-i-k-o-r-i-t-a — c,h then need a,r - there's no 'a' before...
	// Actually chikorita: c,h,i,k,o,r,i,t,a - for "char": c at 0, h at 1, a at end, r at 5 - but a is after r so subsequence fails for c-h-a-r order.
	// Wait: c(0), h(1), a(8), r(?) - r is at 5 which is before a at 8. So isSubsequence needs increasing indices: c=0,h=1,a=8, then r after 8? No r after 8. So chikorita doesn't match "char".
	for i := 0; i < 3; i++ {
		if hits[i].ID != 4 && hits[i].ID != 5 && hits[i].ID != 6 {
			t.Fatalf("hit[%d]=%#v, expected char family first", i, hits[i])
		}
	}
	// Among starts-with, tie-break by dex number.
	if hits[0].ID != 4 || hits[1].ID != 5 || hits[2].ID != 6 {
		t.Fatalf("dex tie-break failed: %#v", hits[:3])
	}
}

func TestSearchArmRanksPrefixAboveContains(t *testing.T) {
	idx := search.Build([]domain.SpeciesSummary{
		{ID: 4, Slug: "charmander", DisplayName: "Charmander"},
		{ID: 999, Slug: "armaldo", DisplayName: "Armaldo"},
	})
	hits := idx.Search("arm")
	if len(hits) < 2 {
		t.Fatalf("hits = %#v", hits)
	}
	if hits[0].Slug != "armaldo" {
		t.Fatalf("expected armaldo (starts-with) before charmander (contains), got %#v", hits)
	}
	if hits[1].Slug != "charmander" {
		t.Fatalf("expected charmander second, got %#v", hits)
	}
}

func TestResolveNameNumberAndMrMime(t *testing.T) {
	idx := fixtureIndex()
	cases := []struct {
		target string
		slug   string
		ok     bool
	}{
		{"25", "pikachu", true},
		{"#025", "pikachu", true},
		{"pikachu", "pikachu", true},
		{"mr. mime", "mr-mime", true},
		{"mr-mime", "mr-mime", true},
		{"missingno", "", false},
		{"char", "", false}, // not exact
	}
	for _, tt := range cases {
		got, ok := idx.Resolve(tt.target)
		if ok != tt.ok {
			t.Fatalf("Resolve(%q) ok=%v, want %v", tt.target, ok, tt.ok)
		}
		if ok && got.Slug != tt.slug {
			t.Fatalf("Resolve(%q) = %q, want %q", tt.target, got.Slug, tt.slug)
		}
	}
}

func TestParseDexNumber(t *testing.T) {
	id, ok := search.ParseDexNumber("#025")
	if !ok || id != 25 {
		t.Fatalf("got %d %v", id, ok)
	}
	if _, ok := search.ParseDexNumber("pikachu"); ok {
		t.Fatal("name should not parse as dex")
	}
}
