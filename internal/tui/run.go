package tui

import (
	"context"
	"errors"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
)

// runModel wraps Model so Init also starts summary auto-scroll (tests use Model alone).
type runModel struct {
	Model
}

func (m runModel) Init() tea.Cmd {
	return m.Model.Init()
}

func (m runModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.Model.Update(msg)
	m.Model = next.(Model)
	return m, cmd
}

// Run starts the browse TUI with cells-then-DCS native paint and blocks until quit.
func Run(ctx context.Context, opts Options) error {
	if opts.Context == nil {
		opts.Context = ctx
	}
	if opts.Index == nil {
		return fmt.Errorf("browse requires a species index")
	}
	m := NewModel(opts)
	comp := newCompositorWriter(os.Stdout)
	m.composite = true
	m.compositor = comp
	p := tea.NewProgram(runModel{Model: m}, tea.WithContext(ctx), tea.WithOutput(comp))
	final, err := p.Run()
	if fm, ok := final.(runModel); ok {
		fm.Model.shutdown()
	} else {
		m.shutdown()
	}
	if err != nil && !errors.Is(err, tea.ErrInterrupted) {
		return err
	}
	return nil
}
