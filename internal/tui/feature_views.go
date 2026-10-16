package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Graphixa/Pokedex/internal/output"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/textwidth"
	"github.com/Graphixa/Pokedex/internal/tui/components"
	"github.com/Graphixa/Pokedex/internal/tui/style"
	"github.com/Graphixa/Pokedex/internal/tui/views"
)

func featureBinding(keys, label, action string) key.Binding {
	return key.NewBinding(key.WithKeys(keys), key.WithHelp(label, action))
}

func (m Model) featureView() string {
	w, h := m.width, m.height
	if w < 40 || h < 18 {
		return lipgloss.Place(max(1, w), max(1, h), lipgloss.Center, lipgloss.Center,
			textwidth.Truncate("Resize to 40 × 18", max(1, w))+"\n"+textwidth.Truncate("Esc closes this view", max(1, w)))
	}
	if m.featureError != "" {
		return m.featureFrame("Favourites", strings.Split(lipgloss.NewStyle().Width(w-4).Render(m.featureError), "\n"),
			[][]key.Binding{{featureBinding("enter", "enter / esc", "close")}})
	}
	return m.compareView()
}

// Shared full-screen chrome matches browse and dossier (FontGet-style key chips).
// Reserve footer rows before sizing the card so narrow terminals keep all keys.
func (m Model) featureFrame(title string, lines []string, bindings [][]key.Binding) string {
	var footer []string
	for _, row := range bindings {
		footer = append(footer, style.RenderCommandHelp(row, m.width))
	}
	for i := range lines {
		lines[i] = " " + textwidth.Truncate(lines[i], m.width-4)
	}
	card := components.InnerCard(title, m.width, m.height-len(footer), lines)
	return card + "\n" + strings.Join(footer, "\n")
}

func featureAccent(s string) string {
	return lipgloss.NewStyle().Foreground(style.PrimaryColor()).Bold(true).Render(s)
}

var filterStepPrompts = []string{"COLLECTION", "POKÉMON TYPE", "SORT METHOD"}

// filterView paints a wizard modal over a dimmed browse backdrop.
func (m Model) filterView(browse string) string {
	w, h := m.width, m.height
	if w < 40 || h < 18 {
		return lipgloss.Place(max(1, w), max(1, h), lipgloss.Center, lipgloss.Center,
			textwidth.Truncate("Resize to 40 × 18", max(1, w))+"\n"+textwidth.Truncate("Esc closes this view", max(1, w)))
	}
	// Compact help so 40-col keeps esc; ←/⌫ still work unbound in the footer.
	footer := style.RenderCommandHelp([]key.Binding{
		featureBinding("esc", "esc", "cancel"),
		featureBinding("up", "↑/↓", "choose"),
		featureBinding("enter", "enter", "next"),
		featureBinding(" ", "space", "apply"),
	}, w)
	bodyH := h - 1
	if bodyH < 8 {
		bodyH = h
		footer = ""
	}
	cardW := min(56, w-4)
	if cardW < 36 {
		cardW = min(w, 36)
	}
	card := m.filterWizardCard(cardW)
	bg := lipgloss.NewStyle().Faint(true).Render(ansi.Strip(browse))
	merged := overlayCenterCard(bg, card, w, bodyH)
	if footer == "" {
		return merged
	}
	return merged + "\n" + textwidth.PadRight(textwidth.Truncate(footer, w), w)
}

func (m Model) filterWizardCard(cardW int) string {
	step := m.filterStep
	if step < 0 {
		step = 0
	}
	if step > 2 {
		step = 2
	}
	innerW := cardW - 2
	var body []string
	body = append(body, "")
	body = append(body, featureAccent("  "+filterStepPrompts[step]))
	body = append(body, "")
	body = append(body, m.filterStepChoices(innerW-2)...)
	body = append(body, "")
	count := len(m.filterSpecies(m.index.Search(m.queryValue()), m.draftOptions))
	body = append(body, "  "+featureAccent(fmt.Sprintf("%d matches", count)))
	if step == 2 {
		body = append(body, "  "+style.StatusMuted(m.filterDraftSummary()))
	}
	body = append(body, "")
	footer := filterStepDots(step)
	if count == 0 {
		footer += "  " + style.StatusMuted("Try another type or reset.")
	}
	body = append(body, "  "+footer)
	body = append(body, "")
	return components.InnerCard("Filter and Sort", cardW, len(body)+2, body)
}

