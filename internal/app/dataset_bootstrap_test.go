package app

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Graphixa/Pokedex/internal/cache"
)

func TestThrottledCLIProgress(t *testing.T) {
	var buf bytes.Buffer
	report := throttledCLIProgress(&buf)
	report(cache.Progress{Stage: "records", Done: 1, Total: 10, Message: "Bulbasaur"})
	report(cache.Progress{Stage: "records", Done: 2, Total: 10, Message: "Ivysaur"})
	out := buf.String()
	if !strings.Contains(out, "Building dataset") {
		t.Fatalf("output=%q", out)
	}
	if strings.Count(out, "\n") > 1 {
		t.Fatalf("expected compact output, got %q", out)
	}
}

func TestFormatBootstrapProgressStages(t *testing.T) {
	t.Parallel()
	if got := formatBootstrapProgress(cache.Progress{Stage: "evolution", Done: 3, Total: 10}); !strings.Contains(got, "evolution") {
		t.Fatalf("got=%q", got)
	}
	if got := formatBootstrapProgress(cache.Progress{Stage: "index", Done: 1, Total: 1, Message: "Indexed 25 species"}); got != "Indexed 25 species" {
		t.Fatalf("got=%q", got)
	}
}

// Locks CLI bootstrap copy so typed-stage refactors cannot silently rewrite it.
func TestFormatBootstrapProgressExactCopy(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		p    cache.Progress
		want string
	}{
		{name: "index in progress", p: cache.Progress{Stage: "index", Done: 0, Total: 0}, want: "Fetching species index"},
		{name: "index complete message", p: cache.Progress{Stage: "index", Done: 1, Total: 1, Message: "Indexed 25 species"}, want: "Indexed 25 species"},
		{name: "enrich", p: cache.Progress{Stage: "enrich", Done: 25, Total: 100}, want: "Enriching types 25% (25/100)"},
		{name: "records", p: cache.Progress{Stage: "records", Done: 1, Total: 10}, want: "Building dataset 10% (1/10 records)"},
		{name: "evolution", p: cache.Progress{Stage: "evolution", Done: 3, Total: 10}, want: "Fetching evolution chains 30% (3/10)"},
		{name: "moves", p: cache.Progress{Stage: "moves", Done: 2, Total: 8}, want: "Fetching moves 25% (2/8)"},
		{name: "dataset message", p: cache.Progress{Stage: "dataset", Message: "Activating dataset"}, want: "Activating dataset"},
		{name: "dataset default", p: cache.Progress{Stage: "dataset"}, want: "Building dataset"},
		{name: "empty stage with totals", p: cache.Progress{Stage: "", Done: 5, Total: 10}, want: "Building dataset"},
		{name: "unknown stage", p: cache.Progress{Stage: "unknown", Done: 5, Total: 10}, want: "Building dataset 50% (5/10)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := formatBootstrapProgress(tc.p); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
