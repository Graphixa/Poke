package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/term"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/tui/bootstrap"
)

// runDatasetBootstrap shows a progress TUI when stderr is a terminal, otherwise
// prints throttled progress lines.
func (h *Handler) runDatasetBootstrap(ctx context.Context, svc *Services) error {
	debuglog.StartupMark("startup.bootstrap_begin")
	run := func(ctx context.Context, progress cache.ProgressFunc) error {
		return svc.Cache.BootstrapDataset(ctx, progress)
	}
	if isTerminal(h.Stderr) {
		result, err := bootstrap.Run(ctx, bootstrap.Deps{Run: run})
		if err != nil {
			return err
		}
		if result.Aborted {
			return context.Canceled
		}
		return nil
	}
	return run(ctx, throttledCLIProgress(h.Stderr))
}

// cacheDatasetProgress returns TUI progress on a terminal stderr, otherwise throttled lines.
func cacheDatasetProgress(ctx context.Context, w io.Writer, run func(context.Context, cache.ProgressFunc) error) error {
	if isTerminal(w) {
		result, err := bootstrap.Run(ctx, bootstrap.Deps{Run: run})
		if err != nil {
			return err
		}
		if result.Aborted {
			return context.Canceled
		}
		return nil
	}
	return run(ctx, throttledCLIProgress(w))
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

type throttledProgress struct {
	w         io.Writer
	mu        sync.Mutex
	lastWrite time.Time
	lastPct   int
	lastStage string
}

func throttledCLIProgress(w io.Writer) cache.ProgressFunc {
	if w == nil {
		return nil
	}
	t := &throttledProgress{w: w}
	return t.report
}

func (t *throttledProgress) report(p cache.Progress) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	if p.Total <= 0 {
		if p.Message != "" {
			_, _ = fmt.Fprintln(t.w, p.Message)
			t.lastWrite = now
		}
		return
	}

	pct := p.Done * 100 / p.Total
	stage := p.Stage
	if stage == "" {
		stage = "dataset"
	}
	if p.Done != p.Total && pct/5 == t.lastPct/5 && stage == t.lastStage && now.Sub(t.lastWrite) < 2*time.Second {
		return
	}
	t.lastPct = pct
	t.lastStage = stage
	t.lastWrite = now

	line := formatBootstrapProgress(p)
	if p.Done != p.Total {
		_, _ = fmt.Fprintf(t.w, "\r%s", line)
	} else {
		_, _ = fmt.Fprintf(t.w, "%s\n", line)
	}
}

func formatBootstrapProgress(p cache.Progress) string {
	stage := p.Stage
	if stage == "" {
		stage = "dataset"
	}
	pct := 0
	if p.Total > 0 {
		pct = p.Done * 100 / p.Total
	}
	switch stage {
	case "index":
		if p.Done == p.Total && p.Total > 0 {
			return p.Message
		}
		return "Fetching species index"
	case "enrich":
		return fmt.Sprintf("Enriching types %d%% (%d/%d)", pct, p.Done, p.Total)
	case "records":
		return fmt.Sprintf("Building dataset %d%% (%d/%d records)", pct, p.Done, p.Total)
	case "evolution":
		return fmt.Sprintf("Fetching evolution chains %d%% (%d/%d)", pct, p.Done, p.Total)
	case "moves":
		return fmt.Sprintf("Fetching moves %d%% (%d/%d)", pct, p.Done, p.Total)
	case "dataset":
		if p.Message != "" {
			return p.Message
		}
		return "Building dataset"
	default:
		return fmt.Sprintf("Building dataset %d%% (%d/%d)", pct, p.Done, p.Total)
	}
}
