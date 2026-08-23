package usererr_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/app"
	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/pokeapi"
	"github.com/Graphixa/Pokedex/internal/usererr"
)

func TestFormatRemediation(t *testing.T) {
	cases := []struct {
		err      error
		wantMsg  string
		wantHint string
	}{
		{&app.NotFoundError{Query: "missingno"}, "Pokémon not found: missingno", "poke search"},
		{&cache.OfflineError{Err: pokeapi.ErrOffline}, "cache is empty", "poke update"},
		{&pokeapi.RateLimitError{RetryAfter: 2 * time.Second}, "rate limited", "Wait"},
		{cache.ErrEmpty, "empty", "poke update"},
		{cache.ErrCorrupt, "corrupt", "poke update"},
	}
	for _, tc := range cases {
		got := usererr.Format(tc.err)
		lower := strings.ToLower(got)
		if !strings.Contains(lower, strings.ToLower(tc.wantMsg)) {
			t.Fatalf("msg missing %q in %q", tc.wantMsg, got)
		}
		if !strings.Contains(got, "Hint:") {
			t.Fatalf("expected Hint in %q", got)
		}
		if tc.wantHint != "" && !strings.Contains(lower, strings.ToLower(tc.wantHint)) {
			t.Fatalf("hint missing %q in %q", tc.wantHint, got)
		}
	}
}

func TestMessageOfflineTab(t *testing.T) {
	got := usererr.Message(pokeapi.ErrOffline)
	if !strings.Contains(got, "offline") && !strings.Contains(got, "Unavailable") {
		t.Fatalf("got %q", got)
	}
	got = usererr.Message(&pokeapi.RateLimitError{})
	if !strings.Contains(got, "Rate limited") {
		t.Fatalf("got %q", got)
	}
}
