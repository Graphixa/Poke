package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

// Clock provides the current time for TTL and manifest stamps.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Store is a file-backed JSON cache under the application cache directory.
type Store struct {
	root  string
	clock Clock

	mu            sync.RWMutex
	activeID      string
	activeIDKnown bool

	snapshotMu sync.RWMutex
	loaded     *loadedSnapshot
	loadedErr  error

	speciesIndexWrites atomic.Uint32
}

func NewStore(root string, clock Clock) *Store {
	if clock == nil {
		clock = systemClock{}
	}
	return &Store{root: root, clock: clock}
}

func (s *Store) Path() string {
	return s.root
}

func (s *Store) EnsureRoot() error {
	return os.MkdirAll(s.root, 0o700)
}

func (s *Store) Clear() error {
	if s.root == "" {
		return errors.New("cache: refuse to clear empty path")
	}
	if err := os.RemoveAll(s.root); err != nil {
		return fmt.Errorf("clear cache: %w", err)
	}
	s.invalidateActiveDatasetCache()
	return s.EnsureRoot()
}

// getActiveDatasetID returns the cached active snapshot id when present.
func (s *Store) getActiveDatasetID() (string, error) {
	s.mu.RLock()
	if s.activeIDKnown {
		id := s.activeID
		s.mu.RUnlock()
		if id == "" {
			return "", ErrNotFound
		}
		return id, nil
	}
	s.mu.RUnlock()

	id, err := s.ActiveDatasetID()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.activeIDKnown = true
		s.activeID = ""
		return "", err
	}
	s.activeIDKnown = true
	s.activeID = id
	return id, nil
}

func (s *Store) setActiveDatasetID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeIDKnown = true
	s.activeID = id
}

func (s *Store) invalidateActiveDatasetCache() {
	s.mu.Lock()
	s.activeIDKnown = false
	s.activeID = ""
	s.mu.Unlock()
	s.invalidateLoadedSnapshot()
}

func (s *Store) SpeciesIndex(context.Context) ([]domain.SpeciesSummary, error) {
	var index []domain.SpeciesSummary
	if _, err := s.readJSON(s.indexPath(), &index); err != nil {
		return nil, err
	}
	return index, nil
}

func (s *Store) IndexFetchedAt() (time.Time, error) {
	man, err := s.readManifest()
	if err != nil {
		return time.Time{}, err
	}
	return man.IndexFetchedAt, nil
}

func (s *Store) HasSpeciesIndex() bool {
	_, err := os.Stat(s.indexPath())
	return err == nil
}

func (s *Store) Pokemon(_ context.Context, id int) (domain.Pokemon, error) {
	value, ok, err := s.lookupPokemon(id)
	if err != nil {
		return domain.Pokemon{}, err
	}
	if !ok {
		return domain.Pokemon{}, ErrNotFound
	}
	return value, nil
}

func (s *Store) Species(_ context.Context, id int) (domain.Species, error) {
	value, ok, err := s.lookupSpecies(id)
	if err != nil {
		return domain.Species{}, err
	}
	if !ok {
		return domain.Species{}, ErrNotFound
	}
	return value, nil
}

func (s *Store) EvolutionChain(_ context.Context, id int) (domain.EvolutionTree, error) {
	value, ok, err := s.lookupEvolution(id)
	if err != nil {
		return domain.EvolutionTree{}, err
	}
	if !ok {
		return domain.EvolutionTree{}, ErrNotFound
	}
	return value, nil
}