func (m Model) filterDraftSummary() string {
	collection := "All"
	if m.draftOptions.FavouritesOnly {
		collection = "Favourites"
	}
	typ := "All types"
	if m.draftOptions.Type > 0 && m.draftOptions.Type < len(filterTypes) {
		typ = output.TitleFromSlug(filterTypes[m.draftOptions.Type])
	}
	sort := filterSorts[0]
	if m.draftOptions.Sort >= 0 && m.draftOptions.Sort < len(filterSorts) {
		sort = filterSorts[m.draftOptions.Sort]
	}
	return collection + " · " + typ + " · " + sort
}

func filterStepDots(step int) string {
	parts := make([]string, 3)
	for i := range parts {
		if i == step {
			parts[i] = featureAccent("●")
		} else {
			parts[i] = style.StatusMuted("○")
		}
	}
	return strings.Join(parts, " ")
}

func (m Model) filterStepChoices(width int) []string {
	const window = 7
	switch m.filterStep {
	case 0:
		opts := []string{"All Pokémon", "Favourites only"}
		active := 0
		if m.draftOptions.FavouritesOnly {
			active = 1
		}
		return filterChoiceWindow(opts, active, window, width, false)
	case 1:
		labels := make([]string, len(filterTypes))
		for i, t := range filterTypes {
			if i == 0 {
				labels[i] = "All"
			} else {
				labels[i] = output.TitleFromSlug(t)
			}
		}
		return filterChoiceWindow(labels, m.draftOptions.Type, window, width, true)
	default:
		return filterChoiceWindow(filterSorts, m.draftOptions.Sort, window, width, false)
	}
}

func filterChoiceWindow(opts []string, active, window, width int, colorTypes bool) []string {
	if active < 0 {
		active = 0
	}
	if active >= len(opts) {
		active = len(opts) - 1
	}
	if window > len(opts) {
		window = len(opts)
	}
	start := active - window/2
	if start < 0 {
		start = 0
	}
	if start+window > len(opts) {
		start = len(opts) - window
	}
	bar := len(opts) > window
	// Fixed column from the longest label in the full list (not the visible window).
	colW := 0
	for _, o := range opts {
		if w := 2 + textwidth.Width(o); w > colW { // "> "/"  " are 2 cells
			colW = w
		}
	}
	if bar {
		colW += 2 // small gap before the track
		colW = min(colW, max(4, width-2))
	} else {
		colW = min(colW, width)
	}
	out := make([]string, 0, window)
	for row := 0; row < window; row++ {
		i := start + row
		label := opts[i]
		prefix := "  "
		if i == active {
			prefix = "> "
		}
		text := prefix + label
		if colorTypes && i > 0 {
			colored := style.TypeLabelText(filterTypes[i], label)
			if i == active {
				text = featureAccent(prefix) + colored
			} else {
				text = prefix + colored
			}
		} else if i == active {
			text = featureAccent(text)
		} else {
			text = style.StatusMuted(text)
		}
		line := "  " + textwidth.PadRight(textwidth.Truncate(text, colW), colW)
		if bar {
			line += " " + filterScrollThumb(row, window, start, len(opts))
		}
		out = append(out, textwidth.Truncate(line, width))
	}
	return out
}

// filterScrollThumb is a 1-col track (│) with a moving thumb (┃).
func filterScrollThumb(row, window, start, total int) string {
	thumbH := max(1, (window*window)/total)
	maxStart := total - window
	thumbTop := 0
	if maxStart > 0 {
		thumbTop = (start * (window - thumbH)) / maxStart
	}
	if row >= thumbTop && row < thumbTop+thumbH {
		return featureAccent("┃")
	}
	return style.StatusMuted("│")
}

