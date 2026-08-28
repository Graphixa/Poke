package views

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/output"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/tui/components"
	"github.com/Graphixa/Pokedex/internal/tui/style"
)

// FrameRenderer is injected to avoid an import cycle with package tui.
type FrameRenderer struct {
	Calculate func(termW, termH, headerH, footerH, margin, sep int) (leftW, rightW, panelH int)
	Combine   func(title string, leftW, rightW, height int, left, right string) string
	Fill      func(view string, width, height int) string
}

// DefaultFrame uses a local fallback when Frame is nil (tests / snapshots).
var DefaultFrame FrameRenderer

func init() {
	DefaultFrame = FrameRenderer{
		Calculate: func(termW, termH, headerH, footerH, margin, sep int) (int, int, int) {
			if sep <= 0 {
				sep = 1
			}
			availW := termW - margin
			availH := termH - headerH - footerH
			if availW < 40 {
				availW = 40
			}
			if availH < 10 {
				availH = 10
			}
			area := availW - sep
			left := area / 3
			right := area - left
			if left < 20 {
				left = 20
			}
			if right < 20 {
				right = 20
			}
			return left, right, availH
		},
		Combine: fallbackCombine,
		Fill: func(view string, width, height int) string {
			if width <= 0 || height <= 0 {
				return view
			}
			return lipgloss.NewStyle().Width(width).MaxWidth(width).MaxHeight(height).Render(view)
		},
	}
}

// BrowseData is the pure view model for browse mode.
type BrowseData struct {
	Width, Height int

	Query      string
	SearchView string
	Rows       []components.ListRow
	ListOff    int
	ListRows   int

	Selected    domain.SpeciesSummary
	HasSelected bool

	ShowSprite    bool
	VerboseFooter bool
	OnePane       bool
	TooSmall      bool
	ShowListPane  bool

	BrowseView int
	ViewLabels []string
	BodySize   int // sprite longest-side step when BrowseView is Sprite
	ViewLines  []string

	// Panel column widths (list-first). Zero falls back to Calculate 1/3–2/3.
	LeftWidth  int
	RightWidth int

	HeaderTypes   string
	HeaderGenus   string
	HeaderLoading bool
	HeaderErr     string

	ShowHelp bool
	HelpBody string

	ShowDialog    bool
	DialogSection int
	DialogLabels  []string
	DialogLines   []string
	DialogOff     int

	SpriteANSI   string
	SpriteNative bool

	WarmHint string

	Frame *FrameRenderer
}

// Browse renders the sprite-first layout from tui-layout-concept.md.
func Browse(d BrowseData) string {
	if d.Width <= 0 {
		d.Width = 120
	}
	if d.Height <= 0 {
		d.Height = 50
	}
	fr := DefaultFrame
	if d.Frame != nil {
		fr = *d.Frame
	}

	if d.ShowHelp {
		return components.HelpOverlay(d.HelpBody, d.Width, d.Height)
	}
	if d.TooSmall {
		return components.TooSmall(d.Width, d.Height)
	}

	base := renderBrowseBase(d, fr)
	if d.ShowDialog {
		title := "Details"
		if d.HasSelected {
			title = fmt.Sprintf("%s %s", d.Selected.DisplayName, output.FormatDex(d.Selected.ID))
		}
		return components.OverlayDialog(base, title, d.DialogSection, d.DialogLabels, d.DialogLines, d.DialogOff, d.Width, d.Height)
	}
	return base
}

