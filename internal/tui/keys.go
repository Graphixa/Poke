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
	Game      key.Binding
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
			key.WithHelp("tab", "focus"),
		),
		TabPrev: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("S-Tab", "focus"),
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
			key.WithHelp("esc", "quit"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "info"),
		),
		ViewCycle: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "view"),
		),
		ForceQuit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
		Game: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "game"),
		),
	}
}

// DialogHelpOpts configures dossier footer chips for the current focus.
type DialogHelpOpts struct {
	ContentFocus bool
	Moves        bool
	MoveDetail   bool
	Picker       bool
}

// DialogShortHelp returns footer bindings for the full-screen dossier.
// G/M are top-border chips (like F4), not footer.
func (k keyMap) DialogShortHelp(o DialogHelpOpts) []key.Binding {
	if o.Picker {
		return []key.Binding{
			key.NewBinding(key.WithKeys("up"), key.WithHelp("↑/↓", "pick")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		}
	}
	if o.MoveDetail {
		return []key.Binding{
			key.NewBinding(key.WithKeys("up"), key.WithHelp("↑/↓", "move")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		}
	}
	out := []key.Binding{
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "focus")),
	}
	if !o.ContentFocus {
		out = append(out,
			key.NewBinding(key.WithKeys("up"), key.WithHelp("↑/↓", "section")),
			k.PageUp,
		)
	} else if o.Moves {
		out = append(out,
			key.NewBinding(key.WithKeys("up"), key.WithHelp("↑/↓", "move")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "detail")),
			k.PageUp,
		)
	} else {
		out = append(out,
			key.NewBinding(key.WithKeys("up"), key.WithHelp("↑/↓", "scroll")),
			k.PageUp,
		)
	}
	out = append(out, key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")))
	return out
}

// ShortHelp is the browse footer binding list (FontGet-style chips).
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Up,
		k.Enter,
		k.ViewCycle,
		k.Filters,
		k.Compare,
		k.Clear,
	}
}

// escHelpDesc is "clear" when esc will wipe search/filters, else "quit".
func (m Model) escHelpDesc() string {
	if m.queryValue() != "" || m.viewOptions != (viewOptions{}) {
		return "clear"
	}
	return "quit"
}

// browseFooterHelp is ShortHelp with a live esc label.
func (m Model) browseFooterHelp() []key.Binding {
	k := m.keys
	esc := key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", m.escHelpDesc()))
	return []key.Binding{k.Up, k.Enter, k.ViewCycle, k.Filters, k.Compare, esc}
}
