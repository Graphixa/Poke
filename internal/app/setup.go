package app

import (
	"context"
	"fmt"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/render/sprite/assetwarm"
	spriteimage "github.com/Graphixa/Pokedex/internal/render/sprite/image"
	"github.com/Graphixa/Pokedex/internal/tui/setup"
)

// runSetupIfNeeded launches the first-run wizard when setup.completed is false.
// Returns updated config and whether browse should start a background artwork warmer.
func (h *Handler) runSetupIfNeeded(ctx context.Context, svc *Services) (config.Config, bool, error) {
	cfg := svc.Config
	if !cfg.NeedsSetup() {
		bg := cfg.EffectiveArtworkMode() == config.ArtworkModeBackground && cfg.SpriteHDEnabled()
		return cfg, bg, nil
	}

	img := spriteimage.New()
	caps := sprite.DetectCapabilities(img.Available())
	protocol := "chafa"
	if caps.NativeImage {
		if p := img.ProtocolName(); p != "" {
			protocol = p
		} else {
			protocol = "native"
		}
	}

	details := NewDetailService(svc.Cache)
	loader := warmPokemonLoader{details}
	resolver := &sprite.SourceResolver{Store: svc.Cache, API: svc.API}
	workers := svc.Cache.MaxConcurrency()

	warmAssets := func(ctx context.Context, mode assetwarm.Mode, index []domain.SpeciesSummary, progress cache.ProgressFunc) error {
		w := assetwarm.New(assetwarm.Config{
			Mode:    mode,
			Workers: workers,
			Loader:  loader,
			Resolver: resolver,
			Index:   index,
			Progress: func(p cache.Progress) {
				if progress != nil {
					progress(p)
				}
			},
		})
		w.Start(ctx)
		err := w.Wait(ctx)
		w.Stop()
		return err
	}

	result, err := setup.Run(ctx, setup.Deps{
		Config:   cfg,
		Native:   caps.NativeImage,
		Protocol: protocol,
		EnsureIndex: func(ctx context.Context, progress cache.ProgressFunc) ([]domain.SpeciesSummary, error) {
			index, _, err := svc.Cache.EnsureIndex(ctx, progress)
			if err != nil {
				return nil, err
			}
			if cache.NeedsTypeEnrichment(index) {
				svc.Cache.BackgroundEnrich(ctx, progress)
			}
			return index, nil
		},
		WarmPixels: func(ctx context.Context, index []domain.SpeciesSummary, progress cache.ProgressFunc) error {
			return warmAssets(ctx, assetwarm.ModePixel, index, progress)
		},
		EstimateHD: func(ctx context.Context, index []domain.SpeciesSummary) int64 {
			return assetwarm.EstimateArtworkBytes(ctx, index, loader, assetwarm.HTTPSizeSampler{})
		},
		WarmArtwork: func(ctx context.Context, index []domain.SpeciesSummary, progress cache.ProgressFunc) error {
			return warmAssets(ctx, assetwarm.ModeArtwork, index, progress)
		},
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
	svc.Config = result.Config
	return result.Config, result.StartBackgroundArtwork, nil
}
