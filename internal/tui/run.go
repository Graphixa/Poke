package tui

import (
	"context"
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// Run starts the browse TUI and blocks until quit.
func Run(ctx context.Context, opts Options) error {
	if opts.Context == nil {
		opts.Context = ctx
	}
	if opts.Index == nil {
		return fmt.Errorf("browse requires a species index")
	}
	m := NewModel(opts)
	p := tea.NewProgram(m, tea.WithContext(ctx))
	_, err := p.Run()
	if err != nil && !errors.Is(err, tea.ErrInterrupted) {
		return err
	}
	return nil
}
