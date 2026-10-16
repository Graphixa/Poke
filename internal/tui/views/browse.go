package views

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/output"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/textwidth"
	"github.com/Graphixa/Pokedex/internal/tui/components"
	"github.com/Graphixa/Pokedex/internal/tui/style"
)

// FrameRenderer is injected to avoid an import cycle with package tui.
type FrameRenderer struct {
	Calculate func(termW, termH, headerH, footerH, margin, sep int) (leftW, rightW, panelH int)
	Combine   func(title string, leftW, rightW, height int, left, right, topRight string) string
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

	Query       string
	SearchView  string
	BrowseTitle string
	ListHint    string
	Rows        []components.ListRow
	ListOff     int
	ListRows    int
	ListNameCol int // padded name column; 0 derives from visible rows

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

	// Glance cards (C3).
	HasBrowseStats bool
	Stats          domain.Stats
	HasCore        bool // bottom Abilities card has data (browse.json or dossier)
	Abilities      []domain.AbilitySlot

	ShowDialog    bool
	DialogSection int
	DialogLabels  []string
	DialogLines   []string
	DialogOff     int

	SpriteANSI   string
	SpriteNative bool
	ImageState   int // 0 blank, 1 loading, 2 ready, 3 unavailable

	WarmHint   string
	FooterHint string
	// FrameTopRight is optional chrome in the right pane's top border (e.g. F4 chip).
	FrameTopRight string

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

	if d.TooSmall {
		return components.TooSmall(d.Width, d.Height)
	}
	if d.ShowDialog {
		title := "Details"
		if d.HasSelected {
			title = fmt.Sprintf("%s %s", d.Selected.DisplayName, output.FormatDex(d.Selected.ID))
		}
		footer := d.FooterHint
		if footer != "" && !strings.HasPrefix(footer, " ") {
			footer = " " + footer
		}
		return components.DossierFrame(title, d.DialogSection, d.DialogLabels, d.DialogLines, d.DialogOff, d.Width, d.Height, footer)
	}
	return renderBrowseBase(d, fr)
}

