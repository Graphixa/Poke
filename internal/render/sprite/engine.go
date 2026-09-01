package sprite

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"sync"

	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/domain"
)

// ArtefactCache stores derived ANSI and native graphics frames.
type ArtefactCache interface {
	Artefact(ctx context.Context, key string) (string, error)
	StoreArtefact(ctx context.Context, key, ansi string) error
	Native(ctx context.Context, key string) (string, error)
	StoreNative(ctx context.Context, key, frame string) error
}

// ClearNativeFunc clears prior native image placements (best-effort).
type ClearNativeFunc func()

// ClearNativeRegionFunc returns a scoped native clear sequence for WxH cells.
type ClearNativeRegionFunc func(cellsW, cellsH int, bg color.RGBA) string

// Engine orchestrates PNG resolution, mode selection, rendering, and caching.
type Engine struct {
	Source     *SourceResolver
	Modes      *ModeResolver
	Artefacts  ArtefactCache
	Caps       Capabilities
	Preference RenderMode
	Color      ColorMode
	SourceKind SpriteSource
	Background color.RGBA
	// HDOff forces pixel-tier assets (tests / explicit pixel SourceKind).
	// Product browse leaves this false — consent gates bulk warm only.
	HDOff bool
	// Quality defaults to interactive; warm service sets QualityPrewarm.
	Quality QualityProfile
	// FrameW/FrameH override the fixed frame; zero uses FrameWidth/FrameHeight.
	FrameW int
	FrameH int
	// SixelColors is the sixel palette limit (64 | 128 | 256); zero uses 64.
	SixelColors int
	// ClearNativeFn clears native placements; optional.
	ClearNativeFn ClearNativeFunc
	// ClearNativeRegionFn emits scoped Sixel/Kitty clear for WxH cells; optional.
	ClearNativeRegionFn ClearNativeRegionFunc
	// OnNativeFail is called when native image rendering fails (session fallback).
	OnNativeFail func()

	memOnce       sync.Once
	nativeMemOnce sync.Once
	ansiMem       *frameMemCache
	nativeMem     *frameMemCache

	persistOnce sync.Once
	persistQ    *persistQueue
}

func (e *Engine) ansiMemory() *frameMemCache {
	if e == nil {
		return nil
	}
	e.memOnce.Do(func() {
		e.ansiMem = newFrameMemCacheBytes(defaultAnsiMemCacheBytes)
	})
	return e.ansiMem
}

func (e *Engine) nativeMemory() *frameMemCache {
	if e == nil {
		return nil
	}
	e.nativeMemOnce.Do(func() {
		e.nativeMem = newFrameMemCacheBytes(defaultNativeMemCacheBytes)
	})
	return e.nativeMem
}

// RenderResult is either ANSI cell-art or a placeholder request.
type RenderResult struct {
	ANSI           string
	UsePlaceholder bool
	CacheKey       string
	Backend        string
	// Native is true when ANSI embeds Kitty/iTerm/Sixel graphics sequences.
	Native bool
	// AssetKind is "artwork" or "pixel" for the PNG that was encoded.
	AssetKind string
}

// protocolNamer is implemented by the image renderer to tag native cache keys.
type protocolNamer interface {
	ProtocolName() string
}

// ClearNative clears prior native image placements (best-effort).
func (e *Engine) ClearNative() {
	if e != nil && e.ClearNativeFn != nil {
		e.ClearNativeFn()
	}
}

// ClearNativeRegion emits a scoped clear sequence for a WxH cell art box.
func (e *Engine) ClearNativeRegion(cellsW, cellsH int) string {
	if e == nil || e.ClearNativeRegionFn == nil || cellsW < 1 || cellsH < 1 {
		return ""
	}
	bg := e.Background
	if bg.A == 0 {
		bg = DefaultBackground
	}
	return e.ClearNativeRegionFn(cellsW, cellsH, bg)
}

// InvalidateSpecies drops mem-cached frames for a species so artwork pop-in
// re-encodes after named PNGs are overwritten on disk.
func (e *Engine) InvalidateSpecies(speciesID int) {
	if e == nil || speciesID < 1 {
		return
	}
	// NamedAssetKey format: named-{id}-{slug}-{kind}; match on named-{id}-
	e.ansiMemory().InvalidatePrefix(fmt.Sprintf("named-%d-", speciesID))
	e.nativeMemory().InvalidatePrefix(fmt.Sprintf("named-%d-", speciesID))
}

