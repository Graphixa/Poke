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

// ArtefactCache stores derived halfblock ANSI and native graphics frames.
type ArtefactCache interface {
	Halfblock(ctx context.Context, key string) (string, error)
	StoreHalfblock(ctx context.Context, key, ansi string) error
	Native(ctx context.Context, key string) (string, error)
	StoreNative(ctx context.Context, key, frame string) error
}

// ClearNativeFunc clears prior native image placements (best-effort).
type ClearNativeFunc func()

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
	// ClearNativeFn clears native placements; optional.
	ClearNativeFn ClearNativeFunc
	// OnNativeFail is called when native image rendering fails (session fallback).
	OnNativeFail func()

	memOnce sync.Once
	mem     *ansiMemCache
}

func (e *Engine) memory() *ansiMemCache {
	if e == nil {
		return nil
	}
	e.memOnce.Do(func() {
		e.mem = newANSIMemCache(512)
	})
	return e.mem
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

// InvalidateSpecies drops mem-cached frames for a species so artwork pop-in
// re-encodes after named PNGs are overwritten on disk.
func (e *Engine) InvalidateSpecies(speciesID int) {
	if e == nil || speciesID < 1 {
		return
	}
	// NamedAssetKey format: named-{id}-{slug}-{kind}; match on named-{id}-
	e.memory().InvalidatePrefix(fmt.Sprintf("named-%d-", speciesID))
}

// InvalidateSpeciesNative drops mem + disk native frames for a species.
func (e *Engine) InvalidateSpeciesNative(ctx context.Context, speciesID int) {
	e.InvalidateSpecies(speciesID)
	if e == nil || speciesID < 1 {
		return
	}
	if nativeStore, ok := e.Artefacts.(NativeSpeciesStore); ok {
		_ = nativeStore.InvalidateNativeSpecies(speciesID)
	}
}

// NativeSpeciesStore deletes on-disk native frames for a species.
type NativeSpeciesStore interface {
	InvalidateNativeSpecies(speciesID int) error
}

// Render converts a sprite into a frame. cells is the square edge requested by
// the TUI (bucketed); when ≤0 the engine's configured/default frame size is used.
func (e *Engine) Render(ctx context.Context, pokemon domain.Pokemon, cells int) RenderResult {
	return e.renderAtQuality(ctx, pokemon, cells, "", "")
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

// HasNativeCache reports whether a native frame exists on disk for species at cells.
func (e *Engine) HasNativeCache(ctx context.Context, pokemon domain.Pokemon, cells int) bool {
	if e == nil || e.Artefacts == nil || e.Source == nil || pokemon.ID == 0 {
		return false
	}
	res, _ := e.resolveNativeAsset(ctx, pokemon, pokemon.ID)
	if res.UsePlaceholder || len(res.PNG) == 0 || res.ContentHash == "" {
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
	key := NativeCacheKey(res.ContentHash, e.protocolName(), fw, fh, bg)
	frame, err := e.Artefacts.Native(ctx, key)
	return err == nil && frame != ""
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
			return RenderResult{UsePlaceholder: true}
		}

		tier := AssetTierHD
		if e.SourceKind == SourcePixel {
			tier = AssetTierPixel
		}
		if rend.Name() == "image" {
			// Native always prefers HD with silent pixel fallback inside resolveNativeAsset.
		} else if e.HDOff {
			tier = AssetTierPixel
		}
		// Reuse decoded PNG from a prior native attempt when falling to chafa
		// even if the preferred tier label differs (avoids double Resolve).
		needResolve := img == nil
		if needResolve || (tier != lastTier && rend.Name() == "image") {
			var (
				res       SourceResult
				usedTier  AssetTier
			)
			if rend.Name() == "image" {
				res, usedTier = e.resolveNativeAsset(ctx, pokemon, pokemonID)
			} else {
				res = e.Source.Resolve(ctx, pokemon, tier)
				usedTier = tier
			}
			if res.UsePlaceholder || len(res.PNG) == 0 {
				if rend.Name() == "image" {
					// Missing sprite bytes are not a protocol failure — keep
					// native enabled and fall through to chafa for this Pokémon.
					debuglog.Renderf("image asset miss pokemon=%d; falling back without disabling native", pokemonID)
					continue
				}
				return e.placeholderResult(opts.Label, opts.Width, opts.Height)
			}
			hash = res.ContentHash
			lastTier = usedTier
			assetKind = res.AssetKind
			if assetKind == "" {
				assetKind = namedKindForTier(usedTier)
			}
			opts.ContentHash = hash

			// Mem → disk lookup before decode — avoids PNG decode on hit.
			if hash != "" {
				if rend.Name() == "image" {
					key := nativeCacheKey(hash, rend, opts.Width, opts.Height, opts.BackgroundColor)
					if cached, ok := e.memory().Get(key); ok {
						debuglog.Renderf("%s mem cache hit key=%s frame=%dx%d", rend.Name(), key, opts.Width, opts.Height)
						return RenderResult{ANSI: cached, CacheKey: key, Backend: "image", Native: true, AssetKind: assetKind}
					}
					if e.Artefacts != nil {
						if cached, err := e.Artefacts.Native(ctx, key); err == nil && cached != "" {
							e.memory().Put(key, cached)
							debuglog.Renderf("%s disk cache hit key=%s frame=%dx%d", rend.Name(), key, opts.Width, opts.Height)
							return RenderResult{ANSI: cached, CacheKey: key, Backend: "image", Native: true, AssetKind: assetKind}
						}
					}
				} else if rend.Name() != "placeholder" {
					color := effectiveColor(opts.ColorMode, e.Caps)
					key := CacheKey(hash, opts.Source, rend.Name(), opts.Width, opts.Height, color, opts.BackgroundColor)
					if cached, ok := e.memory().Get(key); ok {
						debuglog.Renderf("%s mem cache hit key=%s frame=%dx%d", rend.Name(), key, opts.Width, opts.Height)
						return RenderResult{ANSI: cached, CacheKey: key, Backend: rend.Name(), AssetKind: assetKind}
					}
					if e.Artefacts != nil {
						if cached, err := e.Artefacts.Halfblock(ctx, key); err == nil && cached != "" {
							e.memory().Put(key, cached)
							debuglog.Renderf("%s cache hit key=%s frame=%dx%d", rend.Name(), key, opts.Width, opts.Height)
							return RenderResult{ANSI: cached, CacheKey: key, Backend: rend.Name(), AssetKind: assetKind}
						}
					}
					opts.ColorMode = color
				}
			}

			decoded, err := DecodeImage(res.PNG)
			if err != nil {
				debuglog.Renderf("sprite decode fail pokemon=%d err=%v", pokemonID, err)
				if rend.Name() == "image" {
					debuglog.Renderf("image decode miss pokemon=%d; falling back without disabling native", pokemonID)
					continue
				}
				return e.placeholderResult(opts.Label, opts.Width, opts.Height)
			}
			img = decoded
		}

		key := ""
		switch {
		case rend.Name() == "image" && opts.ContentHash != "":
			key = nativeCacheKey(opts.ContentHash, rend, opts.Width, opts.Height, opts.BackgroundColor)
		case rend.Name() != "image" && rend.Name() != "placeholder" && opts.ContentHash != "":
			color := effectiveColor(opts.ColorMode, e.Caps)
			key = CacheKey(opts.ContentHash, opts.Source, rend.Name(), opts.Width, opts.Height, color, opts.BackgroundColor)
			opts.ColorMode = color
		}

		frame, err := rend.Render(ctx, img, opts)
		if err != nil || frame.IsEmpty() {
			debuglog.Renderf("%s render fail pokemon=%d err=%v; falling back", rend.Name(), pokemonID, err)
			// Selection changes cancel in-flight encodes — that is not a protocol
			// failure. Only disable native for real backend errors.
			if rend.Name() == "image" && e.OnNativeFail != nil && !errors.Is(err, context.Canceled) && ctx.Err() == nil {
				e.OnNativeFail()
			}
			continue
		}
		if err := ctx.Err(); err != nil {
			return RenderResult{UsePlaceholder: true}
		}
		if key != "" {
			e.memory().Put(key, frame.Text)
			if e.Artefacts != nil {
				if frame.Native {
					_ = e.Artefacts.StoreNative(ctx, key, frame.Text)
				} else {
					_ = e.Artefacts.StoreHalfblock(ctx, key, frame.Text)
				}
			}
		}
		debuglog.Renderf("%s render pokemon=%d frame=%dx%d key=%s kind=%s", rend.Name(), pokemonID, opts.Width, opts.Height, key, assetKind)
		if frame.Backend == "placeholder" {
			return RenderResult{ANSI: frame.Text, UsePlaceholder: true, Backend: "placeholder"}
		}
		return RenderResult{ANSI: frame.Text, CacheKey: key, Backend: frame.Backend, Native: frame.Native, AssetKind: assetKind}
	}
	return e.placeholderResult(opts.Label, opts.Width, opts.Height)
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

// nativeCacheKey builds a mem+disk key for Kitty/iTerm/Sixel frames (bg + version).
func nativeCacheKey(contentHash string, rend Renderer, w, h int, bg color.RGBA) string {
	proto := ""
	if namer, ok := rend.(protocolNamer); ok {
		proto = namer.ProtocolName()
	}
	return NativeCacheKey(contentHash, proto, w, h, bg)
}
