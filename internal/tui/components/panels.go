package components

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/output"
	"github.com/Graphixa/Pokedex/internal/textwidth"
	"github.com/Graphixa/Pokedex/internal/tui/style"
)

func borderSty() lipgloss.Style {
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(style.BorderColor())
}

func mutedSty() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(style.BorderColor()).Faint(true)
}

func boldSty() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(style.PrimaryColor()).Bold(true)
}

// Search renders the left-column search prompt (fallback when textinput view is absent).
func Search(query string, width int) string {
	if width < 8 {
		width = 8
	}
	prompt := "> "
	body := query
	if body == "" {
		body = mutedSty().Render("Search Pokémon…")
		line := prompt + body
		return textwidth.PadRight(line, width)
	}
	line := prompt + body
	return textwidth.PadRight(textwidth.Truncate(line, width), width)
}

// PadSearchLine fits a pre-rendered bubbles/textinput row into the column width.
func PadSearchLine(line string, width int) string {
	if width < 8 {
		width = 8
	}
	return textwidth.PadRight(textwidth.Truncate(line, width), width)
}

// ListRow is one browse list entry.
type ListRow struct {
	Summary  domain.SpeciesSummary
	Selected bool
}

// List renders the filtered species list. Empty lists show a plain-text cue.
func List(rows []ListRow, width, height int, abbreviateTypes func([]string, int) string) string {
	if height < 1 {
		height = 1
	}
	if width < 12 {
		width = 12
	}
	if len(rows) == 0 {
		msg := style.StatusMuted("No matches")
		lines := make([]string, height)
		lines[0] = textwidth.PadRight(textwidth.Truncate(msg, width), width)
		for i := 1; i < height; i++ {
			lines[i] = strings.Repeat(" ", width)
		}
		return strings.Join(lines, "\n")
	}
	lines := make([]string, 0, height)
	for i := 0; i < height; i++ {
		if i >= len(rows) {
			lines = append(lines, strings.Repeat(" ", width))
			continue
		}
		row := rows[i]
		marker := "  "
		if row.Selected {
			marker = "> "
		}
		nameWidth := width/2 - 1
		if nameWidth < 8 {
			nameWidth = 8
		}
		typeBudget := width - nameWidth - 8 - len(marker)
		if typeBudget < 4 {
			typeBudget = 4
		}
		types := ""
		if abbreviateTypes != nil {
			types = abbreviateTypes(row.Summary.Types, typeBudget)
		} else {
			types = style.TypeSummary(row.Summary.Types)
		}
		line := fmt.Sprintf("%s%s %s  %s",
			marker,
			textwidth.PadRight(textwidth.Truncate(row.Summary.DisplayName, nameWidth), nameWidth),
			output.FormatDex(row.Summary.ID),
			types,
		)
		lines = append(lines, textwidth.PadRight(textwidth.Truncate(line, width), width))
	}
	return strings.Join(lines, "\n")
}

// SpritePlaceholder renders an unbordered 1:1 square with a centred [ Name ].
func SpritePlaceholder(name string, side int) string {
	if side < 5 {
		side = 5
	}
	label := "[ " + name + " ]"
	if textwidth.Width(label) > side {
		label = textwidth.Truncate(label, side)
	}
	return lipgloss.Place(side, side, lipgloss.Center, lipgloss.Center, label)
}

// SpritePanel renders precomputed half-block ANSI as an unbordered square.
// ansi must already be cells×cells lines; it is clipped/padded to side×side.
func SpritePanel(ansi string, side int) string {
	if side < 5 {
		side = 5
	}
	lines := strings.Split(ansi, "\n")
	if len(lines) > side {
		lines = lines[:side]
	}
	for len(lines) < side {
		lines = append(lines, strings.Repeat(" ", side))
	}
	for i := range lines {
		w := lipgloss.Width(lines[i])
		if w < side {
			lines[i] = lines[i] + strings.Repeat(" ", side-w)
		} else if w > side {
			lines[i] = lipgloss.NewStyle().MaxWidth(side).Render(lines[i])
			if lipgloss.Width(lines[i]) < side {
				lines[i] += strings.Repeat(" ", side-lipgloss.Width(lines[i]))
			}
		}
	}
	return strings.Join(lines, "\n")
}