func renderBrowseBase(d BrowseData, fr FrameRenderer) string {
	footer := components.FooterSplit(d.VerboseFooter, d.Width, d.WarmHint)
	footerH := 1
	margin := 0
	title := "Pokedex"

	if d.OnePane {
		bodyH := d.Height - footerH
		var body string
		if d.ShowListPane || !d.HasSelected {
			search := renderSearch(d, d.Width)
			listH := bodyH - 1
			visible := visibleRows(d.Rows, d.ListOff, listH)
			body = search + "\n" + components.List(visible, d.Width, listH, abbreviate)
		} else {
			body = renderRight(d, d.Width, bodyH)
		}
		out := body + "\n" + footer
		if fr.Fill != nil {
			return fr.Fill(out, d.Width, d.Height)
		}
		return out
	}

	leftW, rightW, panelH := fr.Calculate(d.Width, d.Height, 0, footerH, margin, 1)
	if d.LeftWidth > 0 && d.RightWidth > 0 {
		leftW, rightW = d.LeftWidth, d.RightWidth
		panelH = d.Height - footerH
		if panelH < 10 {
			panelH = 10
		}
	}
	leftContentW := leftW - 1
	rightContentW := rightW - 1
	if leftContentW < 10 {
		leftContentW = 10
	}
	if rightContentW < 10 {
		rightContentW = 10
	}
	innerH := panelH - 2
	if innerH < 5 {
		innerH = 5
	}

	left := renderLeft(d, leftContentW, innerH)
	right := renderRight(d, rightContentW, innerH)
	frame := fr.Combine(title, leftW, rightW, panelH, left, right)
	out := frame + "\n" + footer
	if fr.Fill != nil {
		return fr.Fill(out, d.Width, d.Height)
	}
	return out
}

func renderLeft(d BrowseData, width, height int) string {
	search := renderSearch(d, width)
	sep := lipgloss.NewStyle().Foreground(style.BorderColor()).Render(strings.Repeat("─", max(1, width)))
	listH := height - 2
	if listH < 3 {
		listH = 3
	}
	visible := visibleRows(d.Rows, d.ListOff, listH)
	list := components.List(visible, width, listH, abbreviate)
	col := lipgloss.JoinVertical(lipgloss.Left, search, sep, list)
	colLines := strings.Split(col, "\n")
	if len(colLines) > height {
		col = strings.Join(colLines[:height], "\n")
	}
	return col
}

func renderSearch(d BrowseData, width int) string {
	if d.SearchView != "" {
		return components.PadSearchLine(d.SearchView, width)
	}
	return components.Search(d.Query, width)
}

