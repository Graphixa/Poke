package app

import (
	"context"
	"os"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/cache/dossierwarm"
	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/render/sprite/assetwarm"
	spritechafa "github.com/Graphixa/Pokedex/internal/render/sprite/chafa"
	spriteimage "github.com/Graphixa/Pokedex/internal/render/sprite/image"
	"github.com/Graphixa/Pokedex/internal/render/sprite/placeholder"
	"github.com/Graphixa/Pokedex/internal/render/sprite/warm"
	"github.com/Graphixa/Pokedex/internal/search"
	"github.com/Graphixa/Pokedex/internal/tui"
	"github.com/Graphixa/Pokedex/internal/tui/style"
)

type tuiDetails struct {
	*DetailService
}

func (t tuiDetails) LoadDossier(ctx context.Context, speciesID int) (domain.Dossier, error) {
	return t.DetailService.LoadDossier(ctx, speciesID)
}

func (t tuiDetails) PokemonForSprite(ctx context.Context, speciesID int) (domain.Pokemon, error) {
	return t.DetailService.PokemonForSprite(ctx, speciesID)
}

type tuiSprites struct {
	*sprite.Engine
}

func (t tuiSprites) Render(ctx context.Context, pokemon domain.Pokemon, cells int) (string, bool, bool) {
	if t.Engine == nil {
		return "", true, false
	}
	res := t.Engine.Render(ctx, pokemon, cells)
	return res.ANSI, res.UsePlaceholder, res.Native
}

func (t tuiSprites) ClearNative() {
	if t.Engine != nil {
		t.Engine.ClearNative()
	}
}

type warmPokemonLoader struct {
	details *DetailService
}

func (w warmPokemonLoader) Pokemon(ctx context.Context, speciesID int) (domain.Pokemon, error) {
	return w.details.PokemonForSprite(ctx, speciesID)
}

type dossierLoader struct {
	details *DetailService
}

func (d dossierLoader) LoadDossier(ctx context.Context, speciesID int) (domain.Dossier, error) {
	return d.details.LoadDossier(ctx, speciesID)
}

func (h *Handler) openBrowse(ctx context.Context, req BrowseRequest) error {
	svc, err := h.Services()
	if err != nil {
		return err
	}

	cfg, startBgArtwork, err := h.runSetupIfNeeded(ctx, svc)
	if err != nil {
		return err
	}
	svc.Config = cfg

	index, _, err := svc.Cache.EnsureIndex(ctx, nil)
	if err != nil {
		return err
	}
	if cache.NeedsTypeEnrichment(index) {
		svc.Cache.BackgroundEnrich(ctx, nil)
	}

	img := spriteimage.New()
	caps := sprite.DetectCapabilities(img.Available())
	debuglog.Renderf("sprite caps native=%v protocol=%q term_program=%q wt_session=%v",
		caps.NativeImage, img.ProtocolName(), os.Getenv("TERM_PROGRAM"), os.Getenv("WT_SESSION") != "")
	colorMode := sprite.ColorMode(svc.Config.Render.Color)
	if colorMode == "" || colorMode == sprite.ColorAuto {
		if caps.TrueColor {
			colorMode = sprite.ColorTrueColor
		} else {
			colorMode = sprite.Color256
		}
	}
	sourceKind := sprite.SpriteSource(svc.Config.Render.SpriteSource)
	if sourceKind == "" {
		sourceKind = sprite.SourcePixel
	}

	engine := &sprite.Engine{
		Source: &sprite.SourceResolver{
			Store: svc.Cache,
			API:   svc.API,
		},
		Modes: sprite.NewModeResolver(
			img,
			spritechafa.Renderer{},
			placeholder.Renderer{},
		),
		Artefacts:     svc.Cache,
		Caps:          caps,
		Preference:    sprite.RenderMode(svc.Config.EffectiveSpriteMode()),
		Color:         colorMode,
		SourceKind:    sourceKind,
		Background:    style.BaseRGBA(),
		HDOff:         !svc.Config.SpriteHDEnabled(),
		ClearNativeFn: spriteimage.ClearBestEffort,
		OnNativeFail:  img.Disable,
	}

	details := NewDetailService(svc.Cache)
	workers := svc.Cache.MaxConcurrency()
	opts := tui.Options{
		Index:         search.Build(index),
		Details:       tuiDetails{details},
		Pokemon:       tuiDetails{details},
		Sprites:       tuiSprites{engine},
		Selected:      req.Selected,
		Prefill:       req.Prefill,
		Context:       ctx,
		DisableSprite: !svc.Config.TUI.ShowSprite || svc.Config.EffectiveSpriteMode() == "off",
	}

	progressCh := make(chan tui.WarmProgress, 16)
	var useProgress bool

	// Sprite warm first (visible art), then dossier warm (Enter snappiness).
	prewarm := warm.Mode(svc.Config.EffectiveSpritePrewarm())
	var spriteWarmer *warm.Service
	if prewarm != warm.ModeOff && !opts.DisableSprite {
		useProgress = true
		spriteWarmer = warm.New(warm.Config{
			Mode:     prewarm,
			Cells:    sprite.MaxFrameSize,
			Workers:  workers,
			Loader:   warmPokemonLoader{details},
			Renderer: engine,
			Index:    index,
			Progress: func(p cache.Progress) {
				select {
				case progressCh <- tui.WarmProgress{Done: p.Done, Total: p.Total, Message: p.Message}:
				default:
				}
			},
		})
		spriteWarmer.Start(ctx)
		defer spriteWarmer.Stop()
		opts.Warmer = spriteWarmer
	}

	wantArtworkBG := startBgArtwork ||
		(svc.Config.EffectiveArtworkMode() == config.ArtworkModeBackground && svc.Config.SpriteHDEnabled())
	var artworkWarmer *assetwarm.Service
	if wantArtworkBG && caps.NativeImage && !opts.DisableSprite {
		useProgress = true
		artworkWarmer = assetwarm.New(assetwarm.Config{
			Mode:     assetwarm.ModeArtwork,
			Workers:  workers,
			Loader:   warmPokemonLoader{details},
			Resolver: &sprite.SourceResolver{Store: svc.Cache, API: svc.API},
			Index:    index,
			Progress: func(p cache.Progress) {
				select {
				case progressCh <- tui.WarmProgress{Done: p.Done, Total: p.Total, Message: p.Message}:
				default:
				}
			},
		})
		artworkWarmer.Start(ctx)
		defer artworkWarmer.Stop()
		opts.ArtworkWarmer = artworkWarmer
	}

	dossierMode := dossierwarm.Mode(svc.Config.EffectiveDossierPrewarm())
	var dossierWarmer *dossierwarm.Service
	if dossierMode != dossierwarm.ModeOff {
		useProgress = true
		dossierWarmer = dossierwarm.New(dossierwarm.Config{
			Mode:    dossierMode,
			Workers: workers,
			Loader:  dossierLoader{details},
			Index:   index,
			Progress: func(p cache.Progress) {
				select {
				case progressCh <- tui.WarmProgress{Done: p.Done, Total: p.Total, Message: p.Message}:
				default:
				}
			},
		})
		dossierWarmer.Start(ctx)
		defer dossierWarmer.Stop()
		opts.DossierWarmer = dossierWarmer
	}

	if useProgress {
		defer close(progressCh)
		opts.WarmEvents = progressCh
	}

	return tui.Run(ctx, opts)
}
