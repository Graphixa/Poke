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

	"github.com/Graphixa/Pokedex/internal/domain"
)

// AssetTier selects which sprite URL family to prefer.
type AssetTier int

const (
	// AssetTierPixel uses front_default (small game sprites) for chafa.
	AssetTierPixel AssetTier = iota
	// AssetTierHD prefers official artwork, then home, then front_default.
	AssetTierHD
)

// PNGStore persists sprite PNG bytes by content hash and URL mapping.
type PNGStore interface {
	SpriteByURL(ctx context.Context, url string) (contentHash string, data []byte, err error)
	StoreSpriteByURL(ctx context.Context, url, contentHash string, data []byte) error
}

// NamedPNGStore is the preferred on-disk layout: sprites/<id-slug>/ and official-artwork/.
type NamedPNGStore interface {
	NamedSprite(ctx context.Context, id int, slug, kind string) ([]byte, error)
	StoreNamedSprite(ctx context.Context, id int, slug, kind, contentHash string, data []byte) error
}

// Downloader fetches raw sprite bytes from a URL.
type Downloader interface {
	GetSprite(ctx context.Context, spriteURL string) (io.ReadCloser, error)
}

// SourceResolver selects a sprite URL by tier, downloads through the cache, and validates PNG.
type SourceResolver struct {
	Store PNGStore
	API   Downloader
}

// SourceResult is either validated PNG bytes or a signal to use the placeholder.
type SourceResult struct {
	ContentHash    string
	PNG            []byte
	URL            string
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

	if named, ok := r.Store.(NamedPNGStore); ok && pokemon.ID != 0 {
		if data, err := named.NamedSprite(ctx, pokemon.ID, pokemon.Slug, kind); err == nil && len(data) > 0 {
			if ValidatePNG(data) == nil {
				hash := ContentHash(data)
				return SourceResult{ContentHash: hash, PNG: data, URL: URLForTier(pokemon, t)}
			}
		}
	}

	url := URLForTier(pokemon, t)
	if url == "" {
		return SourceResult{UsePlaceholder: true}
	}
	if r.Store != nil {
		hash, data, err := r.Store.SpriteByURL(ctx, url)
		if err == nil && len(data) > 0 {
			if ValidatePNG(data) == nil {
				if named, ok := r.Store.(NamedPNGStore); ok && pokemon.ID != 0 {
					_ = named.StoreNamedSprite(ctx, pokemon.ID, pokemon.Slug, kind, hash, data)
				}
				return SourceResult{ContentHash: hash, PNG: data, URL: url}
			}
		}
	}
	if r.API == nil {
		return SourceResult{UsePlaceholder: true}
	}
	data, err := download(ctx, r.API, url)
	if err != nil || len(data) == 0 {
		return SourceResult{UsePlaceholder: true}
	}
	if err := ValidatePNG(data); err != nil {
		return SourceResult{UsePlaceholder: true}
	}
	hash := ContentHash(data)
	if r.Store != nil {
		_ = r.Store.StoreSpriteByURL(ctx, url, hash, data)
		if named, ok := r.Store.(NamedPNGStore); ok && pokemon.ID != 0 {
			_ = named.StoreNamedSprite(ctx, pokemon.ID, pokemon.Slug, kind, hash, data)
		}
	}
	return SourceResult{ContentHash: hash, PNG: data, URL: url}
}

func namedKindForTier(tier AssetTier) string {
	if tier == AssetTierHD {
		return "artwork"
	}
	return "pixel"
}

// URLForTier picks the best sprite URL for the given asset tier.
func URLForTier(pokemon domain.Pokemon, tier AssetTier) string {
	switch tier {
	case AssetTierHD:
		if pokemon.Sprites.OfficialArtwork != "" {
			return pokemon.Sprites.OfficialArtwork
		}
		if pokemon.Sprites.HomeFront != "" {
			return pokemon.Sprites.HomeFront
		}
		return pokemon.Sprites.FrontDefault
	default:
		return pokemon.Sprites.FrontDefault
	}
}

// TierForRenderer returns the asset tier for a logical renderer class.
func TierForRenderer(name string) AssetTier {
	if name == "image" {
		return AssetTierHD
	}
	return AssetTierPixel
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
