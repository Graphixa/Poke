package components

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Poke/internal/textwidth"
	"github.com/Graphixa/Poke/internal/tui/style"
)

// InnerCard draws a rounded box of fixed width×height with default border ink.
func InnerCard(title string, width, height int, bodyLines []string) string {
	return InnerCardBorder(title, width, height, bodyLines, style.BorderColor())
}

// InnerCardBorder is InnerCard with an explicit border colour (focus chrome).
func InnerCardBorder(title string, width, height int, bodyLines []string, borderFg color.Color) string {
	if width < 4 {
		width = 4
	}
	if height < 2 {
		height = 2
	}
	innerW := width - 2
	innerH := height - 2
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}

	border := lipgloss.NewStyle().Foreground(borderFg)
	horiz := border.Render("─")
	vert := border.Render("│")
	topL := border.Render("╭")
	topR := border.Render("╮")
	botL := border.Render("╰")
	botR := border.Render("╯")

	var top string
	title = strings.TrimSpace(title)
	if title == "" {
		top = topL + strings.Repeat(horiz, width-2) + topR
	} else {
		titleSt := lipgloss.NewStyle().Foreground(style.PrimaryColor()).Bold(true)
		label := " " + title + " "
		maxLabel := width - 4
		if maxLabel < 3 {
			top = topL + strings.Repeat(horiz, width-2) + topR
		} else {
			if textwidth.Width(label) > maxLabel {
				label = textwidth.Truncate(label, maxLabel)
			}
			prefix := topL + horiz + titleSt.Render(label) + horiz
			remain := width - lipgloss.Width(prefix) - 1
			if remain < 0 {
				remain = 0
			}
			top = prefix + strings.Repeat(horiz, remain) + topR
		}
	}
	bottom := botL + strings.Repeat(horiz, width-2) + botR

	lines := make([]string, 0, height)
	lines = append(lines, top)
	for i := 0; i < innerH; i++ {
		cell := ""
		if i < len(bodyLines) {
			cell = bodyLines[i]
		}
		cell = textwidth.PadRight(textwidth.Truncate(cell, innerW), innerW)
		lines = append(lines, vert+cell+vert)
	}
	lines = append(lines, bottom)
	return strings.Join(lines, "\n")
}
