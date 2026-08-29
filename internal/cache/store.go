package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Graphixa/Pokedex/internal/domain"
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
	defer s.mu.Unlock()
	s.activeIDKnown = false
	s.activeID = ""
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
	var value domain.Pokemon
	if datasetID, err := s.getActiveDatasetID(); err == nil {
		if _, err := s.readJSON(s.SnapshotPokemonPath(datasetID, id), &value); err != nil {
			return domain.Pokemon{}, err
		}
		return value, nil
	}
	if _, err := s.readJSON(s.pokemonPath(id), &value); err != nil {
		return domain.Pokemon{}, err
	}
	return value, nil
}

func (s *Store) Species(_ context.Context, id int) (domain.Species, error) {
	var value domain.Species
	if datasetID, err := s.getActiveDatasetID(); err == nil {
		if _, err := s.readJSON(s.SnapshotSpeciesPath(datasetID, id), &value); err != nil {
			return domain.Species{}, err
		}
		return value, nil
	}
	if _, err := s.readJSON(s.speciesPath(id), &value); err != nil {
		return domain.Species{}, err
	}
	return value, nil
}

func (s *Store) EvolutionChain(_ context.Context, id int) (domain.EvolutionTree, error) {
	var value domain.EvolutionTree
	if datasetID, err := s.getActiveDatasetID(); err == nil {
		if _, err := s.readJSON(s.SnapshotEvolutionPath(datasetID, id), &value); err != nil {
			return domain.EvolutionTree{}, err
		}
		return value, nil
	}
	if _, err := s.readJSON(s.evolutionPath(id), &value); err != nil {
		return domain.EvolutionTree{}, err
	}
	return value, nil
}

func (s *Store) Encounters(_ context.Context, pokemonID int) ([]domain.Encounter, error) {
	var value []domain.Encounter
	if datasetID, err := s.getActiveDatasetID(); err == nil {
		if _, err := s.readJSON(s.SnapshotEncountersPath(datasetID, pokemonID), &value); err != nil {
			return nil, err
		}
		return value, nil
	}
	if _, err := s.readJSON(s.encountersPath(pokemonID), &value); err != nil {
		return nil, err
	}
	return value, nil
}

func (s *Store) Move(_ context.Context, id int) (domain.Move, error) {
	var value domain.Move
	if datasetID, err := s.getActiveDatasetID(); err == nil {
		if _, err := s.readJSON(s.SnapshotMovePath(datasetID, id), &value); err != nil {
			return domain.Move{}, err
		}
		return value, nil
	}
	if _, err := s.readJSON(s.movePath(id), &value); err != nil {
		return domain.Move{}, err
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

func (s *Store) Halfblock(_ context.Context, key string) ([]byte, error) {
	data, err := os.ReadFile(s.halfblockPath(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Store) StoreHalfblock(_ context.Context, key string, data []byte) error {
	return writeAtomic(s.halfblockPath(key), data, 0o600)
}

func (s *Store) Native(_ context.Context, key string) ([]byte, error) {
	data, err := os.ReadFile(s.nativePath(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Store) StoreNative(_ context.Context, key string, data []byte) error {
	return writeAtomic(s.nativePath(key), data, 0o600)
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

func (s *Store) StorePokemon(_ context.Context, pokemon domain.Pokemon, fetchedAt time.Time) error {
	return s.writeJSON(s.pokemonPath(pokemon.ID), fetchedAt, pokemon)
}

func (s *Store) StoreSpecies(_ context.Context, species domain.Species, fetchedAt time.Time) error {
	return s.writeJSON(s.speciesPath(species.ID), fetchedAt, species)
}

func (s *Store) StoreEvolutionChain(_ context.Context, id int, tree domain.EvolutionTree, fetchedAt time.Time) error {
	return s.writeJSON(s.evolutionPath(id), fetchedAt, tree)
}

func (s *Store) StoreEncounters(_ context.Context, pokemonID int, encounters []domain.Encounter, fetchedAt time.Time) error {
	return s.writeJSON(s.encountersPath(pokemonID), fetchedAt, encounters)
}

func (s *Store) StoreMove(_ context.Context, move domain.Move, fetchedAt time.Time) error {
	return s.writeJSON(s.movePath(move.ID), fetchedAt, move)
}

func (s *Store) StoreSprite(_ context.Context, contentHash string, data []byte) error {
	return writeAtomic(s.spritePath(contentHash), data, 0o600)
}

func (s *Store) ResourceFetchedAt(kind string, id int) (time.Time, error) {
	path, err := s.resourcePath(kind, id)
	if err != nil {
		return time.Time{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return time.Time{}, ErrNotFound
	}
	if err != nil {
		return time.Time{}, err
	}
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		_ = s.quarantine(path, data)
		return time.Time{}, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	return env.FetchedAt, nil
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
	if err != nil {
		rel = filepath.Base(path)
	}
	stamp := s.clock.Now().UTC().Format("20060102T150405")
	dest := filepath.Join(s.root, "corrupt", fmt.Sprintf("%s.%s", filepath.ToSlash(rel), stamp))
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(dest, data, 0o600); err != nil {
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
func (s *Store) pokemonPath(id int) string {
	return s.legacyPokemonPath(id)
}
func (s *Store) speciesPath(id int) string {
	return s.legacySpeciesPath(id)
}
func (s *Store) evolutionPath(id int) string {
	return s.legacyEvolutionPath(id)
}
func (s *Store) encountersPath(id int) string {
	return s.legacyEncountersPath(id)
}
func (s *Store) movePath(id int) string {
	return filepath.Join(s.root, "moves", strconv.Itoa(id)+".json")
}
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
	return filepath.Join(s.root, "native", key+".frame")
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
	prefix := fmt.Sprintf("named-%d-", speciesID)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".frame") {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (s *Store) resourcePath(kind string, id int) (string, error) {
	switch kind {
	case "pokemon":
		return s.pokemonPath(id), nil
	case "species":
		return s.speciesPath(id), nil
	case "evolution":
		return s.evolutionPath(id), nil
	case "encounters":
		return s.encountersPath(id), nil
	case "moves":
		return s.movePath(id), nil
	default:
		return "", fmt.Errorf("unknown resource kind %q", kind)
	}
}
