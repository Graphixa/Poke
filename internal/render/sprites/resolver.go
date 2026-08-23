package sprites

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

// PNGStore persists sprite PNG bytes by content hash and URL mapping.
type PNGStore interface {
	SpriteByURL(ctx context.Context, url string) (contentHash string, data []byte, err error)
	StoreSpriteByURL(ctx context.Context, url, contentHash string, data []byte) error
}

// Downloader fetches raw sprite bytes from a URL.
type Downloader interface {
	GetSprite(ctx context.Context, spriteURL string) (io.ReadCloser, error)
}

// Resolver selects front_default, downloads through the cache, and validates PNG.
type Resolver struct {
	Store PNGStore
	API   Downloader
}

// Result is either validated PNG bytes or a signal to use the name placeholder.
type Result struct {
	ContentHash    string
	PNG            []byte
	UsePlaceholder bool
}

// Resolve returns front_default PNG bytes, or UsePlaceholder when missing/invalid.
func (r *Resolver) Resolve(ctx context.Context, pokemon domain.Pokemon) Result {
	url := pokemon.Sprites.FrontDefault
	if url == "" {
		return Result{UsePlaceholder: true}
	}
	if r.Store != nil {
		hash, data, err := r.Store.SpriteByURL(ctx, url)
		if err == nil && len(data) > 0 {
			if ValidatePNG(data) == nil {
				return Result{ContentHash: hash, PNG: data}
			}
		}
	}
	if r.API == nil {
		return Result{UsePlaceholder: true}
	}
	data, err := download(ctx, r.API, url)
	if err != nil || len(data) == 0 {
		return Result{UsePlaceholder: true}
	}
	if err := ValidatePNG(data); err != nil {
		return Result{UsePlaceholder: true}
	}
	hash := ContentHash(data)
	if r.Store != nil {
		_ = r.Store.StoreSpriteByURL(ctx, url, hash, data)
	}
	return Result{ContentHash: hash, PNG: data}
}

// ValidatePNG reports whether data is a decodable PNG image.
func ValidatePNG(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("sprites: empty png")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("sprites: invalid png: %w", err)
	}
	if format != "png" {
		return fmt.Errorf("sprites: not png (%s)", format)
	}
	if cfg.Width < 1 || cfg.Height < 1 {
		return fmt.Errorf("sprites: empty dimensions")
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
