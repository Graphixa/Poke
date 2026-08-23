package views

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/output"
	"github.com/Graphixa/Pokedex/internal/tui/components"
)

// BrowseData is the pure view model for browse mode.
type BrowseData struct {
	Width, Height int

	Query    string
	Rows     []components.ListRow
	ListOff  int
	ListRows int

	Selected    domain.SpeciesSummary
	HasSelected bool

	ShowSprite    bool
	ShowSummary   bool
	ShortTabs     bool
	VerboseFooter bool
	OnePane       bool
	TooSmall      bool
	ShowListPane  bool // one-pane: list while filtering

	ActiveTab int
	TabLabels []string

	Info components.MainInfo

	DetailTitle string
	DetailLines []string
	DetailOff   int

	ShowHelp bool

	// SpriteANSI is half-block cell art for the sprite square; empty uses placeholder.
	SpriteANSI string
	SpriteSide int
}

// Browse renders the wireframe layout from tui-example.md.
func Browse(d BrowseData) string {
	if d.Width <= 0 {
		d.Width = 120
	}
	if d.Height <= 0 {
		d.Height = 36
	}

	if d.ShowHelp {
		return components.HelpOverlay(d.Width, d.Height)
	}
	if d.TooSmall {
		return components.TooSmall(d.Width, d.Height)
	}

	title := "Pokedex — Browse Mode"
	footer := components.Footer(d.VerboseFooter, d.Width)
	bodyHeight := d.Height - 2 // title + footer
	if bodyHeight < 5 {
		bodyHeight = 5
	}

	var body string
	if d.OnePane {
		body = renderOnePane(d, bodyHeight)
	} else {
		body = renderTwoPane(d, bodyHeight)
	}

	// Keep title and footer intact; trim only the body if it overruns.
	// Stable line budget reduces flicker on resize.
	bodyLines := strings.Split(body, "\n")
	if len(bodyLines) > bodyHeight {
		bodyLines = bodyLines[:bodyHeight]
		body = strings.Join(bodyLines, "\n")
	} else if len(bodyLines) < bodyHeight {
		pad := make([]string, bodyHeight-len(bodyLines))
		for i := range pad {
			pad[i] = ""
		}
		body = body + "\n" + strings.Join(pad, "\n")
	}
	return title + "\n" + body + "\n" + footer
}

func renderOnePane(d BrowseData, height int) string {
	if d.ShowListPane || !d.HasSelected {
		search := components.Search(d.Query, d.Width)
		listH := height - 1
		visible := visibleRows(d.Rows, d.ListOff, listH)
		list := components.List(visible, d.Width, listH, abbreviate)
		return search + "\n" + list
	}
	return renderRight(d, d.Width, height)
}

func renderTwoPane(d BrowseData, height int) string {
	leftW := d.Width / 3
	if leftW < 24 {
		leftW = 24
	}
	rightW := d.Width - leftW
	if rightW < 20 {
		rightW = 20
		leftW = d.Width - rightW
	}

	left := renderLeft(d, leftW, height)
	right := renderRight(d, rightW, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func renderLeft(d BrowseData, width, height int) string {
	search := components.Search(d.Query, width)
	spriteH := 0
	var sprite string
	if d.ShowSprite && d.HasSelected {
		side := d.SpriteSide
		if side <= 0 {
			side = spriteSideFor(width, height)
		}
		if side >= 5 {
			if d.SpriteANSI != "" {
				sprite = components.SpritePanel(d.SpriteANSI, side)
			} else {
				sprite = components.SpritePlaceholder(d.Selected.DisplayName, side)
			}
			spriteH = lipgloss.Height(sprite)
		}
	}
	listH := height - 1 - spriteH
	if listH < 3 {
		listH = 3
	}
	visible := visibleRows(d.Rows, d.ListOff, listH)
	list := components.List(visible, width, listH, abbreviate)

	parts := []string{search, list}
	if sprite != "" {
		parts = append(parts, sprite)
	}
	col := lipgloss.JoinVertical(lipgloss.Left, parts...)
	// Hard-clamp to the pane height so JoinHorizontal cannot push the footer off-screen.
	colLines := strings.Split(col, "\n")
	if len(colLines) > height {
		col = strings.Join(colLines[:height], "\n")
	}
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		Render(col)
}

// SpriteSideFor mirrors the left-column sprite sizing used by the browse layout.
func SpriteSideFor(width, height int) int {
	return spriteSideFor(width, height)
}

func spriteSideFor(width, height int) int {
	side := width - 2
	maxSide := height / 3
	if maxSide < 6 {
		maxSide = 6
	}
	if side > maxSide {
		side = maxSide
	}
	if side < 5 {
		side = 5
	}
	if side > height-4 {
		side = height - 4
	}
	if side < 5 {
		return 0
	}
	return side
}

func renderRight(d BrowseData, width, height int) string {
	if !d.HasSelected {
		return lipgloss.NewStyle().Width(width).Height(height).Render("Select a Pokémon")
	}

	header := components.Header(d.Selected.DisplayName, d.Selected.ID, width)
	info := components.InfoBlock(d.Info, width-2)
	tabs := components.Tabs(d.ActiveTab, d.TabLabels, width)

	used := lipgloss.Height(header) + lipgloss.Height(info) + lipgloss.Height(tabs) + 2 // dividers
	detailH := height - used
	if detailH < 4 {
		detailH = 4
	}
	card := components.DetailCardTitled(d.DetailTitle, d.DetailLines, d.DetailOff, width, detailH)

	divider := strings.Repeat("─", max(1, width))
	col := lipgloss.JoinVertical(lipgloss.Left,
		header,
		info,
		divider,
		tabs,
		divider,
		card,
	)
	return lipgloss.NewStyle().Width(width).Height(height).Render(col)
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

func abbreviate(types []string, max int) string {
	joined := output.JoinTypes(types)
	if max <= 0 || len(joined) <= max {
		return joined
	}
	if len(types) < 2 {
		return truncate(joined, max)
	}
	first := output.TitleFromSlug(types[0])
	second := output.TitleFromSlug(types[1])
	if len([]rune(second)) > 4 {
		second = string([]rune(second)[:4])
	}
	out := first + "/" + second
	if len(out) > max {
		return truncate(out, max)
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

// FormatHeight formats decimeters as meters.
func FormatHeight(dm int) string {
	return fmt.Sprintf("%.1f m", float64(dm)/10)
}

// FormatWeight formats hectograms as kilograms.
func FormatWeight(hg int) string {
	return fmt.Sprintf("%.1f kg", float64(hg)/10)
}
