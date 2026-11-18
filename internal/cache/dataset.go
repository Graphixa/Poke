package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Graphixa/Poke/internal/dataset"
	"github.com/Graphixa/Poke/internal/debuglog"
	"github.com/Graphixa/Poke/internal/domain"
)

// DatasetSchemaVersion is the current snapshot layout version.
const DatasetSchemaVersion = dataset.SchemaVersion

const appDataVersion = 3

type activePointer struct {
	SchemaVersion int    `json:"schema_version"`
	DatasetID     string `json:"dataset_id"`
}

// DatasetManifest describes one completed (or staging) metadata snapshot.
type DatasetManifest struct {
	SchemaVersion   int       `json:"schema_version"`
	DatasetVersion  string    `json:"dataset_version"`
	Source          string    `json:"source,omitempty"`
	SourceSHA       string    `json:"source_sha,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	PokeAPIBase     string    `json:"pokeapi_base"`
	PokemonCount    int       `json:"pokemon_count"`
	SpeciesCount    int       `json:"species_count,omitempty"`
	EncounterCount  int       `json:"encounter_count,omitempty"`
	EvolutionCount  int       `json:"evolution_count,omitempty"`
	MoveCount       int       `json:"move_count,omitempty"`
	StorageFailures int       `json:"storage_failures,omitempty"`
	AppDataVersion  int       `json:"app_data_version"`
	Complete        bool      `json:"complete"`
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
	return err == nil && man.Complete && supportedDatasetSchema(man.SchemaVersion)
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

func (s *Store) bootstrapReportPath(id string) string {
	return filepath.Join(s.datasetDir(id), "bootstrap.json")
}

// WriteBootstrapReport persists bootstrap diagnostics for a staging snapshot.
func (s *Store) WriteBootstrapReport(id string, report DatasetBootstrapReport) error {
	if id == "" {
		return errors.New("cache: bootstrap report missing dataset id")
	}
	data, err := json.Marshal(report)
	if err != nil {
		return err
	}
	return writeAtomic(s.bootstrapReportPath(id), data, 0o600)
}

// ReadBootstrapReport loads diagnostics for a snapshot.
func (s *Store) ReadBootstrapReport(id string) (DatasetBootstrapReport, error) {
	data, err := os.ReadFile(s.bootstrapReportPath(id))
	if errors.Is(err, os.ErrNotExist) {
		return DatasetBootstrapReport{}, ErrNotFound
	}
	if err != nil {
		return DatasetBootstrapReport{}, err
	}
	var report DatasetBootstrapReport
	if err := json.Unmarshal(data, &report); err != nil {
		return DatasetBootstrapReport{}, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	return report, nil
}

// ActivateDataset atomically points active.json at a complete snapshot.
// Incomplete or wrong-schema snapshots are refused so a failed refresh cannot
// replace a good one. Callers must build+validate the replacement first.
func (s *Store) ActivateDataset(id string) error {
	man, err := s.ReadDatasetManifest(id)
	if err != nil {
		return err
	}
	if !man.Complete {
		return fmt.Errorf("cache: refuse to activate incomplete dataset %q", id)
	}
	if !supportedDatasetSchema(man.SchemaVersion) {
		return fmt.Errorf("cache: refuse to activate dataset %q with schema %d (want %d)", id, man.SchemaVersion, dataset.SchemaVersion)
	}
	ptr := activePointer{SchemaVersion: man.SchemaVersion, DatasetID: id}
	data, err := json.Marshal(ptr)
	if err != nil {
		return err
	}
	if err := writeAtomic(s.activePath(), data, 0o600); err != nil {
		return err
	}
	s.setActiveDatasetID(id)
	s.invalidateLoadedSnapshot()
	debuglog.Cachef("dataset.activate id=%s count=%d", id, man.PokemonCount)
	return nil
}

// StoreBrowse writes the compact browse summary for a snapshot.
func (s *Store) StoreBrowse(id string, entries []domain.BrowseEntry) error {
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	return writeAtomicRename(s.browsePath(id), data, 0o600)
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
	return BrowseSummaries(entries), nil
}

// BrowseSummaries maps browse rows to search/index summaries (one pass).
func BrowseSummaries(entries []domain.BrowseEntry) []domain.SpeciesSummary {
	out := make([]domain.SpeciesSummary, len(entries))
	for i, e := range entries {
		out[i] = e.Summary()
		if out[i].SpritePath == "" && e.ID != 0 {
			out[i].SpritePath = SpriteRelPath(e.ID, e.Slug)
		}
	}
	return out
}

// BrowseByID indexes browse rows by species ID.
func BrowseByID(entries []domain.BrowseEntry) map[int]domain.BrowseEntry {
	byID := make(map[int]domain.BrowseEntry, len(entries))
	for _, e := range entries {
		byID[e.ID] = e
	}
	return byID
}

// EnsureDatasetDir creates the snapshot directory.
func (s *Store) EnsureDatasetDir(id string) error {
	return os.MkdirAll(s.datasetDir(id), 0o700)
}

// RemoveDatasetDir deletes a staging or failed snapshot directory.
func (s *Store) RemoveDatasetDir(id string) error {
	if id == "" {
		return nil
	}
	return os.RemoveAll(s.datasetDir(id))
}