func renderBrowseBase(d BrowseData, fr FrameRenderer) string {
	footer := components.FooterSplit(d.VerboseFooter, d.Width, d.FooterHint, d.WarmHint)
	footerH := 1
	margin := 0
	title := "Pokedex"
	if d.BrowseTitle != "" {
		title = d.BrowseTitle
	}

	if d.OnePane {
		bodyH := d.Height - footerH
		var body string
		if d.ShowListPane || !d.HasSelected {
			search := renderSearch(d, d.Width)
			listH := bodyH - 1
			prefix := search + "\n"
			if d.ListHint != "" {
				prefix += textwidth.PadRight(textwidth.Truncate(d.ListHint, d.Width), d.Width) + "\n"
				listH--
			}
			visible := visibleRows(d.Rows, d.ListOff, listH)
			body = prefix + components.List(visible, d.Width, listH, d.ListNameCol)
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
	frame := fr.Combine(title, leftW, rightW, panelH, left, right, d.FrameTopRight)
	out := frame + "\n" + footer
	if fr.Fill != nil {
		return fr.Fill(out, d.Width, d.Height)
	}
	return out
}

func renderLeft(d BrowseData, width, height int) string {
	search := renderSearch(d, width)
	sep := lipgloss.NewStyle().Foreground(style.BorderColor()).Render(strings.Repeat("─", max(1, width)))
	if d.ListHint != "" {
		sep = textwidth.PadRight(textwidth.Truncate(d.ListHint, width), width)
	}
	listH := height - 2
	if listH < 3 {
		listH = 3
	}
	visible := visibleRows(d.Rows, d.ListOff, listH)
	list := components.List(visible, width, listH, d.ListNameCol)
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
	const headerRows = glanceHeaderRows
	bodyAvail := height - headerRows
	if bodyAvail < 3 {
		bodyAvail = 3
	}

	blank := strings.Repeat(" ", width)
	lines := strings.Split(header, "\n")
	if len(lines) > headerRows {
		lines = lines[:headerRows]
	}
	for len(lines) < headerRows {
		lines = append(lines, blank)
	}

	if d.BrowseView == 1 {
		g := ComputeHeroArt(width, height)
		body := renderHeroBody(d, width, bodyAvail, g)
		bodyLines := strings.Split(strings.TrimRight(body, "\n"), "\n")
		need := len(bodyLines)
		if need > bodyAvail {
			need = bodyAvail
		}
		lines = append(lines, bodyLines[:need]...)
		for len(lines) < height {
			lines = append(lines, blank)
		}
		if len(lines) > height {
			lines = lines[:height]
		}
		return strings.Join(lines, "\n")
	}

	g := ComputeGlance(width, height, d.HasCore)

	body := renderGlanceBody(d, width, bodyAvail, g)
	bodyLines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	// Prefer keeping glance cards (including About) over trailing blank pad when
	// the body is somehow taller than bodyAvail — truncate blanks, not cards.
	need := len(bodyLines)
	if need > bodyAvail {
		need = bodyAvail
	}
	lines = append(lines, bodyLines[:need]...)
	for len(lines) < height {
		lines = append(lines, blank)
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

func renderGlanceBody(d BrowseData, width, bodyH int, g GlanceLayout) string {
	if !g.ShowSprite {
		if g.ShowStats {
			return renderGlanceTextCards(d, width, bodyH, g)
		}
		cueW, cueH := sprite.FrameSizeFor(sprite.SmallestFrameBucket())
		return padBlock(components.SpriteSizeCue(cueW, cueH), width, bodyH)
	}

	artInner := renderArtInner(d, g.ArtW, g.ArtH)
	// Inner box = card minus borders; padArtInCard centres sprite with content pad.
	artBody := padArtInCard(artInner, g.ArtW, g.ArtH, g.ArtCardW-2, g.ArtCardH-2)
	artCard := components.InnerCard("", g.ArtCardW, g.ArtCardH, strings.Split(artBody, "\n"))

	tileW := g.ArtCardW
	var upper string
	if g.ShowStats {
		statsCard := components.InnerCard("Stats", g.StatsW, g.StatsH, GlanceStatsBody(glanceStatsLines(d)))
		upper = joinHorizontalTop(artCard, statsCard, g.UpperH, g.ArtCardW+glanceCardGap+g.StatsW, g.ArtCardW, g.StatsW, glanceCardGap)
		tileW = g.ArtCardW + glanceCardGap + g.StatsW
	} else {
		upper = artCard
	}

	if !g.ShowAbout || g.AboutH < glanceMinAboutH {
		return padBlock(upper, width, bodyH)
	}

	aboutW := tileW
	if aboutW > width {
		aboutW = width
	}
	aboutInner := aboutW - 2 - 2*glanceContentPad
	aboutBody := GlanceStatsBody(components.GlanceAbilityGrid(d.Abilities, aboutInner))
	aboutCard := components.InnerCard("Abilities", aboutW, g.AboutH, aboutBody)
	combined := strings.TrimRight(upper, "\n") + "\n" + aboutCard
	return padBlock(combined, width, bodyH)
}

func renderHeroBody(d BrowseData, width, bodyH int, g GlanceLayout) string {
	if !g.ShowSprite {
		cueW, cueH := sprite.FrameSizeFor(HeroBucket)
		return padBlock(components.SpriteSizeCue(cueW, cueH), width, bodyH)
	}
	artInner := renderArtInner(d, g.ArtW, g.ArtH)
	artBody := padArtInCard(artInner, g.ArtW, g.ArtH, g.ArtCardW-2, g.ArtCardH-2)
	artCard := components.InnerCard("", g.ArtCardW, g.ArtCardH, strings.Split(artBody, "\n"))
	return placeBlock(artCard, width, bodyH, g.ArtCardX, g.ArtCardY, g.ArtCardW)
}

// placeBlock puts block at (x,y) inside a width×height canvas of spaces.
func placeBlock(block string, width, height, x, y, blockW int) string {
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	src := strings.Split(strings.TrimRight(block, "\n"), "\n")
	out := make([]string, height)
	blank := strings.Repeat(" ", width)
	padL := strings.Repeat(" ", x)
	for i := 0; i < height; i++ {
		if i < y || i-y >= len(src) {
			out[i] = blank
			continue
		}
		line := padL + src[i-y]
		for lipgloss.Width(line) < width {
			line += " "
		}
		if lipgloss.Width(line) > width && !strings.Contains(line, "\x1b") {
			line = lipgloss.NewStyle().MaxWidth(width).Render(line)
		}
		_ = blockW
		out[i] = line
	}
	return strings.Join(out, "\n")
}

func renderGlanceTextCards(d BrowseData, width, bodyH int, g GlanceLayout) string {
	statsW := g.StatsW
	if statsW > width {
		statsW = width
	}
	statsCard := components.InnerCard("Stats", statsW, g.StatsH, GlanceStatsBody(glanceStatsLines(d)))
	upper := statsCard
	if !g.ShowAbout || g.AboutH < glanceMinAboutH {
		return padBlock(upper, width, bodyH)
	}
	aboutInner := statsW - 2 - 2*glanceContentPad
	aboutCard := components.InnerCard("Abilities", statsW, g.AboutH, GlanceStatsBody(components.GlanceAbilityGrid(d.Abilities, aboutInner)))
	return padBlock(strings.TrimRight(upper, "\n")+"\n"+aboutCard, width, bodyH)
}

func glanceStatsLines(d BrowseData) []string {
	if d.HasBrowseStats || d.HasCore {
		return components.StatsBars(d.Stats)
	}
	return []string{"—"}
}

func renderArtInner(d BrowseData, artW, artH int) string {
	// Loading/blank: empty themed reservation until the frame is ready (pop-in).
	// No spinner — ticks would churn View and wipe native graphics.
	ready := d.ImageState == 2 || (d.ImageState == 0 && (d.SpriteANSI != "" || d.SpriteNative))
	if d.ShowSprite && d.SpriteNative && ready {
		return components.SpritePanel("", artW, artH, true)
	}
	if d.ShowSprite && d.SpriteANSI != "" && ready {
		return components.SpritePanel(d.SpriteANSI, artW, artH, false)
	}
	if d.ShowSprite {
		return components.SpritePanel("", artW, artH, true)
	}
	return components.SpriteSizeCue(artW, artH)
}

// padArtInCard centres the art WxH block inside the card's inner box.
func padArtInCard(art string, artW, artH, innerW, innerH int) string {
	left := (innerW - artW) / 2
	if left < 0 {
		left = 0
	}
	top := (innerH - artH) / 2
	if top < 0 {
		top = 0
	}
	padL := strings.Repeat(" ", left)
	lines := strings.Split(art, "\n")
	out := make([]string, 0, innerH)
	for i := 0; i < top; i++ {
		out = append(out, strings.Repeat(" ", innerW))
	}
	for _, line := range lines {
		row := padL + line
		// Avoid measuring ANSI/native payloads with lipgloss — only pad plain rows.
		if !strings.Contains(line, "\x1b") {
			for lipgloss.Width(row) < innerW {
				row += " "
			}
		}
		out = append(out, row)
	}
	for len(out) < innerH {
		out = append(out, strings.Repeat(" ", innerW))
	}
	if len(out) > innerH {
		out = out[:innerH]
	}
	return strings.Join(out, "\n")
}

func joinHorizontalTop(left, right string, height, totalW, leftW, rightW, gap int) string {
	leftLines := strings.Split(strings.TrimRight(left, "\n"), "\n")
	rightLines := strings.Split(strings.TrimRight(right, "\n"), "\n")
	gapStr := strings.Repeat(" ", gap)
	out := make([]string, height)
	for i := 0; i < height; i++ {
		l, r := "", ""
		if i < len(leftLines) {
			l = leftLines[i]
		} else {
			l = strings.Repeat(" ", leftW)
		}
		if i < len(rightLines) {
			r = rightLines[i]
		} else {
			r = strings.Repeat(" ", rightW)
		}
		for lipgloss.Width(l) < leftW {
			l += " "
		}
		for lipgloss.Width(r) < rightW {
			r += " "
		}
		line := l + gapStr + r
		for lipgloss.Width(line) < totalW {
			line += " "
		}
		out[i] = line
	}
	return strings.Join(out, "\n")
}

func padBlock(block string, width, height int) string {
	lines := strings.Split(strings.TrimRight(block, "\n"), "\n")
	out := make([]string, height)
	blank := strings.Repeat(" ", width)
	for i := 0; i < height; i++ {
		if i < len(lines) {
			line := lines[i]
			for lipgloss.Width(line) < width {
				line += " "
			}
			out[i] = line
		} else {
			out[i] = blank
		}
	}
	return strings.Join(out, "\n")
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

// SpriteArtOrigin returns the 1-indexed terminal CUP row/col of the sprite art
// box for native overlay painting, plus the art cell size for clearing.
// ok is false when the sprite is not shown or native art is not active.
func SpriteArtOrigin(d BrowseData) (row, col, artW, artH int, ok bool) {
	return spriteArtRegion(d, true)
}

// SpriteArtRegion returns the art box geometry for the current browse layout
// even when the selected species is still on a text placeholder (used to wipe
// leftover Kitty/iTerm/Sixel from the previous selection).
func SpriteArtRegion(d BrowseData) (row, col, artW, artH int, ok bool) {
	return spriteArtRegion(d, false)
}

func spriteArtRegion(d BrowseData, requireNative bool) (row, col, artW, artH int, ok bool) {
	if !d.ShowSprite {
		return 0, 0, 0, 0, false
	}
	if requireNative && !d.SpriteNative {
		return 0, 0, 0, 0, false
	}
	if d.TooSmall || d.OnePane || d.ShowDialog {
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

	var g GlanceLayout
	if d.BrowseView == 1 {
		g = ComputeHeroArt(rightContentW, innerH)
	} else {
		g = ComputeGlance(rightContentW, innerH, d.HasCore)
	}
	if !g.ShowSprite {
		return 0, 0, 0, 0, false
	}

	// Combined frame: row 1 = top border; content starts at 2.
	// Header (2) then art card; sprite inset by pad (+ hero body offset).
	row = 2 + glanceHeaderRows + g.ArtCardY + 1 + g.ArtPadY // +1 for art card top border
	col = leftW + 2 + g.ArtCardX + 1 + g.ArtPadX            // +1 for art card left border
	artW, artH = g.ArtW, g.ArtH
	if row < 1 || col < 1 || artW < 1 || artH < 1 {
		return 0, 0, 0, 0, false
	}
	return row, col, artW, artH, true
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func fallbackCombine(title string, leftW, rightW, height int, leftContent, rightContent, topRight string) string {
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
	rightInnerTop := rightW - 1
	rightTop := strings.Repeat("─", rightInnerTop)
	if tr := strings.TrimSpace(topRight); tr != "" {
		suf := " " + tr + "  " // match leading space; then ─ before ╮
		need := lipgloss.Width(suf) + 1
		if need < rightInnerTop {
			rightTop = strings.Repeat("─", rightInnerTop-need) + suf + "─"
		} else {
			rightTop = lipgloss.NewStyle().MaxWidth(rightInnerTop).Render(suf)
		}
	}
	top := "╭" + topTitle + strings.Repeat("─", remain) + "┬" + rightTop + "╮"
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
