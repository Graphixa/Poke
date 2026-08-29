package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// NamedSpriteMeta records provenance for a named PNG (official-artwork or sprites).
type NamedSpriteMeta struct {
	URL       string    `json:"url"`
	SHA256    string    `json:"sha256"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *Store) namedSpriteMetaPath(id int, slug, kind string) (string, error) {
	pngPath, err := s.namedSpritePath(id, slug, kind)
	if err != nil {
		return "", err
	}
	return pngPath + ".meta.json", nil
}

// ReadNamedSpriteMeta returns sidecar metadata when present.
func (s *Store) ReadNamedSpriteMeta(id int, slug, kind string) (NamedSpriteMeta, error) {
	path, err := s.namedSpriteMetaPath(id, slug, kind)
	if err != nil {
		return NamedSpriteMeta{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return NamedSpriteMeta{}, ErrNotFound
	}
	if err != nil {
		return NamedSpriteMeta{}, err
	}
	var meta NamedSpriteMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		_ = s.quarantine(path, data)
		return NamedSpriteMeta{}, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	return meta, nil
}

// writeNamedSpriteMeta persists sidecar metadata beside the PNG.
func (s *Store) writeNamedSpriteMeta(id int, slug, kind string, meta NamedSpriteMeta) error {
	path, err := s.namedSpriteMetaPath(id, slug, kind)
	if err != nil {
		return err
	}
	if meta.UpdatedAt.IsZero() {
		meta.UpdatedAt = s.clock.Now()
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return writeAtomic(path, data, 0o600)
}

// HasNamedSprite reports whether the named PNG exists on disk.
func (s *Store) HasNamedSprite(id int, slug, kind string) bool {
	path, err := s.namedSpritePath(id, slug, kind)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}
