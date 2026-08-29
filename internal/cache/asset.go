package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

// AssetMapping is the Pokémon → content-hash pointer (no PNG bytes).
type AssetMapping struct {
	PokemonID int    `json:"pokemon_id"`
	Source    string `json:"source"`
	Hash      string `json:"hash"`
	URL       string `json:"url,omitempty"`
	Path      string `json:"path,omitempty"`
}

func (s *Store) artworkObjectPath(hash string) string {
	if len(hash) < 2 {
		return filepath.Join(s.root, "artwork", "objects", hash+".png")
	}
	return filepath.Join(s.root, "artwork", "objects", hash[:2], hash+".png")
}

func (s *Store) artworkMappingPath(pokemonID int, source string) string {
	return filepath.Join(s.root, "artwork", "mappings", fmt.Sprintf("%d-%s.json", pokemonID, source))
}

// NamedAssetMeta returns hash/path for a named sprite without reading PNG bytes.
func (s *Store) NamedAssetMeta(_ context.Context, id int, slug, kind string) (hash, path string, ready bool, err error) {
	if id < 1 {
		return "", "", false, nil
	}
	src := kind
	if src == "" {
		src = SpriteKindPixel
	}
	if mapping, merr := s.readAssetMapping(id, src); merr == nil && mapping.Hash != "" {
		obj := s.artworkObjectPath(mapping.Hash)
		if _, statErr := os.Stat(obj); statErr == nil {
			return mapping.Hash, obj, true, nil
		}
	}
	path, err = s.namedSpritePath(id, slug, kind)
	if err != nil {
		return "", "", false, err
	}
	if _, statErr := os.Stat(path); statErr != nil {
		return "", "", false, nil
	}
	hash = sprite.NamedAssetKey(id, slug, kind)
	if meta, merr := s.ReadNamedSpriteMeta(id, slug, kind); merr == nil && meta.SHA256 != "" {
		hash = meta.SHA256
	}
	return hash, path, true, nil
}

func (s *Store) readAssetMapping(pokemonID int, source string) (AssetMapping, error) {
	data, err := os.ReadFile(s.artworkMappingPath(pokemonID, source))
	if errors.Is(err, os.ErrNotExist) {
		return AssetMapping{}, ErrNotFound
	}
	if err != nil {
		return AssetMapping{}, err
	}
	var m AssetMapping
	if err := json.Unmarshal(data, &m); err != nil {
		return AssetMapping{}, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	return m, nil
}

// StoreContentAsset writes original PNG bytes by content hash and a mapping file.
func (s *Store) StoreContentAsset(pokemonID int, source, url, hash string, data []byte) error {
	if hash == "" || len(data) == 0 {
		return errors.New("cache: refuse to store empty artwork")
	}
	obj := s.artworkObjectPath(hash)
	if err := writeAtomic(obj, data, 0o600); err != nil {
		return err
	}
	rel, _ := filepath.Rel(s.root, obj)
	mapping := AssetMapping{
		PokemonID: pokemonID,
		Source:    source,
		Hash:      hash,
		URL:       url,
		Path:      filepath.ToSlash(rel),
	}
	raw, err := json.Marshal(mapping)
	if err != nil {
		return err
	}
	debuglog.Cachef("artwork.download.complete id=%d source=%s hash=%s", pokemonID, source, hash)
	return writeAtomic(s.artworkMappingPath(pokemonID, source), raw, 0o600)
}

// Service.NamedAssetMeta implements sprite.NamedAssetMetaStore.
func (s *Service) NamedAssetMeta(ctx context.Context, id int, slug, kind string) (string, string, bool, error) {
	return s.store.NamedAssetMeta(ctx, id, slug, kind)
}

// ReadAssetBytes reads PNG bytes from a path under the cache root.
func (s *Service) ReadAssetBytes(_ context.Context, path string) ([]byte, error) {
	return s.store.ReadAssetBytes(path)
}

func (s *Store) ReadAssetBytes(path string) ([]byte, error) {
	if path == "" {
		return nil, ErrNotFound
	}
	var full string
	if filepath.IsAbs(path) {
		rel, err := filepath.Rel(s.root, path)
		if err != nil || stripUnsafeRel(rel) {
			return nil, ErrNotFound
		}
		full = filepath.Join(s.root, rel)
	} else {
		if stripUnsafeRel(path) {
			return nil, ErrNotFound
		}
		full = filepath.Join(s.root, path)
	}
	data, err := os.ReadFile(full)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return data, err
}

func (s *Service) StoreContentAsset(pokemonID int, source, url, hash string, data []byte) error {
	return s.store.StoreContentAsset(pokemonID, source, url, hash, data)
}
