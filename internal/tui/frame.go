package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Pokedex/internal/tui/style"
)

// PanelLayout is the FontGet-style split used by browse and theme TUIs.
type PanelLayout struct {
	LeftWidth    int
	RightWidth   int
	PanelHeight  int
	ContentWidth int // left+sep+right
}

// CalculatePanelLayout computes a ~1/3–2/3 split (wireframe / FontGet theme).
// margin is total horizontal margin (both sides). header/footer are reserved rows outside the frame.
func CalculatePanelLayout(termW, termH, headerH, footerH, margin, sep int) PanelLayout {
	if sep <= 0 {
		sep = 1
	}
	if margin < 0 {
		margin = 0
	}
	availW := termW - margin
	availH := termH - headerH - footerH
	if availW < 40 {
		availW = 40
	}
	if availH < 10 {
		availH = 10
	}
	panelArea := availW - sep
	if panelArea < 0 {
		panelArea = 0
	}
	// Wireframe: left ~1/3. FontGet theme uses 30%; we use 1/3 for browse parity.
	left := panelArea / 3
	right := panelArea - left
	if left < 20 {
		left = 20
	}
	if right < 20 {
		right = 20
	}
	if left+right+sep > termW && termW > sep {
		panelArea = termW - sep
		left = panelArea / 3
		right = panelArea - left
	}
	return PanelLayout{
		LeftWidth:    left,
		RightWidth:   right,
		PanelHeight:  availH,
		ContentWidth: left + sep + right,
	}
}

// RenderCombinedFrame draws a shared outer border with title in the top edge:
//
//	╭─ Title ────────┬────────── topRight ─╮
//	│ left           │ right               │
//	╰────────────────┴─────────────────────╯
//
// topRight is optional (e.g. FontGet-style "F4 favourite" chip), right-aligned
// in the right pane's top border. Matches FontGet browse chrome.
func RenderCombinedFrame(title string, leftW, rightW, height int, leftContent, rightContent, topRight string) string {
	if leftW < 4 {
		leftW = 4
	}
	if rightW < 4 {
		rightW = 4
	}
	if height < 3 {
		height = 3
	}

	leftInnerW := leftW - 1 // minus left border; content fills to separator
	rightInnerW := rightW - 1
	if leftInnerW < 0 {
		leftInnerW = 0
	}
	if rightInnerW < 0 {
		rightInnerW = 0
	}
	contentH := height - 2
	if contentH < 1 {
		contentH = 1
	}

	borderFg := style.BorderColor()
	titleStyle := lipgloss.NewStyle().Foreground(style.PrimaryColor()).Bold(true)
	borderCh := lipgloss.NewStyle().Foreground(borderFg)
	sepCh := lipgloss.NewStyle().Foreground(borderFg)

	topL := borderCh.Render("╭")
	topR := borderCh.Render("╮")
	botL := borderCh.Render("╰")
	botR := borderCh.Render("╯")
	topTee := borderCh.Render("┬")
	botTee := borderCh.Render("┴")
	vert := borderCh.Render("│")
	sep := sepCh.Render("│")
	horiz := borderCh.Render("─")

	titleRendered := titleStyle.Render(title)

	// ╭─ Title ────┬…  left segment visual width must equal leftW
	prefix := topL + horiz + " " + titleRendered + " "
	prefixW := lipgloss.Width(prefix)
	remainLeft := leftW - prefixW
	if remainLeft < 0 {
		// Truncate title to fit.
		maxTitle := leftW - lipgloss.Width(topL+horiz+"  ") - 1
		if maxTitle < 1 {
			maxTitle = 1
		}
		titleRendered = titleStyle.Render(lipgloss.NewStyle().MaxWidth(maxTitle).Render(title))
		prefix = topL + horiz + " " + titleRendered + " "
		prefixW = lipgloss.Width(prefix)
		remainLeft = leftW - prefixW
		if remainLeft < 0 {
			remainLeft = 0
		}
	}
	rightTop := frameRightTop(horiz, topR, rightW, topRight)
	top := prefix + strings.Repeat(horiz, remainLeft) + topTee + rightTop
	// Fix total if right side drifted.
	want := leftW + 1 + rightW
	if lipgloss.Width(top) != want {
		leftFill := leftW - prefixW
		if leftFill < 0 {
			leftFill = 0
		}
		top = prefix + strings.Repeat(horiz, leftFill) + topTee + frameRightTop(horiz, topR, rightW, topRight)
	}

	bottom := botL + strings.Repeat(horiz, leftW-1) + botTee + strings.Repeat(horiz, rightW-1) + botR

	leftLines := padLines(leftContent, leftInnerW, contentH)
	rightLines := padLines(rightContent, rightInnerW, contentH)

	var b strings.Builder
	b.WriteString(top)
	b.WriteByte('\n')
	for i := 0; i < contentH; i++ {
		b.WriteString(vert)
		b.WriteString(leftLines[i])
		b.WriteString(sep)
		b.WriteString(rightLines[i])
		b.WriteString(vert)
		if i < contentH-1 {
			b.WriteByte('\n')
		}
	}
	b.WriteByte('\n')
	b.WriteString(bottom)
	return b.String()
}

// frameRightTop is the right pane's top border (width rightW), ending in topR.
// When label is set, it is right-aligned before the corner.
func frameRightTop(horiz, topR string, rightW int, label string) string {
	inner := rightW - 1 // minus topR
	if inner < 0 {
		inner = 0
	}
	label = strings.TrimSpace(label)
	if label == "" {
		return strings.Repeat(horiz, inner) + topR
	}
	// Symmetric spaces around the chip, then one ─ before ╮:
	// ──  F4  favourite  ─╮
	suffix := " " + label + "  "
	sw := lipgloss.Width(suffix) + 1 // +1 for the trailing horiz
	if sw >= inner {
		return lipgloss.NewStyle().MaxWidth(inner).Render(suffix) + topR
	}
	return strings.Repeat(horiz, inner-sw) + suffix + horiz + topR
}

func padLines(content string, width, height int) []string {
	raw := strings.Split(strings.TrimRight(content, "\n"), "\n")
	out := make([]string, height)
	for i := 0; i < height; i++ {
		line := ""
		if i < len(raw) {
			line = raw[i]
		}
		w := lipgloss.Width(line)
		switch {
		case w < width:
			out[i] = line + strings.Repeat(" ", width-w)
		case w > width:
			out[i] = lipgloss.NewStyle().MaxWidth(width).Render(line)
			if lipgloss.Width(out[i]) < width {
				out[i] += strings.Repeat(" ", width-lipgloss.Width(out[i]))
			}
		default:
			out[i] = line
		}
	}
	return out
}

// FillTerminalArea constrains the view to the terminal size.
// Do not append CSI J (erase-below): with Sixel overlays that erase can punch
// holes through the graphic when the cursor is not at the true footer.
func FillTerminalArea(view string, width, height int) string {
	if width <= 0 || height <= 0 {
		return view
	}
	return lipgloss.NewStyle().Width(width).MaxWidth(width).MaxHeight(height).Render(view)
}