// InvalidateSpeciesNative drops mem + disk native frames for a species.
func (e *Engine) InvalidateSpeciesNative(ctx context.Context, speciesID int) {
	e.InvalidateSpecies(speciesID)
	if e == nil || speciesID < 1 {
		return
	}
	if nativeStore, ok := e.Artefacts.(NativeSpeciesStore); ok {
		if hashes, ok := nativeStore.(NativeSpeciesHashes); ok {
			for h := range hashes.SpeciesAssetHashes(speciesID) {
				e.ansiMemory().InvalidatePrefix(h)
				e.nativeMemory().InvalidatePrefix(h)
			}
		}
		_ = nativeStore.InvalidateNativeSpecies(speciesID)
	}
}

// NativeSpeciesHashes exposes stored PNG hashes for mem-cache invalidation.
type NativeSpeciesHashes interface {
	SpeciesAssetHashes(speciesID int) map[string]struct{}
}

// NativeSpeciesStore deletes on-disk native frames for a species.
type NativeSpeciesStore interface {
	InvalidateNativeSpecies(speciesID int) error
}

// Render converts a sprite into a frame. cells is the square edge requested by
// the TUI (bucketed); when ≤0 the engine's configured/default frame size is used.
// Warm frames are served from LookupCached; misses fall through to RenderCold.
func (e *Engine) Render(ctx context.Context, pokemon domain.Pokemon, cells int) RenderResult {
	if res, hit := e.LookupCached(ctx, pokemon, cells); hit {
		return res
	}
	return e.RenderCold(ctx, pokemon, cells)
}

// LookupCached returns a previously rendered frame from the process-local LRU
// or disk derived/native artefact cache. It never reads original PNG, decodes,
// resizes, quantises, encodes, or performs HTTP.
func (e *Engine) LookupCached(ctx context.Context, pokemon domain.Pokemon, cells int) (RenderResult, bool) {
	end := debuglog.Trace(debuglog.Render, "asset.meta.lookup")
	defer end()
	if e == nil || e.Modes == nil || e.Source == nil {
		return RenderResult{}, false
	}
	if err := ctx.Err(); err != nil {
		return RenderResult{}, false
	}
	if e.Preference == ModeOff {
		return RenderResult{}, false
	}
	fw, fh := e.frameSize()
	if cells >= MinFrameSize {
		fw, fh = FrameSizeFor(cells)
	}
	rend := e.Modes.Resolve(e.Caps, e.Preference)
	if rend == nil || rend.Name() == "placeholder" {
		return RenderResult{}, false
	}
	opts := e.renderOptions(displayName(pokemon), fw, fh, "", "")
	for _, ref := range e.lookupAssetRefs(ctx, pokemon, rend.Name() == "image") {
		if !ref.Ready {
			continue
		}
		kind := ref.Kind
		if kind == "" {
			kind = namedKindForTier(AssetTierHD)
			if ref.Source == SourcePixel {
				kind = namedKindForTier(AssetTierPixel)
			}
		}
		canonical := ref.Hash
		if rend.Name() == "image" && !IsHexSHA256(canonical) {
			canonical = ""
		}
		opts.ContentHash = canonical
		if rend.Name() == "image" {
			if hit, result := e.lookupNativeFrame(ctx, rend, opts, canonical, kind); hit {
				return result, true
			}
			continue
		}
		if canonical == "" {
			continue
		}
		if hit, result := e.lookupDerived(ctx, rend, opts, canonical, kind); hit {
			return result, true
		}
	}
	return RenderResult{}, false
}

// RenderCold encodes a frame. Callers must treat this as expensive: PNG read,
// decode, resize, and backend encode may all run. Prefer LookupCached first.
func (e *Engine) RenderCold(ctx context.Context, pokemon domain.Pokemon, cells int) RenderResult {
	noteCold()
	done := debuglog.Trace(debuglog.Render, "render.cache.miss")
	defer done()
	return e.renderAtQuality(ctx, pokemon, cells, "", "")
}

