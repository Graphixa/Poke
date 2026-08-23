package textwidth

import "testing"

func TestTruncateWideRunes(t *testing.T) {
	// Fullwidth CJK typically takes 2 columns.
	s := "ポケモン"
	got := Truncate(s, 5)
	if Width(got) > 5 {
		t.Fatalf("width %d > 5 for %q", Width(got), got)
	}
	if got == s {
		t.Fatal("expected truncation")
	}
}

func TestPadRight(t *testing.T) {
	got := PadRight("ab", 5)
	if Width(got) != 5 {
		t.Fatalf("width=%d got=%q", Width(got), got)
	}
}
