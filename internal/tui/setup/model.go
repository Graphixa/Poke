// Package setup is the first-run flow: a theme, an update check, a short note,
// then a preloader if the Pokédex or pictures are still downloading.
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
	stepIntro
	stepWait
)

// Progress is one download tick, already summed across every part.
// Stage picks the grey line. The screen does not show Done and Total.
type Progress struct {
	Stage string
	Done  int
	Total int
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
	title  string
	width  int
	height int

	themes   []theme.Option
	themeIdx int
	yes      bool

	prog     progress.Model
	barWidth int
	lines    []string
	lineIdx  int
	spin     int
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
	stage       string
	done, total int
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
		title:    refreshTitle,
	}
	if deps.Ask {
		m.title = initialTitle
	} else {
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
			cmds = append(cmds, waitTicks())
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
				m.step = stepIntro
			}
		case stepIntro:
			if key == "enter" {
				if m.errMsg != "" && !m.workDone {
					return m, nil
				}
				if m.workDone {
					return m.finish()
				}
				m.step = stepWait
				return m, waitTicks()
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

	case spinTick:
		if n := len(preload.SpinDots); n > 0 {
			m.spin = (m.spin + 1) % n
		}
		if m.quitting || m.step != stepWait {
			return m, nil
		}
		return m, tickSpin()

	case progressMsg:
		m.done, m.total = msg.done, msg.total
		m.detail = StageLabel(msg.stage)
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
	if w < 1 {
		w = 80
	}
	if h < 1 {
		h = 24
	}
	var body string
	switch m.step {
	case stepWait:
		body = m.loadView(w, h)
	case stepIntro:
		body = m.introView(w, h)
	default:
		body = m.questionView(w, h)
	}
	v := tea.NewView(body)
	v.AltScreen = true
	return v
}

func (m Model) questionView(width, height int) string {
	pal := style.CurrentPalette()
	head := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(pal.Components))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Placeholders))
	sel := lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Primary)).Bold(true)
	errSt := lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Error))

	var body []string
	if m.step == stepTheme {
		body = append(body, wrapText(head, "Pick a theme", width)...)
		body = append(body, "")
		for _, line := range strings.Split(strings.TrimRight(m.themeLines(sel, muted), "\n"), "\n") {
			body = append(body, centerLine(line, width))
		}
	} else {
		active := 1
		if m.yes {
			active = 0
		}
		body = append(body, wrapText(head, "Check for updates when you open Poke?", width)...)
		body = append(body, "")
		body = append(body, centerLine(components.ViewSwitcher(active, []string{"Yes", "No"}, 0), width))
	}
	if m.errMsg != "" {
		body = append(body, "")
		body = append(body, wrapText(errSt, m.errMsg, width)...)
	}
	return setupFrame(m.title, width, height, body)
}

func (m Model) introView(width, height int) string {
	body := wrapText(lipgloss.NewStyle(), setupIntro, width)
	body = append(body, "", centerLine(components.ViewSwitcher(0, []string{"OK"}, 0), width))
	if m.errMsg != "" {
		errSt := lipgloss.NewStyle().Foreground(lipgloss.Color(style.CurrentPalette().Error))
		body = append(body, "")
		body = append(body, wrapText(errSt, m.errMsg, width)...)
	}
	return setupFrame(m.title, width, height, body)
}

func (m Model) loadView(width, height int) string {
	pal := style.CurrentPalette()
	head := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(pal.Components))
	bodySt := lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Placeholders))
	errSt := lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Error))

	var body []string
	for _, line := range m.spinLines() {
		body = append(body, centerLine(line, width))
	}
	body = append(body,
		"",
		centerLine(head.Render(m.headline()), width),
		"",
		centerLine(m.prog.View(), width),
	)
	if m.detail != "" {
		body = append(body, centerLine(bodySt.Render(m.detail), width))
	}
	if m.errMsg != "" {
		body = append(body, "")
		body = append(body, wrapText(errSt, m.errMsg, width)...)
	}
	return setupFrame(m.title, width, height, body)
}

const initialTitle = "Poke - Initial Setup"
const refreshTitle = "Refreshing Pokédex"

const setupIntro = "Poke will import the Pokémon data and download the artwork.\nThis only happens once, and it won't take long."

var stageLabels = map[string]string{
	"index":     "Fetching the Pokémon species list",
	"records":   "Gathering Pokémon records",
	"evolution": "Getting evolution information",
	"moves":     "Learning Pokémon movesets",
	"dataset":   "Reading the game list",
	"ability":   "Studying Pokémon abilities",
	"form":      "Investigating Pokémon forms",
	"import":    "Importing Pokémon data",
	"artwork":   "Downloading Pokémon artwork",
}

// StageLabel is the one setup loading line for a download stage.
func StageLabel(stage string) string { return stageLabels[stage] }

func setupFrame(title string, width, height int, body []string) string {
	innerH := height - 2
	if innerH < 1 {
		innerH = 1
	}
	pad := 0
	if innerH > len(body) {
		pad = (innerH - len(body)) / 2
	}
	lines := make([]string, 0, innerH)
	for i := 0; i < pad; i++ {
		lines = append(lines, "")
	}
	lines = append(lines, body...)
	return components.InnerCard(title, width, height, lines)
}

func wrapText(st lipgloss.Style, text string, width int) []string {
	return strings.Split(st.Width(innerWidth(width)).Align(lipgloss.Center).Render(text), "\n")
}

func centerLine(line string, width int) string {
	return lipgloss.PlaceHorizontal(innerWidth(width), lipgloss.Center, line)
}

func innerWidth(width int) int {
	w := width - 2
	if w < 1 {
		return 1
	}
	return w
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
			case ch <- progressMsg{stage: p.Stage, done: p.Done, total: p.Total}:
			default:
			}
		})
		ch <- workDoneMsg{err: err}
		close(ch)
	}()
	return m, listenEvents(ch)
}

type spinTick struct{}

func tickLine() tea.Cmd {
	return tea.Tick(lineEvery, func(time.Time) tea.Msg { return lineTick{} })
}

func tickSpin() tea.Cmd {
	every := preload.SpinEvery
	if every <= 0 {
		every = time.Millisecond
	}
	return tea.Tick(every, func(time.Time) tea.Msg { return spinTick{} })
}

func waitTicks() tea.Cmd {
	return tea.Batch(tickLine(), tickSpin())
}

func (m Model) spinLines() []string {
	dots := preload.SpinDots
	if len(dots) == 0 {
		return nil
	}
	w, h := 0, 0
	for _, d := range dots {
		if d[0]+1 > w {
			w = d[0] + 1
		}
		if d[1]+1 > h {
			h = d[1] + 1
		}
	}
	grid := make([][]rune, h)
	for r := range grid {
		grid[r] = []rune(strings.Repeat(" ", w))
	}
	for _, d := range dots {
		grid[d[1]][d[0]] = '·'
	}
	hot := dots[m.spin%len(dots)]
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color(style.CurrentPalette().Placeholders))
	out := make([]string, h)
	for r := range grid {
		var b strings.Builder
		for c, ch := range grid[r] {
			switch {
			case r == hot[1] && c == hot[0]:
				b.WriteString("●")
			case ch != ' ':
				b.WriteString(dim.Render(string(ch)))
			default:
				b.WriteRune(' ')
			}
		}
		out[r] = b.String()
	}
	return out
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
