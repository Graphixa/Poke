package components

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/textwidth"
	"github.com/Graphixa/Pokedex/internal/tui/style"
)

// ViewSwitcher renders `[ Sprite ] · Summary · Stats · Evolution` centre-aligned.
func ViewSwitcher(active int, labels []string, width int) string {
	return centreSwitcher(active, labels, width)
}

// DialogSectionSwitcher is the same pattern for dialog sections.
func DialogSectionSwitcher(active int, labels []string, width int) string {
	return centreSwitcher(active, labels, width)
}

func centreSwitcher(active int, labels []string, width int) string {
	parts := make([]string, 0, len(labels))
	for i, label := range labels {
		if i == active {
			parts = append(parts, boldSty().Render("[ "+label+" ]"))
		} else {
			parts = append(parts, mutedSty().Render(label))
		}
	}
	line := strings.Join(parts, mutedSty().Render(" · "))
	if width <= 0 {
		return line
	}
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, line)
}

// SpriteSizeCue tells the user the terminal is too small for the fixed sprite frame.
func SpriteSizeCue(width, height int) string {
	if width < 1 {
		width = sprite.MaxFrameSize
	}
	if height < 1 {
		height = sprite.MaxFrameSize
	}
	msg := fmt.Sprintf("Widen / taller terminal\nfor %d×%d sprite", sprite.FrameWidth, sprite.FrameHeight)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, mutedSty().Render(msg))
}

const (
	dialogPadX = 3
	dialogPadY = 1
)

// OverlayDialog composites a near-fullscreen detail dialog over a dimmed background.
func OverlayDialog(background, title string, section int, sectionLabels, bodyLines []string, bodyOff, termW, termH int) string {
	if termW < 20 {
		termW = 20
	}
	if termH < 10 {
		termH = 10
	}
	padX, padY := dialogPadX, dialogPadY
	dialogW := termW - 2*padX
	dialogH := termH - 2*padY
	if dialogW < 40 {
		dialogW = min(termW, 40)
		padX = maxInt(0, (termW-dialogW)/2)
	}
	if dialogH < 12 {
		dialogH = min(termH, 12)
		padY = maxInt(0, (termH-dialogH)/2)
	}

	innerW := dialogW - 4
	if innerW < 20 {
		innerW = 20
	}
	switcher := DialogSectionSwitcher(section, sectionLabels, innerW)
	divider := lipgloss.NewStyle().Foreground(style.BorderColor()).Render(strings.Repeat("─", maxInt(1, innerW)))
	footer := mutedSty().Render("Tab section · ↑↓ scroll · Esc close")

	headerH := 4 // title row + switcher + divider + footer spacing accounted below
	bodyH := dialogH - headerH - 3
	if bodyH < 4 {
		bodyH = 4
	}
	body := clipLines(bodyLines, bodyOff, bodyH, innerW)

	var inner strings.Builder
	inner.WriteString(switcher)
	inner.WriteByte('\n')
	inner.WriteString(divider)
	inner.WriteByte('\n')
	inner.WriteString(body)
	inner.WriteByte('\n')
	inner.WriteString(footer)

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(style.BorderColor()).
		Padding(0, 1).
		Width(dialogW).
		Height(dialogH).
		Render(inner.String())

	// Integrate title on top border when possible.
	card = stampDialogTitle(card, title, dialogW)

	dimmed := lipgloss.NewStyle().Faint(true).Render(background)
	bgLayer := lipgloss.NewLayer(dimmed)
	dlgLayer := lipgloss.NewLayer(card).X(padX).Y(padY).Z(1)
	return lipgloss.NewCompositor(bgLayer, dlgLayer).Render()
}

func stampDialogTitle(card, title string, width int) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return card
	}
	lines := strings.Split(card, "\n")
	if len(lines) == 0 {
		return card
	}
	tw := lipgloss.Width(lines[0])
	if tw <= 0 {
		tw = width
	}
	label := " " + title + " "
	maxLabel := tw - 4
	if maxLabel < 3 {
		return card
	}
	if textwidth.Width(label) > maxLabel {
		label = textwidth.Truncate(label, maxLabel)
	}
	border := lipgloss.NewStyle().Foreground(style.BorderColor())
	titleSt := boldSty()
	prefix := border.Render("╭─") + titleSt.Render(label) + border.Render("─")
	remain := tw - lipgloss.Width(prefix) - 1
	if remain < 0 {
		remain = 0
	}
	lines[0] = prefix + strings.Repeat(border.Render("─"), remain) + border.Render("╮")
	return strings.Join(lines, "\n")
}

func clipLines(lines []string, off, height, width int) string {
	if off < 0 {
		off = 0
	}
	out := make([]string, 0, height)
	for i := 0; i < height; i++ {
		idx := off + i
		if idx < len(lines) {
			out = append(out, textwidth.PadRight(textwidth.Truncate(lines[idx], width), width))
		} else {
			out = append(out, strings.Repeat(" ", width))
		}
	}
	return strings.Join(out, "\n")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
