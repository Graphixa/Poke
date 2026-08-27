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

func (t tuiSprites) Render(ctx context.Context, pokemon domain.Pokemon, cells int) (string, bool, bool, string) {
	if t.Engine == nil {
		return "", true, false, ""
	}
	res := t.Engine.Render(ctx, pokemon, cells)
	return res.ANSI, res.UsePlaceholder, res.Native, res.AssetKind
}

func (t tuiSprites) ClearNative() {
	if t.Engine != nil {
		t.Engine.ClearNative()
	}
}

func (t tuiSprites) InvalidateSpecies(speciesID int) {
	if t.Engine != nil {
		t.Engine.InvalidateSpecies(speciesID)
	}
}

// sendWarmProgress never drops SpeciesID events so selected pop-in cannot miss.
// Aggregate-only ticks (SpeciesID==0) may drop when the buffer is full.
func sendWarmProgress(ch chan<- tui.WarmProgress, p cache.Progress) {
	msg := tui.WarmProgress{
		Done: p.Done, Total: p.Total, Message: p.Message,
		SpeciesID: p.SpeciesID, Stage: p.Stage,
	}
	if p.SpeciesID != 0 {
		ch <- msg
		return
	}
	select {
	case ch <- msg:
	default:
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

func (d dossierLoader) WarmCore(ctx context.Context, speciesID int) error {
	return d.details.WarmCore(ctx, speciesID)
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
		sourceKind = sprite.SourceArtwork
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
		// Consent gates bulk warm, not resolve — always use artwork tier.
		HDOff:         false,
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

	progressCh := make(chan tui.WarmProgress, 128)
	var useProgress bool

	// Chafa artefact warm only helps chafa-only terminals. Native (Sixel/Kitty/
	// iTerm) browse uses interactive encode + prefetch; chafa warm would steal CPU.
	prewarm := warm.Mode(svc.Config.EffectiveSpritePrewarm())
	var spriteWarmer *warm.Service
	if prewarm != warm.ModeOff && !opts.DisableSprite && !caps.NativeImage {
		useProgress = true
		spriteWarmer = warm.New(warm.Config{
			Mode:     prewarm,
			Cells:    sprite.MaxFrameSize,
			Workers:  workers,
			Loader:   warmPokemonLoader{details},
			Renderer: engine,
			Index:    index,
			Progress: func(p cache.Progress) {
				sendWarmProgress(progressCh, p)
			},
		})
		spriteWarmer.Start(ctx)
		defer spriteWarmer.Stop()
		opts.Warmer = spriteWarmer
	}

	wantArtworkBG := startBgArtwork ||
		(svc.Config.EffectiveArtworkMode() == config.ArtworkModeBackground && svc.Config.SpriteHDEnabled())
	var artworkWarmer *assetwarm.Service
	if wantArtworkBG && !opts.DisableSprite {
		useProgress = true
		artworkWarmer = assetwarm.New(assetwarm.Config{
			Mode:     assetwarm.ModeArtwork,
			Stage:    "artwork",
			Workers:  assetwarm.ArtworkWorkers(workers),
			Loader:   warmPokemonLoader{details},
			Resolver: &sprite.SourceResolver{Store: svc.Cache, API: svc.API},
			Index:    index,
			Progress: func(p cache.Progress) {
				sendWarmProgress(progressCh, p)
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
		dl := dossierLoader{details}
		cfg := dossierwarm.Config{
			Mode:    dossierMode,
			Workers: workers,
			Loader:  dl,
			Index:   index,
			Progress: func(p cache.Progress) {
				sendWarmProgress(progressCh, p)
			},
		}
		if dossierMode == dossierwarm.ModeWindow {
			cfg.CoreLoader = dl
		}
		dossierWarmer = dossierwarm.New(cfg)
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