func renderRight(d BrowseData, width, height int) string {
	if !d.HasSelected {
		return lipgloss.NewStyle().Width(width).Height(height).Render("Select a Pokémon")
	}

	header := renderHeader(d, width)
	headerBlock := countLines(header) + 1 // trailing gap under header
	bodyAvail := height - headerBlock
	if bodyAvail < 3 {
		bodyAvail = 3
	}

	body := renderViewBody(d, width, bodyAvail)

	used := headerBlock + countLines(body)
	spacerN := height - used
	if spacerN < 0 {
		spacerN = 0
	}

	var b strings.Builder
	b.WriteString(header)
	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(body)
	for i := 0; i < spacerN; i++ {
		b.WriteByte('\n')
		b.WriteString(strings.Repeat(" ", width))
	}
	col := b.String()
	lines := strings.Split(col, "\n")
	if len(lines) > height {
		col = strings.Join(lines[:height], "\n")
	}
	return col
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func renderHeader(d BrowseData, width int) string {
	name := d.Selected.DisplayName
	dex := output.FormatDex(d.Selected.ID)
	title := boldName(name) + strings.Repeat(" ", max(1, width-lipgloss.Width(name)-lipgloss.Width(dex))) + dex
	if lipgloss.Width(title) > width {
		title = components.Header(name, d.Selected.ID, width)
	} else {
		// Rebuild with proper padding via Header helper.
		title = components.Header(name, d.Selected.ID, width)
	}
	sub := d.HeaderTypes
	if d.HeaderGenus != "" {
		if sub != "" {
			sub += " · "
		}
		sub += d.HeaderGenus
	}
	if d.HeaderLoading {
		sub = style.StatusLoading("Loading…")
	}
	if d.HeaderErr != "" {
		sub = style.StatusError(d.HeaderErr)
	}
	if sub == "" {
		sub = "—"
	}
	sub = lipgloss.NewStyle().MaxWidth(width).Render(sub)
	return lipgloss.JoinVertical(lipgloss.Left, title, sub)
}

func boldName(name string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(style.PrimaryColor()).Render(name)
}

func renderViewBody(d BrowseData, width, maxH int) string {
	// Text views use the full right pane — not the sprite's art box.
	if d.BrowseView != 0 {
		h := maxH
		if h < 1 {
			h = 1
		}
		return renderTextBody(d.ViewLines, width, h)
	}

	side := d.BodySize
	if side < sprite.MinFrameSize {
		side = sprite.MinFrameSize
	}
	artW, artH := sprite.FrameSizeFor(side)
	outerW, outerH := sprite.OuterFor(side)
	if outerW > width {
		outerW = width
		artW = outerW - 2*sprite.FramePadX
		if artW < 1 {
			artW = outerW
		}
		artH = artW / sprite.CellAspectY
		if artH < 1 {
			artH = 1
		}
		outerH = artH + 2*sprite.FramePadY
	}
	if outerH > maxH && maxH > 0 {
		// Prefer keeping pad; shrink art if the pane is shorter than the outer box.
		artH = maxH - 2*sprite.FramePadY
		if artH < 1 {
			artH = maxH
			outerH = maxH
		} else {
			outerH = maxH
			artW = artH * sprite.CellAspectY
			if artW+2*sprite.FramePadX > outerW {
				artW = outerW - 2*sprite.FramePadX
			}
			if artW < 1 {
				artW = 1
			}
		}
	}

	var inner string
	if d.ShowSprite && d.SpriteNative {
		// Blank cell reservation only — graphics are painted via tea.Raw.
		inner = components.SpritePanel("", artW, artH, true)
	} else if d.ShowSprite && d.SpriteANSI != "" {
		inner = components.SpritePanel(d.SpriteANSI, artW, artH, false)
	} else if d.ShowSprite {
		inner = components.SpritePlaceholder(d.Selected.DisplayName, artW, artH)
	} else {
		inner = components.SpriteSizeCue(artW, artH)
	}

	body := padFrame(inner, artW, artH, outerW, outerH, d.SpriteNative)
	if d.SpriteNative {
		return centerNativeBlock(body, width, outerW)
	}
	if lipgloss.Width(body) < width {
		return lipgloss.PlaceHorizontal(width, lipgloss.Center, body)
	}
	return body
}

// centerNativeBlock left-pads a reserved native sprite box without measuring
// escape sequences (lipgloss width/PlaceHorizontal can corrupt DCS payloads).
func centerNativeBlock(body string, paneW, outerW int) string {
	if outerW >= paneW || paneW < 1 {
		return body
	}
	left := (paneW - outerW) / 2
	if left < 1 {
		return body
	}
	pad := strings.Repeat(" ", left)
	lines := strings.Split(body, "\n")
	for i := range lines {
		lines[i] = pad + lines[i]
	}
	return strings.Join(lines, "\n")
}

// spriteArtGeometry mirrors renderViewBody sizing for the browse sprite box.
func spriteArtGeometry(d BrowseData, width, maxH int) (artW, artH, outerW, outerH, padX, padY, centerLeft int) {
	side := d.BodySize
	if side < sprite.MinFrameSize {
		side = sprite.MinFrameSize
	}
	artW, artH = sprite.FrameSizeFor(side)
	outerW, outerH = sprite.OuterFor(side)
	if outerW > width {
		outerW = width
		artW = outerW - 2*sprite.FramePadX
		if artW < 1 {
			artW = outerW
		}
		artH = artW / sprite.CellAspectY
		if artH < 1 {
			artH = 1
		}
		outerH = artH + 2*sprite.FramePadY
	}
	if outerH > maxH && maxH > 0 {
		artH = maxH - 2*sprite.FramePadY
		if artH < 1 {
			artH = maxH
			outerH = maxH
		} else {
			outerH = maxH
			artW = artH * sprite.CellAspectY
			if artW+2*sprite.FramePadX > outerW {
				artW = outerW - 2*sprite.FramePadX
			}
			if artW < 1 {
				artW = 1
			}
		}
	}
	padX = (outerW - artW) / 2
	padY = (outerH - artH) / 2
	if padX < 0 {
		padX = 0
	}
	if padY < 0 {
		padY = 0
	}
	if outerW < width {
		centerLeft = (width - outerW) / 2
		if centerLeft < 0 {
			centerLeft = 0
		}
	}
	return artW, artH, outerW, outerH, padX, padY, centerLeft
}

// SpriteArtOrigin returns the 1-indexed terminal CUP row/col of the sprite art
// box for native overlay painting, plus the art cell size for clearing.
// ok is false when the sprite is not shown.
func SpriteArtOrigin(d BrowseData) (row, col, artW, artH int, ok bool) {
	if !d.ShowSprite || !d.SpriteNative {
		return 0, 0, 0, 0, false
	}
	if d.TooSmall || d.OnePane || d.ShowHelp || d.ShowDialog || d.BrowseView != 0 {
		return 0, 0, 0, 0, false
	}
	if d.Width < 1 || d.Height < 1 {
		return 0, 0, 0, 0, false
	}

	footerH := 1
	panelH := d.Height - footerH
	if panelH < 10 {
		panelH = 10
	}
	leftW, rightW := d.LeftWidth, d.RightWidth
	if leftW < 1 || rightW < 1 {
		return 0, 0, 0, 0, false
	}
	rightContentW := rightW - 1
	if rightContentW < 10 {
		rightContentW = 10
	}
	innerH := panelH - 2
	if innerH < 5 {
		innerH = 5
	}

	// Header is always two lines (title + types/genus) plus one gap line.
	headerBlock := 3
	bodyAvail := innerH - headerBlock
	if bodyAvail < 3 {
		bodyAvail = 3
	}
	aw, ah, _, _, padX, padY, centerLeft := spriteArtGeometry(d, rightContentW, bodyAvail)

	// Combined frame: row 1 = top border; content rows start at 2.
	// Right pane content starts at column leftW+2 (after left border, left
	// content, and the centre separator).
	row = 2 + headerBlock + padY
	col = leftW + 2 + centerLeft + padX
	if row < 1 || col < 1 || aw < 1 || ah < 1 {
		return 0, 0, 0, 0, false
	}
	return row, col, aw, ah, true
}

func padFrame(inner string, artW, artH, outerW, outerH int, native bool) string {
	if outerW <= artW && outerH <= artH {
		return inner
	}
	padX := (outerW - artW) / 2
	padY := (outerH - artH) / 2
	if padX < 0 {
		padX = 0
	}
	if padY < 0 {
		padY = 0
	}
	blank := strings.Repeat(" ", outerW)
	if native {
		return padNativeFrame(inner, artW, artH, outerW, outerH, padX, padY, blank)
	}
	innerLines := strings.Split(inner, "\n")
	out := make([]string, 0, outerH)
	for i := 0; i < padY; i++ {
		out = append(out, blank)
	}
	sidePad := strings.Repeat(" ", padX)
	for i := 0; i < artH; i++ {
		line := ""
		if i < len(innerLines) {
			line = innerLines[i]
		}
		for lipgloss.Width(line) < artW {
			line += " "
		}
		if lipgloss.Width(line) > artW {
			line = lipgloss.NewStyle().MaxWidth(artW).Render(line)
		}
		out = append(out, sidePad+line+strings.Repeat(" ", max(0, outerW-padX-lipgloss.Width(line))))
	}
	for len(out) < outerH {
		out = append(out, blank)
	}
	if len(out) > outerH {
		out = out[:outerH]
	}
	return strings.Join(out, "\n")
}

// padNativeFrame pads around a native sprite block without MaxWidth-clipping
// the graphics sequence embedded on the first art line.
func padNativeFrame(inner string, artW, artH, outerW, outerH, padX, padY int, blank string) string {
	innerLines := strings.Split(inner, "\n")
	out := make([]string, 0, outerH)
	for i := 0; i < padY; i++ {
		out = append(out, blank)
	}
	sidePad := strings.Repeat(" ", padX)
	rightPad := strings.Repeat(" ", max(0, outerW-padX-artW))
	for i := 0; i < artH; i++ {
		line := ""
		if i < len(innerLines) {
			line = innerLines[i]
		}
		if i == 0 {
			// First line may start with an opaque escape sequence + artW spaces.
			// Do not measure or clip the sequence; keep side padding around the reserved box.
			out = append(out, sidePad+line+rightPad)
			continue
		}
		// Remaining reservation lines are plain spaces of artW.
		if lipgloss.Width(line) < artW {
			line += strings.Repeat(" ", artW-lipgloss.Width(line))
		}
		out = append(out, sidePad+line+rightPad)
	}
	for len(out) < outerH {
		out = append(out, blank)
	}
	if len(out) > outerH {
		out = out[:outerH]
	}
	return strings.Join(out, "\n")
}

func renderTextBody(lines []string, width, height int) string {
	out := make([]string, 0, height)
	for i := 0; i < height; i++ {
		if i < len(lines) {
			out = append(out, padLine(lines[i], width))
		} else {
			out = append(out, strings.Repeat(" ", width))
		}
	}
	return strings.Join(out, "\n")
}

func padLine(s string, width int) string {
	w := lipgloss.Width(s)
	if w < width {
		return s + strings.Repeat(" ", width-w)
	}
	if w > width {
		return lipgloss.NewStyle().MaxWidth(width).Render(s)
	}
	return s
}

func visibleRows(rows []components.ListRow, off, count int) []components.ListRow {
	if off < 0 {
		off = 0
	}
	if off > len(rows) {
		off = len(rows)
	}
	end := off + count
	if end > len(rows) {
		end = len(rows)
	}
	return rows[off:end]
}

func abbreviate(types []string, maxW int) string {
	joined := output.JoinTypes(types)
	if maxW <= 0 || len(joined) <= maxW {
		return joined
	}
	if len(types) < 2 {
		return truncate(joined, maxW)
	}
	first := output.TitleFromSlug(types[0])
	second := output.TitleFromSlug(types[1])
	if len([]rune(second)) > 4 {
		second = string([]rune(second)[:4])
	}
	out := first + "/" + second
	if len(out) > maxW {
		return truncate(out, maxW)
	}
	return out
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n <= 1 {
		return string(runes[:n])
	}
	return string(runes[:n-1]) + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func fallbackCombine(title string, leftW, rightW, height int, leftContent, rightContent string) string {
	if leftW < 4 {
		leftW = 4
	}
	if rightW < 4 {
		rightW = 4
	}
	if height < 3 {
		height = 3
	}
	leftInner := leftW - 1
	rightInner := rightW - 1
	contentH := height - 2
	topTitle := "─ " + title + " "
	remain := leftW - 1 - len([]rune(topTitle))
	if remain < 0 {
		remain = 0
	}
	top := "╭" + topTitle + strings.Repeat("─", remain) + "┬" + strings.Repeat("─", rightW-1) + "╮"
	bottom := "╰" + strings.Repeat("─", leftW-1) + "┴" + strings.Repeat("─", rightW-1) + "╯"
	leftLines := strings.Split(strings.TrimRight(leftContent, "\n"), "\n")
	rightLines := strings.Split(strings.TrimRight(rightContent, "\n"), "\n")
	var b strings.Builder
	b.WriteString(top)
	b.WriteByte('\n')
	for i := 0; i < contentH; i++ {
		l, r := "", ""
		if i < len(leftLines) {
			l = leftLines[i]
		}
		if i < len(rightLines) {
			r = rightLines[i]
		}
		for lipgloss.Width(l) < leftInner {
			l += " "
		}
		for lipgloss.Width(r) < rightInner {
			r += " "
		}
		if lipgloss.Width(l) > leftInner {
			l = lipgloss.NewStyle().MaxWidth(leftInner).Render(l)
		}
		if lipgloss.Width(r) > rightInner {
			r = lipgloss.NewStyle().MaxWidth(rightInner).Render(r)
		}
		b.WriteString("│")
		b.WriteString(l)
		b.WriteString("│")
		b.WriteString(r)
		b.WriteString("│")
		if i < contentH-1 {
			b.WriteByte('\n')
		}
	}
	b.WriteByte('\n')
	b.WriteString(bottom)
	return b.String()
}