func (e *Engine) lookupAssetRefs(ctx context.Context, pokemon domain.Pokemon, native bool) []AssetRef {
	if native {
		ref := e.resolveNativeAssetRef(ctx, pokemon)
		if ref.Ready {
			return []AssetRef{ref}
		}
		return nil
	}
	tier := AssetTierHD
	if e.SourceKind == SourcePixel {
		tier = AssetTierPixel
	}
	if e.HDOff {
		tier = AssetTierPixel
	}
	ref := e.Source.ResolveAsset(ctx, pokemon, tier)
	if ref.Ready && ref.Hash != "" {
		return []AssetRef{ref}
	}
	return nil
}

// WarmRender forces the chafa backend at interactive (fast) quality for
// background warm. Full Mosaic prewarm is too expensive and unused on native
// terminals (browse skips chafa warm when NativeImage is set).
func (e *Engine) WarmRender(ctx context.Context, pokemon domain.Pokemon, cells int) RenderResult {
	return e.renderAtQuality(ctx, pokemon, cells, QualityInteractive, ModeChafa)
}

// NativeWarmRender encodes Kitty/iTerm/Sixel frames into the native artefact cache.
func (e *Engine) NativeWarmRender(ctx context.Context, pokemon domain.Pokemon, cells int) RenderResult {
	return e.renderAtQuality(ctx, pokemon, cells, QualityInteractive, ModeImage)
}

// PersistNativeFrame persists a native frame and blocks until stored (background warm path).
func (e *Engine) PersistNativeFrame(ctx context.Context, cacheKey, frame string) error {
	return e.PersistFrameWait(ctx, cacheKey, frame)
}

// HasNativeCache reports whether a native frame exists on disk for species at cells.
func (e *Engine) HasNativeCache(ctx context.Context, pokemon domain.Pokemon, cells int) bool {
	if e == nil || e.Artefacts == nil || e.Source == nil || pokemon.ID == 0 {
		return false
	}
	ref := e.resolveNativeAssetRef(ctx, pokemon)
	if !ref.Ready {
		return false
	}
	fw, fh := e.frameSize()
	if cells >= MinFrameSize {
		fw, fh = FrameSizeFor(cells)
	}
	bg := e.Background
	if bg.A == 0 {
		bg = DefaultBackground
	}
	canonical := ref.Hash
	if !IsHexSHA256(canonical) {
		canonical = ""
	}
	return e.hasNativeFrame(ctx, canonical, fw, fh, bg, e.SixelColors)
}

func (e *Engine) protocolName() string {
	if e == nil || e.Modes == nil {
		return ""
	}
	for _, rend := range e.Modes.Renderers {
		if namer, ok := rend.(protocolNamer); ok {
			if p := namer.ProtocolName(); p != "" {
				return p
			}
		}
	}
	return ""
}

func (e *Engine) renderAtQuality(ctx context.Context, pokemon domain.Pokemon, cells int, quality QualityProfile, forceMode RenderMode) RenderResult {
	if e == nil {
		return RenderResult{UsePlaceholder: true}
	}
	if err := ctx.Err(); err != nil {
		debuglog.Renderf("render.cancelled")
		NoteCancelled()
		return RenderResult{UsePlaceholder: true}
	}
	fw, fh := e.frameSize()
	if cells >= MinFrameSize {
		fw, fh = FrameSizeFor(cells)
	}
	label := displayName(pokemon)

	pref := e.Preference
	if forceMode != "" {
		pref = forceMode
	}
	if pref == ModeOff {
		return e.placeholderResult(label, fw, fh)
	}

	opts := e.renderOptions(label, fw, fh, "", quality)
	return e.renderChain(ctx, pokemon, opts, pokemon.ID, pref)
}

// RenderChafa forces the chafa text backend at the engine's configured quality.
func (e *Engine) RenderChafa(ctx context.Context, pokemon domain.Pokemon, cells int) RenderResult {
	return e.renderAtQuality(ctx, pokemon, cells, "", ModeChafa)
}

