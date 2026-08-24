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
// Production wires tui.RenderCombinedFrame / CalculatePanelLayout / FillTerminalArea.
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
	SearchView string // pre-rendered bubbles/textinput row; falls back to Query
	Rows       []components.ListRow
	ListOff    int
	ListRows   int

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
	HelpBody string // bubbles/help full view when ShowHelp
	ShowInfo bool

	SpriteANSI string
	SpriteSide int

	// SummaryOff is the auto-scroll line offset within the summary block.
	SummaryOff int
	// IdentityHeight is the fixed height of the (info+summary)|sprite band.
	IdentityHeight int

	// Frame overrides DefaultFrame when set.
	Frame *FrameRenderer
}

// Browse renders the wireframe layout from tui-example.md using FontGet-style
// shared outer border with title in the top edge.
func Browse(d BrowseData) string {
	if d.Width <= 0 {
		d.Width = 120
	}
	if d.Height <= 0 {
		d.Height = 36
	}
	fr := DefaultFrame
	if d.Frame != nil {
		fr = *d.Frame
	}

	if d.ShowHelp {
		return components.HelpOverlay(d.HelpBody, d.Width, d.Height)
	}
	if d.ShowInfo {
		title := "Pokémon Info"
		if d.HasSelected {
			title = fmt.Sprintf("%s %s", d.Selected.DisplayName, output.FormatDex(d.Selected.ID))
		}
		return components.InfoModal(title, d.DetailLines, d.Width, d.Height)
	}
	if d.TooSmall {
		return components.TooSmall(d.Width, d.Height)
	}

	footer := components.Footer(d.VerboseFooter, d.Width)
	footerH := 1
	margin := 0
	title := "Pokedex — Browse Mode"

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
	// Inner content width accounts for the shared frame's left/right border chars.
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
	listH := height - 2 // search + divider
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

// SpriteSideFor returns the fixed sprite frame width when the right pane can
// host it; otherwise 0. Height is unused (frame is square) but kept for callers.
func SpriteSideFor(rightWidth, _ int) int {
	if rightWidth < sprite.MinRightWidthForSprite {
		return 0
	}
	return sprite.FrameWidth
}

func renderRight(d BrowseData, width, height int) string {
	if !d.HasSelected {
		return lipgloss.NewStyle().Width(width).Height(height).Render("Select a Pokémon")
	}

	identity := renderIdentityBand(d, width)
	tabs := components.Tabs(d.ActiveTab, d.TabLabels, width)
	divider := lipgloss.NewStyle().Foreground(style.BorderColor()).Render(strings.Repeat("─", max(1, width)))

	used := lipgloss.Height(identity) + lipgloss.Height(tabs) + 2 // dividers
	detailH := height - used
	if detailH < 4 {
		detailH = 4
	}
	card := components.DetailCardTitled(d.DetailTitle, d.DetailLines, d.DetailOff, width, detailH)

	col := lipgloss.JoinVertical(lipgloss.Left,
		identity,
		divider,
		tabs,
		divider,
		card,
	)
	lines := strings.Split(col, "\n")
	if len(lines) > height {
		col = strings.Join(lines[:height], "\n")
	}
	return col
}

// renderIdentityBand matches the wireframe: main info + summary stacked on the
// left, unbordered sprite on the right, separated by " │ ".
func renderIdentityBand(d BrowseData, width int) string {
	side := 0
	frameH := 0
	var art string
	if d.ShowSprite {
		side = d.SpriteSide
		if side <= 0 {
			side = SpriteSideFor(width, 0)
		}
		if side >= sprite.FrameWidth {
			side = sprite.FrameWidth
			frameH = sprite.FrameHeight
			if d.SpriteANSI != "" {
				art = components.SpritePanel(d.SpriteANSI, side, frameH)
			} else if d.HasSelected {
				art = components.SpritePlaceholder(d.Selected.DisplayName, side, frameH)
			}
		} else {
			side = 0
			art = ""
		}
	}

	bandH := d.IdentityHeight
	if bandH <= 0 {
		bandH = frameH
		if bandH < identityMinBand {
			bandH = identityMinBand
		}
	}
	if art != "" && bandH < frameH {
		bandH = frameH
	}

	sepW := sprite.SeparatorWidth
	infoW := width
	if art != "" {
		infoW = width - side - sepW
		if infoW < sprite.MinInfoWidthBesideSprite {
			art = ""
			side = 0
			infoW = width
		}
	}

	core := d.Info
	core.ShowSummary = false // summary is rendered below as its own block
	header := components.Header(d.Selected.DisplayName, d.Selected.ID, infoW)
	info := components.InfoBlock(core, infoW)
	text := lipgloss.JoinVertical(lipgloss.Left, header, info)

	if d.Info.ShowSummary {
		remain := bandH - lipgloss.Height(text) - 1 // blank gap before Summary
		if remain < 3 {
			remain = 3
		}
		summary := components.SummaryColumn(d.Info.Summary, infoW, remain, d.SummaryOff)
		text = lipgloss.JoinVertical(lipgloss.Left, text, "", summary)
	}

	text = lipgloss.NewStyle().Width(infoW).Height(bandH).MaxHeight(bandH).Render(text)
	if art == "" {
		return text
	}
	sep := lipgloss.NewStyle().Foreground(style.BorderColor()).Render(" │ ")
	art = lipgloss.NewStyle().Width(side).Height(bandH).MaxHeight(bandH).Render(art)
	return lipgloss.JoinHorizontal(lipgloss.Top, text, sep, art)
}

const identityMinBand = 8

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
