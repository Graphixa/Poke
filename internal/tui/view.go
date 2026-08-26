package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/tui/components"
	"github.com/Graphixa/Pokedex/internal/tui/style"
	"github.com/Graphixa/Pokedex/internal/tui/views"
)

func init() {
	views.DefaultFrame = views.FrameRenderer{
		Calculate: func(termW, termH, headerH, footerH, margin, sep int) (int, int, int) {
			lay := CalculatePanelLayout(termW, termH, headerH, footerH, margin, sep)
			return lay.LeftWidth, lay.RightWidth, lay.PanelHeight
		},
		Combine: RenderCombinedFrame,
		Fill:    FillTerminalArea,
	}
}

func (m Model) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}
	content := views.Browse(m.browseData())
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m Model) browseData() views.BrowseData {
	lay := m.layout()

	rows := make([]components.ListRow, len(m.filtered))
	for i, s := range m.filtered {
		rows[i] = components.ListRow{Summary: s, Selected: i == m.cursor}
	}

	query := m.queryValue()
	search := m.search
	searchW := lay.leftWidth() - 4
	if searchW < 10 {
		searchW = 10
	}
	search.SetWidth(searchW)

	bodySide := lay.SpriteSize
	if bodySide < sprite.MinFrameSize {
		bodySide = sprite.MinFrameSize
	}

	data := views.BrowseData{
		Width:         m.width,
		Height:        m.height,
		Query:         query,
		SearchView:    search.View(),
		Rows:          rows,
		ListOff:       m.listOff,
		ListRows:      m.listVisibleRows(),
		ShowSprite:    lay.ShowSprite,
		VerboseFooter: lay.VerboseFooter,
		OnePane:       lay.OnePane,
		TooSmall:      lay.TooSmall,
		ShowListPane:  query != "",
		BrowseView:    int(m.browseView),
		ViewLabels:    AllBrowseViewLabels(),
		BodySize:      bodySide,
		LeftWidth:     lay.leftWidth(),
		RightWidth:    lay.rightWidth(),
		ViewLines:     m.browseViewLines(),
		ShowHelp:      m.showHelp,
		HelpBody:      m.help.FullHelpView(m.keys.FullHelp()),
		ShowDialog:    m.showDialog,
		DialogSection: int(m.dialogSection),
		DialogLabels:  AllDialogSectionLabels(),
		DialogLines:   m.dialogContentLines(),
		DialogOff:     m.dialogVP.YOffset(),
	}

	if sel, ok := m.selected(); ok {
		data.HasSelected = true
		data.Selected = sel
		data.HeaderTypes = style.TypeSummary(sel.Types)
		data.HeaderGenus = ""
		if r := m.resources[sel.ID]; r != nil {
			if r.HasCore {
				data.HeaderTypes = style.TypeSummary(pokemonTypeNames(r.Pokemon))
				data.HeaderGenus = r.Species.Genus
			}
			if r.HasSprite && !r.SpritePlaceholder && r.SpriteANSI != "" {
				data.SpriteANSI = r.SpriteANSI
			}
		}
	}
	if m.warmActive && m.warmTotal > 0 {
		data.WarmHint = fmt.Sprintf("warming %d/%d", m.warmDone, m.warmTotal)
	}
	return data
}

func pokemonTypeNames(p domain.Pokemon) []string {
	out := make([]string, 0, len(p.Types))
	for _, t := range p.Types {
		out = append(out, t.Name)
	}
	return out
}