func (e *Engine) renderChain(ctx context.Context, pokemon domain.Pokemon, opts RenderOptions, pokemonID int, preference RenderMode) RenderResult {
	if e.Modes == nil || len(e.Modes.Renderers) == 0 {
		return e.placeholderResult(opts.Label, opts.Width, opts.Height)
	}
	if e.Source == nil {
		return e.placeholderResult(opts.Label, opts.Width, opts.Height)
	}

	start := e.Modes.Resolve(e.Caps, preference)
	started := false
	var (
		lastTier  AssetTier = -1
		img       image.Image
		hash      string
		assetKind string
		png       []byte
	)
	for _, rend := range e.Modes.Renderers {
		if !started {
			if start == nil || rend.Name() != start.Name() {
				continue
			}
			started = true
		}
		if !rend.Supported(e.Caps) && (start == nil || rend.Name() != start.Name()) {
			continue
		}
		if err := ctx.Err(); err != nil {
			debuglog.Renderf("render.cancelled pokemon=%d", pokemonID)
			NoteCancelled()
			return RenderResult{UsePlaceholder: true}
		}

		tier := AssetTierHD
		if e.SourceKind == SourcePixel {
			tier = AssetTierPixel
		}
		if e.HDOff {
			tier = AssetTierPixel
		}

		needResolve := hash == "" || img == nil
		if needResolve || (tier != lastTier && rend.Name() == "image") {
			var ref AssetRef
			if rend.Name() == "image" {
				ref = e.resolveNativeAssetRef(ctx, pokemon)
			} else {
				ref = e.Source.ResolveAsset(ctx, pokemon, tier)
			}
			if !ref.Ready || ref.Hash == "" {
				var res SourceResult
				if rend.Name() == "image" {
					res, _ = e.resolveNativeAsset(ctx, pokemon, pokemonID)
				} else {
					res = e.Source.Resolve(ctx, pokemon, tier)
				}
				if res.UsePlaceholder || (len(res.PNG) == 0 && res.ContentHash == "") {
					if rend.Name() == "image" {
						debuglog.Renderf("image asset miss pokemon=%d; falling back without disabling native", pokemonID)
						continue
					}
					return e.placeholderResult(opts.Label, opts.Width, opts.Height)
				}
				hash = res.ContentHash
				png = res.PNG
				assetKind = res.AssetKind
				ref.Hash = hash
				ref.Ready = true
				ref.Kind = assetKind
			} else {
				hash = ref.Hash
				assetKind = ref.Kind
				png = nil
			}
			lastTier = tier
			if assetKind == "" {
				assetKind = namedKindForTier(tier)
			}
			if rend.Name() == "image" {
				hash = e.ensureCanonicalHash(ctx, pokemon, ref, png)
			}
			opts.ContentHash = hash

			if hash != "" || (rend.Name() == "image" && ref.Ready) {
				if rend.Name() == "image" {
					canonical := hash
					if !IsHexSHA256(canonical) {
						canonical = ""
					}
					if hit, result := e.lookupNativeFrame(ctx, rend, opts, canonical, assetKind); hit {
						return result
					}
				} else if hash != "" {
					if hit, result := e.lookupDerived(ctx, rend, opts, hash, assetKind); hit {
						return result
					}
				}
			}

			if len(png) == 0 && ref.Ready {
				data, err := e.Source.ReadAsset(ctx, pokemon, ref)
				if err != nil || len(data) == 0 {
					if rend.Name() == "image" {
						debuglog.Renderf("image asset read miss pokemon=%d; falling back without disabling native", pokemonID)
						continue
					}
					return e.placeholderResult(opts.Label, opts.Width, opts.Height)
				}
				png = data
			}
			if len(png) == 0 {
				if rend.Name() == "image" {
					continue
				}
				return e.placeholderResult(opts.Label, opts.Width, opts.Height)
			}
			endDecode := debuglog.Trace(debuglog.Render, "png.decode")
			decoded, err := DecodeImage(png)
			endDecode()
			if err != nil {
				debuglog.Renderf("sprite decode fail pokemon=%d err=%v", pokemonID, err)
				if rend.Name() == "image" {
					continue
				}
				return e.placeholderResult(opts.Label, opts.Width, opts.Height)
			}
			img = decoded
		} else if hash != "" {
			if hit, result := e.lookupDerived(ctx, rend, opts, hash, assetKind); hit {
				return result
			}
		}

		key := ""
		switch {
		case rend.Name() == "image" && IsHexSHA256(opts.ContentHash):
			key = nativeCacheKey(opts.ContentHash, rend, opts.Width, opts.Height, opts.BackgroundColor, opts.SixelColors)
		case rend.Name() != "image" && rend.Name() != "placeholder" && opts.ContentHash != "":
			color := effectiveColor(opts.ColorMode, e.Caps)
			key = CacheKey(opts.ContentHash, opts.Source, rend.Name(), opts.Width, opts.Height, color, opts.BackgroundColor)
			opts.ColorMode = color
		}

		debuglog.Renderf("render.generate.start backend=%s pokemon=%d", rend.Name(), pokemonID)
		encodeName := "native.encode"
		switch rend.Name() {
		case "chafa":
			encodeName = "chafa"
		case "image":
			encodeName = "native.encode"
		default:
			encodeName = rend.Name() + ".encode"
		}
		endEncode := debuglog.Trace(debuglog.Render, encodeName)
		frame, err := rend.Render(ctx, img, opts)
		endEncode()
		if err != nil || frame.IsEmpty() {
			debuglog.Renderf("%s render fail pokemon=%d err=%v; falling back", rend.Name(), pokemonID, err)
			if rend.Name() == "image" && e.OnNativeFail != nil && !errors.Is(err, context.Canceled) && ctx.Err() == nil {
				e.OnNativeFail()
			}
			continue
		}
		if err := ctx.Err(); err != nil {
			debuglog.Renderf("render.cancelled backend=%s pokemon=%d", rend.Name(), pokemonID)
			NoteCancelled()
			return RenderResult{UsePlaceholder: true}
		}
		if key != "" {
			e.nativeMemory().Put(key, frame.Text)
			if e.Artefacts != nil && !frame.Native {
				if err := e.Artefacts.StoreArtefact(ctx, key, frame.Text); err != nil {
					debuglog.Renderf("render.store.artefact err=%v key=%s", err, key)
				}
			}
		}
		debuglog.Renderf("render.generate.complete backend=%s pokemon=%d frame=%dx%d key=%s kind=%s", rend.Name(), pokemonID, opts.Width, opts.Height, key, assetKind)
		if frame.Backend == "placeholder" {
			return RenderResult{ANSI: frame.Text, UsePlaceholder: true, Backend: "placeholder"}
		}
		return RenderResult{ANSI: frame.Text, CacheKey: key, Backend: frame.Backend, Native: frame.Native, AssetKind: assetKind}
	}
	return e.placeholderResult(opts.Label, opts.Width, opts.Height)
}

