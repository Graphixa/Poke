package search

import "strings"

// Match quality ordered best → worst. Lower is better.
const (
	RankExact       = 0
	RankStartsWith  = 1
	RankWordPrefix  = 2
	RankContains    = 3
	RankSubsequence = 4
	RankNone        = 100
)

// Rank returns the best match quality of query against a species slug and display name.
// query, slug, and display should already be normalized except Rank also normalizes defensively.
func Rank(query, slug, display string) int {
	q := Normalize(query)
	if q == "" {
		return RankNone
	}
	candidates := []string{Normalize(slug), Normalize(display)}
	best := RankNone
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if r := rankOne(q, c); r < best {
			best = r
		}
	}
	return best
}

func rankOne(query, candidate string) int {
	if query == candidate {
		return RankExact
	}
	if strings.HasPrefix(candidate, query) {
		return RankStartsWith
	}
	for _, tok := range Tokens(candidate) {
		if strings.HasPrefix(tok, query) {
			return RankWordPrefix
		}
	}
	if strings.Contains(candidate, query) {
		return RankContains
	}
	if isSubsequence(query, candidate) {
		return RankSubsequence
	}
	return RankNone
}

func isSubsequence(query, candidate string) bool {
	qi := 0
	for i := 0; i < len(candidate) && qi < len(query); i++ {
		if candidate[i] == query[qi] {
			qi++
		}
	}
	return qi == len(query)
}
