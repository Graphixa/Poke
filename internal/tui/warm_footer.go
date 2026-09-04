package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Pokedex/internal/tui/components"
	"github.com/Graphixa/Pokedex/internal/tui/style"
)

func formatWarmFooterRight(stage WarmStage, done, total int) string {
	if total <= 0 || done >= total {
		return ""
	}
	if stage != WarmStageNative {
		return ""
	}
	return formatNativeImageCacheFooter(done, total)
}

func warmBarStyles() (label, fill, track, count lipgloss.Style) {
	pal := style.CurrentPalette()
	label = lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Placeholders)).Faint(true)
	fill = lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Primary))
	track = lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Secondary)).Faint(true)
	count = lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Secondary))
	return label, fill, track, count
}

func formatNativeImageCacheFooter(done, total int) string {
	if total <= 0 || done >= total {
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
	line := labelSty.Render("Image Cache") + " " + bar + " " + count
	return fitWarmFooterRight(line)
}

func fitWarmFooterRight(s string) string {
	if lipgloss.Width(s) <= components.FooterRightWidth {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(components.FooterRightWidth).Render(s)
}

// FormatNativeCacheBarForTest exports the native progress label for tests.
func FormatNativeCacheBarForTest(_ int, done, total int) string {
	return formatNativeImageCacheFooter(done, total)
}
