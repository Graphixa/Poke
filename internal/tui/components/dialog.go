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
		width = sprite.MinFrameSize
	}
	if height < 1 {
		height = sprite.MinFrameSize / sprite.CellAspectY
	}
	msg := fmt.Sprintf("Widen / taller terminal\nfor %d×%d sprite", sprite.MinFrameSize, sprite.MinFrameSize/sprite.CellAspectY)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, mutedSty().Render(msg))
}

const (
	dialogWidePadX    = 7
	dialogWidePadY    = 2
	dialogMinBodyWide = 56
)

// OverlayDialog composites an inset detail dialog over a dimmed background.
func OverlayDialog(background, title string, section int, sectionLabels, bodyLines []string, bodyOff, termW, termH int) string {
	if termW < 20 {
		termW = 20
	}
	if termH < 10 {
		termH = 10
	}
	padX, padY := dialogInsets(termW, termH)
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
	inner := renderDialogInner(section, sectionLabels, bodyLines, bodyOff, innerW, dialogH)

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(style.BorderColor()).
		Padding(1, 0).
		Width(dialogW).
		Height(dialogH).
		Render(inner)

	// Integrate title on top border when possible.
	card = stampDialogTitle(card, title, dialogW)

	dimmed := lipgloss.NewStyle().Faint(true).Render(background)
	bgLayer := lipgloss.NewLayer(dimmed)
	dlgLayer := lipgloss.NewLayer(card).X(padX).Y(padY).Z(1)
	return lipgloss.NewCompositor(bgLayer, dlgLayer).Render()
}

func dialogInsets(termW, termH int) (int, int) {
	padX := dialogWidePadX
	switch {
	case termW < 80:
		padX = 2
	case termW < 90:
		padX = 4
	case termW < 100:
		padX = 6
	}
	padY := dialogWidePadY
	if termH < 18 {
		padY = 1
	}
	return padX, padY
}

func renderDialogInner(section int, labels, bodyLines []string, bodyOff, innerW, dialogH int) string {
	railW := dialogRailWidth(labels)
	// The rendered rows own their horizontal padding. This keeps it visible
	// after the card is joined and clipped instead of relying on style width.
	wrapRow := func(row string) string {
		return " " + textwidth.PadRight(textwidth.Truncate(row, innerW), innerW) + " "
	}
	// Reserve one cell on either side of the divider.
	stacked := innerW-railW-3 < dialogMinBodyWide
	if stacked {
		bodyH := maxInt(1, dialogH-6)
		pager := dialogSectionPager(section, labels, innerW)
		divider := lipgloss.NewStyle().
			Foreground(style.BorderColor()).
			Render(strings.Repeat("─", maxInt(1, innerW)))
		body := clipLines(bodyLines, bodyOff, bodyH, innerW)
		lines := []string{wrapRow(pager), wrapRow(divider)}
		for _, line := range strings.Split(body, "\n") {
			lines = append(lines, wrapRow(line))
		}
		return strings.Join(lines, "\n")
	}

	bodyH := maxInt(1, dialogH-4)
	rail := dialogSectionRail(section, labels, railW, bodyH)
	bodyW := innerW - railW - 3
	body := clipLines(bodyLines, bodyOff, bodyH, bodyW)
	separator := lipgloss.NewStyle().
		Foreground(style.BorderColor()).
		Render(strings.Repeat("│\n", bodyH-1) + "│")
	content := lipgloss.JoinHorizontal(lipgloss.Top, rail, " ", separator, " ", body)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = " " + line + " "
	}
	return strings.Join(lines, "\n")
}

func dialogRailWidth(labels []string) int {
	width := 16
	for _, label := range labels {
		if w := textwidth.Width(label) + 4; w > width {
			width = w
		}
	}
	if width > 22 {
		width = 22
	}
	return width
}

func dialogSectionRail(active int, labels []string, width, height int) string {
	lines := make([]string, 0, height)
	for i, label := range labels {
		prefix := "  "
		sty := lipgloss.NewStyle().Foreground(style.SecondaryColor())
		if i == active {
			prefix = "> "
			sty = lipgloss.NewStyle().Foreground(style.PrimaryColor()).Bold(true)
		}
		line := textwidth.PadRight(textwidth.Truncate(prefix+label, width), width)
		lines = append(lines, sty.Render(line))
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

func dialogSectionPager(active int, labels []string, width int) string {
	if len(labels) == 0 {
		return strings.Repeat(" ", width)
	}
	if active < 0 || active >= len(labels) {
		active = 0
	}
	pager := fmt.Sprintf("‹  %d/%d  %s  ›", active+1, len(labels), labels[active])
	pager = textwidth.Truncate(pager, width)
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, boldSty().Render(pager))
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
	// Match RenderCombinedFrame: primary, bold title text on the themed border.
	titleSt := lipgloss.NewStyle().Foreground(style.PrimaryColor()).Bold(true)
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
