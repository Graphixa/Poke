package sprite

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"strings"

	"github.com/Graphixa/Poke/internal/domain"
)

// AssetTier selects which sprite URL family to prefer.
type AssetTier int

const (
	// AssetTierPixel uses front_default (small game sprites).
	AssetTierPixel AssetTier = iota
	// AssetTierHD prefers official artwork CDN, then silent Home/pixel fallbacks.
	AssetTierHD
)

// AssetStore persists sprite PNGs (hash, named layout, metadata, and path reads).
type AssetStore interface {
	SpriteByURL(ctx context.Context, url string) (contentHash string, data []byte, err error)
	StoreSpriteByURL(ctx context.Context, url, contentHash string, data []byte) error
	NamedSprite(ctx context.Context, id int, slug, kind string) ([]byte, error)
	StoreNamedSprite(ctx context.Context, id int, slug, kind, contentHash string, data []byte) error
	StoreNamedSpriteWithURL(ctx context.Context, id int, slug, kind, url, sha256 string, data []byte) error
	NamedAssetMeta(ctx context.Context, id int, slug, kind string) (hash, path string, ready bool, err error)
	ReadAssetBytes(ctx context.Context, path string) ([]byte, error)
}

// Downloader fetches raw sprite bytes from a URL.
type Downloader interface {
	GetSprite(ctx context.Context, spriteURL string) (io.ReadCloser, error)
}

// SourceResolver selects a sprite URL by tier, downloads through the cache, and validates PNG.
type SourceResolver struct {
	Store AssetStore
	API   Downloader
}

// SourceResult is either validated PNG bytes or a signal to use the placeholder.
type SourceResult struct {
	ContentHash    string
	PNG            []byte
	URL            string
	AssetKind      string // "artwork" | "pixel"
	UsePlaceholder bool
}

// Resolve returns PNG bytes for the given tier, or UsePlaceholder when missing/invalid.
// When tier is omitted (zero value), AssetTierPixel is used.
func (r *SourceResolver) Resolve(ctx context.Context, pokemon domain.Pokemon, tier ...AssetTier) SourceResult {
	t := AssetTierPixel
	if len(tier) > 0 {
		t = tier[0]
	}
	kind := namedKindForTier(t)

	if r.Store != nil && pokemon.ID != 0 {
		if data, err := r.Store.NamedSprite(ctx, pokemon.ID, pokemon.Slug, kind); err == nil && len(data) > 0 {
			notePNGRead()
			hash := ContentHash(data)
			if h, _, ready, _ := r.Store.NamedAssetMeta(ctx, pokemon.ID, pokemon.Slug, kind); ready && IsHexSHA256(h) {
				hash = h
			}
			url := URLForTier(pokemon, t)
			if url == "" {
				url = fallbackCDNURL(pokemon.ID, t)
			}
			return SourceResult{ContentHash: hash, PNG: data, URL: url, AssetKind: kind}
		}
	}

	urls := resolveURLs(pokemon, t)
	if len(urls) == 0 {
		return SourceResult{UsePlaceholder: true}
	}
	for _, tryURL := range urls {
		storeKind := storeKindForURL(t, tryURL, pokemon)
		if r.Store != nil {
			hash, data, err := r.Store.SpriteByURL(ctx, tryURL)
			if err == nil && len(data) > 0 && ValidatePNG(data) == nil {
				notePNGRead()
				if pokemon.ID != 0 {
					_ = r.Store.StoreNamedSprite(ctx, pokemon.ID, pokemon.Slug, storeKind, hash, data)
					_ = r.Store.StoreNamedSpriteWithURL(ctx, pokemon.ID, pokemon.Slug, storeKind, tryURL, hash, data)
				}
				return SourceResult{ContentHash: hash, PNG: data, URL: tryURL, AssetKind: storeKind}
			}
		}
		if r.API == nil {
			continue
		}
		data, err := downloadDeduped(ctx, r.API, tryURL)
		if err != nil || len(data) == 0 || ValidatePNG(data) != nil {
			continue
		}
		hash := ContentHash(data)
		if r.Store != nil {
			if pokemon.ID != 0 {
				_ = r.Store.StoreNamedSprite(ctx, pokemon.ID, pokemon.Slug, storeKind, hash, data)
				_ = r.Store.StoreNamedSpriteWithURL(ctx, pokemon.ID, pokemon.Slug, storeKind, tryURL, hash, data)
			} else {
				_ = r.Store.StoreSpriteByURL(ctx, tryURL, hash, data)
			}
		}
		return SourceResult{ContentHash: hash, PNG: data, URL: tryURL, AssetKind: storeKind}
	}
	return SourceResult{UsePlaceholder: true}
}

