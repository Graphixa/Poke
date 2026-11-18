package components

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Poke/internal/domain"
	"github.com/Graphixa/Poke/internal/output"
	"github.com/Graphixa/Poke/internal/tui/style"
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

// GlanceAbilityGrid lays out glance abilities as:
//
//	[Slot 1] [Slot 2]   — fixed half-width cells (empty slot still shows label)
//	[Hidden ........]   — full-width bottom row (no Slot 4 / no "(hidden)" suffix)
func GlanceAbilityGrid(abilities []domain.AbilitySlot, innerW int) []string {
	var slot1, slot2, hidden *domain.AbilitySlot
	for i := range abilities {
		a := &abilities[i]
		switch {
		case a.IsHidden || a.Slot >= 3:
			if hidden == nil {
				hidden = a
			}
		case a.Slot == 2:
			slot2 = a
		case a.Slot == 1:
			slot1 = a
		default:
			if slot1 == nil {
				slot1 = a
			} else if slot2 == nil {
				slot2 = a
			}
		}
	}
	const gap = 2
	if innerW < 16 {
		innerW = 16
	}
	cellW := (innerW - gap) / 2
	if cellW < 8 {
		cellW = 8
	}
	fit := func(s string, w int) string {
		if lipgloss.Width(s) > w {
			return lipgloss.NewStyle().MaxWidth(w).Render(s)
		}
		for lipgloss.Width(s) < w {
			s += " "
		}
		return s
	}
	primary := func(label string) string {
		return lipgloss.NewStyle().Foreground(style.PrimaryColor()).Render(label)
	}
	regular := func(label string, a *domain.AbilitySlot) string {
		if a == nil {
			return fit(primary(label)+"  —", cellW)
		}
		return fit(primary(label)+"  "+output.TitleFromSlug(a.Name), cellW)
	}
	var hiddenLine string
	if hidden == nil {
		hiddenLine = fit(primary("Hidden")+"  —", innerW)
	} else {
		hiddenLine = fit(primary("Hidden")+"  "+output.TitleFromSlug(hidden.Name), innerW)
	}
	pad := strings.Repeat(" ", gap)
	return []string{
		regular("Slot 1", slot1) + pad + regular("Slot 2", slot2),
		hiddenLine,
	}
}
