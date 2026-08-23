package textwidth

import (
	"github.com/mattn/go-runewidth"
)

// Width returns the terminal display width of s (handles wide runes).
func Width(s string) int {
	return runewidth.StringWidth(s)
}

// Truncate shortens s to at most width display columns, appending an ellipsis
// when truncated. ANSI-free plain text only.
func Truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if Width(s) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	return runewidth.Truncate(s, width, "…")
}

// PadRight pads s with spaces to width display columns.
func PadRight(s string, width int) string {
	w := Width(s)
	if w >= width {
		return s
	}
	return s + spaces(width-w)
}

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}