// resolveURLs lists preferred then silent fallback URLs for a tier.
func resolveURLs(pokemon domain.Pokemon, tier AssetTier) []string {
	urls := make([]string, 0, 5)
	seen := map[string]struct{}{}
	add := func(u string) {
		if u == "" {
			return
		}
		if _, ok := seen[u]; ok {
			return
		}
		seen[u] = struct{}{}
		urls = append(urls, u)
	}
	if tier == AssetTierHD {
		add(pokemon.Sprites.OfficialArtwork)
		add(CDNArtworkURL(pokemon.ID))
		add(pokemon.Sprites.HomeFront)
		add(pokemon.Sprites.FrontDefault)
		add(CDNPixelURL(pokemon.ID))
		return urls
	}
	add(URLForTier(pokemon, tier))
	add(fallbackCDNURL(pokemon.ID, tier))
	return urls
}

// storeKindForURL chooses the named on-disk kind. Pixel fallbacks under HD must
// never poison the official-artwork tree.
func storeKindForURL(tier AssetTier, url string, pokemon domain.Pokemon) string {
	if tier != AssetTierHD {
		return "pixel"
	}
	if isArtworkURL(url, pokemon) {
		return "artwork"
	}
	return "pixel"
}

func isArtworkURL(url string, pokemon domain.Pokemon) bool {
	if url == "" {
		return false
	}
	if pokemon.Sprites.OfficialArtwork != "" && url == pokemon.Sprites.OfficialArtwork {
		return true
	}
	if art := CDNArtworkURL(pokemon.ID); art != "" && url == art {
		return true
	}
	// Home fronts are HD-class; keep under artwork only when explicitly home URL.
	if pokemon.Sprites.HomeFront != "" && url == pokemon.Sprites.HomeFront {
		return true
	}
	return strings.Contains(url, "/official-artwork/")
}

func fallbackCDNURL(id int, tier AssetTier) string {
	if tier == AssetTierHD {
		return CDNArtworkURL(id)
	}
	return CDNPixelURL(id)
}

func namedKindForTier(tier AssetTier) string {
	if tier == AssetTierHD {
		return "artwork"
	}
	return "pixel"
}

// URLForTier picks the best sprite URL for the given asset tier.
// HD: OfficialArtwork, else CDN artwork (never FrontDefault ahead of CDN).
func URLForTier(pokemon domain.Pokemon, tier AssetTier) string {
	switch tier {
	case AssetTierHD:
		if pokemon.Sprites.OfficialArtwork != "" {
			return pokemon.Sprites.OfficialArtwork
		}
		return CDNArtworkURL(pokemon.ID)
	default:
		if pokemon.Sprites.FrontDefault != "" {
			return pokemon.Sprites.FrontDefault
		}
		return CDNPixelURL(pokemon.ID)
	}
}

// TierForRenderer returns the asset tier for a logical renderer class.
// Product path uses official artwork for both native and chafa.
func TierForRenderer(name string) AssetTier {
	_ = name
	return AssetTierHD
}

// ValidatePNG reports whether data is a decodable PNG image.
func ValidatePNG(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("sprite: empty png")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("sprite: invalid png: %w", err)
	}
	if format != "png" {
		return fmt.Errorf("sprite: not png (%s)", format)
	}
	if cfg.Width < 1 || cfg.Height < 1 {
		return fmt.Errorf("sprite: empty dimensions")
	}
	return nil
}

// ContentHash returns the hex SHA-256 of data.
func ContentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func download(ctx context.Context, api Downloader, url string) ([]byte, error) {
	rc, err := api.GetSprite(ctx, url)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
