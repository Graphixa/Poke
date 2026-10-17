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

// SpriteSizeCue tells the user the terminal is too small for the sprite frame.
func SpriteSizeCue(width, height int) string {
	minSide := sprite.SmallestFrameBucket()
	if width < 1 {
		width = minSide
	}
	if height < 1 {
		height = minSide / sprite.CellAspectY
	}
	msg := fmt.Sprintf("Widen / taller terminal\nfor %d×%d sprite", minSide, minSide/sprite.CellAspectY)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, mutedSty().Render(msg))
}

const dialogMinBodyWide = 56

// DialogGeom is the dossier frame geometry used by both render and key handling.
type DialogGeom struct {
	Stacked bool
	BodyH   int
	BodyW   int // usable body column width (right pane or full stacked width)
	InnerW  int
}

// DialogLayout returns whether the dossier uses the stacked pager vs side rail.
// termH is the full terminal height; one row is reserved for the footer.
func DialogLayout(termW, termH int, sectionLabels []string) DialogGeom {
	if termW < 20 {
		termW = 20
	}
	if termH < 10 {
		termH = 10
	}
	frameH := termH - 1 // footer
	if frameH < 8 {
		frameH = termH
	}
	dialogW := termW
	dialogH := frameH
	innerW := dialogW - 4
	if innerW < 20 {
		innerW = 20
	}
	railW := dialogRailWidth(sectionLabels)
	stacked := innerW-railW-3 < dialogMinBodyWide
	h := dialogH - 4 // border + vertical padding
	if stacked {
		h = dialogH - 6 // pager + divider
	}
	if h < 1 {
		h = 1
	}
	bodyW := innerW
	if !stacked {
		bodyW = innerW - railW - 3
		if bodyW < 20 {
			bodyW = 20
		}
	}
	return DialogGeom{Stacked: stacked, BodyH: h, BodyW: bodyW, InnerW: innerW}
}

// DossierChrome is stationary chrome around the scroll body (not in the border title).
type DossierChrome struct {
	ContextLines []string // Form / Game / Method / notice
	StickyLines  []string // e.g. move table header; not scrolled
	PositionHint string   // e.g. "Rows 8–24 of 42"; pinned below scroll
	SidebarFocus bool     // strong section highlight when sidebar owns focus
}

// DossierFrame renders a full-terminal dossier (title card + footer), not an overlay.
func DossierFrame(title string, section int, sectionLabels, bodyLines []string, bodyOff, termW, termH int, footer string) string {
	return DossierFrameEx(title, section, sectionLabels, bodyLines, bodyOff, termW, termH, footer, DossierChrome{})
}

// DossierFrameEx is DossierFrame with context chrome.
func DossierFrameEx(title string, section int, sectionLabels, bodyLines []string, bodyOff, termW, termH int, footer string, chrome DossierChrome) string {
	if termW < 20 {
		termW = 20
	}
	if termH < 10 {
		termH = 10
	}
	geom := DialogLayout(termW, termH, sectionLabels)
	frameH := termH - 1
	if frameH < 8 {
		frameH = termH
	}
	inner := renderDossierInner(section, sectionLabels, bodyLines, bodyOff, geom, frameH, chrome)

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(style.BorderColor()).
		Padding(1, 0).
		Width(termW).
		Height(frameH).
		Render(inner)
	card = stampDialogTitle(card, title, termW)

	if footer == "" {
		return card
	}
	footer = textwidth.PadRight(textwidth.Truncate(footer, termW), termW)
	return card + "\n" + footerSty().Render(footer)
}

