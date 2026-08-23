package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/Graphixa/Pokedex/internal/render/halfblock"
)

// SpriteByURL returns cached PNG bytes for a sprite URL when present.
func (s *Service) SpriteByURL(ctx context.Context, url string) (contentHash string, data []byte, err error) {
	urlHash := sha256Hex(url)
	contentHash, err = s.store.SpriteURLLookup(ctx, urlHash)
	if err != nil {
		return "", nil, err
	}
	data, err = s.store.Sprite(ctx, contentHash)
	if err != nil {
		return "", nil, err
	}
	return contentHash, data, nil
}

// StoreSpriteByURL persists PNG bytes under their content hash and maps the URL.
func (s *Service) StoreSpriteByURL(ctx context.Context, url, contentHash string, data []byte) error {
	if contentHash == "" || len(data) == 0 {
		return errors.New("cache: refuse to store empty sprite")
	}
	if err := s.store.StoreSprite(ctx, contentHash, data); err != nil {
		return err
	}
	return s.store.StoreSpriteURL(ctx, sha256Hex(url), contentHash)
}

// Halfblock returns a cached half-block artefact by key.
func (s *Service) Halfblock(ctx context.Context, key string) (string, error) {
	data, err := s.store.Halfblock(ctx, key)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// StoreHalfblock persists a derived half-block artefact.
func (s *Service) StoreHalfblock(ctx context.Context, key, ansi string) error {
	if key == "" {
		return errors.New("cache: refuse to store empty halfblock key")
	}
	// Validate key shape so callers cannot write outside halfblock/.
	if _, _, _, _, _, err := halfblock.ParseCacheKey(key); err != nil {
		return err
	}
	return s.store.StoreHalfblock(ctx, key, []byte(ansi))
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
