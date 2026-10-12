package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
	"github.com/Graphixa/Pokedex/internal/output"
	"github.com/Graphixa/Pokedex/internal/textwidth"
	"github.com/Graphixa/Pokedex/internal/tui/components"
	"github.com/Graphixa/Pokedex/internal/tui/style"
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
	if m.comparePin == nil {
		return m.featureFrame("Compare", []string{"Choose a Pokémon with F3 to begin."}, [][]key.Binding{{featureBinding("esc", "esc", "back")}})
	}
	a := m.comparisonEntry(m.comparePin.Summary)
	if !a.HasStats {
		a = *m.comparePin
	}
	s, _ := m.selected()
	b := m.comparisonEntry(s)
	// At small widths keep numbers and differences; wider terminals add bars.
	inner := m.width - 4
	col := (inner - 18) / 2
	row := func(label, left, right, delta string) string {
		return textwidth.PadRight(label, 9) + textwidth.PadRight(textwidth.Truncate(left, col), col) + "  " +
			textwidth.PadRight(textwidth.Truncate(right, col), col) + " " + textwidth.PadRight(delta, 6)
	}
	lines := []string{row("", style.StatusMuted("PINNED"), featureAccent("SELECTED"), "Δ"),
		row("", featureAccent(a.Summary.DisplayName), featureAccent(b.Summary.DisplayName), ""),
		row("", output.FormatDex(a.Summary.ID), output.FormatDex(b.Summary.ID), ""),
		row("", style.TypeSummary(a.Summary.Types), style.TypeSummary(b.Summary.Types), ""), ""}
	av := []int{a.Stats.HP, a.Stats.Attack, a.Stats.Defense, a.Stats.SpecialAttack, a.Stats.SpecialDefense, a.Stats.Speed, a.Stats.Total()}
	bv := []int{b.Stats.HP, b.Stats.Attack, b.Stats.Defense, b.Stats.SpecialAttack, b.Stats.SpecialDefense, b.Stats.Speed, b.Stats.Total()}
	for i, label := range []string{"HP", "Attack", "Defence", "Sp. Atk", "Sp. Def", "Speed", "Total"} {
		left, right, delta := "—", "—", "—"
		stat := func(v int, higher bool) string {
			text := fmt.Sprintf("%3d", v)
			if i < 6 && col >= 16 {
				width := min(20, col-5)
				filled := min(width, max(0, v*width/255))
				text += " " + strings.Repeat("█", filled) + style.StatusMuted(strings.Repeat("░", width-filled))
			}
			if higher {
				return featureAccent(text)
			}
			return text
		}
		if a.HasStats {
			left = stat(av[i], b.HasStats && av[i] > bv[i])
		}
		if b.HasStats {
			right = stat(bv[i], a.HasStats && bv[i] > av[i])
		}
		if a.HasStats && b.HasStats {
			delta = fmt.Sprintf("%+d", bv[i]-av[i])
			if av[i] == bv[i] {
				delta = "="
			}
		}
		lines = append(lines, row(label, left, right, delta))
	}
	if m.height >= 20 {
		lines = append(lines, "", style.StatusMuted("Δ selected − pinned · — unavailable"))
	}
	return m.featureFrame("Compare · base stats", lines, [][]key.Binding{
		{featureBinding("up", "↑/↓", "opponent"), featureBinding("backspace", "⌫", "unpin"), featureBinding("esc", "esc", "back")},
	})
}