func renderDossierInner(section int, labels, bodyLines []string, bodyOff int, geom DialogGeom, frameH int, chrome DossierChrome) string {
	innerW := geom.InnerW
	wrapRow := func(row string) string {
		return " " + textwidth.PadRight(textwidth.Truncate(row, innerW), innerW) + " "
	}
	reserve := chromeBodyReserve(chrome)
	sec := lipgloss.NewStyle().Foreground(style.SecondaryColor())
	muted := lipgloss.NewStyle().Foreground(style.SecondaryColor())
	if geom.Stacked {
		scrollH := geom.BodyH - reserve
		if scrollH < 1 {
			scrollH = 1
		}
		pager := dialogSectionPager(section, labels, innerW)
		divider := lipgloss.NewStyle().
			Foreground(style.BorderColor()).
			Render(strings.Repeat("─", maxInt(1, innerW)))
		lines := []string{wrapRow(pager), wrapRow(divider)}
		for _, row := range chrome.ContextLines {
			lines = append(lines, wrapRow(sec.Render(row)))
		}
		if len(chrome.ContextLines) > 0 {
			lines = append(lines, wrapRow(""))
		}
		for _, row := range chrome.StickyLines {
			lines = append(lines, wrapRow(row))
		}
		body := clipLines(bodyLines, bodyOff, scrollH, innerW)
		for _, line := range strings.Split(body, "\n") {
			lines = append(lines, wrapRow(line))
		}
		if chrome.PositionHint != "" {
			lines = append(lines, wrapRow(muted.Render(chrome.PositionHint)))
		}
		return strings.Join(lines, "\n")
	}

	railW := dialogRailWidth(labels)
	bodyH := geom.BodyH
	bodyW := geom.BodyW
	scrollH := bodyH - reserve
	if scrollH < 1 {
		scrollH = 1
	}
	var parts []string
	for _, row := range chrome.ContextLines {
		parts = append(parts, textwidth.PadRight(textwidth.Truncate(sec.Render(row), bodyW), bodyW))
	}
	if len(chrome.ContextLines) > 0 {
		parts = append(parts, strings.Repeat(" ", bodyW))
	}
	for _, row := range chrome.StickyLines {
		parts = append(parts, textwidth.PadRight(textwidth.Truncate(row, bodyW), bodyW))
	}
	parts = append(parts, clipLines(bodyLines, bodyOff, scrollH, bodyW))
	if chrome.PositionHint != "" {
		parts = append(parts, textwidth.PadRight(textwidth.Truncate(muted.Render(chrome.PositionHint), bodyW), bodyW))
	}
	bodyBlock := strings.Join(parts, "\n")
	blank := strings.Repeat(" ", bodyW)
	for strings.Count(bodyBlock, "\n")+1 < bodyH {
		bodyBlock += "\n" + blank
	}
	rail := dialogSectionRail(section, labels, railW, bodyH, chrome.SidebarFocus)
	separator := lipgloss.NewStyle().
		Foreground(style.BorderColor()).
		Render(strings.Repeat("│\n", bodyH-1) + "│")
	content := lipgloss.JoinHorizontal(lipgloss.Top, rail, " ", separator, " ", bodyBlock)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = " " + line + " "
	}
	_ = frameH
	return strings.Join(lines, "\n")
}

func chromeBodyReserve(chrome DossierChrome) int {
	n := 0
	if len(chrome.ContextLines) > 0 {
		n += len(chrome.ContextLines) + 1
	}
	n += len(chrome.StickyLines)
	if chrome.PositionHint != "" {
		n++
	}
	return n
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

func dialogSectionRail(active int, labels []string, width, height int, sidebarFocus bool) string {
	lines := make([]string, 0, height)
	for i, label := range labels {
		var prefix string
		var sty lipgloss.Style
		switch {
		case i == active && sidebarFocus:
			prefix = "▶ "
			sty = lipgloss.NewStyle().Foreground(style.PrimaryColor()).Bold(true)
		case i == active:
			prefix = "● "
			sty = lipgloss.NewStyle().Foreground(style.SecondaryColor())
		default:
			prefix = "  "
			sty = lipgloss.NewStyle().Foreground(style.SecondaryColor())
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
