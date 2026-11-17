package app

import (
	"context"
	"fmt"
	"io"

	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/update"
)

// releaseUpdater adapts internal/update to cli.Updater.
type releaseUpdater struct {
	configPath string
	interval   int
}

func (u releaseUpdater) Check(ctx context.Context) (current, latest string, needsUpdate bool, err error) {
	result, err := update.CheckForUpdates(ctx)
	if err != nil {
		return "", "", false, err
	}
	return result.Current, result.Latest, result.NeedsUpdate, nil
}

func (u releaseUpdater) ApplyLatest(ctx context.Context) error {
	return update.UpdateToLatest(ctx)
}

func (u releaseUpdater) ApplyVersion(ctx context.Context, version string) error {
	return update.UpdateToVersion(ctx, version)
}

func (u releaseUpdater) RecordCheck(lastChecked, nextCheck string) error {
	if u.configPath == "" {
		return nil
	}
	return config.RecordUpdateCheck(u.configPath, lastChecked, nextCheck)
}

func (u releaseUpdater) CheckIntervalHours() int {
	if u.interval > 0 {
		return u.interval
	}
	return update.UpdateDeclinedGracePeriodHours
}

// maybeStartupUpdateCheck runs a non-blocking release check when configured.
func maybeStartupUpdateCheck(paths config.Paths, cfg config.Config, stderr io.Writer) {
	if !cfg.Update.CheckForUpdates {
		return
	}
	go update.PerformStartupCheck(
		context.Background(),
		cfg.Update.CheckForUpdates,
		cfg.Update.UpdateCheckInterval,
		cfg.Update.LastUpdateCheck,
		func(result *update.CheckResult) {
			_ = config.RecordUpdateCheck(paths.ConfigFile, update.GetLastCheckedTimestamp(), "")
			if result == nil || result.Error != nil || !result.UpdateAvailable {
				return
			}
			if !update.ShouldShowUpdatePrompt(cfg.Update.NextUpdateCheck) {
				return
			}
			if stderr == nil {
				return
			}
			fmt.Fprintf(stderr, "poke: update available (v%s → v%s); run poke update\n",
				result.CurrentVersion, result.LatestVersion)
		},
	)
}
