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
	"github.com/Graphixa/Pokedex/internal/render/sprite/halfblock"
	spriteimage "github.com/Graphixa/Pokedex/internal/render/sprite/image"
	"github.com/Graphixa/Pokedex/internal/render/sprite/nativewarm"
	"github.com/Graphixa/Pokedex/internal/render/sprite/placeholder"
	"github.com/Graphixa/Pokedex/internal/render/sprite/warm"
	"github.com/Graphixa/Pokedex/internal/search"
	"github.com/Graphixa/Pokedex/internal/tui"
	"github.com/Graphixa/Pokedex/internal/tui/style"
)

type tuiDetails struct {
	*DetailService
}

func (t tuiDetails) WarmCore(ctx context.Context, speciesID int) error {
	return t.DetailService.WarmCore(ctx, speciesID)
}

func (t tuiDetails) LoadDossierCore(ctx context.Context, speciesID int, slug string) (domain.Dossier, bool, error) {
	return t.DetailService.LoadDossierCore(ctx, speciesID, slug)
}

func (t tuiDetails) LoadDossierExtras(ctx context.Context, speciesID int, slug string, core domain.Dossier) (domain.Dossier, error) {
	return t.DetailService.LoadDossierExtras(ctx, speciesID, slug, core)
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
		t.Engine.InvalidateSpeciesNative(context.Background(), speciesID)
	}
}

func (t tuiSprites) ClearNativeRegion(cellsW, cellsH int) string {
	if t.Engine == nil {
		return ""
	}
	return t.Engine.ClearNativeRegion(cellsW, cellsH)
}

func (t tuiSprites) PersistFrame(ctx context.Context, cacheKey, frame string) error {
	if t.Engine == nil {
		return nil
	}
	return t.Engine.PersistFrame(ctx, cacheKey, frame)
}

