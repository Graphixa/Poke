package app

import "testing"

func TestDrainPendingStdinNoPanic(t *testing.T) {
	// Non-TTY / empty buffer: must be a safe no-op (CI pipes, IDE runs).
	drainPendingStdin()
}

func TestParseCellSizeReport(t *testing.T) {
	cases := []struct {
		in   string
		w, h int
		ok   bool
	}{
		{"\x1b[6;17;10t", 10, 17, true},
		{"junk\x1b[6;20;10tmore", 10, 20, true}, // surrounded by queued input
		{"\x1b[6;17t", 0, 0, false},             // missing width
		{"\x1b[4;340;800t", 0, 0, false},        // wrong op (text-area report)
		{"", 0, 0, false},
		{"\x1b[6;;t", 0, 0, false},
		// DECRPM (bubbletea unicode-core mode 2027) must not parse as cell size.
		{"\x1b[?2027;3$y", 0, 0, false},
		{"\x1b[?2027;3$y\x1b[6;20;10t", 10, 20, true},
	}
	for _, c := range cases {
		w, h, ok := parseCellSizeReport([]byte(c.in))
		if ok != c.ok || w != c.w || h != c.h {
			t.Fatalf("parse(%q) = %d,%d,%v want %d,%d,%v", c.in, w, h, ok, c.w, c.h, c.ok)
		}
	}
}
