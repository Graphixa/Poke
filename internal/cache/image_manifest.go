package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PokemonImageCacheState records per-Pokémon image completeness for one spec.
type PokemonImageCacheState struct {
	SpeciesID  int    `json:"species_id"`
	SourceHash string `json:"source_hash"`
	SpecID     string `json:"spec_id"`
	Complete   bool   `json:"complete"`
}

// ImageCacheManifest answers which Pokémon are fully image-cache complete
// without scanning every .frame file on the UI hot path.
// This is the runtime source of truth for Pokémon-level image-cache completeness.
type ImageCacheManifest struct {
	Version      string                         `json:"version"`
	SpecID       string                         `json:"spec_id"`
	DatasetID    string                         `json:"dataset_id,omitempty"`
	PokemonCount int                            `json:"pokemon_count,omitempty"`
	Pokemon      map[int]PokemonImageCacheState `json:"pokemon"`
}

func (s *Store) imageCacheManifestPath() string {
	return filepath.Join(s.root, "native", "image-manifest.json")
}

func (s *Store) readImageCacheManifestDisk() (ImageCacheManifest, error) {
	path := s.imageCacheManifestPath()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ImageCacheManifest{Version: ImageCacheManifestVersion, Pokemon: map[int]PokemonImageCacheState{}}, nil
	}
	if err != nil {
		return ImageCacheManifest{}, err
	}
	var m ImageCacheManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return ImageCacheManifest{}, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	if m.Pokemon == nil {
		m.Pokemon = map[int]PokemonImageCacheState{}
	}
	if m.Version == "" {
		m.Version = ImageCacheManifestVersion
	}
	return m, nil
}

// ReadImageCacheManifest loads the aggregate image-cache manifest when present.
func (s *Store) ReadImageCacheManifest() (ImageCacheManifest, error) {
	return s.imageManifest().Snapshot(), nil
}

// WriteImageCacheManifest persists the aggregate image-cache manifest.
func (s *Store) WriteImageCacheManifest(m ImageCacheManifest) error {
	if err := s.writeImageCacheManifestDisk(m); err != nil {
		return err
	}
	st := s.imageManifest()
	st.mu.Lock()
	st.m = m
	st.loaded = true
	st.dirty = false
	st.pending = 0
	st.mu.Unlock()
	return nil
}

func (s *Store) writeImageCacheManifestDisk(m ImageCacheManifest) error {
	if m.Pokemon == nil {
		m.Pokemon = map[int]PokemonImageCacheState{}
	}
	if m.Version == "" {
		m.Version = ImageCacheManifestVersion
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return writeAtomic(s.imageCacheManifestPath(), data, 0o600)
}

func (s *Store) invalidateImageCachePokemon(speciesID int) error {
	return s.imageManifest().Invalidate(speciesID)
}

const (
	imageManifestFlushEvery   = 25
	imageManifestFlushMaxWait = 500 * time.Millisecond
)

// imageManifestState is the single in-memory authority for image-manifest.json.
type imageManifestState struct {
	store     *Store
	mu        sync.Mutex
	loaded    bool
	m         ImageCacheManifest
	dirty     bool
	pending   int
	lastFlush time.Time
}

func (s *Store) imageManifest() *imageManifestState {
	if s.imgManifest == nil {
		s.imgManifest = &imageManifestState{store: s}
	}
	return s.imgManifest
}

func (st *imageManifestState) ensureLoaded() error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.loaded {
		return nil
	}
	m, err := st.store.readImageCacheManifestDisk()
	if err != nil {
		return err
	}
	st.m = m
	st.loaded = true
	return nil
}

// SeedSession replaces in-memory manifest at session start (already reconciled).
func (st *imageManifestState) SeedSession(m ImageCacheManifest) {
	if m.Pokemon == nil {
		m.Pokemon = map[int]PokemonImageCacheState{}
	}
	st.mu.Lock()
	st.m = m
	st.loaded = true
	st.dirty = false
	st.pending = 0
	st.lastFlush = time.Now()
	st.mu.Unlock()
}

