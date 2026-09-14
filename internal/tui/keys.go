package tui

import "charm.land/bubbles/v2/key"

// keyMap holds browse key bindings (Charm bubbles/key pattern).
// Letter keys are not used as hotkeys — they type into the always-on list filter.
type keyMap struct {
	Up         key.Binding
	Down       key.Binding
	TabNext    key.Binding
	TabPrev    key.Binding
	PageUp     key.Binding
	PageDown   key.Binding
	Home       key.Binding
	End        key.Binding
	Clear      key.Binding
	Help       key.Binding
	Enter      key.Binding
	ViewGlance key.Binding
	ViewArt    key.Binding
	Quit       key.Binding
	ForceQuit  key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("↑/↓", "move"),
		),
		Down: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("↑/↓", "move"),
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
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "details"),
		),
		ViewGlance: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "glance"),
		),
		ViewArt: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "art"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
		ForceQuit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
	}
}

// DialogShortHelp implements the dossier overlay footer (bubbles/help).
func (k keyMap) DialogShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "section")),
		key.NewBinding(key.WithKeys("up"), key.WithHelp("↑/↓", "scroll")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
	}
}

// ShortHelp implements help.KeyMap for the footer strip.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("type"), key.WithHelp("type", "filter")),
		k.Up,
		k.Enter,
		k.ViewGlance,
		k.ViewArt,
		k.Clear,
		k.Quit,
	}
}

// FullHelp implements help.KeyMap for the ? overlay.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			key.NewBinding(key.WithKeys("type"), key.WithHelp("type", "filter list (incl. letters)")),
			key.NewBinding(key.WithKeys("backspace"), key.WithHelp("backspace", "delete from search")),
			k.Clear,
			k.Up,
			k.Down,
		},
		{
			k.ViewGlance,
			k.ViewArt,
			k.Enter,
			k.PageUp,
			k.PageDown,
		},
		{
			k.TabNext,
			k.TabPrev,
			k.Quit,
			k.ForceQuit,
			k.Help,
		},
	}
}
