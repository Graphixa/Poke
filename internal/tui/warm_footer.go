package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Pokedex/internal/tui/style"
)

const warmFooterRightWidth = 38

func formatWarmFooterRight(stage string, done, total int) string {
	if total <= 0 || done >= total {
		return ""
	}
	switch stage {
	case "native":
		return formatNativeCacheBar(done, total)
	case "artwork":
		return formatArtworkCacheBar(done, total)
	default:
		if stage != "" {
			return formatWarmBar(stage, done, total)
		}
		return formatWarmBar("warming", done, total)
	}
}

func warmBarStyles() (label, fill, track, count lipgloss.Style) {
	pal := style.CurrentPalette()
	label = lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Placeholders)).Faint(true)
	fill = lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Primary))
	track = lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Secondary)).Faint(true)
	count = lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Secondary))
	return label, fill, track, count
}

func formatNativeCacheBar(done, total int) string {
	return formatWarmBar("Cache", done, total)
}

func formatArtworkCacheBar(done, total int) string {
	return formatWarmBar("Artwork", done, total)
}

func formatWarmBar(labelText string, done, total int) string {
	if total <= 0 {
		return ""
	}
	labelSty, fillSty, trackSty, countSty := warmBarStyles()
	const barW = 10
	filled := done * barW / total
	if filled > barW {
		filled = barW
	}
	bar := fillSty.Render(strings.Repeat("█", filled)) +
		trackSty.Render(strings.Repeat("░", barW-filled))
	count := countSty.Render(fmt.Sprintf("%d/%d", done, total))
	line := labelSty.Render(labelText) + " " + bar + " " + count
	return fitWarmFooterRight(line)
}

func fitWarmFooterRight(s string) string {
	if lipgloss.Width(s) <= warmFooterRightWidth {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(warmFooterRightWidth).Render(s)
}

func repeatRune(r rune, n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]rune, n)
	for i := range b {
		b[i] = r
	}
	return string(b)
}

// FormatNativeCacheBarForTest exports the native progress label for tests.
func FormatNativeCacheBarForTest(done, total int) string {
	return formatNativeCacheBar(done, total)
}
