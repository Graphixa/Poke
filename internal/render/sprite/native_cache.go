package sprite

import (
	"context"
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

// LegacyNativeHash is the pre-SHA256 cache identity used only for read fallback.
func LegacyNativeHash(id int, slug, kind string) string {
	return NamedAssetKey(id, slug, kind)
}

func (e *Engine) legacyHashForRef(pokemon domain.Pokemon, ref AssetRef) string {
	kind := ref.Kind
	if kind == "" {
		kind = namedKindForTier(AssetTierHD)
		if ref.Source == SourcePixel {
			kind = namedKindForTier(AssetTierPixel)
		}
	}
	return LegacyNativeHash(pokemon.ID, pokemon.Slug, kind)
}

func (e *Engine) lookupNativeFrame(ctx context.Context, rend Renderer, opts RenderOptions, canonicalHash, legacyHash, assetKind string, migrate bool) (bool, RenderResult) {
	if rend == nil || rend.Name() != "image" {
		return false, RenderResult{}
	}
	try := func(hash string) (string, string, bool) {
		if hash == "" {
			return "", "", false
		}
		key := nativeCacheKey(hash, rend, opts.Width, opts.Height, opts.BackgroundColor)
		if cached, ok := e.memory().Get(key); ok {
			noteMemHit()
			debuglog.Renderf("render.cache.hit backend=%s store=mem key=%s", rend.Name(), key)
			return key, cached, true
		}
		if e.Artefacts != nil {
			if cached, err := e.Artefacts.Native(ctx, key); err == nil && cached != "" {
				noteDiskHit()
				e.memory().Put(key, cached)
				debuglog.Renderf("render.cache.hit backend=%s store=disk key=%s", rend.Name(), key)
				return key, cached, true
			}
		}
		return key, "", false
	}

	if IsHexSHA256(canonicalHash) {
		if key, cached, ok := try(canonicalHash); ok {
			return true, RenderResult{ANSI: cached, CacheKey: key, Backend: "image", Native: true, AssetKind: assetKind}
		}
	}
	if legacyHash != "" && legacyHash != canonicalHash {
		if key, cached, ok := try(legacyHash); ok {
			outKey := key
			if migrate && IsHexSHA256(canonicalHash) {
				outKey = nativeCacheKey(canonicalHash, rend, opts.Width, opts.Height, opts.BackgroundColor)
				e.memory().Put(outKey, cached)
				if e.Artefacts != nil {
					_ = e.Artefacts.StoreNative(ctx, outKey, cached)
				}
			}
			return true, RenderResult{ANSI: cached, CacheKey: outKey, Backend: "image", Native: true, AssetKind: assetKind}
		}
	}
	if IsHexSHA256(canonicalHash) {
		key := nativeCacheKey(canonicalHash, rend, opts.Width, opts.Height, opts.BackgroundColor)
		debuglog.Renderf("render.cache.miss backend=%s key=%s", rend.Name(), key)
	} else if legacyHash != "" {
		key := nativeCacheKey(legacyHash, rend, opts.Width, opts.Height, opts.BackgroundColor)
		debuglog.Renderf("render.cache.miss backend=%s key=%s", rend.Name(), key)
	}
	return false, RenderResult{}
}

func (e *Engine) hasNativeFrame(ctx context.Context, canonicalHash, legacyHash string, fw, fh int, bg color.RGBA) bool {
	if e == nil || e.Artefacts == nil {
		return false
	}
	proto := e.protocolName()
	if IsHexSHA256(canonicalHash) {
		key := NativeCacheKey(canonicalHash, proto, fw, fh, bg)
		if frame, err := e.Artefacts.Native(ctx, key); err == nil && frame != "" {
			return true
		}
	}
	if legacyHash != "" && legacyHash != canonicalHash {
		key := NativeCacheKey(legacyHash, proto, fw, fh, bg)
		if frame, err := e.Artefacts.Native(ctx, key); err == nil && frame != "" {
			return true
		}
	}
	return false
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
		if urlStore, ok := e.Source.Store.(NamedPNGStoreWithURL); ok {
			_ = urlStore.StoreNamedSpriteWithURL(ctx, pokemon.ID, pokemon.Slug, kind, ref.URL, hash, png)
		} else if named, ok := e.Source.Store.(NamedPNGStore); ok {
			_ = named.StoreNamedSprite(ctx, pokemon.ID, pokemon.Slug, kind, hash, png)
		}
	}
	return hash
}
