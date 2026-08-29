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
