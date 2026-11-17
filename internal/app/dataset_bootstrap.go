package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/term"

	"github.com/Graphixa/Poke/internal/cache"
	"github.com/Graphixa/Poke/internal/debuglog"
	"github.com/Graphixa/Poke/internal/tui/bootstrap"
)

// runDatasetBootstrap shows a progress TUI when stderr is a terminal, otherwise
// prints throttled progress lines.
func (h *Handler) runDatasetBootstrap(ctx context.Context, svc *Services) error {
	debuglog.StartupMark("startup.bootstrap_begin")
	run := func(ctx context.Context, progress cache.DatasetProgressFunc) error {
		return svc.Cache.BootstrapDataset(ctx, progress)
	}
	if isTerminal(h.Stderr) {
		result, err := bootstrap.Run(ctx, bootstrap.Deps{Run: adaptBootstrapRun(run)})
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

func adaptBootstrapRun(run func(context.Context, cache.DatasetProgressFunc) error) func(context.Context, bootstrap.ProgressFunc) error {
	return func(ctx context.Context, progress bootstrap.ProgressFunc) error {
		return run(ctx, func(p cache.DatasetProgress) {
			if progress == nil {
				return
			}
			progress(bootstrap.Progress{
				Stage:   bootstrapStageFromDataset(p.Stage),
				Done:    p.Done,
				Total:   p.Total,
				Message: p.Message,
			})
		})
	}
}

func bootstrapStageFromDataset(stage cache.DatasetStage) bootstrap.Stage {
	switch stage {
	case cache.DatasetStageIndex:
		return bootstrap.StageIndex
	case cache.DatasetStageEnrich:
		// Enrich is dataset work and gets an explicit bootstrap label.
		return bootstrap.StageEnrich
	case cache.DatasetStageRecords:
		return bootstrap.StageRecords
	case cache.DatasetStageEvolution:
		return bootstrap.StageEvolution
	case cache.DatasetStageMoves:
		return bootstrap.StageMoves
	case cache.DatasetStageDataset:
		return bootstrap.StageDataset
	default:
		return ""
	}
}

func cacheUpdateProgress(ctx context.Context, w io.Writer, run func(context.Context, updateProgressFunc) error) error {
	if isTerminal(w) {
		result, err := bootstrap.Run(ctx, bootstrap.Deps{Run: adaptUpdateRun(run)})
		if err != nil {
			return err
		}
		if result.Aborted {
			return context.Canceled
		}
		return nil
	}
	return run(ctx, throttledCLIUpdateProgress(w))
}

func adaptUpdateRun(run func(context.Context, updateProgressFunc) error) func(context.Context, bootstrap.ProgressFunc) error {
	return func(ctx context.Context, progress bootstrap.ProgressFunc) error {
		return run(ctx, func(p updateProgress) {
			if progress == nil {
				return
			}
			stage := bootstrap.StageArtwork
			if p.kind == updateProgressDataset {
				stage = bootstrapStageFromDataset(p.datasetStage)
			}
			progress(bootstrap.Progress{
				Stage:   stage,
				Done:    p.done,
				Total:   p.total,
				Message: p.message,
			})
		})
	}
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
	lastStage cache.DatasetStage
}

func throttledCLIProgress(w io.Writer) cache.DatasetProgressFunc {
	if w == nil {
		return nil
	}
	t := &throttledProgress{w: w}
	return t.report
}

func throttledCLIUpdateProgress(w io.Writer) updateProgressFunc {
	if w == nil {
		return nil
	}
	dataset := throttledCLIProgress(w)
	lastArtworkBucket := -1
	return func(p updateProgress) {
		if p.kind == updateProgressDataset {
			dataset(cache.DatasetProgress{
				Stage:   p.datasetStage,
				Done:    p.done,
				Total:   p.total,
				Message: p.message,
			})
			return
		}
		if p.total <= 0 {
			if p.message != "" {
				_, _ = fmt.Fprintln(w, p.message)
			}
			return
		}
		pct := p.done * 100 / p.total
		bucket := pct / 5
		if p.done != p.total && bucket == lastArtworkBucket {
			return
		}
		lastArtworkBucket = bucket
		_, _ = fmt.Fprintf(w, "Downloading artwork %d%% (%d/%d)\n", pct, p.done, p.total)
	}
}

func (t *throttledProgress) report(p cache.DatasetProgress) {
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
		stage = cache.DatasetStageDataset
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

func formatBootstrapProgress(p cache.DatasetProgress) string {
	stage := p.Stage
	if stage == "" {
		stage = cache.DatasetStageDataset
	}
	pct := 0
	if p.Total > 0 {
		pct = p.Done * 100 / p.Total
	}
	switch stage {
	case cache.DatasetStageIndex:
		if p.Done == p.Total && p.Total > 0 {
			return p.Message
		}
		return "Fetching species index"
	case cache.DatasetStageEnrich:
		return fmt.Sprintf("Enriching types %d%% (%d/%d)", pct, p.Done, p.Total)
	case cache.DatasetStageRecords:
		return fmt.Sprintf("Building dataset %d%% (%d/%d records)", pct, p.Done, p.Total)
	case cache.DatasetStageEvolution:
		return fmt.Sprintf("Fetching evolution chains %d%% (%d/%d)", pct, p.Done, p.Total)
	case cache.DatasetStageMoves:
		return fmt.Sprintf("Fetching moves %d%% (%d/%d)", pct, p.Done, p.Total)
	case cache.DatasetStageDataset:
		if p.Message != "" {
			return p.Message
		}
		return "Building dataset"
	default:
		return fmt.Sprintf("Building dataset %d%% (%d/%d)", pct, p.Done, p.Total)
	}
}
