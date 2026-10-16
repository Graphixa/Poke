package tui

import "charm.land/bubbles/v2/key"

// keyMap holds browse key bindings (Charm bubbles/key pattern).
// Letter keys are not used as hotkeys — they type into the always-on list filter.
type keyMap struct {
	Filters   key.Binding
	Compare   key.Binding
	Favourite key.Binding
	Up        key.Binding
	Down      key.Binding
	Left      key.Binding
	Right     key.Binding
	TabNext   key.Binding
	TabPrev   key.Binding
	PageUp    key.Binding
	PageDown  key.Binding
	Home      key.Binding
	End       key.Binding
	Clear     key.Binding
	Enter     key.Binding
	ViewCycle key.Binding
	ForceQuit key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Filters:   key.NewBinding(key.WithKeys("f2"), key.WithHelp("F2", "filter")),
		Compare:   key.NewBinding(key.WithKeys("f3"), key.WithHelp("F3", "compare")),
		Favourite: key.NewBinding(key.WithKeys("f4"), key.WithHelp("F4", "favourite")),
		Up: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("↑/↓", "move"),
		),
		Down: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("↑/↓", "move"),
		),
		Left: key.NewBinding(
			key.WithKeys("left"),
			key.WithHelp("←/→", "section"),
		),
		Right: key.NewBinding(
			key.WithKeys("right"),
			key.WithHelp("←/→", "section"),
		),
		TabNext: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "section"),
		),
		TabPrev: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("S-Tab", "prev section"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("PgUp/PgDn", "scroll"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("PgUp/PgDn", "scroll"),
		),
		Home: key.NewBinding(
			key.WithKeys("home"),
			key.WithHelp("Home/End", "jump"),
		),
		End: key.NewBinding(
			key.WithKeys("end"),
			key.WithHelp("Home/End", "jump"),
		),
		Clear: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "clear / quit"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "details"),
		),
		ViewCycle: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "view"),
		),
		ForceQuit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
	}
}

// DialogShortHelp returns footer bindings for the full-screen dossier.
// Rail: ↑/↓ move the section list. Stacked: ↑/↓ scroll; ←/→ still change
// sections but stay off the short footer.
func (k keyMap) DialogShortHelp(stacked bool) []key.Binding {
	if stacked {
		return []key.Binding{
			key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "section")),
			key.NewBinding(key.WithKeys("up"), key.WithHelp("↑/↓", "scroll")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
		}
	}
	return []key.Binding{
		key.NewBinding(key.WithKeys("up"), key.WithHelp("↑/↓", "section")),
		key.NewBinding(key.WithKeys("pgup"), key.WithHelp("PgUp/PgDn", "scroll")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
	}
}

// ShortHelp is the browse footer binding list (FontGet-style chips).
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Enter,
		k.ViewCycle,
		k.Filters,
		k.Compare,
		k.Favourite,
		k.Clear,
	}
}
