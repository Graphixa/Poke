package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
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

// StoreSpriteByURL persists PNG bytes under their content hash and maps the URL (legacy).
func (s *Service) StoreSpriteByURL(ctx context.Context, url, contentHash string, data []byte) error {
	if contentHash == "" || len(data) == 0 {
		return errors.New("cache: refuse to store empty sprite")
	}
	if err := s.store.StoreSprite(ctx, contentHash, data); err != nil {
		return err
	}
	return s.store.StoreSpriteURL(ctx, sha256Hex(url), contentHash)
}

// NamedSprite returns PNG bytes from the named sprites/ or official-artwork/ tree.
func (s *Service) NamedSprite(ctx context.Context, id int, slug, kind string) ([]byte, error) {
	return s.store.NamedSprite(ctx, id, slug, kind)
}

// StoreNamedSprite writes PNG bytes into the named sprite tree and mirrors a content-hash copy
// so halfblock keys (content-hash based) keep working.
func (s *Service) StoreNamedSprite(ctx context.Context, id int, slug, kind, contentHash string, data []byte) error {
	if len(data) == 0 {
		return errors.New("cache: refuse to store empty named sprite")
	}
	if err := s.store.StoreNamedSprite(ctx, id, slug, kind, data); err != nil {
		return err
	}
	if contentHash != "" {
		_ = s.store.StoreSprite(ctx, contentHash, data)
	}
	return nil
}

// Halfblock returns a cached derived sprite artefact by key.
func (s *Service) Halfblock(ctx context.Context, key string) (string, error) {
	data, err := s.store.Halfblock(ctx, key)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// StoreHalfblock persists a derived sprite artefact (chafa ANSI, …).
func (s *Service) StoreHalfblock(ctx context.Context, key, ansi string) error {
	if key == "" {
		return errors.New("cache: refuse to store empty halfblock key")
	}
	// Validate key shape so callers cannot write outside halfblock/.
	if _, _, _, _, _, _, _, _, err := sprite.ParseCacheKey(key); err != nil {
		return err
	}
	return s.store.StoreHalfblock(ctx, key, []byte(ansi))
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
