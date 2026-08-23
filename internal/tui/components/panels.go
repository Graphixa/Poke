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

var (
	borderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
	mutedStyle  = lipgloss.NewStyle().Faint(true)
	boldStyle   = lipgloss.NewStyle().Bold(true)
)

// Search renders the left-column search prompt.
func Search(query string, width int) string {
	if width < 8 {
		width = 8
	}
	prompt := "> "
	body := query
	if body == "" {
		body = mutedStyle.Render("Search Pokémon…")
		line := prompt + body
		return textwidth.PadRight(line, width)
	}
	line := prompt + body
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

// SpritePlaceholder renders a bordered 1:1 square with [ Name ].
func SpritePlaceholder(name string, side int) string {
	if side < 5 {
		side = 5
	}
	label := "[ " + name + " ]"
	innerW := side - 2
	if innerW < 3 {
		innerW = 3
	}
	if textwidth.Width(label) > innerW {
		label = textwidth.Truncate(label, innerW)
	}
	inner := lipgloss.Place(innerW, innerW, lipgloss.Center, lipgloss.Center, label)
	return borderStyle.Render(inner)
}

// SpritePanel renders precomputed half-block ANSI inside a bordered square.
// ansi must already be cells×cells lines; it is clipped to the inner square.
func SpritePanel(ansi string, side int) string {
	if side < 5 {
		side = 5
	}
	inner := side - 2
	if inner < 3 {
		inner = 3
	}
	lines := strings.Split(ansi, "\n")
	if len(lines) > inner {
		lines = lines[:inner]
	}
	for len(lines) < inner {
		lines = append(lines, strings.Repeat(" ", inner))
	}
	for i := range lines {
		// Pad visually when ANSI width is shorter; lipgloss.Width ignores escapes.
		w := lipgloss.Width(lines[i])
		if w < inner {
			lines[i] = lines[i] + strings.Repeat(" ", inner-w)
		}
	}
	return borderStyle.Render(strings.Join(lines, "\n"))
}

// Header renders "Name #NNN".
func Header(name string, id int, width int) string {
	line := fmt.Sprintf("%s %s", name, output.FormatDex(id))
	return boldStyle.Render(textwidth.PadRight(textwidth.Truncate(line, width), width))
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

// Tabs renders the six detail tabs with underline on the active tab.
func Tabs(active int, labels []string, width int) string {
	parts := make([]string, 0, len(labels))
	for i, label := range labels {
		parts = append(parts, style.TabToken(label, i == active))
	}
	line := strings.Join(parts, " ")
	return textwidth.PadRight(textwidth.Truncate(line, width), width)
}

// DetailCardTitled renders a card with an inline title in the top border region.
func DetailCardTitled(title string, lines []string, offset, width, height int) string {
	if width < 12 {
		width = 12
	}
	if height < 4 {
		height = 4
	}
	innerW := width - 2
	innerH := height - 2
	header := "─  " + title + "  "
	for textwidth.Width(header) < innerW {
		header += "─"
	}
	header = textwidth.Truncate(header, innerW)

	bodyLines := make([]string, 0, innerH-1)
	for i := 0; i < innerH-1; i++ {
		idx := offset + i
		if idx >= 0 && idx < len(lines) {
			bodyLines = append(bodyLines, textwidth.PadRight(textwidth.Truncate(lines[idx], innerW), innerW))
		} else {
			bodyLines = append(bodyLines, strings.Repeat(" ", innerW))
		}
	}
	content := header + "\n" + strings.Join(bodyLines, "\n")
	return borderStyle.Width(innerW).Height(innerH).Render(content)
}

// Footer renders the compact hint line.
func Footer(verbose bool, width int) string {
	var text string
	if verbose {
		text = " Type to filter • ↑↓ move • Tab next panel • PgUp/PgDn scroll details • Esc clear • q quit • ? help"
	} else {
		text = " type • ↑↓ • Tab • PgUp/PgDn • Esc • q • ?"
	}
	return mutedStyle.Render(textwidth.PadRight(textwidth.Truncate(text, width), width))
}

// HelpOverlay renders the keybinding help card.
func HelpOverlay(width, height int) string {
	body := strings.Join([]string{
		"Browse help",
		"",
		"Type          Filter list (including q)",
		"Backspace     Delete from search",
		"Esc           Clear search (or close help)",
		"↑ / ↓         Move selection",
		"Tab / S-Tab   Cycle detail tabs",
		"PgUp / PgDn   Scroll detail panel",
		"Home / End    Jump in detail panel",
		"q             Quit when search empty",
		"Ctrl+C        Always quit",
		"?             Toggle this help",
		"",
		"Colour is optional: active tab is underlined;",
		"errors use [ERROR], warnings use [!].",
		"",
		"Press Esc or ? to close",
	}, "\n")
	styleCard := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(min(width-4, 56))
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
