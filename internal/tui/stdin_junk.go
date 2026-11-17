package tui

import (
	"regexp"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Leaked CSI reply tails (ESC/prefix already consumed by a prior read) type
// into focused browse search as printable runes.
//
// DECRPM (Bubble Tea mode 2026/2027): CSI ?2027;3$y → "027;3$y" / "2027;3$y"
// XTWINOPS cell pixels (our \x1b[16t probe): CSI 6;H;W t → "6;20;10t"
var (
	decrpmJunkSuffix   = regexp.MustCompile(`\??\d{1,6};\d{1,2}\$y`)
	cellSizeJunkSuffix = regexp.MustCompile(`(?:\d{1,4};){2}\d{1,4}t`)
)

func scrubTerminalJunk(q string) string {
	for {
		next := decrpmJunkSuffix.ReplaceAllString(q, "")
		next = cellSizeJunkSuffix.ReplaceAllString(next, "")
		if next == q {
			return q
		}
		q = next
	}
}

// isTerminalJunkKey drops a key/paste that is entirely a CSI reply fragment.
// Mixed payloads (e.g. "pika027;3$y") pass through; updateSearch scrubs after apply.
func isTerminalJunkKey(msg tea.KeyPressMsg) bool {
	text := msg.Text
	if text == "" {
		return false
	}
	if strings.ContainsRune(text, '\x1b') {
		return true
	}
	if len([]rune(text)) < 2 {
		return false
	}
	return scrubTerminalJunk(text) == ""
}
