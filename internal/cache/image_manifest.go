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

// ReadImageCacheManifest loads the aggregate image-cache manifest when present.
func (s *Store) ReadImageCacheManifest() (ImageCacheManifest, error) {
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

// WriteImageCacheManifest persists the aggregate image-cache manifest.
func (s *Store) WriteImageCacheManifest(m ImageCacheManifest) error {
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
	if speciesID < 1 {
		return nil
	}
	m, err := s.ReadImageCacheManifest()
	if err != nil {
		return err
	}
	if len(m.Pokemon) == 0 {
		return nil
	}
	delete(m.Pokemon, speciesID)
	return s.WriteImageCacheManifest(m)
}

const (
	imageManifestFlushEvery   = 25
	imageManifestFlushMaxWait = 500 * time.Millisecond
)

// imageManifestWriter batches persist of in-memory manifest copies.
type imageManifestWriter struct {
	store     *Store
	mu        sync.Mutex
	dirty     bool
	pending   int
	lastFlush time.Time
	m         ImageCacheManifest
}

func newImageManifestWriter(store *Store, m ImageCacheManifest) *imageManifestWriter {
	if m.Pokemon == nil {
		m.Pokemon = map[int]PokemonImageCacheState{}
	}
	w := &imageManifestWriter{store: store, m: m, lastFlush: time.Now()}
	w.dirty = false
	return w
}

func (w *imageManifestWriter) set(st PokemonImageCacheState) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.m.Pokemon[st.SpeciesID] = st
	w.dirty = true
	w.pending++
}

func (w *imageManifestWriter) snapshot() ImageCacheManifest {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := ImageCacheManifest{
		Version: w.m.Version, SpecID: w.m.SpecID,
		DatasetID: w.m.DatasetID, PokemonCount: w.m.PokemonCount,
		Pokemon: make(map[int]PokemonImageCacheState, len(w.m.Pokemon)),
	}
	for id, st := range w.m.Pokemon {
		out.Pokemon[id] = st
	}
	return out
}

func (w *imageManifestWriter) maybeFlush(force bool) error {
	w.mu.Lock()
	if !w.dirty {
		w.mu.Unlock()
		return nil
	}
	now := time.Now()
	if !force && w.pending < imageManifestFlushEvery && now.Sub(w.lastFlush) < imageManifestFlushMaxWait {
		w.mu.Unlock()
		return nil
	}
	m := ImageCacheManifest{
		Version: w.m.Version, SpecID: w.m.SpecID,
		DatasetID: w.m.DatasetID, PokemonCount: w.m.PokemonCount,
		Pokemon: make(map[int]PokemonImageCacheState, len(w.m.Pokemon)),
	}
	for id, st := range w.m.Pokemon {
		m.Pokemon[id] = st
	}
	w.dirty = false
	w.pending = 0
	w.lastFlush = now
	w.mu.Unlock()
	return w.store.WriteImageCacheManifest(m)
}

func (w *imageManifestWriter) flush() error {
	return w.maybeFlush(true)
}
