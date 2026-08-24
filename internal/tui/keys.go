package tui

import "charm.land/bubbles/v2/key"

// keyMap holds browse key bindings (Charm bubbles/key pattern).
type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	TabNext  key.Binding
	TabPrev  key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Home     key.Binding
	End      key.Binding
	Clear    key.Binding
	Help     key.Binding
	Info     key.Binding
	Quit     key.Binding
	ForceQuit key.Binding
	// Unbound keys that must not reach the search field.
	Unbound key.Binding
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
			key.WithHelp("tab", "tabs"),
		),
		TabPrev: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("S-Tab", "prev tab"),
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
			key.WithHelp("Home/End", "jump detail"),
		),
		End: key.NewBinding(
			key.WithKeys("end"),
			key.WithHelp("Home/End", "jump detail"),
		),
		Clear: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "clear / close"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Info: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "info"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
		ForceQuit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
		Unbound: key.NewBinding(
			key.WithKeys("enter", "r", "/"),
		),
	}
}

// ShortHelp implements help.KeyMap for the footer strip.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("type"), key.WithHelp("type", "filter")),
		k.Up,
		k.TabNext,
		k.Info,
		k.PageDown,
		k.Clear,
		k.Quit,
	}
}

// FullHelp implements help.KeyMap for the ? overlay.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			key.NewBinding(key.WithKeys("type"), key.WithHelp("type", "filter list (incl. q/i)")),
			key.NewBinding(key.WithKeys("backspace"), key.WithHelp("backspace", "delete from search")),
			k.Clear,
			k.Up,
			k.Down,
		},
		{
			k.TabNext,
			k.TabPrev,
			k.PageUp,
			k.PageDown,
			k.Home,
			k.End,
		},
		{
			k.Info,
			k.Quit,
			k.ForceQuit,
			k.Help,
		},
	}
}