// overlayCenterCard places card centered over bg, keeping left/right backdrop peek.
func overlayCenterCard(bg, card string, width, height int) string {
	bgLines := strings.Split(strings.TrimRight(bg, "\n"), "\n")
	for len(bgLines) < height {
		bgLines = append(bgLines, "")
	}
	if len(bgLines) > height {
		bgLines = bgLines[:height]
	}
	for i := range bgLines {
		bgLines[i] = textwidth.PadRight(textwidth.Truncate(bgLines[i], width), width)
	}
	cardLines := strings.Split(strings.TrimRight(card, "\n"), "\n")
	cardH := len(cardLines)
	cardW := 0
	for _, l := range cardLines {
		if w := textwidth.Width(l); w > cardW {
			cardW = w
		}
	}
	top := max(0, (height-cardH)/2)
	left := max(0, (width-cardW)/2)
	for i, cl := range cardLines {
		y := top + i
		if y >= height {
			break
		}
		cw := textwidth.Width(cl)
		leftPart := textwidth.PadRight(ansi.Cut(bgLines[y], 0, left), left)
		rightStart := left + cw
		rightPart := ""
		if rightStart < width {
			rightPart = textwidth.PadRight(ansi.Cut(bgLines[y], rightStart, width), width-rightStart)
		}
		bgLines[y] = textwidth.PadRight(textwidth.Truncate(leftPart+cl+rightPart, width), width)
	}
	return strings.Join(bgLines, "\n")
}

func (m Model) compareView() string {
	if m.comparePin == nil && m.compareA.Summary.ID == 0 {
		return m.featureFrame("Compare", []string{"Choose a Pokémon with F3 to begin."}, [][]key.Binding{{featureBinding("esc", "esc", "back")}})
	}
	a, b := m.compareA, m.compareB
	if a.Summary.ID == 0 && m.comparePin != nil {
		a = m.comparisonEntry(m.comparePin.Summary)
		if !a.HasStats {
			a = *m.comparePin
		}
	}
	if b.Summary.ID == 0 {
		if s, ok := m.selected(); ok {
			b = m.comparisonEntry(s)
		}
	}

	bindings := []key.Binding{
		featureBinding("tab", "tab", "focus"),
		featureBinding("up", "↑/↓", "change"),
		featureBinding(" ", "space", "swap"),
		featureBinding("esc", "esc", "back"),
	}
	footer := []string{style.RenderCommandHelp(bindings, m.width)}

	g := m.compareLayoutGeom(len(footer))
	rule := lipgloss.NewStyle().Foreground(style.BorderColor()).Render("│")
	body := make([]string, 0, g.innerH)

	// Name/type align with the art card's left edge (padL).
	headLine := func(s string, colW int) string {
		budget := colW - g.padL
		if budget < 1 {
			budget = 1
		}
		return compareFit(strings.Repeat(" ", g.padL)+textwidth.Truncate(s, budget), colW)
	}
	blank := strings.Repeat(" ", g.colWA) + rule + strings.Repeat(" ", g.colWB)
	nameA := fmt.Sprintf("%s %s", a.Summary.DisplayName, output.FormatDex(a.Summary.ID))
	nameB := fmt.Sprintf("%s %s", b.Summary.DisplayName, output.FormatDex(b.Summary.ID))
	if m.compareFocus == 0 {
		nameA = featureAccent(nameA)
	} else {
		nameB = featureAccent(nameB)
	}
	body = append(body, blank)
	body = append(body, headLine(nameA, g.colWA)+rule+headLine(nameB, g.colWB))
	body = append(body, headLine(compareSideHeader(a, m.compareGenus(a.Summary.ID)), g.colWA)+rule+
		headLine(compareSideHeader(b, m.compareGenus(b.Summary.ID)), g.colWB))
	body = append(body, blank)

	av := []int{a.Stats.HP, a.Stats.Attack, a.Stats.Defense, a.Stats.SpecialAttack, a.Stats.SpecialDefense, a.Stats.Speed, a.Stats.Total()}
	bv := []int{b.Stats.HP, b.Stats.Attack, b.Stats.Defense, b.Stats.SpecialAttack, b.Stats.SpecialDefense, b.Stats.Speed, b.Stats.Total()}
	labels := []string{"HP", "Attack", "Defence", "Sp. Attack", "Sp. Defence", "Speed", "Total"}
	deltas := make([]string, len(labels))
	if a.HasStats && b.HasStats {
		for i := range labels {
			deltas[i] = formatCompareDelta(bv[i] - av[i])
		}
	}
	budgetA, budgetB := g.coreW, g.coreW
	if g.drawArt {
		budgetA = g.colWA - g.padL - g.artOW - g.gap
		budgetB = g.colWB - g.padL - g.artOW - g.gap
	}
	statLinesA := compareStatLines(labels, av, a.HasStats, nil, budgetA)
	statLinesB := compareStatLines(labels, bv, b.HasStats, deltas, budgetB)

	if g.drawArt {
		leftBorder, rightBorder := style.BorderColor(), style.BorderColor()
		if m.compareFocus == 0 {
			leftBorder = style.PrimaryColor()
		} else {
			rightBorder = style.PrimaryColor()
		}
		leftCard := m.compareArtCard(0, a, g.artW, g.artH, g.artOW, g.artOH, leftBorder)
		rightCard := m.compareArtCard(1, b, g.artW, g.artH, g.artOW, g.artOH, rightBorder)
		artA := padCompareBlock(leftCard, g.artOW, g.blockH, 0)
		artB := padCompareBlock(rightCard, g.artOW, g.blockH, 0)
		for i := 0; i < g.blockH; i++ {
			sa, sb := "", ""
			if i < len(statLinesA) {
				sa = statLinesA[i]
			}
			if i < len(statLinesB) {
				sb = statLinesB[i]
			}
			body = append(body, compareArtStatRow(g, artA[i], sa, g.colWA)+rule+compareArtStatRow(g, artB[i], sb, g.colWB))
		}
	} else {
		for i := range statLinesA {
			body = append(body, compareFit(statLinesA[i], g.colWA)+rule+compareFit(statLinesB[i], g.colWB))
		}
	}

	for len(body) < g.innerH {
		body = append(body, blank)
	}
	if len(body) > g.innerH {
		body = body[:g.innerH]
	}

	border := lipgloss.NewStyle().Foreground(style.BorderColor())
	vert := border.Render("│")
	bot := border.Render("╰") + strings.Repeat(border.Render("─"), max(0, m.width-2)) + border.Render("╯")

	var frame strings.Builder
	frame.WriteString(compareFrameTitle(m.width))
	frame.WriteByte('\n')
	for i, line := range body {
		// Halves already fill colWA|colWB exactly — never Truncate the joined ANSI row.
		cell := line
		if textwidth.Width(cell) < g.innerW {
			cell = textwidth.PadRight(cell, g.innerW)
		}
		frame.WriteString(vert)
		frame.WriteString(cell)
		frame.WriteString(vert)
		if i < len(body)-1 {
			frame.WriteByte('\n')
		}
	}
	frame.WriteByte('\n')
	frame.WriteString(bot)
	return frame.String() + "\n" + strings.Join(footer, "\n")
}