func (e *Engine) lookupDerived(ctx context.Context, rend Renderer, opts RenderOptions, hash, assetKind string) (bool, RenderResult) {
	if hash == "" {
		return false, RenderResult{}
	}
	if rend.Name() == "image" {
		return e.lookupNativeFrame(ctx, rend, opts, hash, assetKind)
	}
	if rend.Name() == "placeholder" {
		return false, RenderResult{}
	}
	color := effectiveColor(opts.ColorMode, e.Caps)
	key := CacheKey(hash, opts.Source, rend.Name(), opts.Width, opts.Height, color, opts.BackgroundColor)
	if cached, ok := e.ansiMemory().Get(key); ok {
		noteMemHit()
		debuglog.Renderf("render.cache.hit backend=%s store=mem key=%s", rend.Name(), key)
		return true, RenderResult{ANSI: cached, CacheKey: key, Backend: rend.Name(), AssetKind: assetKind}
	}
	if e.Artefacts != nil {
		if cached, err := e.Artefacts.Artefact(ctx, key); err == nil && cached != "" {
			noteDiskHit()
			e.ansiMemory().Put(key, cached)
			debuglog.Renderf("render.cache.hit backend=%s store=disk key=%s", rend.Name(), key)
			return true, RenderResult{ANSI: cached, CacheKey: key, Backend: rend.Name(), AssetKind: assetKind}
		}
	}
	debuglog.Renderf("render.cache.miss backend=%s key=%s", rend.Name(), key)
	return false, RenderResult{}
}

