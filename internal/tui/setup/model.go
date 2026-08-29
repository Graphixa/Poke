// Package setup implements the first-run Bubble Tea wizard: index fetch and
// optional official-artwork warm (artwork is the only product sprite source).
package setup

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite/assetwarm"
	"github.com/Graphixa/Pokedex/internal/tui/style"
)

// Step is the wizard phase.
type Step int

const (
	StepWelcome Step = iota
	StepIndex
	StepArtPrompt
	StepModePrompt
	StepArtWarm
	StepDone
)

// Deps are injected setup capabilities (app layer wires real services).
type Deps struct {
	Config     config.Config
	ConfigPath string
	Native     bool
	Protocol   string // kitty|iterm|sixel|chafa

	EnsureIndex func(ctx context.Context, progress cache.ProgressFunc) ([]domain.SpeciesSummary, error)
	EstimateHD  func(ctx context.Context, index []domain.SpeciesSummary) int64
	WarmArtwork func(ctx context.Context, index []domain.SpeciesSummary, progress cache.ProgressFunc) error
	SaveConfig  func(cfg config.Config) error
}

// Result is returned after the wizard exits successfully.
type Result struct {
	Config                 config.Config
	StartBackgroundArtwork bool
	Aborted                bool
}

// Model is the fullscreen setup wizard.
type Model struct {
	deps   Deps
	result Result

	step   Step
	width  int
	height int

	prog progress.Model

	status   string
	errMsg   string
	done     int
	total    int
	percent  float64
	estimate int64
	index    []domain.SpeciesSummary

	events <-chan tea.Msg
	cancel context.CancelFunc

	quitting bool
}

type indexDoneMsg struct {
	index []domain.SpeciesSummary
	err   error
}
type warmDoneMsg struct {
	err error
}
type estimateDoneMsg struct {
	bytes int64
}
type progressMsg struct {
	done, total int
	message     string
}

// New builds the wizard model.
func New(deps Deps) Model {
	pal := style.CurrentPalette()
	p := progress.New(
		progress.WithDefaultBlend(),
		progress.WithScaled(true),
		progress.WithColors(lipgloss.Color(pal.Primary), lipgloss.Color(pal.Secondary)),
	)
	p.SetWidth(48)
	p.ShowPercentage = true
	cfg := deps.Config
	return Model{
		deps:   deps,
		result: Result{Config: cfg},
		step:   StepWelcome,
		prog:   p,
		status: "Welcome to poke",
	}
}

// Outcome returns the wizard result (valid after Run / quit).
func (m Model) Outcome() Result { return m.result }

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
		key := msg.String()
		if key == "ctrl+c" {
			m.stopWork()
			m.quitting = true
			m.result.Aborted = true
			return m, tea.Quit
		}
		switch m.step {
		case StepWelcome:
			if key == "enter" || key == " " {
				return m.loadIndexForArtwork()
			}
		case StepArtPrompt:
			switch key {
			case "y", "Y", "enter":
				return m.beginEstimate()
			case "n", "N":
				return m.finishDeclined()
			}
		case StepModePrompt:
			switch key {
			case "1", "enter":
				return m.beginArtworkWarm()
			case "2", "b", "B":
				return m.finishBackground()
			case "n", "N":
				return m.finishDeclined()
			}
		case StepDone:
			if key == "enter" || key == " " {
				m.quitting = true
				return m, tea.Quit
			}
		}

	case progressMsg:
		m.done = msg.done
		m.total = msg.total
		if msg.message != "" {
			m.status = msg.message
		}
		if msg.total > 0 {
			m.percent = float64(msg.done) / float64(msg.total)
		}
		return m, tea.Batch(m.prog.SetPercent(m.percent), listenEvents(m.events))

	case indexDoneMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.status = "Index failed"
			return m, nil
		}
		m.index = msg.index
		m.step = StepArtPrompt
		m.status = "Official artwork"
		return m, nil

	case warmDoneMsg:
		m.stopWork()
		if msg.err != nil && msg.err != context.Canceled {
			m.errMsg = msg.err.Error()
			m.status = "Download failed"
			return m, nil
		}
		if m.step == StepArtWarm {
			return m.finishAccepted(config.ArtworkModeBlocking, false)
		}

	case estimateDoneMsg:
		m.estimate = msg.bytes
		if assetwarm.NeedsModePrompt(msg.bytes) {
			m.step = StepModePrompt
			m.status = "Large download"
			return m, nil
		}
		return m.beginArtworkWarm()

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
	pal := style.CurrentPalette()
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(pal.Primary)).Render("poke")
	sub := lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Placeholders))
	body := lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Components))
	ok := lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Success))
	warn := lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Warning))
	errSt := lipgloss.NewStyle().Foreground(lipgloss.Color(pal.Error))

	var b strings.Builder
	fmt.Fprintf(&b, "\n  %s  setup\n\n", title)

	proto := m.deps.Protocol
	if proto == "" {
		if m.deps.Native {
			proto = "native"
		} else {
			proto = "chafa"
		}
	}
	fmt.Fprintf(&b, "  %s\n\n", sub.Render("Sprite path: "+proto))

	switch m.step {
	case StepWelcome:
		fmt.Fprintf(&b, "  %s\n", body.Render("Welcome! The Pokédex dataset is ready."))
		fmt.Fprintf(&b, "  %s\n", body.Render("You can optionally download official artwork for every species."))
		fmt.Fprintf(&b, "\n  %s\n", ok.Render("Press Enter to continue"))
	case StepIndex:
		fmt.Fprintf(&b, "  %s\n\n", body.Render(m.status))
		fmt.Fprintf(&b, "  %s\n", m.prog.View())
		if m.total > 0 {
			fmt.Fprintf(&b, "  %s\n", sub.Render(fmt.Sprintf("%d / %d", m.done, m.total)))
		}
	case StepArtPrompt:
		fmt.Fprintf(&b, "  %s\n", body.Render("Official artwork is used for sprites (native or chafa)."))
		fmt.Fprintf(&b, "  %s\n\n", body.Render("Download the full artwork pack now?"))
		fmt.Fprintf(&b, "  %s\n", ok.Render("[Y] Yes   [N] Skip — download on demand while browsing"))
	case StepModePrompt:
		fmt.Fprintf(&b, "  %s\n", warn.Render(fmt.Sprintf("Estimated download ≈ %s (over %s).",
			assetwarm.FormatBytes(m.estimate), assetwarm.FormatBytes(assetwarm.ArtworkSizeThreshold))))
		fmt.Fprintf(&b, "  %s\n\n", body.Render("Choose how to fetch official artwork:"))
		fmt.Fprintf(&b, "  %s\n", ok.Render("[1] Download now (blocking)"))
		fmt.Fprintf(&b, "  %s\n", body.Render("[2] Background — open browse; art fills in over time"))
		fmt.Fprintf(&b, "  %s\n", sub.Render("[N] Skip bulk download"))
	case StepArtWarm:
		fmt.Fprintf(&b, "  %s\n\n", body.Render(m.status))
		fmt.Fprintf(&b, "  %s\n", m.prog.View())
		fmt.Fprintf(&b, "  %s\n", sub.Render(fmt.Sprintf("%d / %d", m.done, m.total)))
	case StepDone:
		fmt.Fprintf(&b, "  %s\n\n", ok.Render("Ready — press Enter to open the Pokédex"))
	}

	if m.errMsg != "" {
		fmt.Fprintf(&b, "\n  %s\n", errSt.Render(m.errMsg))
	}
	fmt.Fprintf(&b, "\n  %s\n", sub.Render("Ctrl+C quit"))
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func (m Model) loadIndexForArtwork() (tea.Model, tea.Cmd) {
	m.status = "Loading index"
	m.errMsg = ""
	deps := m.deps
	return m, func() tea.Msg {
		index, err := deps.EnsureIndex(context.Background(), nil)
		return indexDoneMsg{index: index, err: err}
	}
}