// Header renders "Name #NNN".
func Header(name string, id int, width int) string {
	line := fmt.Sprintf("%s %s", name, output.FormatDex(id))
	return boldSty().Render(textwidth.PadRight(textwidth.Truncate(line, width), width))
}

// MainInfo holds right-pane identity fields.
type MainInfo struct {
	Types       string
	Genus       string
	Height      string
	Weight      string
	Ability     string
	Hidden      string
	BaseExp     int
	Summary     string
	ShowSummary bool
	Loading     bool
	Err         string
}

// InfoBlock renders the main info section.
func InfoBlock(info MainInfo, width int) string {
	if info.Loading {
		return style.StatusLoading("Loading…")
	}
	if info.Err != "" {
		return style.StatusError(info.Err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Type: %s\n", dash(info.Types))
	fmt.Fprintf(&b, "Species: %s\n", dash(info.Genus))
	fmt.Fprintf(&b, "Height: %s    Weight: %s\n", dash(info.Height), dash(info.Weight))
	fmt.Fprintf(&b, "Ability: %s\n", dash(info.Ability))
	fmt.Fprintf(&b, "Hidden: %s\n", dash(info.Hidden))
	fmt.Fprintf(&b, "Base Exp: %d\n", info.BaseExp)
	if info.ShowSummary {
		fmt.Fprintf(&b, "\nSummary\n")
		for _, line := range wrap(info.Summary, width) {
			fmt.Fprintf(&b, "%s\n", line)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// SummaryColumn renders a "Summary" heading and a clipped, offset-scrollable body.
func SummaryColumn(summary string, width, height, offset int) string {
	if width < 8 {
		width = 8
	}
	if height < 2 {
		height = 2
	}
	title := boldSty().Render("Summary")
	lines := []string{textwidth.PadRight(title, width)}
	bodyH := height - 1
	if summary == "" {
		lines = append(lines, textwidth.PadRight(mutedSty().Render("—"), width))
		for len(lines) < height {
			lines = append(lines, strings.Repeat(" ", width))
		}
		return strings.Join(lines[:height], "\n")
	}
	wrapped := wrap(summary, width)
	if offset < 0 {
		offset = 0
	}
	if offset > len(wrapped) {
		offset = len(wrapped)
	}
	for i := 0; i < bodyH; i++ {
		idx := offset + i
		if idx < len(wrapped) {
			lines = append(lines, textwidth.PadRight(wrapped[idx], width))
		} else {
			lines = append(lines, strings.Repeat(" ", width))
		}
	}
	return strings.Join(lines, "\n")
}

// SummaryWrappedLines returns the wrapped summary lines for scroll math.
func SummaryWrappedLines(summary string, width int) []string {
	if width < 8 {
		width = 8
	}
	return wrap(summary, width)
}

// InfoModal renders a centred overlay with species info details.
func InfoModal(title string, lines []string, width, height int) string {
	body := make([]string, 0, len(lines)+4)
	body = append(body, title, "")
	body = append(body, lines...)
	body = append(body, "", "Press i or Esc to close")
	cardW := min(width-4, 56)
	if cardW < 28 {
		cardW = min(width, 28)
	}
	styleCard := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(style.BorderColor()).
		Padding(1, 2).
		Width(cardW)
	var b strings.Builder
	for i, line := range body {
		if i == 0 {
			b.WriteString(boldSty().Render(line))
		} else {
			b.WriteString(line)
		}
		if i < len(body)-1 {
			b.WriteByte('\n')
		}
	}
	card := styleCard.Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, card)
}

// Tabs renders the six detail tabs with underline on the active tab.
// Uses ANSI-aware width so underline styles are not sliced mid-escape.
func Tabs(active int, labels []string, width int) string {
	parts := make([]string, 0, len(labels))
	for i, label := range labels {
		parts = append(parts, style.TabToken(label, i == active))
	}
	line := strings.Join(parts, " ")
	if width <= 0 {
		return line
	}
	if textwidth.Width(line) <= width {
		return textwidth.PadRight(line, width)
	}
	// Drop inactive trailing tabs until the row fits; never slice ANSI mid-token.
	for len(parts) > 1 && textwidth.Width(strings.Join(parts, " ")) > width {
		parts = parts[:len(parts)-1]
	}
	line = strings.Join(parts, " ")
	if textwidth.Width(line) > width {
		line = lipgloss.NewStyle().MaxWidth(width).Render(line)
	}
	return textwidth.PadRight(line, width)
}

// DetailCardTitled renders a titled rounded card matching tui-example.md:
//
//	╭─  Evolution  ─────────╮
//	│ body…                 │
//	╰───────────────────────╯
func DetailCardTitled(title string, lines []string, offset, width, height int) string {
	if width < 12 {
		width = 12
	}
	if height < 3 {
		height = 3
	}
	innerW := width - 2
	bodyH := height - 2
	if bodyH < 1 {
		bodyH = 1
	}

	border := lipgloss.NewStyle().Foreground(style.BorderColor())
	tl, tr := border.Render("╭"), border.Render("╮")
	bl, br := border.Render("╰"), border.Render("╯")
	vert := border.Render("│")
	horiz := border.Render("─")

	titleBit := "  " + title + "  "
	prefix := tl + horiz + titleBit
	remain := innerW - textwidth.Width(horiz+titleBit)
	if remain < 0 {
		titleBit = textwidth.TruncatePlain("  "+title+"  ", innerW-1)
		prefix = tl + horiz + titleBit
		remain = innerW - textwidth.Width(horiz+titleBit)
		if remain < 0 {
			remain = 0
		}
	}
	top := prefix + strings.Repeat(horiz, remain) + tr
	bottom := bl + strings.Repeat(horiz, innerW) + br

	var b strings.Builder
	b.WriteString(top)
	b.WriteByte('\n')
	for i := 0; i < bodyH; i++ {
		idx := offset + i
		var body string
		if idx >= 0 && idx < len(lines) {
			body = textwidth.PadRight(textwidth.Truncate(lines[idx], innerW), innerW)
		} else {
			body = strings.Repeat(" ", innerW)
		}
		b.WriteString(vert)
		b.WriteString(body)
		b.WriteString(vert)
		if i < bodyH-1 {
			b.WriteByte('\n')
		}
	}
	b.WriteByte('\n')
	b.WriteString(bottom)
	return b.String()
}

// Footer renders the compact hint line (wireframe wording).
func Footer(verbose bool, width int) string {
	var text string
	if verbose {
		text = " Type to filter • ↑↓ move • Tab tabs • i info • PgUp/PgDn scroll • Esc clear • q quit"
	} else {
		text = " type • ↑↓ • Tab • i • PgUp/PgDn • Esc • q"
	}
	return mutedSty().Render(textwidth.PadRight(textwidth.Truncate(text, width), width))
}

// HelpOverlay renders the keybinding help card.
// body is typically bubbles/help FullHelpView output; a built-in fallback is used when empty.
func HelpOverlay(body string, width, height int) string {
	if strings.TrimSpace(body) == "" {
		body = strings.Join([]string{
			"Browse help",
			"",
			"Type to filter • ↑/↓ move • Tab tabs",
			"i info • PgUp/PgDn scroll • Esc clear • q quit",
			"",
			"Press Esc or ? to close",
		}, "\n")
	} else {
		body = "Browse help\n\n" + body + "\n\nPress Esc or ? to close"
	}
	styleCard := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(min(width-4, 64))
	card := styleCard.Render(body)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, card)
}

// TooSmall renders the terminal-too-small full-screen cue.
func TooSmall(width, height int) string {
	msg := strings.Join([]string{
		"Terminal too small",
		"",
		"Resize to at least 40×12",
		"for browse mode.",
		"",
		"Press q or Ctrl+C to quit",
	}, "\n")
	if width < 8 {
		width = 8
	}
	if height < 4 {
		height = 4
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
		style.StatusWarn(msg))
}

func wrap(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	var cur strings.Builder
	for _, word := range words {
		if cur.Len() == 0 {
			cur.WriteString(word)
			continue
		}
		candidate := cur.String() + " " + word
		if textwidth.Width(candidate) > width {
			lines = append(lines, cur.String())
			cur.Reset()
			cur.WriteString(word)
			continue
		}
		cur.WriteByte(' ')
		cur.WriteString(word)
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