func (s *Store) Encounters(_ context.Context, pokemonID int) ([]domain.Encounter, error) {
	value, ok, err := s.lookupEncounters(pokemonID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	return value, nil
}

func (s *Store) Move(_ context.Context, id int) (domain.Move, error) {
	value, ok, err := s.lookupMove(id)
	if err != nil {
		return domain.Move{}, err
	}
	if !ok {
		return domain.Move{}, ErrNotFound
	}
	return value, nil
}

func (s *Store) Sprite(_ context.Context, contentHash string) ([]byte, error) {
	path := s.spritePath(contentHash)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

// SpriteURLLookup returns the content hash previously stored for urlHash (sha256 of URL).
func (s *Store) SpriteURLLookup(_ context.Context, urlHash string) (contentHash string, err error) {
	data, err := os.ReadFile(s.spriteURLPath(urlHash))
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	contentHash = string(bytesTrimSpace(data))
	if contentHash == "" {
		return "", ErrNotFound
	}
	return contentHash, nil
}

func (s *Store) StoreSpriteURL(_ context.Context, urlHash, contentHash string) error {
	return writeAtomic(s.spriteURLPath(urlHash), []byte(contentHash), 0o600)
}

func (s *Store) Artefact(_ context.Context, key string) ([]byte, error) {
	data, err := os.ReadFile(s.halfblockPath(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Store) StoreArtefact(_ context.Context, key string, data []byte) error {
	return writeAtomic(s.halfblockPath(key), data, 0o600)
}

// Halfblock is a legacy alias for Artefact (on-disk directory remains halfblock/).
func (s *Store) Halfblock(ctx context.Context, key string) ([]byte, error) {
	return s.Artefact(ctx, key)
}

func (s *Store) StoreHalfblock(ctx context.Context, key string, data []byte) error {
	return s.StoreArtefact(ctx, key, data)
}

func (s *Store) HasNativeFile(key string) bool {
	_, err := os.Stat(s.nativePath(key))
	return err == nil
}

func (s *Store) Native(_ context.Context, key string) ([]byte, error) {
	path := s.nativePath(key)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	out, err := gzipDecompressFrame(data)
	if err != nil {
		_ = s.quarantine(path, data)
		return nil, ErrNotFound
	}
	return out, nil
}

func (s *Store) StoreNative(_ context.Context, key string, data []byte) error {
	compressed, err := gzipCompressFrame(data)
	if err != nil {
		return err
	}
	return writeAtomicRename(s.nativePath(key), compressed, 0o600)
}

func bytesTrimSpace(b []byte) []byte {
	i, j := 0, len(b)
	for i < j && (b[i] == ' ' || b[i] == '\n' || b[i] == '\r' || b[i] == '\t') {
		i++
	}
	for j > i && (b[j-1] == ' ' || b[j-1] == '\n' || b[j-1] == '\r' || b[j-1] == '\t') {
		j--
	}
	return b[i:j]
}

func (s *Store) StoreSpeciesIndex(_ context.Context, index []domain.SpeciesSummary, fetchedAt time.Time) error {
	s.speciesIndexWrites.Add(1)
	fillIndexSpritePaths(index)
	if err := s.writeJSON(s.indexPath(), fetchedAt, index); err != nil {
		return err
	}
	return s.writeManifest(manifest{
		SchemaVersion:  schemaVersion,
		IndexFetchedAt: fetchedAt.UTC(),
		UpdatedAt:      s.clock.Now().UTC(),
	})
}

func (s *Store) StoreSprite(_ context.Context, contentHash string, data []byte) error {
	return writeAtomic(s.spritePath(contentHash), data, 0o600)
}

func (s *Store) writeJSON(path string, fetchedAt time.Time, value any) error {
	data, err := encodeEnvelope(fetchedAt, value)
	if err != nil {
		return err
	}
	return writeAtomic(path, data, 0o600)
}

func (s *Store) readJSON(path string, dest any) (time.Time, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return time.Time{}, ErrNotFound
	}
	if err != nil {
		return time.Time{}, err
	}
	fetchedAt, err := decodeEnvelope(data, dest)
	if err != nil {
		_ = s.quarantine(path, data)
		return time.Time{}, err
	}
	return fetchedAt, nil
}

func (s *Store) quarantine(path string, data []byte) error {
	rel, err := filepath.Rel(s.root, path)
	if err != nil || !filepath.IsLocal(rel) {
		rel = filepath.Base(path)
	}
	stamp := s.clock.Now().UTC().Format("20060102T150405")
	dest := filepath.Join(s.root, "corrupt", fmt.Sprintf("%s.%s", filepath.ToSlash(rel), stamp))
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return err
	}
	// dest is under s.root/corrupt; rel rejected unless filepath.IsLocal.
	if err := os.WriteFile(dest, data, 0o600); err != nil { // #nosec G703
		return err
	}
	_ = os.Remove(path)
	return nil
}

func (s *Store) readManifest() (manifest, error) {
	data, err := os.ReadFile(s.manifestPath())
	if errors.Is(err, os.ErrNotExist) {
		return manifest{}, ErrNotFound
	}
	if err != nil {
		return manifest{}, err
	}
	var man manifest
	if err := json.Unmarshal(data, &man); err != nil {
		_ = s.quarantine(s.manifestPath(), data)
		return manifest{}, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	if man.SchemaVersion != schemaVersion {
		_ = s.quarantine(s.manifestPath(), data)
		return manifest{}, fmt.Errorf("%w: unsupported schema version %d", ErrCorrupt, man.SchemaVersion)
	}
	return man, nil
}

func (s *Store) writeManifest(man manifest) error {
	data, err := json.Marshal(man)
	if err != nil {
		return err
	}
	return writeAtomic(s.manifestPath(), data, 0o600)
}

func (s *Store) manifestPath() string { return filepath.Join(s.root, "manifest.json") }
func (s *Store) indexPath() string    { return filepath.Join(s.root, "index", "species.json") }
func (s *Store) spritePath(hash string) string {
	return filepath.Join(s.root, "sprites", hash+".png")
}
func (s *Store) spriteURLPath(urlHash string) string {
	return filepath.Join(s.root, "sprites", "urls", urlHash)
}
func (s *Store) halfblockPath(key string) string {
	return filepath.Join(s.root, "halfblock", key+".ansi")
}
func (s *Store) nativePath(key string) string {
	return filepath.Join(s.root, "native", nativeFrameFilename(key))
}

// purgeIncompatibleNativeArtefacts deletes derived native frames that do not
// match the current algorithm version or canonical key format.
func (s *Store) purgeIncompatibleNativeArtefacts(currentVersion string) error {
	dir := filepath.Join(s.root, "native")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !isNativeArtefactName(name) {
			continue
		}
		key, _ := nativeKeyFromFilename(name)
		_, _, _, _, _, ver, _, parseErr := sprite.ParseNativeCacheKey(key)
		if parseErr != nil || ver != currentVersion {
			if rmErr := os.Remove(filepath.Join(dir, name)); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
				return rmErr
			}
		}
	}
	return nil
}

// DeleteNativeForSpecies removes encoded native frames for a species id prefix.
func (s *Store) DeleteNativeForSpecies(speciesID int) error {
	if speciesID < 1 {
		return nil
	}
	dir := filepath.Join(s.root, "native")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	hashes := s.collectSpeciesAssetHashes(speciesID)
	prefix := fmt.Sprintf("named-%d-", speciesID)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !isNativeArtefactName(name) {
			continue
		}
		key, _ := nativeKeyFromFilename(name)
		remove := strings.HasPrefix(key, prefix)
		if !remove {
			if hash, _, _, _, _, _, _, parseErr := sprite.ParseNativeCacheKey(key); parseErr == nil {
				_, remove = hashes[hash]
			}
		}
		if !remove {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// SpeciesAssetHashes returns PNG content hashes stored for a species (artwork + pixel).
func (s *Store) SpeciesAssetHashes(speciesID int) map[string]struct{} {
	return s.collectSpeciesAssetHashes(speciesID)
}

func (s *Store) collectSpeciesAssetHashes(speciesID int) map[string]struct{} {
	out := map[string]struct{}{}
	add := func(h string) {
		if isHexSHA256(h) {
			out[h] = struct{}{}
		}
	}
	for _, kind := range []string{SpriteKindArtwork, SpriteKindPixel} {
		if m, err := s.readAssetMapping(speciesID, kind); err == nil {
			add(m.Hash)
		}
	}
	for _, tree := range []string{"official-artwork", "sprites"} {
		root := filepath.Join(s.root, tree)
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		prefix := fmt.Sprintf("%03d-", speciesID)
		for _, e := range entries {
			if !e.IsDir() || !strings.HasPrefix(e.Name(), prefix) {
				continue
			}
			parts := strings.SplitN(e.Name(), "-", 2)
			if len(parts) < 2 {
				continue
			}
			slug := parts[1]
			for _, kind := range []string{SpriteKindArtwork, SpriteKindPixel} {
				if meta, err := s.ReadNamedSpriteMeta(speciesID, slug, kind); err == nil {
					add(meta.SHA256)
				}
			}
		}
	}
	return out
}
