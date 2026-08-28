package components

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/output"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/textwidth"
	"github.com/Graphixa/Pokedex/internal/tui/style"
)

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

// SpritePlaceholder renders an unbordered frame with a centred [ Name ].
func SpritePlaceholder(name string, width, height int) string {
	if width < 1 {
		width = sprite.FrameWidth
	}
	if height < 1 {
		height = sprite.FrameHeight
	}
	label := "[ " + name + " ]"
	if textwidth.Width(label) > width {
		label = textwidth.Truncate(label, width)
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, label)
}

// SpritePanel renders precomputed sprite art as an unbordered frame.
// For cell-art (chafa), ansi is clipped/padded to width×height cells.
// For native graphics (Kitty/iTerm/Sixel), the escape sequence is kept atomic
// and prefixed onto a width×height space reservation so layout still sees a box.
func SpritePanel(ansi string, width, height int, native bool) string {
	if width < 1 {
		width = sprite.FrameWidth
	}
	if height < 1 {
		height = sprite.FrameHeight
	}
	if native {
		return nativeSpritePanel(ansi, width, height)
	}
	lines := strings.Split(ansi, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	for i := range lines {
		w := lipgloss.Width(lines[i])
		if w < width {
			lines[i] = lines[i] + strings.Repeat(" ", width-w)
		} else if w > width {
			lines[i] = lipgloss.NewStyle().MaxWidth(width).Render(lines[i])
			if lipgloss.Width(lines[i]) < width {
				lines[i] += strings.Repeat(" ", width-lipgloss.Width(lines[i]))
			}
		}
	}
	return strings.Join(lines, "\n")
}

// nativeSpritePanel reserves a WxH cell box without measuring or clipping the
// graphics sequence (MaxWidth would corrupt DCS/Kitty payloads).
func nativeSpritePanel(seq string, width, height int) string {
	lines := make([]string, height)
	for i := range lines {
		lines[i] = strings.Repeat(" ", width)
	}
	if seq != "" {
		lines[0] = seq + lines[0]
	}
	return strings.Join(lines, "\n")
}

// Header renders "Name #NNN".
func Header(name string, id int, width int) string {
	line := fmt.Sprintf("%s %s", name, output.FormatDex(id))
	return boldSty().Render(textwidth.PadRight(textwidth.Truncate(line, width), width))
}

// Footer renders the compact hint line (wireframe wording).
func Footer(verbose bool, width int, warmHint string) string {
	return FooterSplit(verbose, width, warmHint)
}

// FooterSplit renders help keys on the left and a fixed-width status on the right.
func FooterSplit(verbose bool, width int, rightStatus string) string {
	const footerRightWidth = 38
	right := strings.TrimSpace(rightStatus)
	if len(right) > footerRightWidth {
		right = right[:footerRightWidth]
	}
	right = padLeft(padRight(right, footerRightWidth), footerRightWidth)

	leftW := width - footerRightWidth
	if leftW < 10 {
		leftW = width
		right = ""
	}
	var leftText string
	if verbose {
		leftText = " Type to filter • ↑↓ move • Enter/i details • Esc clear • q quit"
	} else {
		leftText = " type • ↑↓ • Enter/i • Esc • q"
	}
	left := textwidth.PadRight(textwidth.Truncate(leftText, leftW), leftW)
	return mutedSty().Render(left + right)
}

func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}

func padLeft(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return strings.Repeat(" ", w-len(s)) + s
}

// HelpOverlay renders the keybinding help card.
// body is typically bubbles/help FullHelpView output; a built-in fallback is used when empty.
func HelpOverlay(body string, width, height int) string {
	if strings.TrimSpace(body) == "" {
		body = strings.Join([]string{
			"Browse help",
			"",
			"Type to filter • ↑/↓ move",
			"Enter/i details • Esc clear • q quit",
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