func compareFrameTitle(width int) string {
	border := lipgloss.NewStyle().Foreground(style.BorderColor())
	titleSt := lipgloss.NewStyle().Foreground(style.PrimaryColor()).Bold(true)
	horiz := border.Render("─")
	label := " Compare "
	prefix := border.Render("╭") + horiz + titleSt.Render(label) + horiz
	remain := width - lipgloss.Width(prefix) - 1
	if remain < 0 {
		remain = 0
	}
	return prefix + strings.Repeat(horiz, remain) + border.Render("╮")
}

func compareFit(s string, width int) string {
	return textwidth.PadRight(textwidth.Truncate(s, width), width)
}

func compareArtStatRow(g compareLayoutGeom, artLine, stat string, colW int) string {
	statBudget := colW - g.padL - g.artOW - g.gap
	if statBudget < 1 {
		statBudget = 1
	}
	// Fixed columns: never let ANSI art width steal the stats budget.
	art := compareFit(artLine, g.artOW)
	st := compareFit(stat, statBudget)
	return compareFit(strings.Repeat(" ", g.padL)+art+strings.Repeat(" ", g.gap)+st, colW)
}

// compareLayoutGeom is shared by compareView and dual native CUP painting.
type compareLayoutGeom struct {
	innerW, innerH           int
	colWA, colWB             int // exact halves: colWA+1+colWB == innerW
	drawArt                  bool
	artW, artH, artOW, artOH int
	padL, gap, coreW, blockH int
}

func (m Model) compareFooterH() int {
	return 1
}

