package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/domain"
)

// DatasetSchemaVersion is the snapshot layout version (independent of envelope schema).
const DatasetSchemaVersion = 2

const appDataVersion = 3

type activePointer struct {
	SchemaVersion int    `json:"schema_version"`
	DatasetID     string `json:"dataset_id"`
}

// DatasetManifest describes one completed (or staging) metadata snapshot.
type DatasetManifest struct {
	SchemaVersion  int       `json:"schema_version"`
	DatasetVersion string    `json:"dataset_version"`
	CreatedAt      time.Time `json:"created_at"`
	PokeAPIBase    string    `json:"pokeapi_base"`
	PokemonCount   int       `json:"pokemon_count"`
	AppDataVersion int       `json:"app_data_version"`
	Complete       bool      `json:"complete"`
}

func (s *Store) activePath() string {
	return filepath.Join(s.root, "active.json")
}

func (s *Store) datasetsRoot() string {
	return filepath.Join(s.root, "datasets")
}

func (s *Store) datasetDir(id string) string {
	return filepath.Join(s.datasetsRoot(), id)
}

func (s *Store) datasetManifestPath(id string) string {
	return filepath.Join(s.datasetDir(id), "manifest.json")
}

func (s *Store) browsePath(id string) string {
	return filepath.Join(s.datasetDir(id), "browse.json")
}

// DatasetIDForTime returns a filesystem-safe snapshot id.
func DatasetIDForTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T150405")
}

