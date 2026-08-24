package tui

import (
	"context"
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// runModel wraps Model so Init also starts summary auto-scroll (tests use Model alone).
type runModel struct {
	Model
}

func (m runModel) Init() tea.Cmd {
	return tea.Batch(m.Model.Init(), tickSummary())
}

func (m runModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.Model.Update(msg)
	m.Model = next.(Model)
	return m, cmd
}

// Run starts the browse TUI and blocks until quit.
func Run(ctx context.Context, opts Options) error {
	if opts.Context == nil {
		opts.Context = ctx
	}
	if opts.Index == nil {
		return fmt.Errorf("browse requires a species index")
	}
	m := runModel{Model: NewModel(opts)}
	p := tea.NewProgram(m, tea.WithContext(ctx))
	_, err := p.Run()
	if err != nil && !errors.Is(err, tea.ErrInterrupted) {
		return err
	}
	return nil
}