func (m Model) compareLayoutGeom(footerH int) compareLayoutGeom {
	if footerH < 1 {
		footerH = m.compareFooterH()
	}
	// Full-screen frame: no outside title row.
	frameH := m.height - footerH
	if frameH < 10 {
		frameH = 10
	}
	g := compareLayoutGeom{
		innerW: m.width - 2,
		innerH: frameH - 2,
		padL:   1,
		gap:    1,
	}
	if g.innerW < 20 {
		g.innerW = 20
	}
	// Equal split: colWA + rule + colWB == innerW (even innerW: B gets the spare column).
	g.colWA = (g.innerW - 1) / 2
	g.colWB = g.innerW - 1 - g.colWA
	colMin := g.colWA
	if g.colWB < colMin {
		colMin = g.colWB
	}
	const (
		headerRows = 4 // blank + name + type + blank
		statsRows  = 8 // 6 stats + divider + total
		minCoreW   = 18
	)
	showArt := !m.layoutTh.DisableSprite && m.width >= 72 && m.height >= 16
	bucket := views.GlanceBucket
	if m.compareArtCells >= sprite.SmallestFrameBucket() {
		bucket = m.compareArtCells
	}
	g.artW, g.artH = sprite.FrameSizeFor(bucket)
	g.artOW, g.artOH = g.artW+2, g.artH+2
	g.blockH = g.artOH
	if statsRows > g.blockH {
		g.blockH = statsRows
	}
	needW := g.padL + g.artOW + g.gap + minCoreW
	g.drawArt = showArt && needW <= colMin && headerRows+g.blockH <= g.innerH
	if g.drawArt {
		g.coreW = colMin - g.padL - g.artOW - g.gap
	} else {
		g.coreW = colMin
		if g.coreW > 28 {
			g.coreW = 28
		}
	}
	return g
}

// compareArtOrigin returns 1-indexed CUP row/col for side 0 (A) or 1 (B).
func (m Model) compareArtOrigin(side int) (row, col, artW, artH int, ok bool) {
	g := m.compareLayoutGeom(m.compareFooterH())
	if !g.drawArt || m.width < 1 || m.height < 1 {
		return 0, 0, 0, 0, false
	}
	// row1=top border, then blank+name+type+blank, card top, then art content.
	artPadY := (g.artOH - 2 - g.artH) / 2
	if artPadY < 0 {
		artPadY = 0
	}
	const headerRows = 4
	row = 1 + headerRows + 1 + 1 + artPadY // border + header + card top + first content
	artPadX := (g.artOW - 2 - g.artW) / 2
	if artPadX < 0 {
		artPadX = 0
	}
	if side == 0 {
		col = 2 + g.padL + 1 + artPadX
	} else {
		col = 2 + g.colWA + 1 + g.padL + 1 + artPadX
	}
	return row, col, g.artW, g.artH, true
}

func formatCompareDelta(diff int) string {
	if diff == 0 {
		return style.StatusMuted("0")
	}
	s := fmt.Sprintf("%+d", diff)
	if diff > 0 {
		return lipgloss.NewStyle().Foreground(style.SuccessColor()).Render(s)
	}
	return lipgloss.NewStyle().Foreground(style.ErrorColor()).Render(s)
}

func compareSideHeader(e compareEntry, genus string) string {
	head := style.TypeSummary(e.Summary.Types)
	if genus != "" {
		head += " · " + genus
	}
	return head
}

