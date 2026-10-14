package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"

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
	if m.showFilters {
		return m.filterView()
	}
	return m.compareView()
}

// Shared full-screen chrome matches browse and dossier, including Bubbles help.
// Reserve footer rows before sizing the card so narrow terminals keep all keys.
func (m Model) featureFrame(title string, lines []string, bindings [][]key.Binding) string {
	h := m.help
	h.SetWidth(m.width)
	var footer []string
	for _, row := range bindings {
		footer = append(footer, h.ShortHelpView(row))
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

func (m Model) filterView() string {
	collection := "All Pokémon"
	if m.draftOptions.FavouritesOnly {
		collection = "Favourites only"
	}
	values := []string{collection, output.TitleFromSlug(filterTypes[m.draftOptions.Type]), filterSorts[m.draftOptions.Sort]}
	labels := []string{"Collection", "Type", "Sort"}
	descriptions := []string{"Choose all Pokémon or your saved favourites.", "Filter by either of a Pokémon's types.", "Favourites stay first within the chosen order."}
	lines := []string{style.StatusMuted("Choose a field, then change its value."), ""}
	for i, label := range labels {
		prefix := "  "
		value := values[i]
		if i == m.filterRow {
			prefix = "> "
			value = "‹ " + value + " ›"
		}
		row := prefix + textwidth.PadRight(label, 12) + value
		if i == m.filterRow {
			row = featureAccent(row)
		}
		lines = append(lines, row)
	}
	count := len(m.filterSpecies(m.index.Search(m.queryValue()), m.draftOptions))
	lines = append(lines, "", style.StatusMuted(descriptions[m.filterRow]), "", featureAccent(fmt.Sprintf("%d matches", count))+" · search is kept")
	if count == 0 {
		lines = append(lines, style.StatusMuted("Try another type or reset these filters."))
	} else if m.queryValue() != "" {
		lines = append(lines, "Search: "+m.queryValue())
	}
	if m.height >= 24 {
		choices, active := []string{"All Pokémon", "Favourites only"}, 0
		if m.draftOptions.FavouritesOnly {
			active = 1
		}
		switch m.filterRow {
		case 1:
			choices, active = filterTypes, m.draftOptions.Type
		case 2:
			choices, active = filterSorts, m.draftOptions.Sort
		}
		lines = append(lines, "", featureAccent("Choices"))
		line := ""
		for i, choice := range choices {
			if m.filterRow == 1 {
				choice = output.TitleFromSlug(choice)
			}
			if i == active {
				choice = featureAccent("[" + choice + "]")
			} else {
				choice = style.StatusMuted(choice)
			}
			if textwidth.Width(line)+textwidth.Width(choice)+3 > m.width-4 && line != "" {
				lines = append(lines, line)
				line = ""
			}
			if line != "" {
				line += " · "
			}
			line += choice
		}
		lines = append(lines, line)
	}
	return m.featureFrame("Filter & sort", lines, [][]key.Binding{
		{featureBinding("up", "↑/↓", "field"), featureBinding("right", "←/→", "change")},
		{featureBinding("enter", "enter", "apply"), featureBinding("backspace", "⌫", "reset"), featureBinding("esc", "esc", "cancel")},
	})
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
	helpM := m.help
	helpM.SetWidth(m.width)
	footer := []string{helpM.ShortHelpView(bindings)}

	g := m.compareLayoutGeom(len(footer))
	rule := lipgloss.NewStyle().Foreground(style.BorderColor()).Render("│")
	body := make([]string, 0, g.innerH)

	colLine := func(s string) string {
		return textwidth.PadRight(textwidth.Truncate(s, g.colW), g.colW)
	}
	nameA := fmt.Sprintf("%s %s", a.Summary.DisplayName, output.FormatDex(a.Summary.ID))
	nameB := fmt.Sprintf("%s %s", b.Summary.DisplayName, output.FormatDex(b.Summary.ID))
	if m.compareFocus == 0 {
		nameA = featureAccent(nameA)
	} else {
		nameB = featureAccent(nameB)
	}
	body = append(body, colLine(nameA)+rule+colLine(nameB))
	body = append(body, colLine(compareSideHeader(a, m.compareGenus(a.Summary.ID)))+rule+
		colLine(compareSideHeader(b, m.compareGenus(b.Summary.ID))))

	av := []int{a.Stats.HP, a.Stats.Attack, a.Stats.Defense, a.Stats.SpecialAttack, a.Stats.SpecialDefense, a.Stats.Speed, a.Stats.Total()}
	bv := []int{b.Stats.HP, b.Stats.Attack, b.Stats.Defense, b.Stats.SpecialAttack, b.Stats.SpecialDefense, b.Stats.Speed, b.Stats.Total()}
	labels := []string{"HP", "Attack", "Defence", "Sp. Attack", "Sp. Defence", "Speed", "Total"}
	statLinesA := make([]string, 0, 8)
	statLinesB := make([]string, 0, 8)
	for i, label := range labels {
		if i == 6 {
			div := compareStatDivider(g.coreW)
			statLinesA = append(statLinesA, div)
			statLinesB = append(statLinesB, div)
		}
		delta := ""
		if a.HasStats && b.HasStats {
			delta = formatCompareDelta(bv[i] - av[i])
		}
		statLinesA = append(statLinesA, compareStatCore(label, av[i], a.HasStats, g.coreW))
		right := compareStatCore(label, bv[i], b.HasStats, g.coreW)
		if delta != "" {
			right += " " + delta
		}
		statLinesB = append(statLinesB, right)
	}

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
			body = append(body, compareArtStatRow(g, artA[i], sa)+rule+compareArtStatRow(g, artB[i], sb))
		}
	} else {
		for i := range statLinesA {
			body = append(body, colLine(statLinesA[i])+rule+colLine(statLinesB[i]))
		}
	}

	for len(body) < g.innerH {
		body = append(body, strings.Repeat(" ", g.colW)+rule+strings.Repeat(" ", g.colW))
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
		cell := textwidth.PadRight(textwidth.Truncate(line, g.innerW), g.innerW)
		frame.WriteString(vert + cell + vert)
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

func compareArtStatRow(g compareLayoutGeom, artLine, stat string) string {
	gap := strings.Repeat(" ", g.gap)
	pad := strings.Repeat(" ", g.padL)
	statBudget := g.colW - g.padL - g.artOW - g.gap
	if statBudget < 1 {
		statBudget = 1
	}
	return textwidth.PadRight(pad+artLine+gap+textwidth.Truncate(stat, statBudget), g.colW)
}

// compareLayoutGeom is shared by compareView and dual native CUP painting.
type compareLayoutGeom struct {
	innerW, innerH, colW     int
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
	g.colW = (g.innerW - 1) / 2
	if g.colW < 10 {
		g.colW = 10
	}
	const (
		headerRows = 2 // name + type
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
	g.drawArt = showArt && needW <= g.colW && headerRows+g.blockH <= g.innerH
	if g.drawArt {
		budget := g.colW - g.padL - g.artOW - g.gap
		// Leave a little room so B's Δ isn't clipped off the core band.
		g.coreW = budget
		if budget >= 25 {
			g.coreW = budget - 7
		}
		if g.coreW < minCoreW {
			g.coreW = min(minCoreW, budget)
		}
	} else {
		g.coreW = g.colW
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
	// row1=top border, row2=name, row3=type, row4=card top, row5=art
	artPadY := (g.artOH - 2 - g.artH) / 2
	if artPadY < 0 {
		artPadY = 0
	}
	row = 1 + 2 + 1 + 1 + artPadY
	artPadX := (g.artOW - 2 - g.artW) / 2
	if artPadX < 0 {
		artPadX = 0
	}
	if side == 0 {
		col = 2 + g.padL + 1 + artPadX
	} else {
		col = 2 + g.colW + 1 + g.padL + 1 + artPadX
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

// compareStatCore is the symmetrical label+value band (no Δ). Labels use primary accent.
func compareStatCore(label string, v int, has bool, coreW int) string {
	const labelW = 12 // "Sp. Defence"
	valBudget := coreW - labelW
	if valBudget < 4 {
		valBudget = 4
	}
	val := "—"
	if has {
		val = fmt.Sprintf("%d", v)
	}
	gap := valBudget - textwidth.Width(val)
	if gap < 1 {
		gap = 1
		val = textwidth.Truncate(val, valBudget-1)
	}
	return featureAccent(textwidth.PadRight(label, labelW)) + strings.Repeat(" ", gap) + val
}

func compareStatDivider(coreW int) string {
	const labelW = 12
	valBudget := coreW - labelW
	if valBudget < 4 {
		valBudget = 4
	}
	line := strings.Repeat(" ", labelW) + strings.Repeat("─", min(valBudget, 15))
	return textwidth.PadRight(textwidth.Truncate(line, coreW), coreW)
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