func (st *imageManifestState) SetComplete(state PokemonImageCacheState) {
	st.mu.Lock()
	st.m.Pokemon[state.SpeciesID] = state
	st.dirty = true
	st.pending++
	st.mu.Unlock()
}

func (st *imageManifestState) Invalidate(speciesID int) error {
	if speciesID < 1 {
		return nil
	}
	if err := st.ensureLoaded(); err != nil {
		return err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if len(st.m.Pokemon) == 0 {
		return nil
	}
	delete(st.m.Pokemon, speciesID)
	st.dirty = true
	m := cloneImageCacheManifest(st.m)
	if err := st.store.writeImageCacheManifestDisk(m); err != nil {
		return err
	}
	st.dirty = false
	st.pending = 0
	st.lastFlush = time.Now()
	return nil
}

func (st *imageManifestState) UpdateDatasetMetadata(datasetID string, pokemonCount int, specID string) {
	st.mu.Lock()
	if datasetID != "" {
		st.m.DatasetID = datasetID
	}
	if pokemonCount > 0 {
		st.m.PokemonCount = pokemonCount
	}
	if specID != "" {
		st.m.SpecID = specID
	}
	st.mu.Unlock()
}

func (st *imageManifestState) Snapshot() ImageCacheManifest {
	if err := st.ensureLoaded(); err != nil {
		return ImageCacheManifest{Version: ImageCacheManifestVersion, Pokemon: map[int]PokemonImageCacheState{}}
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	out := ImageCacheManifest{
		Version: st.m.Version, SpecID: st.m.SpecID,
		DatasetID: st.m.DatasetID, PokemonCount: st.m.PokemonCount,
		Pokemon: make(map[int]PokemonImageCacheState, len(st.m.Pokemon)),
	}
	for id, s := range st.m.Pokemon {
		out.Pokemon[id] = s
	}
	return out
}

func (st *imageManifestState) MaybeFlush(force bool) error {
	if err := st.ensureLoaded(); err != nil {
		return err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if !st.dirty {
		return nil
	}
	now := time.Now()
	if !force && st.pending < imageManifestFlushEvery && now.Sub(st.lastFlush) < imageManifestFlushMaxWait {
		return nil
	}
	m := cloneImageCacheManifest(st.m)
	if err := st.store.writeImageCacheManifestDisk(m); err != nil {
		return err
	}
	st.dirty = false
	st.pending = 0
	st.lastFlush = now
	return nil
}

func (st *imageManifestState) Flush() error {
	return st.MaybeFlush(true)
}

func cloneImageCacheManifest(m ImageCacheManifest) ImageCacheManifest {
	out := ImageCacheManifest{
		Version: m.Version, SpecID: m.SpecID,
		DatasetID: m.DatasetID, PokemonCount: m.PokemonCount,
		Pokemon: make(map[int]PokemonImageCacheState, len(m.Pokemon)),
	}
	for id, st := range m.Pokemon {
		out.Pokemon[id] = st
	}
	return out
}

// imageManifestWriter batches persist via the store-level manifest state.
type imageManifestWriter struct {
	state *imageManifestState
}

func newImageManifestWriter(store *Store, m ImageCacheManifest) *imageManifestWriter {
	st := store.imageManifest()
	st.SeedSession(m)
	return &imageManifestWriter{state: st}
}

func (w *imageManifestWriter) set(st PokemonImageCacheState) {
	w.state.SetComplete(st)
}

func (w *imageManifestWriter) snapshot() ImageCacheManifest {
	return w.state.Snapshot()
}

func (w *imageManifestWriter) maybeFlush(force bool) error {
	return w.state.MaybeFlush(force)
}

func (w *imageManifestWriter) flush() error {
	return w.state.Flush()
}
