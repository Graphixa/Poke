// Package setup is the first-run flow: a theme, an update check, and a
// preloader if the Pokédex or pictures are still downloading.
package setup

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Poke/internal/config"
	"github.com/Graphixa/Poke/internal/theme"
	"github.com/Graphixa/Poke/internal/tui/components"
	"github.com/Graphixa/Poke/internal/tui/preload"
	"github.com/Graphixa/Poke/internal/tui/style"
)

const lineEvery = preload.LineEvery

const (
	stepTheme = iota
	stepUpdates
	stepWait
)

// Progress is a download tick (dataset or pictures).
type Progress struct {
	Done    int
	Total   int
	Message string
}

// ProgressFunc receives download progress.
type ProgressFunc func(Progress)

// Deps are injected by the app layer.
type Deps struct {
	Config    config.Config
	ThemesDir string
	// Ask shows the theme and update questions. False is the cache-clear preloader.
	Ask bool
	// RunWork downloads whatever is missing. Nil when nothing is missing.
	RunWork    func(ctx context.Context, progress ProgressFunc) error
	SaveConfig func(cfg config.Config) error
}

// Result is returned after setup exits.
type Result struct {
	Config                 config.Config
	StartBackgroundArtwork bool
	Aborted                bool
}

// Model is the setup program.
type Model struct {
	deps   Deps
	result Result

	step   int
	width  int
	height int

	themes   []theme.Option
	themeIdx int
	yes      bool

	prog     progress.Model
	barWidth int
	lines    []string
	lineIdx  int
	done     int
	total    int
	percent  float64
	detail   string
	errMsg   string
	workDone bool

	events <-chan tea.Msg
	cancel context.CancelFunc

	quitting bool
}

type startMsg struct{}
type lineTick struct{}
type progressMsg struct {
	done, total int
	message     string
}
type workDoneMsg struct{ err error }

// New builds the setup model. Call Init via the tea program.
func New(deps Deps) Model {
	m := Model{
		deps:     deps,
		result:   Result{Config: deps.Config},
		barWidth: 48,
		lines:    preload.Shuffle(),
		yes:      deps.Config.Update.CheckForUpdates,
		workDone: deps.RunWork == nil,
	}
	if !deps.Ask {
		m.step = stepWait
	}
	m.themes, _ = theme.ListOptions(deps.ThemesDir, deps.Config.Theme.Name)
	if deps.Ask && len(m.themes) == 0 {
		m.step = stepUpdates
	}
	for i, o := range m.themes {
		if o.Name == themeName(deps.Config.Theme.Name) {
			m.themeIdx = i
			break
		}
	}
	m.applyTheme()
	return m
}

func (m *Model) paintBar() {
	pal := style.CurrentPalette()
	p := progress.New(
		progress.WithDefaultBlend(),
		progress.WithScaled(true),
		progress.WithColors(lipgloss.Color(pal.Primary), lipgloss.Color(pal.Secondary)),
	)
	w := m.barWidth
	if w < 20 {
		w = 20
	}
	p.SetWidth(w)
	p.ShowPercentage = true
	m.prog = p
}

func themeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "dracula"
	}
	return name
}

// Outcome returns the result after Run / quit.
func (m Model) Outcome() Result { return m.result }

func (m Model) Init() tea.Cmd {
	return func() tea.Msg { return startMsg{} }
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case startMsg:
		if m.quitting {
			return m, nil
		}
		var cmds []tea.Cmd
		if m.deps.RunWork != nil && !m.workDone {
			var cmd tea.Cmd
			m, cmd = m.startWork()
			cmds = append(cmds, cmd)
		}
		if m.step == stepWait {
			cmds = append(cmds, tickLine())
		}
		return m, tea.Batch(cmds...)

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		w := msg.Width - 8
		if w < 20 {
			w = 20
		}
		if w > 64 {
			w = 64
		}
		m.barWidth = w
		m.prog.SetWidth(w)
		return m, nil

	case tea.KeyPressMsg:
		key := msg.String()
		if key == "ctrl+c" || key == "esc" {
			return m.abort()
		}
		switch m.step {
		case stepTheme:
			switch key {
			case "up":
				if m.themeIdx > 0 {
					m.themeIdx--
					return m, m.applyTheme()
				}
			case "down":
				if m.themeIdx < len(m.themes)-1 {
					m.themeIdx++
					return m, m.applyTheme()
				}
			case "enter":
				m.step = stepUpdates
			}
		case stepUpdates:
			switch key {
			case "up", "down", "left", "right":
				m.yes = !m.yes
			case "y", "Y":
				m.yes = true
			case "n", "N":
				m.yes = false
			case "enter":
				if m.errMsg != "" && !m.workDone {
					return m, nil
				}
				if m.workDone {
					return m.finish()
				}
				m.step = stepWait
				return m, tickLine()
			}
		}

	case lineTick:
		if len(m.lines) > 0 {
			m.lineIdx = (m.lineIdx + 1) % len(m.lines)
		}
		if m.quitting || m.step != stepWait {
			return m, nil
		}
		return m, tickLine()

	case progressMsg:
		m.done, m.total = msg.done, msg.total
		m.detail = msg.message
		if msg.total > 0 {
			m.percent = float64(msg.done) / float64(msg.total)
		}
		return m, tea.Batch(m.prog.SetPercent(m.percent), listenEvents(m.events))

	case workDoneMsg:
		m.stopWork()
		if msg.err != nil && msg.err != context.Canceled {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		if m.quitting {
			return m, nil
		}
		m.workDone = true
		m.errMsg = ""
		if m.step == stepWait {
			return m.finish()
		}
		return m, nil

	case progress.FrameMsg:
		var cmd tea.Cmd
		m.prog, cmd = m.prog.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}
	w, h := m.width, m.height
	var body string
	switch m.step {
	case stepWait:
		body = preload.Screen(w, h, m.prog.View(), m.headline(), preload.Count(m.done, m.total), m.detail, m.errMsg)
	default:
		body = m.questionView(w, h)
	}
	v := tea.NewView(body)
	v.AltScreen = true
	return v
}

