package textwidth

import (
	"charm.land/lipgloss/v2"
)

// Width returns the terminal display width of s, ignoring ANSI escape sequences.
func Width(s string) int {
	return lipgloss.Width(s)
}

// Truncate shortens s to at most width display columns, appending an ellipsis
// when truncated. Safe for ANSI-styled strings (uses lipgloss Width).
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
	// lipgloss MaxWidth truncates visually without splitting escape sequences.
	return lipgloss.NewStyle().MaxWidth(width-1).Render(s) + "…"
}

// PadRight pads s with spaces to width display columns (ANSI-aware).
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
