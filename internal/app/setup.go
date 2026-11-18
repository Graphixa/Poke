package app

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/Graphixa/Poke/internal/cache"
	"github.com/Graphixa/Poke/internal/config"
	"github.com/Graphixa/Poke/internal/debuglog"
	"github.com/Graphixa/Poke/internal/render/sprite"
	"github.com/Graphixa/Poke/internal/render/sprite/prewarm"
	"github.com/Graphixa/Poke/internal/theme"
	"github.com/Graphixa/Poke/internal/tui/preload"
	"github.com/Graphixa/Poke/internal/tui/setup"
)

// runSetupIfNeeded asks first-run questions when setup is incomplete or forced,
// and downloads a missing Pokédex or picture pack (including after cache clear).
// Returns updated config and whether browse should keep filling pictures in the background.
func (h *Handler) runSetupIfNeeded(ctx context.Context, svc *Services, force bool) (config.Config, bool, error) {
	cfg := svc.Config
	ask := force || cfg.NeedsSetup()
	needData := !svc.Cache.HasActiveDataset()
	needArt := needData || !cfg.ArtworkWarmEnabled()
	if !ask && !needData && !needArt {
		bg := cfg.EffectiveArtworkMode() == config.ArtworkModeBackground
		return cfg, bg, nil
	}

	var run func(context.Context, setup.ProgressFunc) error
	if needData || needArt {
		run = h.prepareWork(svc, needData, needArt)
	}
	result, err := setup.Run(ctx, setup.Deps{
		Config:    cfg,
		ThemesDir: theme.ThemesDir(filepath.Dir(h.Paths.ConfigFile)),
		Ask:       ask,
		RunWork:   run,
		SaveConfig: func(c config.Config) error {
			if err := config.Save(svc.Paths.ConfigFile, c); err != nil {
				return err
			}
			svc.Config = c
			return nil
		},
	})
	if err != nil {
		return cfg, false, err
	}
	if result.Aborted {
		return result.Config, false, fmt.Errorf("setup cancelled")
	}
	debuglog.StartupMark("startup.setup_complete")
	svc.Config = result.Config
	return result.Config, result.StartBackgroundArtwork, nil
}

func downloadReserve(data, art bool) map[string]int {
	// Floors keep later, larger parts on the bar before their real counts arrive.
	// A reported total replaces the floor. pokédex ~1,025, forms ~1,600, moves ~900.
	reserve := map[string]int{}
	if data {
		reserve["records"] = 1025
		reserve["evolution"] = 500
		reserve["moves"] = 900
		reserve["ability"] = 300
		reserve["form"] = 1600
	}
	if art {
		reserve["artwork"] = 1025
	}
	return reserve
}

func (h *Handler) prepareWork(svc *Services, needData, needArt bool) func(context.Context, setup.ProgressFunc) error {
	details := NewDetailService(svc.Cache)
	loader := warmPokemonLoader{details}
	resolver := &sprite.SourceResolver{Store: svc.Cache, API: svc.API}
	workers := svc.Cache.MaxConcurrency()
	return func(ctx context.Context, progress setup.ProgressFunc) error {
		bar := preload.NewBar(downloadReserve(needData, needArt))
		report := func(stage string, done, total int) {
			if progress == nil {
				return
			}
			sumDone, sumTotal := bar.Advance(stage, done, total)
			progress(setup.Progress{Stage: stage, Done: sumDone, Total: sumTotal})
		}
		if needData {
			if err := svc.Cache.BootstrapDataset(ctx, func(p cache.DatasetProgress) {
				report(string(p.Stage), p.Done, p.Total)
			}); err != nil {
				return err
			}
		}
		if !needArt {
			return nil
		}
		index, _, err := svc.Cache.EnsureIndex(ctx, func(p cache.DatasetProgress) {
			report(string(p.Stage), p.Done, p.Total)
		})
		if err != nil {
			return err
		}
		w := prewarm.NewArtwork(index, loader, resolver, workers, func(p prewarm.Progress) {
			report("artwork", p.Done, p.Total)
		})
		w.Start(ctx)
		err = w.Wait(ctx)
		w.Stop()
		return err
	}
}
