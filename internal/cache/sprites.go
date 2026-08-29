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

// StoreNamedSprite writes PNG bytes into the named sprite tree.
// contentHash when a 64-char hex digest is stored as sha256 in sidecar meta (url omitted).
func (s *Service) StoreNamedSprite(ctx context.Context, id int, slug, kind, contentHash string, data []byte) error {
	if len(data) == 0 {
		return errors.New("cache: refuse to store empty named sprite")
	}
	if isHexSHA256(contentHash) {
		return s.store.StoreNamedSpriteWithMeta(ctx, id, slug, kind, "", contentHash, data)
	}
	return s.store.StoreNamedSprite(ctx, id, slug, kind, data)
}

// StoreNamedSpriteWithURL persists PNG bytes and url+sha256 sidecar metadata.
func (s *Service) StoreNamedSpriteWithURL(ctx context.Context, id int, slug, kind, url, sha256 string, data []byte) error {
	if len(data) == 0 {
		return errors.New("cache: refuse to store empty named sprite")
	}
	if sha256 == "" {
		sha256 = sha256HexBytes(data)
	}
	return s.store.StoreNamedSpriteWithMeta(ctx, id, slug, kind, url, sha256, data)
}

// InvalidateNativeSpecies drops mem-cached native frames and deletes on-disk .frame files.
func (s *Service) InvalidateNativeSpecies(speciesID int) error {
	if speciesID < 1 {
		return nil
	}
	if err := s.store.DeleteNativeForSpecies(speciesID); err != nil {
		return err
	}
	return s.store.InvalidateNativeManifestSizes()
}

// SpeciesAssetHashes returns PNG content hashes for a species.
func (s *Service) SpeciesAssetHashes(speciesID int) map[string]struct{} {
	return s.store.SpeciesAssetHashes(speciesID)
}

func sha256HexBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func isHexSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			continue
		}
		return false
	}
	return true
}

// Artefact returns a cached derived sprite artefact by key.
func (s *Service) Artefact(ctx context.Context, key string) (string, error) {
	data, err := s.store.Artefact(ctx, key)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// StoreArtefact persists a derived sprite artefact (chafa ANSI, …).
func (s *Service) StoreArtefact(ctx context.Context, key, ansi string) error {
	if key == "" {
		return errors.New("cache: refuse to store empty artefact key")
	}
	if _, _, _, _, _, _, _, _, err := sprite.ParseCacheKey(key); err != nil {
		return err
	}
	return s.store.StoreArtefact(ctx, key, []byte(ansi))
}

// Halfblock is a legacy alias for Artefact.
func (s *Service) Halfblock(ctx context.Context, key string) (string, error) {
	return s.Artefact(ctx, key)
}

// StoreHalfblock is a legacy alias for StoreArtefact.
func (s *Service) StoreHalfblock(ctx context.Context, key, ansi string) error {
	return s.StoreArtefact(ctx, key, ansi)
}

var _ sprite.ArtefactCache = (*Service)(nil)

// Native returns a cached Kitty/iTerm/Sixel frame by key.
func (s *Service) Native(ctx context.Context, key string) (string, error) {
	data, err := s.store.Native(ctx, key)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// StoreNative persists an encoded native graphics frame (Sixel/Kitty/iTerm).
func (s *Service) StoreNative(ctx context.Context, key, frame string) error {
	if key == "" {
		return errors.New("cache: refuse to store empty native key")
	}
	if _, _, _, _, _, _, err := sprite.ParseNativeCacheKey(key); err != nil {
		return err
	}
	isNew := !s.store.HasNativeFile(key)
	if err := s.store.StoreNative(ctx, key, []byte(frame)); err != nil {
		return err
	}
	s.recordNativeFrameManifest(key, isNew)
	return nil
}

func (s *Service) recordNativeFrameManifest(key string, isNew bool) {
	if !isNew {
		return
	}
	_, _, pxW, pxH, _, ver, err := sprite.ParseNativeCacheKey(key)
	if err != nil || ver != sprite.AlgorithmVersion {
		return
	}
	cells := pxW / sprite.SixelCellPxW
	if cells < sprite.MinFrameSize && pxH > 0 {
		cells = pxH / sprite.SixelCellPxH
	}
	if cells < sprite.MinFrameSize {
		return
	}
	m, err := s.store.ReadNativeManifest()
	if err != nil || m.Version != sprite.AlgorithmVersion {
		return
	}
	if m.Sizes == nil {
		m.Sizes = map[int]NativeSizeStatus{}
	}
	entry := m.Sizes[cells]
	entry.Done++
	entry.Complete = entry.Total > 0 && entry.Done >= entry.Total
	m.Sizes[cells] = entry
	_ = s.store.WriteNativeManifest(m)
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
