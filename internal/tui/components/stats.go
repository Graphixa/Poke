package components

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/tui/style"
)

// StatsBars renders six base-stat bars plus total. Labels use the primary
// accent; bars and numbers stay plain.
func StatsBars(s domain.Stats) []string {
	primary := func(t string) string {
		return lipgloss.NewStyle().Foreground(style.PrimaryColor()).Render(t)
	}
	bar := func(label string, v int) string {
		const maxStat = 255
		filled := v * 12 / maxStat
		if filled > 12 {
			filled = 12
		}
		if filled < 0 {
			filled = 0
		}
		return primary(fmt.Sprintf("%-8s", label)) + fmt.Sprintf(" %s  %3d", strings.Repeat("█", filled)+strings.Repeat("░", 12-filled), v)
	}
	return []string{
		bar("HP", s.HP),
		bar("Attack", s.Attack),
		bar("Defense", s.Defense),
		bar("Sp.Atk", s.SpecialAttack),
		bar("Sp.Def", s.SpecialDefense),
		bar("Speed", s.Speed),
		"─────────────────────",
		primary(fmt.Sprintf("%-8s", "Total")) + fmt.Sprintf("              %3d", s.Total()),
	}
}

// AboutLines formats Height / Weight / Base Exp for the glance About card.
// When width is large enough, all three share one row; otherwise Base Exp wraps.
func AboutLines(height, weight string, baseExp int, innerW int) []string {
	base := fmt.Sprintf("Base Exp  %d", baseExp)
	row1 := fmt.Sprintf("Height  %s    Weight  %s    %s", height, weight, base)
	if innerW > 0 && lipgloss.Width(row1) <= innerW {
		return []string{row1}
	}
	return []string{
		fmt.Sprintf("Height  %s    Weight  %s", height, weight),
		base,
	}
}
