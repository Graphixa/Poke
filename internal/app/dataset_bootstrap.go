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
	"github.com/Graphixa/Poke/internal/tui/setup"
)

func cacheUpdateProgress(ctx context.Context, w io.Writer, run func(context.Context, updateProgressFunc) error) error {
	if isTerminal(w) {
		result, err := setup.Run(ctx, setup.Deps{Ask: false, RunWork: setupUpdateRun(run)})
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

// setupUpdateRun feeds poke refresh into the shared preloader.
// Dataset and artwork stay on one bar. The stage name picks the grey line.
func setupUpdateRun(run func(context.Context, updateProgressFunc) error) func(context.Context, setup.ProgressFunc) error {
	return func(ctx context.Context, progress setup.ProgressFunc) error {
		advance := progressBar(true)
		return run(ctx, func(p updateProgress) {
			if progress == nil {
				return
			}
			stage := string(p.datasetStage)
			if p.kind == updateProgressArtwork {
				stage = "artwork"
			}
			done, total := advance(stage, p.done, p.total)
			progress(setup.Progress{Stage: stage, Done: done, Total: total})
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
	case cache.DatasetStageImport:
		return "Importing Pokémon data"
	default:
		return fmt.Sprintf("Building dataset %d%% (%d/%d)", pct, p.Done, p.Total)
	}
}