// compareStatLines packs label | gap≤2 | value [| sep | Δ] [| padR].
// Spare width trails after the cluster (compareFit) — never inflates the middle gap.
func compareStatLines(labels []string, vals []int, has bool, deltas []string, budget int) []string {
	if budget < 1 {
		budget = 1
	}
	labelW := 0
	for _, l := range labels {
		if w := textwidth.Width(l); w > labelW {
			labelW = w
		}
	}
	valsS := make([]string, len(labels))
	deltasS := make([]string, len(labels))
	valW, deltaW := 0, 0
	for i := range labels {
		s := "—"
		if has {
			s = fmt.Sprintf("%d", vals[i])
		}
		valsS[i] = s
		if w := textwidth.Width(s); w > valW {
			valW = w
		}
		if deltas != nil && i < len(deltas) && deltas[i] != "" {
			deltasS[i] = deltas[i]
			if w := textwidth.Width(deltas[i]); w > deltaW {
				deltaW = w
			}
		}
	}
	const maxGap = 2
	sep, padR := 0, 0
	if deltaW > 0 {
		sep = 2
	}
	gap := maxGap
	fits := func() bool {
		return labelW+gap+valW+sep+deltaW+padR <= budget
	}
	if budget >= labelW+gap+valW+sep+deltaW+1 {
		padR = 1
	}
	if !fits() {
		padR = 0
	}
	if !fits() && sep == 2 {
		sep = 1
	}
	if !fits() {
		gap = 1
	}
	if !fits() {
		room := budget - labelW - gap - sep - padR
		if deltaW > 0 {
			if room < valW+deltaW {
				if room > valW+1 {
					deltaW = room - valW
				} else {
					valW = max(1, room/2)
					deltaW = max(1, room-valW)
				}
			}
		} else if room < valW {
			valW = max(1, room)
		}
	}

	padNum := func(s string, w int) string {
		if textwidth.Width(s) > w {
			s = textwidth.Truncate(s, w)
		}
		return strings.Repeat(" ", w-textwidth.Width(s)) + s
	}
	row := func(label, val, delta string) string {
		line := featureAccent(textwidth.PadRight(label, labelW)) + strings.Repeat(" ", gap) + padNum(val, valW)
		if deltaW > 0 {
			line += strings.Repeat(" ", sep) + padNum(delta, deltaW)
		}
		if padR > 0 {
			line += strings.Repeat(" ", padR)
		}
		return compareFit(line, budget)
	}
	div := strings.Repeat("─", labelW) + strings.Repeat(" ", gap) + strings.Repeat("─", valW)
	if deltaW > 0 {
		div += strings.Repeat(" ", sep) + strings.Repeat("─", deltaW)
	}
	if padR > 0 {
		div += strings.Repeat(" ", padR)
	}
	div = compareFit(div, budget)

	out := make([]string, 0, len(labels)+1)
	for i, label := range labels {
		if i == len(labels)-1 {
			out = append(out, div)
		}
		out = append(out, row(label, valsS[i], deltasS[i]))
	}
	return out
}

func padCompareBlock(block string, width, height, padL int) []string {
	src := strings.Split(strings.TrimRight(block, "\n"), "\n")
	out := make([]string, height)
	pad := strings.Repeat(" ", padL)
	for i := 0; i < height; i++ {
		line := pad
		if i < len(src) {
			line = pad + src[i]
		}
		if !strings.Contains(line, "\x1b") {
			line = textwidth.PadRight(textwidth.Truncate(line, width), width)
		} else {
			for textwidth.Width(line) < width && !strings.Contains(line, "\x1bP") {
				line += " "
			}
		}
		out[i] = line
	}
	return out
}

func (m Model) compareArtCard(side int, e compareEntry, artW, artH, cardW, cardH int, borderFg color.Color) string {
	ansi := ""
	native := false
	if side == 0 && m.compareArtAID == e.Summary.ID {
		ansi, native = m.compareArtA, m.compareArtANative
	} else if side == 1 && m.compareArtBID == e.Summary.ID {
		ansi, native = m.compareArtB, m.compareArtBNative
	}
	if ansi == "" && m.imageSpeciesID == e.Summary.ID && !m.imagePlaceholder && m.imageANSI != "" && !m.imageNative {
		ansi = m.imageANSI
	}
	var inner string
	switch {
	case ansi != "" && !native:
		inner = components.SpritePanel(ansi, artW, artH, false)
	case ansi != "" && native:
		// Cell reservation only — DCS is CUP-painted by emitCompareNativeOverlay.
		blank := make([]string, artH)
		for i := range blank {
			blank[i] = strings.Repeat(" ", artW)
		}
		inner = strings.Join(blank, "\n")
	default:
		inner = components.SpritePlaceholder(e.Summary.DisplayName, artW, artH)
	}
	body := comparePadArtInCard(inner, artW, artH, cardW-2, cardH-2)
	return components.InnerCardBorder("", cardW, cardH, strings.Split(body, "\n"), borderFg)
}

func comparePadArtInCard(art string, artW, artH, innerW, innerH int) string {
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