func (e *Engine) resolveNativeAssetRef(ctx context.Context, pokemon domain.Pokemon) AssetRef {
	tiers := []AssetTier{AssetTierHD, AssetTierPixel}
	if e.HDOff && e.SourceKind == SourcePixel {
		tiers = []AssetTier{AssetTierPixel}
	}
	for _, tier := range tiers {
		ref := e.Source.ResolveAsset(ctx, pokemon, tier)
		if ref.Ready {
			return ref
		}
	}
	return AssetRef{}
}

func (e *Engine) placeholderResult(label string, w, h int) RenderResult {
	if e.Modes != nil {
		for _, rend := range e.Modes.Renderers {
			if rend.Name() != "placeholder" {
				continue
			}
			frame, err := rend.Render(context.Background(), nil, RenderOptions{
				Width: w, Height: h, Label: label,
			})
			if err == nil && frame.Text != "" {
				return RenderResult{ANSI: frame.Text, UsePlaceholder: true, Backend: "placeholder"}
			}
		}
	}
	return RenderResult{UsePlaceholder: true, Backend: "placeholder"}
}

func (e *Engine) renderOptions(label string, fw, fh int, hash string, quality QualityProfile) RenderOptions {
	bg := e.Background
	if bg.A == 0 {
		bg = DefaultBackground
	}
	source := e.SourceKind
	if source == "" {
		source = SourceArtwork
	}
	color := e.Color
	if color == "" {
		color = ColorAuto
	}
	if quality == "" {
		quality = e.Quality
	}
	if quality == "" {
		quality = QualityInteractive
	}
	return RenderOptions{
		Width:           fw,
		Height:          fh,
		ColorMode:       color,
		BackgroundColor: bg,
		AlphaThreshold:  16,
		Source:          source,
		Label:           label,
		ContentHash:     hash,
		Quality:         quality,
		SixelColors:     e.SixelColors,
	}
}

func (e *Engine) frameSize() (w, h int) {
	w, h = e.FrameW, e.FrameH
	if w < 1 {
		w = FrameWidth
	}
	if h < 1 {
		h = FrameHeight
	}
	return w, h
}

func effectiveColor(mode ColorMode, caps Capabilities) ColorMode {
	switch mode {
	case ColorTrueColor:
		if caps.TrueColor {
			return ColorTrueColor
		}
		return Color256
	case Color256:
		return Color256
	case ColorMono:
		return ColorMono
	default: // auto
		if caps.TrueColor {
			return ColorTrueColor
		}
		return Color256
	}
}

func displayName(p domain.Pokemon) string {
	if p.DisplayName != "" {
		return p.DisplayName
	}
	if p.Slug != "" {
		return p.Slug
	}
	return "???"
}

// resolveNativeAsset prefers official artwork, then silent pixel CDN fallback.
func (e *Engine) resolveNativeAsset(ctx context.Context, pokemon domain.Pokemon, pokemonID int) (SourceResult, AssetTier) {
	tiers := []AssetTier{AssetTierHD, AssetTierPixel}
	if e.HDOff && e.SourceKind == SourcePixel {
		tiers = []AssetTier{AssetTierPixel}
	}
	var last SourceResult
	for _, tier := range tiers {
		res := e.Source.Resolve(ctx, pokemon, tier)
		if !res.UsePlaceholder && len(res.PNG) > 0 {
			if tier == AssetTierPixel {
				debuglog.Renderf("image using pixel asset pokemon=%d (hd unavailable)", pokemonID)
			}
			return res, tier
		}
		last = res
		debuglog.Renderf("image asset tier miss pokemon=%d tier=%d", pokemonID, int(tier))
	}
	return last, AssetTierHD
}

// nativeCacheKey builds a mem+disk key for Kitty/iTerm/Sixel frames (bg + version + palette).
func nativeCacheKey(contentHash string, rend Renderer, w, h int, bg color.RGBA, sixelColors int) string {
	proto := ""
	if namer, ok := rend.(protocolNamer); ok {
		proto = namer.ProtocolName()
	}
	return NativeCacheKey(contentHash, proto, w, h, bg, sixelColors)
}
