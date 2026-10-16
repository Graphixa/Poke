package components

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/output"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/textwidth"
	"github.com/Graphixa/Pokedex/internal/tui/style"
)

func mutedSty() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(style.BorderColor())
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
	Favourite     bool
	Comparing     bool // compare pin — name-adjacent ⇄
	DividerBefore bool
	SectionLabel  string // muted chrome row before this entry (e.g. "Favourites · 6")
	Summary       domain.SpeciesSummary
	Selected      bool
}

// CompareListMark is appended to a pinned compare species' display name.
const CompareListMark = " ⇄"

// List renders the filtered species list. Empty lists show a plain-text cue.
func List(rows []ListRow, width, height int, nameCol int) string {
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
	if nameCol < 1 {
		for _, row := range rows {
			w := textwidth.Width(row.Summary.DisplayName)
			if row.Comparing {
				w += textwidth.Width(CompareListMark)
			}
			if w > nameCol {
				nameCol = w
			}
		}
		nameCol += 3
		if nameCol < 8 {
			nameCol = 8
		}
	}
	lines := make([]string, 0, height)
	for i := 0; len(lines) < height; i++ {
		if i >= len(rows) {
			lines = append(lines, strings.Repeat(" ", width))
			continue
		}
		row := rows[i]
		if row.SectionLabel != "" {
			lines = append(lines, style.StatusMuted(textwidth.PadRight(textwidth.Truncate(row.SectionLabel, width), width)))
			if len(lines) == height {
				break
			}
		}
		if row.DividerBefore && i > 0 {
			// Match list chrome / panel borders (not StatusMuted).
			lines = append(lines, lipgloss.NewStyle().Foreground(style.BorderColor()).Render(strings.Repeat("─", width)))
			if len(lines) == height {
				break
			}
		}
		cursor, star := " ", " "
		if row.Selected {
			cursor = ">"
		}
		if row.Favourite {
			star = "★"
		}
		marker := cursor + " " + star + " "
		dex := output.FormatDex(row.Summary.ID)
		rightW := textwidth.Width(dex) + 1 // one column of padding after right-aligned ID
		available := width - textwidth.Width(marker)
		if available < 8+rightW {
			lines = append(lines, textwidth.PadRight(marker+listDisplayName(row, available), width))
			continue
		}
		col := nameCol
		if col > available-rightW-1 {
			col = available - rightW - 1
		}
		if col < 1 {
			col = 1
		}
		namePart := marker + textwidth.PadRight(listDisplayName(row, col), col)
		midW := width - textwidth.Width(namePart) - rightW
		if midW < 0 {
			midW = 0
		}
		line := namePart + strings.Repeat(" ", midW) + dex + " "
		if textwidth.Width(line) < width {
			line = textwidth.PadRight(line, width)
		} else if textwidth.Width(line) > width {
			line = textwidth.Truncate(line, width)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func listDisplayName(row ListRow, budget int) string {
	mark := ""
	if row.Comparing {
		mark = CompareListMark
	}
	mw := textwidth.Width(mark)
	nameBudget := budget - mw
	if nameBudget < 1 {
		nameBudget = 1
	}
	return textwidth.Truncate(row.Summary.DisplayName, nameBudget) + mark
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
// graphics sequence (MaxWidth would corrupt DCS/Kitty payloads). Spaces use the
// theme base background so a cell flush is not terminal-default black.
func nativeSpritePanel(seq string, width, height int) string {
	blank := lipgloss.NewStyle().Background(style.BaseColor()).Render(strings.Repeat(" ", width))
	lines := make([]string, height)
	for i := range lines {
		lines[i] = blank
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
	return FooterSplit(verbose, width, "", warmHint)
}

// FooterRightWidth is the fixed character width of the browse footer status segment.
const FooterRightWidth = 42

// FooterSplit renders active-view keys on the left and an optional fixed-width
// status on the right. When rightStatus is empty, the help line uses the full
// terminal width (no phantom reserved column).
func FooterSplit(verbose bool, width int, hint, rightStatus string) string {
	right := strings.TrimSpace(rightStatus)
	footerRightWidth := 0
	if right != "" {
		footerRightWidth = FooterRightWidth
		right = textwidth.Truncate(right, footerRightWidth)
		if w := textwidth.Width(right); w < footerRightWidth {
			right = strings.Repeat(" ", footerRightWidth-w) + right
		}
	}

	leftW := width
	if footerRightWidth > 0 {
		leftW = width - footerRightWidth
		if leftW < 10 {
			leftW = width
			right = ""
		}
	}
	var leftText string
	if hint != "" {
		leftText = " " + hint
	} else {
		// Fallback when callers omit FooterHint (warm-footer tests / Footer()).
		fallback := []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "details")),
			key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "view")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear/quit")),
		}
		if verbose {
			fallback = append([]key.Binding{
				key.NewBinding(key.WithKeys("up"), key.WithHelp("↑/↓", "move")),
			}, fallback...)
		}
		leftText = " " + style.RenderCommandHelp(fallback, max(1, leftW-1))
	}
	left := textwidth.PadRight(textwidth.Truncate(leftText, leftW), leftW)
	return left + footerSty().Render(right)
}

func footerSty() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(style.SecondaryColor())
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
