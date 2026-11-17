package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Graphixa/Poke/internal/cache"
	"github.com/Graphixa/Poke/internal/config"
	"github.com/Graphixa/Poke/internal/debuglog"
	"github.com/Graphixa/Poke/internal/domain"
	"github.com/Graphixa/Poke/internal/favourites"
	"github.com/Graphixa/Poke/internal/prefs"
	"github.com/Graphixa/Poke/internal/render/sprite"
	spritechafa "github.com/Graphixa/Poke/internal/render/sprite/chafa"
	spriteimage "github.com/Graphixa/Poke/internal/render/sprite/image"
	"github.com/Graphixa/Poke/internal/render/sprite/placeholder"
	"github.com/Graphixa/Poke/internal/render/sprite/prewarm"
	"github.com/Graphixa/Poke/internal/search"
	"github.com/Graphixa/Poke/internal/tui"
	"github.com/Graphixa/Poke/internal/tui/style"
)

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

// gatedNativeWarmer delays sixel/Kitty cache workers until dataset aggregates
// finish preloading (avoids fighting the learnsets/encounters JSON parse).
type gatedNativeWarmer struct {
	inner *prewarm.Service
	ready <-chan struct{}
}

func (g gatedNativeWarmer) EnsureStarted(ctx context.Context, cells int) {
	if g.inner == nil {
		return
	}
	go func() {
		select {
		case <-g.ready:
		case <-ctx.Done():
			return
		}
		g.inner.EnsureStarted(ctx, cells)
	}()
}
func (g gatedNativeWarmer) Prioritize(id int) {
	if g.inner != nil {
		g.inner.Prioritize(id)
	}
}
func (g gatedNativeWarmer) SetNeighborsFromList(list []domain.SpeciesSummary, cursor, radius int) {
	if g.inner != nil {
		g.inner.SetNeighborsFromList(list, cursor, radius)
	}
}
func (g gatedNativeWarmer) SetBuckets(buckets []int) {
	if g.inner != nil {
		g.inner.SetBuckets(buckets)
	}
}
func (g gatedNativeWarmer) SetCells(cells int) {
	if g.inner != nil {
		g.inner.SetCells(cells)
	}
}
func (g gatedNativeWarmer) Snapshot() (done, total int) {
	if g.inner == nil {
		return 0, 0
	}
	return g.inner.Snapshot()
}
func (g gatedNativeWarmer) NoteActivity() {
	if g.inner != nil {
		g.inner.NoteActivity()
	}
}
func (g gatedNativeWarmer) SetForegroundBusy(busy bool) {
	if g.inner != nil {
		g.inner.SetForegroundBusy(busy)
	}
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

	// Caps + cell-size probe first so the XTWINOPS wait overlaps index/wiring.
	img := spriteimage.New()
	caps := sprite.DetectCapabilities(img.Available())
	var cellCh <-chan cellSizeResult
	if caps.NativeImage {
		cellCh = startQueryCellPixelSize()
	}

	var (
		index      []domain.SpeciesSummary
		browseByID map[int]domain.BrowseEntry
	)
	if svc.Cache.HasActiveDataset() {
		// One browse.json read → summaries + glance map (was EnsureIndex + LoadBrowse).
		entries, loadErr := svc.Cache.LoadBrowse()
		if loadErr != nil {
			return loadErr
		}
		if len(entries) == 0 {
			return cache.ErrEmpty
		}
		index = cache.BrowseSummaries(entries)
		browseByID = cache.BrowseByID(entries)
	} else {
		var ensureErr error
		index, _, ensureErr = svc.Cache.EnsureIndex(ctx, nil)
		if ensureErr != nil {
			return ensureErr
		}
	}
	debuglog.StartupMark("startup.index_ready")
	debuglog.StartupMark("startup.dataset_ready")

	idx := search.Build(index)
	if req.Selected == nil && req.Prefill == "" && req.Target != "" {
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
	opts := tui.Options{
		Index:         idx,
		Details:       details,
		Pokemon:       details,
		Catalog:       details,
		Sprites:       tuiSprites{engine},
		Selected:      req.Selected,
		Prefill:       req.Prefill,
		Context:       runCtx,
		DisableSprite: svc.Config.EffectiveSpriteMode() == "off",
		BrowseEntries: browseByID,
	}
	favouriteStore := favourites.Store{Path: filepath.Join(filepath.Dir(svc.Paths.ConfigFile), "favourites.json")}
	if ids, err := favouriteStore.Load(); err != nil {
		opts.FavouriteLoadError = err.Error()
	} else {
		opts.Favourites = ids
		opts.SaveFavourites = favouriteStore.Save
	}
	gameStore := prefs.GameStore{Path: filepath.Join(filepath.Dir(svc.Paths.ConfigFile), "game.json")}
	if vid, err := gameStore.Load(); err == nil && vid > 0 {
		opts.SavedGameVersionID = vid
	}
	opts.SaveGameVersion = gameStore.Save

	warmLoader := warmPokemonLoader{details}
	forward := func(stage tui.WarmStage) func(prewarm.Progress) {
		return func(p prewarm.Progress) {
			if stage == tui.WarmStageNative {
				// Native ticks drive the footer only; a species ID would trigger sprite reloads.
				p.SpeciesID = 0
			} else {
				p.Cells = 0
			}
			sendWarmProgress(progressCh, tui.WarmProgress{
				Done: p.Done, Total: p.Total, Message: p.Message,
				SpeciesID: p.SpeciesID, Stage: stage, Cells: p.Cells,
			})
		}
	}

	// Chafa artefact warm only helps chafa-only terminals. Native (Sixel/Kitty/
	// iTerm) browse uses foreground latest-selection-wins rendering plus
	// active-bucket native warming; chafa warm would steal CPU.
	chafaMode := svc.Config.EffectiveSpritePrewarm()
	if chafaMode != "off" && !opts.DisableSprite && !caps.NativeImage {
		useProgress = true
		spriteWarmer := prewarm.NewChafa(index, warmLoader, engine, chafaMode == "all_pixel",
			forward(tui.WarmStageSpritesRender))
		spriteWarmer.SetBuckets(sprite.BrowseWarmBuckets())
		queueUIReady(func() { spriteWarmer.Start(runCtx) })
		defer spriteWarmer.Stop()
		opts.Warmer = spriteWarmer
	}

	// Aggregates ready before sixel/chafa warm so first dossier Enter is cheap.
	aggregatesReady := make(chan struct{})

	if caps.NativeImage && !opts.DisableSprite {
		useProgress = true
		nativeWarmer := prewarm.NewNative(index, warmLoader, engine, forward(tui.WarmStageNative))
		defer nativeWarmer.Stop()
		opts.NativeWarmer = gatedNativeWarmer{inner: nativeWarmer, ready: aggregatesReady}
	}

	wantArtworkBG := startBgArtwork ||
		svc.Config.EffectiveArtworkMode() == config.ArtworkModeBackground
	if wantArtworkBG && !opts.DisableSprite {
		useProgress = true
		artworkWarmer := prewarm.NewArtwork(index, warmLoader,
			&sprite.SourceResolver{Store: svc.Cache, API: svc.API},
			svc.Cache.MaxConcurrency(), forward(tui.WarmStageArtwork))
		queueUIReady(func() { artworkWarmer.Start(runCtx) })
		defer artworkWarmer.Stop()
		opts.ArtworkWarmer = artworkWarmer
	}

	if dossierMode := svc.Config.EffectiveDossierPrewarm(); dossierMode != "off" {
		dossierWarmer := prewarm.NewDossier(index, details, dossierMode == "all", forward(tui.WarmStageDossier))
		queueUIReady(func() { dossierWarmer.Start(runCtx) })
		defer dossierWarmer.Stop()
		opts.DossierWarmer = dossierWarmer
		if dossierMode == "all" {
			useProgress = true
		}
	}

	if useProgress {
		opts.WarmEvents = progressCh
	}
	opts.OnUIReady = func() {
		go func() {
			debuglog.StartupMark("startup.aggregates_preload_begin")
			_ = svc.Cache.PreloadDataset()
			debuglog.StartupMark("startup.aggregates_preload_done")
			close(aggregatesReady)
			for _, fn := range onUIReady {
				fn()
			}
		}()
	}

	// Probe overlapped wiring; receive before tea owns stdin (≤50ms from start).
	if cellCh != nil {
		if r := <-cellCh; r.ok {
			sprite.SetCellPixelSize(r.pxW, r.pxH)
		}
	}

	// Drop keys left in the console buffer (e.g. setup [Y], or taps during the
	// cell-size probe / warmer spin-up) so they don't type into focused search.
	drainPendingStdin()

	debuglog.StartupMark("startup.tui_enter")
	err = tui.Run(runCtx, opts)
	// Bubble Tea's Kitty keyboard query (CSI ? u → CSI ? flags u) can still be
	// sitting in the console buffer when Ctrl+C / Quit races the input reader —
	// without a drain the shell sees leftover "[?1u".
	drainPendingStdin()
	if lines := debuglog.StartupReport(); len(lines) > 0 && h.Stderr != nil {
		for _, line := range lines {
			_, _ = fmt.Fprintln(h.Stderr, "startup:", line)
		}
	}
	return err
}
