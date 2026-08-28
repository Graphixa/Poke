package tui

import "fmt"

const warmFooterRightWidth = 38

func formatWarmFooterRight(stage string, done, total int) string {
	if total <= 0 || done >= total {
		return ""
	}
	switch stage {
	case "native":
		return formatNativeCacheBar(done, total)
	case "artwork":
		return fmt.Sprintf("Downloading artwork %d/%d", done, total)
	default:
		if stage != "" {
			return fmt.Sprintf("%s %d/%d", stage, done, total)
		}
		return fmt.Sprintf("warming %d/%d", done, total)
	}
}

func formatNativeCacheBar(done, total int) string {
	if total <= 0 {
		return ""
	}
	pct := done * 100 / total
	const barW = 8
	filled := done * barW / total
	if filled > barW {
		filled = barW
	}
	bar := repeatRune('█', filled) + repeatRune('░', barW-filled)
	return fmt.Sprintf("Building image cache [%s] %d%%", bar, pct)
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