func (m Model) questionView(width, height int) string {
	pal := style.CurrentPalette()
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(pal.Primary)).Render("Poke")
	head := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(pal.Components))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Placeholders))
	sel := lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Primary)).Bold(true)
	errSt := lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Error))

	var b strings.Builder
	if m.step == stepTheme {
		fmt.Fprintf(&b, "%s\n\n%s\n\n", title, head.Render("Pick a theme"))
		b.WriteString(m.themeLines(sel, muted))
	} else {
		active := 0
		if m.yes {
			active = 1
		}
		b.WriteString(lipgloss.JoinVertical(lipgloss.Center,
			title,
			"",
			head.Render("Check for updates when you open Poke?"),
			"",
			components.ViewSwitcher(active, []string{"No", "Yes"}, 0),
		))
	}
	if m.errMsg != "" {
		fmt.Fprintf(&b, "\n%s\n", errSt.Render(m.errMsg))
	}
	if width < 1 {
		width = 80
	}
	if height < 1 {
		height = 24
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, b.String())
}

func (m Model) themeLines(sel, muted lipgloss.Style) string {
	const window = 9
	start := m.themeIdx - window/2
	if start < 0 {
		start = 0
	}
	end := start + window
	if end > len(m.themes) {
		end = len(m.themes)
		start = max(0, end-window)
	}
	var b strings.Builder
	for i := start; i < end; i++ {
		label := m.themes[i].DisplayName
		if label == "" {
			label = m.themes[i].Name
		}
		line := "  " + label
		if i == m.themeIdx {
			line = sel.Render("> " + label)
		} else {
			line = muted.Render(line)
		}
		fmt.Fprintf(&b, "%s\n", line)
	}
	return b.String()
}

func (m Model) headline() string {
	if len(m.lines) == 0 {
		return "Loading"
	}
	return m.lines[m.lineIdx%len(m.lines)]
}

func (m *Model) applyTheme() tea.Cmd {
	if m.themeIdx >= 0 && m.themeIdx < len(m.themes) {
		if f, _, err := theme.Resolve(m.deps.ThemesDir, m.themes[m.themeIdx].Name); err == nil {
			theme.Apply(f)
		}
	}
	m.paintBar()
	return m.prog.SetPercent(m.percent)
}

func (m Model) chosenTheme() string {
	if m.themeIdx >= 0 && m.themeIdx < len(m.themes) {
		return m.themes[m.themeIdx].Name
	}
	return themeName(m.deps.Config.Theme.Name)
}

func (m Model) abort() (tea.Model, tea.Cmd) {
	m.stopWork()
	m.quitting = true
	m.result.Aborted = true
	return m, tea.Quit
}

func (m Model) finish() (tea.Model, tea.Cmd) {
	cfg := m.deps.Config
	if m.deps.Ask {
		cfg.Theme.Name = m.chosenTheme()
		cfg.Update.CheckForUpdates = m.yes
		cfg.MarkSetupComplete()
	}
	cfg.ApplyArtworkConsent(config.ArtworkConsentAccepted, config.ArtworkModeBackground)
	m.result.Config = cfg
	m.result.StartBackgroundArtwork = true
	if m.deps.SaveConfig != nil {
		if err := m.deps.SaveConfig(cfg); err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
	}
	m.quitting = true
	return m, tea.Quit
}

func (m Model) startWork() (Model, tea.Cmd) {
	m.errMsg = ""
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	ch := make(chan tea.Msg, 16)
	m.events = ch
	run := m.deps.RunWork
	go func() {
		err := run(ctx, func(p Progress) {
			select {
			case ch <- progressMsg{done: p.Done, total: p.Total, message: p.Message}:
			default:
			}
		})
		ch <- workDoneMsg{err: err}
		close(ch)
	}()
	return m, listenEvents(ch)
}

func tickLine() tea.Cmd {
	return tea.Tick(lineEvery, func(time.Time) tea.Msg { return lineTick{} })
}

func listenEvents(ch <-chan tea.Msg) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

func (m *Model) stopWork() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
}

// Run launches setup and returns the outcome.
func Run(ctx context.Context, deps Deps) (Result, error) {
	m := New(deps)
	p := tea.NewProgram(m, tea.WithContext(ctx))
	final, err := p.Run()
	if err != nil {
		return Result{Aborted: true}, err
	}
	out, ok := final.(Model)
	if !ok {
		return Result{Aborted: true}, fmt.Errorf("setup: unexpected model type")
	}
	return out.Outcome(), nil
}
