package search

import (
	"sort"
	"strconv"
	"strings"

	"github.com/Graphixa/Poke/internal/domain"
)

// Entry is a species summary with precomputed normalized fields for ranking.
type Entry struct {
	Summary     domain.SpeciesSummary
	NormSlug    string
	NormDisplay string
	Tokens      []string
}

// Index is an in-memory searchable species index.
type Index struct {
	entries []Entry
	byID    map[int]Entry
}

// Build constructs a search index from species summaries.
func Build(summaries []domain.SpeciesSummary) *Index {
	entries := make([]Entry, 0, len(summaries))
	byID := make(map[int]Entry, len(summaries))
	for _, sum := range summaries {
		e := Entry{
			Summary:     sum,
			NormSlug:    Normalize(sum.Slug),
			NormDisplay: Normalize(sum.DisplayName),
		}
		e.Tokens = Tokens(e.NormSlug)
		entries = append(entries, e)
		byID[sum.ID] = e
	}
	return &Index{entries: entries, byID: byID}
}

// Len returns the number of indexed species.
func (idx *Index) Len() int {
	if idx == nil {
		return 0
	}
	return len(idx.entries)
}

// All returns every indexed summary in dex order.
func (idx *Index) All() []domain.SpeciesSummary {
	if idx == nil {
		return nil
	}
	out := make([]domain.SpeciesSummary, len(idx.entries))
	for i, e := range idx.entries {
		out[i] = e.Summary
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return strings.ToLower(out[i].DisplayName) < strings.ToLower(out[j].DisplayName)
	})
	return out
}

type ranked struct {
	summary domain.SpeciesSummary
	rank    int
}

// Search returns matching species sorted by rank, then dex number, then name.
// An empty query returns the full index in dex order.
func (idx *Index) Search(query string) []domain.SpeciesSummary {
	if idx == nil {
		return nil
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return idx.All()
	}
	if id, ok := ParseDexNumber(q); ok {
		if e, found := idx.byID[id]; found {
			return []domain.SpeciesSummary{e.Summary}
		}
		return nil
	}

	norm := Normalize(q)
	var hits []ranked
	for _, e := range idx.entries {
		r := Rank(norm, e.NormSlug, e.NormDisplay)
		if r == RankNone {
			continue
		}
		hits = append(hits, ranked{summary: e.Summary, rank: r})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].rank != hits[j].rank {
			return hits[i].rank < hits[j].rank
		}
		if hits[i].summary.ID != hits[j].summary.ID {
			return hits[i].summary.ID < hits[j].summary.ID
		}
		return strings.ToLower(hits[i].summary.DisplayName) < strings.ToLower(hits[j].summary.DisplayName)
	})
	out := make([]domain.SpeciesSummary, len(hits))
	for i, h := range hits {
		out[i] = h.summary
	}
	return out
}

// Resolve finds a single species for an inline target (name, slug, or number).
// Name/slug resolution requires an exact normalized match.
func (idx *Index) Resolve(target string) (domain.SpeciesSummary, bool) {
	if idx == nil {
		return domain.SpeciesSummary{}, false
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return domain.SpeciesSummary{}, false
	}
	if id, ok := ParseDexNumber(target); ok {
		e, found := idx.byID[id]
		return e.Summary, found
	}
	norm := Normalize(target)
	if norm == "" {
		return domain.SpeciesSummary{}, false
	}
	var match domain.SpeciesSummary
	found := false
	for _, e := range idx.entries {
		if e.NormSlug == norm || e.NormDisplay == norm {
			if !found || e.Summary.ID < match.ID {
				match = e.Summary
				found = true
			}
		}
	}
	return match, found
}

// ParseDexNumber accepts "25", "#25", and "#025".
func ParseDexNumber(s string) (int, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "#")
	if s == "" {
		return 0, false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	id, err := strconv.Atoi(s)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}
