package preload

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Poke/internal/tui/style"
)

// Screen is the centered loading frame. bar is an already-rendered progress bar.
func Screen(width, height int, bar, headline, count, detail, errText string) string {
	pal := style.CurrentPalette()
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(pal.Primary)).Render("Poke")
	head := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(pal.Components)).Render(headline)
	sub := lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Placeholders))
	body := lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Components))
	errSt := lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Error))

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n%s\n\n%s\n", title, head, bar)
	if count != "" {
		fmt.Fprintf(&b, "%s\n", sub.Render(count))
	}
	if detail != "" {
		fmt.Fprintf(&b, "%s\n", body.Render(detail))
	}
	if errText != "" {
		fmt.Fprintf(&b, "\n%s", errSt.Render(errText))
	}
	if width < 1 {
		width = 80
	}
	if height < 1 {
		height = 24
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, b.String())
}

// Count renders "55 of 1,025". Total <= 0 yields an empty string.
func Count(done, total int) string {
	if total <= 0 {
		return ""
	}
	return commas(done) + " of " + commas(total)
}

func commas(n int) string {
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre == 0 {
		pre = 3
	}
	b.WriteString(s[:pre])
	for i := pre; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}