// sendWarmProgress is non-blocking for in-progress ticks so warmers cannot
// stall the TUI. A Done>=Total tick must not be dropped: one stale buffered
// message is discarded so completion can replace it.
func sendWarmProgress(ch chan tui.WarmProgress, p cache.Progress) {
	if ch == nil {
		return
	}
	msg := tui.WarmProgress{
		Done: p.Done, Total: p.Total, Message: p.Message,
		SpeciesID: p.SpeciesID, Stage: p.Stage, Cells: p.Cells,
	}
	select {
	case ch <- msg:
		return
	default:
	}
	if p.Total <= 0 || p.Done < p.Total {
		return
	}
	select {
	case <-ch:
	default:
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

func (h *Handler) openBrowse(ctx context.Context, req BrowseRequest) error {
	debuglog.StartupMark("startup.browse")
	svc, err := h.Services()
	if err != nil {
		return err
	}

	if !svc.Cache.HasActiveDataset() {
		if err := h.runDatasetBootstrap(ctx, svc); err != nil {
			return err
		}
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

	if req.Selected == nil && req.Prefill == "" && req.Target != "" {
		idx := search.Build(index)
		if sum, ok := idx.Resolve(req.Target); ok {
			req.Selected = &sum
		} else {
			req.Prefill = req.Target
		}
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	progressCh := make(chan tui.WarmProgress, 128)
	var useProgress bool

	img := spriteimage.New()
	_, _ = spritechafa.Detect()
	caps := sprite.DetectCapabilities(img.Available())
	debuglog.Renderf("render.backend.selected native=%v protocol=%q term_program=%q wt_session=%v",
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
			halfblock.Renderer{},
			placeholder.Renderer{},
		),
		Artefacts:   svc.Cache,
		Caps:        caps,
		Preference:  sprite.RenderMode(svc.Config.EffectiveSpriteMode()),
		Color:       colorMode,
		SourceKind:  sourceKind,
		Background:  style.BaseRGBA(),
		SixelColors: svc.Config.EffectiveSixelColors(),
		// Consent gates bulk warm, not resolve — always use artwork tier.
		HDOff:               false,
		ClearNativeFn:       spriteimage.ClearBestEffort,
		ClearNativeRegionFn: spriteimage.ClearNativeRegion,
		OnNativeFail:        img.Disable,
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
		Context:       runCtx,
		DisableSprite: !svc.Config.TUI.ShowSprite || svc.Config.EffectiveSpriteMode() == "off",
	}

	// Chafa artefact warm only helps chafa-only terminals. Native (Sixel/Kitty/
	// iTerm) browse uses interactive encode + prefetch; chafa warm would steal CPU.
	prewarm := warm.Mode(svc.Config.EffectiveSpritePrewarm())
	var spriteWarmer *warm.Service
	if prewarm != warm.ModeOff && !opts.DisableSprite && !caps.NativeImage {
		useProgress = true
		spriteWarmer = warm.New(warm.Config{
			Mode:     prewarm,
			Cells:    sprite.MaxFrameSize,
			Workers:  sprite.InteractiveWorkers,
			Loader:   warmPokemonLoader{details},
			Renderer: engine,
			Index:    index,
			Progress: func(p cache.Progress) {
				sendWarmProgress(progressCh, p)
			},
		})
		spriteWarmer.Start(runCtx)
		defer spriteWarmer.Stop()
		opts.Warmer = spriteWarmer
	}

	wantArtworkBG := startBgArtwork ||
		(svc.Config.EffectiveArtworkMode() == config.ArtworkModeBackground && svc.Config.SpriteHDEnabled())

	var imageCoord *cache.ImageCacheCoordinator
	var imageSpec cache.ImageCacheSpec
	var nativeWarmer *nativewarm.Service

	forwardWarm := func(p cache.Progress) {
		if imageCoord != nil {
			snap := imageCoord.Progress()
			if snap.Active && p.Stage != "native" {
				if p.SpeciesID != 0 {
					sendWarmProgress(progressCh, imageCoord.AsCacheProgress(p.SpeciesID))
				}
				return
			}
		}
		sendWarmProgress(progressCh, p)
	}

	nativeImageBrowse := caps.NativeImage && !opts.DisableSprite
	if nativeImageBrowse {
		useProgress = true
		opts.WarmEvents = progressCh
		protocol := img.ProtocolName()
		_ = svc.Cache.EnsureNativeManifest(protocol)
		imageSpec = cache.DefaultImageCacheSpec(protocol, string(sourceKind), style.BaseRGBA(), svc.Config.EffectiveSixelColors())
		coord, err := svc.Cache.StartImageCacheSessionDeferred(index, imageSpec)
		if err != nil {
			debuglog.Cachef("image-cache session err=%v", err)
		} else {
			imageCoord = coord
			imageCoord.SetProgressFunc(forwardWarm)
			if imageCoord.Progress().Active {
				imageCoord.PublishStart()
			}
			defer func() { _ = imageCoord.Flush() }()
		}
		nativeWarmer = nativewarm.New(nativewarm.Config{
			Workers:       sprite.InteractiveWorkers,
			Loader:        warmPokemonLoader{details},
			Renderer:      engine,
			Index:         index,
			Manifest:      svc.Cache,
			Protocol:      imageSpec.Protocol,
			RequiredSizes: imageSpec.RequiredSizes,
			Completer:     imageCoord,
			Progress:      forwardWarm,
		})
		defer nativeWarmer.Stop()
		opts.NativeWarmer = nativeWarmer
		if imageCoord != nil {
			imageCoord.SetOnReconcileActive(func() {
				imageCoord.PublishStart()
				nativeWarmer.EnsureStarted(runCtx, sprite.MaxFrameSize)
			})
			go imageCoord.ReconcileImageCacheSession(index)
		} else {
			nativeWarmer.EnsureStarted(runCtx, sprite.MaxFrameSize)
		}
	}

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
			Progress: forwardWarm,
		})
		artworkWarmer.Start(runCtx)
		defer artworkWarmer.Stop()
		opts.ArtworkWarmer = artworkWarmer
	}

	dossierMode := dossierwarm.Mode(svc.Config.EffectiveDossierPrewarm())
	var dossierWarmer *dossierwarm.Service
	if dossierMode != dossierwarm.ModeOff {
		dwCfg := dossierwarm.Config{
			Mode:     dossierMode,
			Workers:  2,
			Loader:   tuiDetails{details},
			Index:    index,
			Progress: forwardWarm,
		}
		if dossierMode == dossierwarm.ModeWindow {
			dwCfg.CoreLoader = tuiDetails{details}
		}
		dossierWarmer = dossierwarm.New(dwCfg)
		dossierWarmer.Start(runCtx)
		defer dossierWarmer.Stop()
		opts.DossierWarmer = dossierWarmer
		if dossierMode == dossierwarm.ModeAll {
			useProgress = true
		}
	}

	if useProgress {
		opts.WarmEvents = progressCh
	}

	debuglog.StartupMark("startup.tui")
	return tui.Run(runCtx, opts)
}
