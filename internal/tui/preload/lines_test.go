package preload

import (
	"strings"
	"testing"
)

func TestLinesAreEditableCopy(t *testing.T) {
	if len(Lines) < 8 {
		t.Fatal("add loading lines to lines.go")
	}
	for _, line := range Lines {
		if strings.TrimSpace(line) == "" {
			t.Fatal("blank loading line")
		}
	}
	got := Shuffle()
	if len(got) != len(Lines) {
		t.Fatalf("shuffle len %d", len(got))
	}
}

func TestBarWeightsLargerPartsMore(t *testing.T) {
	b := NewBar(map[string]int{"records": 10, "form": 30, "artwork": 10})
	done, total := b.Advance("records", 10, 10)
	if done != 10 || total != 50 {
		t.Fatalf("after records %d/%d", done, total)
	}
	done, total = b.Advance("form", 15, 40)
	if done != 25 || total != 60 {
		t.Fatalf("forms outweigh records, got %d/%d", done, total)
	}
	done, total = b.Advance("evolution", 0, 0)
	if done != 25 || total != 60 {
		t.Fatalf("skip changed the bar %d/%d", done, total)
	}
	done, total = b.Advance("artwork", 10, 10)
	if done != 35 || total != 60 {
		t.Fatalf("finished %d/%d", done, total)
	}
}
