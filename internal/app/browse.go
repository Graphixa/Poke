package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Graphixa/Pokedex/internal/cache/dossierwarm"
	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/favourites"
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
func sendWarmProgress(ch chan tui.WarmProgress, msg tui.WarmProgress) {
	if ch == nil {
		return
	}
	select {
	case ch <- msg:
		return
	default:
	}
	if msg.Total <= 0 || msg.Done < msg.Total {
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
	return h.runBrowse(ctx, req)
}

func (h *Handler) runBrowse(ctx context.Context, req BrowseRequest) error {
	debuglog.StartupMark("startup.browse_enter")
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
	debuglog.StartupMark("startup.index_ready")
	debuglog.StartupMark("startup.dataset_ready")

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
	caps := sprite.DetectCapabilities(img.Available())
	if caps.NativeImage {
		// Measure real cell pixels before the engine/warmers exist so every
		// sixel encode fits the art box (overpaint fix). Fallback: 10×20.
		if pxW, pxH, ok := queryCellPixelSize(); ok {
			sprite.SetCellPixelSize(pxW, pxH)
		}
	}
	debuglog.StartupMark("startup.capabilities_ready")
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
	defer engine.ClosePersist()

	var onUIReady []func()
	queueUIReady := func(fn func()) {
		if fn != nil {
			onUIReady = append(onUIReady, fn)
		}
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
	favouriteStore := favourites.Store{Path: filepath.Join(filepath.Dir(svc.Paths.ConfigFile), "favourites.json")}
	if ids, err := favouriteStore.Load(); err != nil {
		opts.FavouriteLoadError = err.Error()
	} else {
		opts.Favourites = ids
		opts.SaveFavourites = favouriteStore.Save
	}
	if entries, err := svc.Cache.LoadBrowse(); err == nil && len(entries) > 0 {
		byID := make(map[int]domain.BrowseEntry, len(entries))
		for _, e := range entries {
			byID[e.ID] = e
		}
		opts.BrowseEntries = byID
	}

	// Chafa artefact warm only helps chafa-only terminals. Native (Sixel/Kitty/
	// iTerm) browse uses foreground latest-selection-wins rendering plus
	// active-bucket native warming; chafa warm would steal CPU.
	prewarm := warm.Mode(svc.Config.EffectiveSpritePrewarm())
	var spriteWarmer *warm.Service
	if prewarm != warm.ModeOff && !opts.DisableSprite && !caps.NativeImage {
		useProgress = true
		spriteWarmer = warm.New(warm.Config{
			Mode:     prewarm,
			Cells:    sprite.BrowseWarmBuckets()[0],
			Workers:  sprite.InteractiveWorkers,
			Loader:   warmPokemonLoader{details},
			Renderer: engine,
			Index:    index,
			Progress: func(p warm.Progress) {
				sendWarmProgress(progressCh, tui.WarmProgress{
					Done: p.Done, Total: p.Total, Message: p.Message,
					SpeciesID: p.SpeciesID, Stage: tui.WarmStageSpritesRender,
				})
			},
		})
		spriteWarmer.SetBuckets(sprite.BrowseWarmBuckets())
		queueUIReady(func() { spriteWarmer.Start(runCtx) })
		defer spriteWarmer.Stop()
		opts.Warmer = spriteWarmer
	}

	wantArtworkBG := startBgArtwork ||
		(svc.Config.EffectiveArtworkMode() == config.ArtworkModeBackground && svc.Config.SpriteHDEnabled())

	var nativeWarmer *nativewarm.Service

	forwardNative := func(p nativewarm.Progress) {
		sendWarmProgress(progressCh, tui.WarmProgress{
			Done: p.Done, Total: p.Total, Message: p.Message,
			Stage: tui.WarmStageNative, Cells: p.Cells,
		})
	}
	forwardAsset := func(p assetwarm.Progress) {
		sendWarmProgress(progressCh, tui.WarmProgress{
			Done: p.Done, Total: p.Total, Message: p.Message,
			SpeciesID: p.SpeciesID, Stage: tui.WarmStageArtwork,
		})
	}
	forwardDossier := func(p dossierwarm.Progress) {
		sendWarmProgress(progressCh, tui.WarmProgress{
			Done: p.Done, Total: p.Total, Message: p.Message,
			SpeciesID: p.SpeciesID, Stage: tui.WarmStageDossier,
		})
	}

	nativeImageBrowse := caps.NativeImage && !opts.DisableSprite
	if nativeImageBrowse {
		useProgress = true
		nativeWarmer = nativewarm.New(nativewarm.Config{
			Loader:   warmPokemonLoader{details},
			Renderer: engine,
			Index:    index,
			Progress: forwardNative,
		})
		defer nativeWarmer.Stop()
		opts.NativeWarmer = nativeWarmer
	}

	var artworkWarmer *assetwarm.Service
	if wantArtworkBG && !opts.DisableSprite {
		useProgress = true
		artworkWarmer = assetwarm.New(assetwarm.Config{
			Mode:     assetwarm.ModeArtwork,
			Workers:  assetwarm.ArtworkWorkers(workers),
			Loader:   warmPokemonLoader{details},
			Resolver: &sprite.SourceResolver{Store: svc.Cache, API: svc.API},
			Index:    index,
			Progress: forwardAsset,
		})
		queueUIReady(func() { artworkWarmer.Start(runCtx) })
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
			Progress: forwardDossier,
		}
		if dossierMode == dossierwarm.ModeWindow {
			dwCfg.CoreLoader = tuiDetails{details}
		}
		dossierWarmer = dossierwarm.New(dwCfg)
		queueUIReady(func() { dossierWarmer.Start(runCtx) })
		defer dossierWarmer.Stop()
		opts.DossierWarmer = dossierWarmer
		if dossierMode == dossierwarm.ModeAll {
			useProgress = true
		}
	}

	if useProgress {
		opts.WarmEvents = progressCh
	}
	opts.OnUIReady = func() {
		for _, fn := range onUIReady {
			fn()
		}
	}

	debuglog.StartupMark("startup.tui_enter")
	err = tui.Run(runCtx, opts)
	if lines := debuglog.StartupReport(); len(lines) > 0 && h.Stderr != nil {
		for _, line := range lines {
			_, _ = fmt.Fprintln(h.Stderr, "startup:", line)
		}
	}
	return err
}
