package components

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Poke/internal/render/sprite"
	"github.com/Graphixa/Poke/internal/textwidth"
	"github.com/Graphixa/Poke/internal/tui/style"
)

// ViewSwitcher renders a centre-aligned · switcher with equal-width chips.
func ViewSwitcher(active int, labels []string, width int) string {
	return centreSwitcher(active, labels, width)
}

// switcherChip keeps active/inactive the same width (pad both sides; bg only when active).
func switcherChip(label string, active bool) string {
	if active {
		return lipgloss.NewStyle().
			Foreground(style.BaseColor()).
			Background(style.ComponentsColor()).
			Bold(true).
			Padding(0, 1).
			Render(label)
	}
	return mutedSty().Render(" " + label + " ")
}

func centreSwitcher(active int, labels []string, width int) string {
	parts := make([]string, 0, len(labels))
	for i, label := range labels {
		parts = append(parts, switcherChip(label, i == active))
	}
	line := strings.Join(parts, mutedSty().Render("·"))
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

// DialogGeom is the dossier frame geometry used by both render and key handling.
type DialogGeom struct {
	BodyH  int
	BodyW  int // usable body width
	InnerW int
}

// DialogLayout returns dossier geometry. Sections are always a horizontal header
// (←/→); body scrolls with ↑/↓. termH is full terminal height; one row is footer.
func DialogLayout(termW, termH int, sectionLabels []string) DialogGeom {
	_ = sectionLabels
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
	innerW := termW - 4
	if innerW < 20 {
		innerW = 20
	}
	h := frameH - 6 // border + padding + section header + divider
	if h < 1 {
		h = 1
	}
	return DialogGeom{BodyH: h, BodyW: innerW, InnerW: innerW}
}

// DossierChrome is stationary chrome around the scroll body (not in the border title).
type DossierChrome struct {
	ContextLines []string // notice rows above body
	StickyLines  []string // e.g. move table header; not scrolled
	PositionHint string   // e.g. "Rows 8–24 of 42"; pinned below scroll
	TopRight     string   // right-aligned top-border chips (M/G), like browse F4
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
	card = stampDialogTitle(card, title, termW, chrome.TopRight)

	if footer == "" {
		return card
	}
	footer = textwidth.PadRight(textwidth.Truncate(footer, termW), termW)
	return card + "\n" + footerSty().Render(footer)
}

func renderDossierInner(section int, labels, bodyLines []string, bodyOff int, geom DialogGeom, frameH int, chrome DossierChrome) string {
	_ = frameH
	innerW := geom.InnerW
	wrapRow := func(row string) string {
		return " " + textwidth.PadRight(textwidth.Truncate(row, innerW), innerW) + " "
	}
	reserve := chromeBodyReserve(chrome)
	sec := lipgloss.NewStyle().Foreground(style.SecondaryColor())
	muted := lipgloss.NewStyle().Foreground(style.SecondaryColor())
	scrollH := geom.BodyH - reserve
	if scrollH < 1 {
		scrollH = 1
	}
	header := dialogSectionHeader(section, labels, innerW)
	divider := lipgloss.NewStyle().
		Foreground(style.BorderColor()).
		Render(strings.Repeat("─", maxInt(1, innerW)))
	lines := []string{wrapRow(header), wrapRow(divider)}
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

// dialogSectionHeader is a full · switcher when it fits, else compact ‹ n/N Label ›.
func dialogSectionHeader(active int, labels []string, width int) string {
	if len(labels) == 0 {
		return strings.Repeat(" ", width)
	}
	if active < 0 || active >= len(labels) {
		active = 0
	}
	parts := make([]string, 0, len(labels))
	for i, label := range labels {
		parts = append(parts, switcherChip(label, i == active))
	}
	full := strings.Join(parts, mutedSty().Render("·"))
	if lipgloss.Width(full) <= width {
		return lipgloss.PlaceHorizontal(width, lipgloss.Center, full)
	}
	pager := fmt.Sprintf("‹  %d/%d  %s  ›", active+1, len(labels), labels[active])
	pager = textwidth.Truncate(pager, width)
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, boldSty().Render(pager))
}

func stampDialogTitle(card, title string, width int, topRight string) string {
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
	horiz := border.Render("─")
	prefix := border.Render("╭─") + titleSt.Render(label) + horiz
	remain := tw - lipgloss.Width(prefix) - 1 // minus ╮
	if remain < 0 {
		remain = 0
	}
	topRight = strings.TrimSpace(topRight)
	if topRight == "" {
		lines[0] = prefix + strings.Repeat(horiz, remain) + border.Render("╮")
		return strings.Join(lines, "\n")
	}
	// Right-align chips before ╮ (same idea as browse F4): ──  M  method  G  choose game  ─╮
	suffix := " " + topRight + "  "
	sw := lipgloss.Width(suffix) + 1 // +1 trailing horiz
	if sw >= remain {
		lines[0] = prefix + lipgloss.NewStyle().MaxWidth(remain).Render(suffix) + border.Render("╮")
	} else {
		lines[0] = prefix + strings.Repeat(horiz, remain-sw) + suffix + horiz + border.Render("╮")
	}
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
