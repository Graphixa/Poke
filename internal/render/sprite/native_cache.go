package sprite

import (
	"context"
	"fmt"
	"image/color"

	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/domain"
)

// IsHexSHA256 reports whether s is a 64-char lowercase/uppercase hex SHA-256 digest.
func IsHexSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}

func (e *Engine) lookupNativeFrame(ctx context.Context, rend Renderer, opts RenderOptions, canonicalHash, assetKind string) (bool, RenderResult) {
	if rend == nil || rend.Name() != "image" {
		return false, RenderResult{}
	}
	if !IsHexSHA256(canonicalHash) {
		debuglog.Renderf("render.cache.miss backend=%s (no canonical hash)", rend.Name())
		return false, RenderResult{}
	}
	colors := EffectiveSixelColors(opts.SixelColors)
	key := nativeCacheKey(canonicalHash, rend, opts.Width, opts.Height, opts.BackgroundColor, colors)
	if cached, ok := e.nativeMemory().Get(key); ok {
		noteMemHit()
		debuglog.Renderf("render.cache.hit backend=%s store=mem key=%s", rend.Name(), key)
		return true, RenderResult{ANSI: cached, CacheKey: key, Backend: "image", Native: true, AssetKind: assetKind}
	}
	if e.Artefacts != nil {
		if cached, err := e.Artefacts.Native(ctx, key); err == nil && cached != "" {
			noteDiskHit()
			e.nativeMemory().Put(key, cached)
			debuglog.Renderf("render.cache.hit backend=%s store=disk key=%s", rend.Name(), key)
			return true, RenderResult{ANSI: cached, CacheKey: key, Backend: "image", Native: true, AssetKind: assetKind}
		}
	}
	debuglog.Renderf("render.cache.miss backend=%s key=%s", rend.Name(), key)
	return false, RenderResult{}
}

type nativeFileStore interface {
	HasNativeFile(key string) bool
}

func (e *Engine) hasNativeFrame(ctx context.Context, canonicalHash string, fw, fh int, bg color.RGBA, sixelColors int) bool {
	if e == nil || !IsHexSHA256(canonicalHash) {
		return false
	}
	proto := e.protocolName()
	key := NativeCacheKey(canonicalHash, proto, fw, fh, bg, sixelColors)
	if _, ok := e.nativeMemory().Get(key); ok {
		return true
	}
	if e.Artefacts == nil {
		return false
	}
	frame, err := e.Artefacts.Native(ctx, key)
	return err == nil && frame != ""
}

func (e *Engine) hasNativeFile(canonicalHash string, fw, fh int, bg color.RGBA, sixelColors int) bool {
	if e == nil || !IsHexSHA256(canonicalHash) {
		return false
	}
	if e.Artefacts == nil {
		return false
	}
	checker, ok := e.Artefacts.(nativeFileStore)
	if !ok {
		return false
	}
	proto := e.protocolName()
	key := NativeCacheKey(canonicalHash, proto, fw, fh, bg, sixelColors)
	return checker.HasNativeFile(key)
}

// NativeCacheSpecKey identifies the active native render specification for progress.
func NativeCacheSpecKey(cells int) string {
	return fmt.Sprintf("native:%d", cells)
}

func (e *Engine) ensureCanonicalHash(ctx context.Context, pokemon domain.Pokemon, ref AssetRef, png []byte) string {
	if IsHexSHA256(ref.Hash) {
		return ref.Hash
	}
	if len(png) == 0 && ref.Ready && e.Source != nil {
		data, err := e.Source.ReadAsset(ctx, pokemon, ref)
		if err == nil {
			png = data
		}
	}
	if len(png) == 0 {
		return ""
	}
	hash := ContentHash(png)
	kind := ref.Kind
	if kind == "" {
		kind = namedKindForTier(AssetTierHD)
		if ref.Source == SourcePixel {
			kind = namedKindForTier(AssetTierPixel)
		}
	}
	if e.Source != nil && e.Source.Store != nil && pokemon.ID != 0 {
		_ = e.Source.Store.StoreNamedSpriteWithURL(ctx, pokemon.ID, pokemon.Slug, kind, ref.URL, hash, png)
	}
	return hash
}
