package bootstrap

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Pokedex/internal/tui/style"
)

// Stage is a bootstrap progress-screen phase.
type Stage string

const (
	StageIndex     Stage = "index"
	StageEnrich    Stage = "enrich"
	StageRecords   Stage = "records"
	StageEvolution Stage = "evolution"
	StageMoves     Stage = "moves"
	StageDataset   Stage = "dataset"
	StageArtwork   Stage = "artwork"
)

// Progress is the bootstrap TUI's view of app-classified update work.
type Progress struct {
	Stage   Stage
	Done    int
	Total   int
	Message string
}

// ProgressFunc receives bootstrap progress. App adapts cache.DatasetProgress into this.
type ProgressFunc func(Progress)

// Deps are injected bootstrap capabilities (app layer wires the cache service).
type Deps struct {
	Run func(ctx context.Context, progress ProgressFunc) error
}

// Result is returned after the progress screen exits.
type Result struct {
	Aborted bool
}

// Model is the fullscreen dataset bootstrap progress screen.
type Model struct {
	deps   Deps
	result Result

	width  int
	height int

	prog progress.Model

	stage   Stage
	status  string
	errMsg  string
	done    int
	total   int
	percent float64

	events <-chan tea.Msg
	cancel context.CancelFunc

	quitting bool
}

type progressMsg struct {
	stage       Stage
	done, total int
	message     string
}
type doneMsg struct {
	err error
}
type bootstrapStartMsg struct{}

// New builds the bootstrap progress model.
func New(deps Deps) Model {
	pal := style.CurrentPalette()
	p := progress.New(
		progress.WithDefaultBlend(),
		progress.WithScaled(true),
		progress.WithColors(lipgloss.Color(pal.Primary), lipgloss.Color(pal.Secondary)),
	)
	p.SetWidth(48)
	p.ShowPercentage = true
	return Model{
		deps:   deps,
		prog:   p,
		status: "Starting…",
	}
}

// Outcome returns the result (valid after Run / quit).
func (m Model) Outcome() Result { return m.result }

func (m Model) Init() tea.Cmd {
	return func() tea.Msg { return bootstrapStartMsg{} }
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case bootstrapStartMsg:
		return m.attachBootstrap()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		w := msg.Width - 8
		if w < 20 {
			w = 20
		}
		if w > 64 {
			w = 64
		}
		m.prog.SetWidth(w)
		return m, nil

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			m.stopWork()
			m.quitting = true
			m.result.Aborted = true
			return m, tea.Quit
		}

	case progressMsg:
		m.stage = msg.stage
		m.done = msg.done
		m.total = msg.total
		if msg.message != "" {
			m.status = msg.message
		}
		if msg.total > 0 {
			m.percent = float64(msg.done) / float64(msg.total)
		} else {
			m.percent = 0
		}
		return m, tea.Batch(m.prog.SetPercent(m.percent), listenEvents(m.events))

	case doneMsg:
		m.stopWork()
		if msg.err != nil && msg.err != context.Canceled {
			m.errMsg = msg.err.Error()
			m.status = "Bootstrap failed"
			return m, nil
		}
		m.quitting = true
		return m, tea.Quit

	case progress.FrameMsg:
		var cmd tea.Cmd
		m.prog, cmd = m.prog.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) View() tea.View {
	if m.quitting && m.errMsg == "" {
		return tea.NewView("")
	}
	pal := style.CurrentPalette()
	brand := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(pal.Primary)).Render("poke")
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(pal.Components)).Render("Pokedex")
	sub := lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Placeholders))
	body := lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Components))
	errSt := lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Error))

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", brand)
	fmt.Fprintf(&b, "%s\n\n", title)
	fmt.Fprintf(&b, "%s\n\n", stageRail(m.stage, pal))
	fmt.Fprintf(&b, "%s\n", m.prog.View())
	if m.total > 0 {
		fmt.Fprintf(&b, "%s\n", sub.Render(fmt.Sprintf("%d / %d", m.done, m.total)))
	}
	if m.status != "" {
		fmt.Fprintf(&b, "%s\n", body.Render(m.status))
	}
	if m.errMsg != "" {
		fmt.Fprintf(&b, "\n%s\n", errSt.Render(m.errMsg))
	}
	fmt.Fprintf(&b, "\n%s", sub.Render("Ctrl+C cancel"))

	w, h := m.width, m.height
	if w < 1 {
		w = 80
	}
	if h < 1 {
		h = 24
	}
	v := tea.NewView(lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, b.String()))
	v.AltScreen = true
	return v
}

// bootstrapRail is the ordered first-run progress stages (short labels).
var bootstrapRail = []struct {
	id    Stage
	label string
}{
	{StageIndex, "index"},
	{StageEnrich, "types"},
	{StageRecords, "records"},
	{StageEvolution, "evo"},
	{StageMoves, "moves"},
	{StageDataset, "ready"},
}

func stageRail(current Stage, pal style.Palette) string {
	idx := stageIndex(current)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Placeholders))
	done := lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Secondary))
	active := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(pal.Primary))
	sep := dim.Render(" · ")

	var b strings.Builder
	for i, s := range bootstrapRail {
		if i > 0 {
			b.WriteString(sep)
		}
		switch {
		case i < idx:
			b.WriteString(done.Render(s.label))
		case i == idx:
			b.WriteString(active.Render(s.label))
		default:
			b.WriteString(dim.Render(s.label))
		}
	}
	if current == StageArtwork {
		b.WriteString(sep)
		b.WriteString(active.Render("art"))
	}
	return b.String()
}

func stageIndex(stage Stage) int {
	if stage == StageArtwork {
		return len(bootstrapRail) // past ready; art appended separately
	}
	for i, s := range bootstrapRail {
		if s.id == stage {
			return i
		}
	}
	return -1
}

func stageLabel(stage Stage) string {
	switch stage {
	case StageIndex:
		return "index"
	case StageEnrich:
		return "types"
	case StageRecords:
		return "records"
	case StageEvolution:
		return "evo"
	case StageMoves:
		return "moves"
	case StageDataset:
		return "ready"
	case StageArtwork:
		return "art"
	default:
		if stage == "" {
			return "Working"
		}
		return string(stage)
	}
}

func (m Model) attachBootstrap() (tea.Model, tea.Cmd) {
	m.errMsg = ""
	m.done, m.total, m.percent = 0, 0, 0
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	ch := make(chan tea.Msg, 16)
	m.events = ch
	deps := m.deps
	go func() {
		err := deps.Run(ctx, func(p Progress) {
			select {
			case ch <- progressMsg{stage: p.Stage, done: p.Done, total: p.Total, message: p.Message}:
			default:
			}
		})
		ch <- doneMsg{err: err}
		close(ch)
	}()
	return m, tea.Batch(m.prog.SetPercent(0), listenEvents(ch))
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

// Run launches the bootstrap progress screen and returns the outcome.
func Run(ctx context.Context, deps Deps) (Result, error) {
	m := New(deps)
	p := tea.NewProgram(m, tea.WithContext(ctx))
	final, err := p.Run()
	if err != nil {
		return Result{Aborted: true}, err
	}
	out, ok := final.(Model)
	if !ok {
		return Result{Aborted: true}, fmt.Errorf("bootstrap: unexpected model type")
	}
	return out.Outcome(), nil
}
