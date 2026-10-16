package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Pokedex/internal/theme"
	"github.com/Graphixa/Pokedex/internal/tui/style"
)

// ThemePicker is a FontGet-inspired interactive theme selector with live preview.
type ThemePicker struct {
	options   []theme.Option
	index     int
	scroll    int
	current   string
	themesDir string
	save      func(name string) error
	width     int
	height    int
	quitting  bool
	err       string
	applied   bool
}

// ThemePickerOptions configures the theme selector.
type ThemePickerOptions struct {
	Current   string
	ThemesDir string
	Save      func(name string) error
}

// NewThemePicker builds the selection model.
func NewThemePicker(opts ThemePickerOptions) (*ThemePicker, error) {
	list, err := theme.ListOptions(opts.ThemesDir, opts.Current)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("no themes available")
	}
	idx := 0
	cur := strings.TrimSpace(opts.Current)
	if cur == "" {
		cur = "catppuccin"
	}
	for i, o := range list {
		if o.Name == cur {
			idx = i
			break
		}
	}
	if f, _, err := theme.Resolve(opts.ThemesDir, list[idx].Name); err == nil {
		theme.Apply(f)
	}
	return &ThemePicker{
		options:   list,
		index:     idx,
		current:   cur,
		themesDir: opts.ThemesDir,
		save:      opts.Save,
	}, nil
}

func (m ThemePicker) Init() tea.Cmd { return nil }

func (m ThemePicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureVisible()
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.index > 0 {
				m.index--
				m.preview()
				m.ensureVisible()
			}
		case "down", "j":
			if m.index < len(m.options)-1 {
				m.index++
				m.preview()
				m.ensureVisible()
			}
		case "enter":
			name := m.options[m.index].Name
			if m.save != nil {
				if err := m.save(name); err != nil {
					m.err = err.Error()
					return m, nil
				}
			}
			if f, _, err := theme.Resolve(m.themesDir, name); err == nil {
				theme.Apply(f)
			}
			m.applied = true
			m.current = name
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *ThemePicker) preview() {
	m.err = ""
	if f, _, err := theme.Resolve(m.themesDir, m.options[m.index].Name); err == nil {
		theme.Apply(f)
	} else {
		m.err = err.Error()
	}
}

func (m *ThemePicker) ensureVisible() {
	visible := m.listVisible()
	if m.index < m.scroll {
		m.scroll = m.index
	}
	if m.index >= m.scroll+visible {
		m.scroll = m.index - visible + 1
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
}

func (m ThemePicker) listVisible() int {
	lay := CalculatePanelLayout(m.width, m.height, 0, 1, 2, 1)
	h := lay.PanelHeight - 4
	if h < 3 {
		h = 3
	}
	return h
}

func (m ThemePicker) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}
	if m.width == 0 {
		m.width = 100
	}
	if m.height == 0 {
		m.height = 30
	}

	lay := CalculatePanelLayout(m.width, m.height, 0, 1, 2, 1)
	left := m.renderList(lay.LeftWidth-1, lay.PanelHeight-2)
	right := m.renderPreview(lay.RightWidth-1, lay.PanelHeight-2)
	frame := RenderCombinedFrame("Theme Selection", lay.LeftWidth, lay.RightWidth, lay.PanelHeight, left, right)

	help := " ↑/↓ Navigate  Enter Select  Esc Cancel"
	cur := "Current: " + m.current
	gap := m.width - lipgloss.Width(help) - lipgloss.Width(cur) - 2
	if gap < 1 {
		gap = 1
	}
	footer := help + strings.Repeat(" ", gap) + lipgloss.NewStyle().Foreground(style.PrimaryColor()).Render(cur)
	out := " " + strings.ReplaceAll(frame, "\n", "\n ") + "\n" + footer
	v := tea.NewView(FillTerminalArea(out, m.width, m.height))
	v.AltScreen = true
	return v
}

func (m ThemePicker) renderList(width, height int) string {
	if width < 12 {
		width = 12
	}
	visible := height - 2
	if visible < 1 {
		visible = 1
	}
	var lines []string
	end := m.scroll + visible
	if end > len(m.options) {
		end = len(m.options)
	}
	for i := m.scroll; i < end; i++ {
		o := m.options[i]
		prefix := "  "
		if i == m.index {
			prefix = "> "
		}
		mark := " "
		if o.Name == m.current {
			mark = "✓"
		}
		label := fmt.Sprintf("%s%s %s", prefix, mark, o.DisplayName)
		if lipgloss.Width(label) > width {
			label = lipgloss.NewStyle().MaxWidth(width).Render(label)
		}
		styleLine := lipgloss.NewStyle()
		if i == m.index {
			styleLine = styleLine.Foreground(style.PrimaryColor()).Bold(true)
		} else {
			styleLine = styleLine.Foreground(style.SecondaryColor())
		}
		lines = append(lines, styleLine.Render(label))
	}
	for len(lines) < visible {
		lines = append(lines, "")
	}
	return lipgloss.NewStyle().Padding(1, 1).Render(strings.Join(lines, "\n"))
}

func (m ThemePicker) renderPreview(width, height int) string {
	p := style.CurrentPalette()
	swatch := func(name, hex string) string {
		if hex == "" {
			return fmt.Sprintf("%-12s (terminal default)", name)
		}
		chip := lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Bold(true).Render("██")
		return fmt.Sprintf("%s %-12s %s", chip, name, hex)
	}
	sel := m.options[m.index]
	body := []string{
		lipgloss.NewStyle().Foreground(style.PrimaryColor()).Bold(true).Render(sel.DisplayName),
		"Source: " + sel.Source,
		"",
		swatch("primary", p.Primary),
		swatch("secondary", p.Secondary),
		swatch("components", p.Components),
		swatch("placeholders", p.Placeholders),
		swatch("base", p.Base),
		swatch("warning", p.Warning),
		swatch("error", p.Error),
		swatch("success", p.Success),
		"",
		lipgloss.NewStyle().Foreground(style.BorderColor()).Render(strings.Repeat("─", themePickerMax(8, width-4))),
		"",
		"Sample chrome:",
		lipgloss.NewStyle().Foreground(style.PrimaryColor()).Bold(true).Render("Pokedex — Browse Mode"),
		"[Stats] " + lipgloss.NewStyle().Foreground(style.PrimaryColor()).Underline(true).Render("[Evolution]") + " [Moves]",
	}
	if m.err != "" {
		body = append(body, "", style.StatusError(m.err))
	}
	return lipgloss.NewStyle().Padding(1, 1).Width(width).Height(height).Render(strings.Join(body, "\n"))
}

func themePickerMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// RunThemePicker launches the theme selector program.
func RunThemePicker(opts ThemePickerOptions) error {
	m, err := NewThemePicker(opts)
	if err != nil {
		return err
	}
	p := tea.NewProgram(m)
	_, err = p.Run()
	return err
}
