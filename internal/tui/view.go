package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/output"
	"github.com/Graphixa/Pokedex/internal/tui/components"
	"github.com/Graphixa/Pokedex/internal/tui/style"
	"github.com/Graphixa/Pokedex/internal/tui/views"
	"github.com/Graphixa/Pokedex/internal/usererr"
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
	labels := make([]string, int(tabCount))
	for i := Tab(0); i < tabCount; i++ {
		labels[i] = i.Label(lay.ShortTabs)
	}

	rows := make([]components.ListRow, len(m.filtered))
	for i, s := range m.filtered {
		rows[i] = components.ListRow{Summary: s, Selected: i == m.cursor}
	}

	query := m.queryValue()
	search := m.search
	searchW := m.width/3 - 4
	if searchW < 10 {
		searchW = 10
	}
	search.SetWidth(searchW)

	data := views.BrowseData{
		Width:          m.width,
		Height:         m.height,
		Query:          query,
		SearchView:     search.View(),
		Rows:           rows,
		ListOff:        m.listOff,
		ListRows:       m.listVisibleRows(),
		ShowSprite:     lay.ShowSprite,
		ShowSummary:    lay.ShowSummary,
		ShortTabs:      lay.ShortTabs,
		VerboseFooter:  lay.VerboseFooter,
		OnePane:        lay.OnePane,
		TooSmall:       lay.TooSmall,
		ShowListPane:   query != "",
		ActiveTab:      int(m.activeTab),
		TabLabels:      labels,
		DetailTitle:    m.activeTab.Label(false),
		DetailLines:    m.detailContentLines(),
		DetailOff:      m.detailVP.YOffset(),
		ShowHelp:       m.showHelp,
		HelpBody:       m.help.FullHelpView(m.keys.FullHelp()),
		ShowInfo:       m.showInfo,
		SummaryOff:     m.summaryVP.YOffset(),
		IdentityHeight: m.identityBandHeight(),
	}

	if sel, ok := m.selected(); ok {
		data.HasSelected = true
		data.Selected = sel
		data.Info = m.mainInfo(lay.ShowSummary)
		data.SpriteSide = m.currentSpriteSide()
		if r := m.resources[sel.ID]; r != nil && r.HasSprite && !r.SpritePlaceholder && r.SpriteANSI != "" {
			data.SpriteANSI = r.SpriteANSI
		}
		if m.showInfo {
			data.DetailLines = m.infoModalLines()
		}
	}
	return data
}

func (m Model) infoModalLines() []string {
	sel, ok := m.selected()
	if !ok {
		return []string{"No Pokémon selected."}
	}
	r := m.resources[sel.ID]
	if r == nil || !r.HasCore {
		return []string{"Loading…"}
	}
	lines := []string{
		"Type: " + style.TypeSummary(pokemonTypeNames(r.Pokemon)),
		"Species: " + dashOr(r.Species.Genus),
		"Height: " + views.FormatHeight(r.Pokemon.HeightDecimeter),
		"Weight: " + views.FormatWeight(r.Pokemon.WeightHectogram),
		"",
	}
	lines = append(lines, formatInfoLines(r.Species)...)
	if s := latestSummary(r.Species); s != "" {
		lines = append(lines, "", "Flavour:", s)
	}
	return lines
}

func dashOr(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func (m Model) mainInfo(showSummary bool) components.MainInfo {
	sel, ok := m.selected()
	if !ok {
		return components.MainInfo{}
	}
	r := m.resources[sel.ID]
	info := components.MainInfo{
		Types:       style.TypeSummary(sel.Types),
		ShowSummary: showSummary,
	}
	if r == nil {
		info.Loading = true
		return info
	}
	if r.loadingCore && !r.HasCore {
		info.Loading = true
		return info
	}
	if r.CoreErr != nil && !r.HasCore {
		info.Err = usererr.Message(r.CoreErr)
		return info
	}
	if !r.HasCore {
		info.Loading = true
		return info
	}
	p, s := r.Pokemon, r.Species
	info.Types = style.TypeSummary(pokemonTypeNames(p))
	info.Genus = s.Genus
	info.Height = views.FormatHeight(p.HeightDecimeter)
	info.Weight = views.FormatWeight(p.WeightHectogram)
	info.BaseExp = p.BaseExperience
	abilities, hidden := splitAbilities(p.Abilities)
	info.Ability = strings.Join(abilities, " / ")
	info.Hidden = hidden
	if showSummary {
		info.Summary = latestSummary(s)
	}
	return info
}

func pokemonTypeNames(p domain.Pokemon) []string {
	out := make([]string, 0, len(p.Types))
	for _, t := range p.Types {
		out = append(out, t.Name)
	}
	return out
}

func splitAbilities(abilities []domain.AbilitySlot) (normal []string, hidden string) {
	for _, a := range abilities {
		name := output.TitleFromSlug(a.Name)
		if a.IsHidden {
			hidden = name
			continue
		}
		normal = append(normal, name)
	}
	return normal, hidden
}