// ActiveDatasetID returns the activated snapshot id, or ErrNotFound.
func (s *Store) ActiveDatasetID() (string, error) {
	data, err := os.ReadFile(s.activePath())
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	var ptr activePointer
	if err := json.Unmarshal(data, &ptr); err != nil {
		_ = s.quarantine(s.activePath(), data)
		return "", fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	if ptr.DatasetID == "" {
		return "", ErrNotFound
	}
	return ptr.DatasetID, nil
}

// HasActiveDataset reports whether a complete active snapshot exists.
func (s *Store) HasActiveDataset() bool {
	id, err := s.ActiveDatasetID()
	if err != nil {
		return false
	}
	man, err := s.ReadDatasetManifest(id)
	return err == nil && man.Complete && man.SchemaVersion == DatasetSchemaVersion
}

// ReadDatasetManifest loads a snapshot manifest.
func (s *Store) ReadDatasetManifest(id string) (DatasetManifest, error) {
	data, err := os.ReadFile(s.datasetManifestPath(id))
	if errors.Is(err, os.ErrNotExist) {
		return DatasetManifest{}, ErrNotFound
	}
	if err != nil {
		return DatasetManifest{}, err
	}
	var man DatasetManifest
	if err := json.Unmarshal(data, &man); err != nil {
		return DatasetManifest{}, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	return man, nil
}

// WriteDatasetManifest persists a snapshot manifest (compact JSON).
func (s *Store) WriteDatasetManifest(man DatasetManifest) error {
	if man.DatasetVersion == "" {
		return errors.New("cache: dataset manifest missing id")
	}
	data, err := json.Marshal(man)
	if err != nil {
		return err
	}
	return writeAtomic(s.datasetManifestPath(man.DatasetVersion), data, 0o600)
}

// ActivateDataset atomically points active.json at a complete snapshot.
// Incomplete snapshots are refused so a failed refresh cannot replace a good one.
func (s *Store) ActivateDataset(id string) error {
	man, err := s.ReadDatasetManifest(id)
	if err != nil {
		return err
	}
	if !man.Complete {
		return fmt.Errorf("cache: refuse to activate incomplete dataset %q", id)
	}
	ptr := activePointer{SchemaVersion: DatasetSchemaVersion, DatasetID: id}
	data, err := json.Marshal(ptr)
	if err != nil {
		return err
	}
	if err := writeAtomic(s.activePath(), data, 0o600); err != nil {
		return err
	}
	s.setActiveDatasetID(id)
	debuglog.Cachef("dataset.activate id=%s count=%d", id, man.PokemonCount)
	return nil
}

// StoreBrowse writes the compact browse summary for a snapshot.
func (s *Store) StoreBrowse(id string, entries []domain.BrowseEntry) error {
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	return writeAtomic(s.browsePath(id), data, 0o600)
}

// LoadBrowse reads browse.json from the active snapshot.
func (s *Store) LoadBrowse() ([]domain.BrowseEntry, error) {
	id, err := s.getActiveDatasetID()
	if err != nil {
		return nil, err
	}
	return s.loadBrowseID(id)
}

func (s *Store) loadBrowseID(id string) ([]domain.BrowseEntry, error) {
	data, err := os.ReadFile(s.browsePath(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var entries []domain.BrowseEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		_ = s.quarantine(s.browsePath(id), data)
		return nil, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	return entries, nil
}

// LoadBrowseSummaries returns browse rows as SpeciesSummary for search/index.
func (s *Store) LoadBrowseSummaries() ([]domain.SpeciesSummary, error) {
	entries, err := s.LoadBrowse()
	if err != nil {
		return nil, err
	}
	out := make([]domain.SpeciesSummary, len(entries))
	for i, e := range entries {
		out[i] = e.Summary()
		if out[i].SpritePath == "" && e.ID != 0 {
			out[i].SpritePath = SpriteRelPath(e.ID, e.Slug)
		}
	}
	return out, nil
}

// EnsureDatasetDir creates the snapshot directory.
func (s *Store) EnsureDatasetDir(id string) error {
	return os.MkdirAll(s.datasetDir(id), 0o700)
}

// SnapshotPokemonPath is the per-resource pokemon JSON inside a snapshot.
func (s *Store) SnapshotPokemonPath(datasetID string, id int) string {
	return filepath.Join(s.datasetDir(datasetID), "pokemon", fmt.Sprintf("%d.json", id))
}

func (s *Store) SnapshotSpeciesPath(datasetID string, id int) string {
	return filepath.Join(s.datasetDir(datasetID), "species", fmt.Sprintf("%d.json", id))
}

func (s *Store) SnapshotEvolutionPath(datasetID string, id int) string {
	return filepath.Join(s.datasetDir(datasetID), "evolution", fmt.Sprintf("%d.json", id))
}

func (s *Store) SnapshotEncountersPath(datasetID string, pokemonID int) string {
	return filepath.Join(s.datasetDir(datasetID), "encounters", fmt.Sprintf("%d.json", pokemonID))
}

func (s *Store) SnapshotMovePath(datasetID string, id int) string {
	return filepath.Join(s.datasetDir(datasetID), "moves", fmt.Sprintf("%d.json", id))
}

func (s *Store) writeSnapshotJSON(path string, fetchedAt time.Time, value any) error {
	return s.writeJSON(path, fetchedAt, value)
}

// StoreSnapshotPokemon writes a pokemon record into a staging snapshot.
func (s *Store) StoreSnapshotPokemon(datasetID string, pokemon domain.Pokemon, fetchedAt time.Time) error {
	return s.writeSnapshotJSON(s.SnapshotPokemonPath(datasetID, pokemon.ID), fetchedAt, pokemon)
}

func (s *Store) StoreSnapshotSpecies(datasetID string, species domain.Species, fetchedAt time.Time) error {
	return s.writeSnapshotJSON(s.SnapshotSpeciesPath(datasetID, species.ID), fetchedAt, species)
}

func (s *Store) StoreSnapshotEvolution(datasetID string, id int, tree domain.EvolutionTree, fetchedAt time.Time) error {
	return s.writeSnapshotJSON(s.SnapshotEvolutionPath(datasetID, id), fetchedAt, tree)
}

func (s *Store) StoreSnapshotEncounters(datasetID string, pokemonID int, encounters []domain.Encounter, fetchedAt time.Time) error {
	return s.writeSnapshotJSON(s.SnapshotEncountersPath(datasetID, pokemonID), fetchedAt, encounters)
}

func (s *Store) StoreSnapshotMove(datasetID string, move domain.Move, fetchedAt time.Time) error {
	return s.writeSnapshotJSON(s.SnapshotMovePath(datasetID, move.ID), fetchedAt, move)
}