func (m Model) beginIndex() (tea.Model, tea.Cmd) {
	m.step = StepIndex
	m.status = "Fetching species index"
	m.errMsg = ""
	m.done, m.total, m.percent = 0, 0, 0
	deps := m.deps
	return m, tea.Batch(m.prog.SetPercent(0), func() tea.Msg {
		index, err := deps.EnsureIndex(context.Background(), nil)
		return indexDoneMsg{index: index, err: err}
	})
}

func (m Model) beginEstimate() (tea.Model, tea.Cmd) {
	m.status = "Estimating artwork size…"
	m.errMsg = ""
	index := m.index
	deps := m.deps
	return m, func() tea.Msg {
		return estimateDoneMsg{bytes: deps.EstimateHD(context.Background(), index)}
	}
}

func (m Model) beginArtworkWarm() (tea.Model, tea.Cmd) {
	m.step = StepArtWarm
	m.status = "Downloading official artwork"
	m.done, m.total, m.percent = 0, len(m.index), 0
	m.errMsg = ""
	m.stopWork()
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	ch := make(chan tea.Msg, 16)
	m.events = ch
	deps := m.deps
	index := m.index
	go func() {
		err := deps.WarmArtwork(ctx, index, func(p cache.Progress) {
			select {
			case ch <- progressMsg{done: p.Done, total: p.Total, message: p.Message}:
			default:
			}
		})
		ch <- warmDoneMsg{err: err}
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

func (m Model) finishDeclined() (tea.Model, tea.Cmd) {
	return m.persistAndDone(config.ArtworkConsentDeclined, config.ArtworkModeOff, false)
}

func (m Model) finishAccepted(mode string, background bool) (tea.Model, tea.Cmd) {
	return m.persistAndDone(config.ArtworkConsentAccepted, mode, background)
}

func (m Model) finishBackground() (tea.Model, tea.Cmd) {
	return m.finishAccepted(config.ArtworkModeBackground, true)
}

func (m Model) persistAndDone(consent, mode string, background bool) (tea.Model, tea.Cmd) {
	cfg := m.result.Config
	cfg.ApplyArtworkConsent(consent, mode)
	cfg.MarkSetupComplete()
	m.result.Config = cfg
	m.result.StartBackgroundArtwork = background
	if m.deps.SaveConfig != nil {
		if err := m.deps.SaveConfig(cfg); err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
	}
	m.step = StepDone
	m.status = "Setup complete"
	return m, nil
}

// Run launches the setup wizard and returns the outcome.
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
