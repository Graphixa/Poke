package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Graphixa/Poke/internal/dataset"
	"github.com/Graphixa/Poke/internal/debuglog"
	"github.com/Graphixa/Poke/internal/domain"
)

type loadedSnapshot struct {
	store *Store
	id    string

	mu sync.Mutex

	pokemon       map[int]domain.Pokemon
	pokemonErr    error
	species       map[int]domain.Species
	speciesErr    error
	moves         map[int]domain.Move
	movesErr      error
	evolutions    map[int]domain.EvolutionTree
	evolutionsErr error
	encounters    map[int][]domain.Encounter
	encountersErr error
	learnsets     map[int][]dataset.DatasetLearn
	learnsetsErr  error
	versions      map[int]domain.Version
	versionsErr   error
	forms         map[int]domain.PokemonForm
	formsErr      error
	types         map[string]domain.TypeInfo
	typesErr      error
	abilities     map[int]domain.Ability
	abilitiesErr  error
}

func (s *Store) invalidateLoadedSnapshot() {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	s.loaded = nil
	s.loadedErr = nil
}

func (s *Store) getLoadedSnapshot() (*loadedSnapshot, error) {
	s.snapshotMu.RLock()
	if s.loaded != nil || s.loadedErr != nil {
		ds, err := s.loaded, s.loadedErr
		s.snapshotMu.RUnlock()
		return ds, err
	}
	s.snapshotMu.RUnlock()

	activeID, activeErr := s.getActiveDatasetID()

	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	if s.loaded != nil || s.loadedErr != nil {
		return s.loaded, s.loadedErr
	}
	ds, err := s.openSnapshotWithID(activeID, activeErr)
	s.loaded = ds
	s.loadedErr = err
	return ds, err
}

func (s *Store) openSnapshotWithID(id string, idErr error) (*loadedSnapshot, error) {
	if errors.Is(idErr, ErrNotFound) {
		return nil, nil
	}
	if idErr != nil {
		return nil, idErr
	}
	man, err := s.ReadDatasetManifest(id)
	if err != nil {
		return nil, err
	}
	if man.SchemaVersion != dataset.SchemaVersion {
		return nil, nil
	}
	return &loadedSnapshot{store: s, id: id}, nil
}

func (d *loadedSnapshot) ensurePokemon() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.pokemon != nil || d.pokemonErr != nil {
		return d.pokemonErr
	}
	pokemonFile, err := d.store.LoadAggregatePokemon(d.id)
	if err != nil {
		d.pokemonErr = err
		return err
	}
	pokemon := make(map[int]domain.Pokemon, len(pokemonFile))
	for pokemonID, compact := range pokemonFile {
		pokemon[pokemonID] = dataset.PokemonToDomain(compact, nil)
	}
	d.pokemon = pokemon
	return nil
}

func (d *loadedSnapshot) ensureSpecies() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.species != nil || d.speciesErr != nil {
		return d.speciesErr
	}
	speciesFile, err := d.store.LoadAggregateSpecies(d.id)
	if err != nil {
		d.speciesErr = err
		return err
	}
	species := make(map[int]domain.Species, len(speciesFile))
	for speciesID, compact := range speciesFile {
		species[speciesID] = dataset.SpeciesToDomain(compact)
	}
	d.species = species
	return nil
}

func (d *loadedSnapshot) loadMovesLocked() error {
	movesFile, err := d.store.LoadAggregateMoves(d.id)
	if err != nil {
		d.movesErr = err
		return err
	}
	moves := make(map[int]domain.Move, len(movesFile))
	for moveID, compact := range movesFile {
		moves[moveID] = dataset.MoveToDomain(compact)
	}
	d.moves = moves
	return nil
}

func (d *loadedSnapshot) ensureMoves() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.moves != nil || d.movesErr != nil {
		return d.movesErr
	}
	return d.loadMovesLocked()
}

func (d *loadedSnapshot) ensureEvolutions() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.evolutions != nil || d.evolutionsErr != nil {
		return d.evolutionsErr
	}
	evolutions, err := d.store.LoadAggregateEvolutions(d.id)
	if err != nil {
		d.evolutionsErr = err
		return err
	}
	evoTrees := make(map[int]domain.EvolutionTree, len(evolutions))
	for chainID, compact := range evolutions {
		evoTrees[chainID] = dataset.EvolutionToDomain(compact)
	}
	d.evolutions = evoTrees
	return nil
}

func (d *loadedSnapshot) ensureEncounters() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.encounters != nil || d.encountersErr != nil {
		return d.encountersErr
	}
	encounters, err := d.store.LoadAggregateEncounters(d.id)
	if err != nil {
		d.encountersErr = err
		return err
	}
	enc := make(map[int][]domain.Encounter, len(encounters))
	for pokemonID, rows := range encounters {
		enc[pokemonID] = dataset.EncountersToDomain(rows)
	}
	d.encounters = enc
	return nil
}

func (d *loadedSnapshot) ensureLearnsets() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.learnsets != nil || d.learnsetsErr != nil {
		return d.learnsetsErr
	}
	d.learnsets, d.learnsetsErr = d.store.LoadAggregateLearnsets(d.id)
	return d.learnsetsErr
}

func (s *Store) activeSnapshot() (*loadedSnapshot, error) {
	ds, err := s.getLoadedSnapshot()
	if err != nil {
		return nil, err
	}
	return ds, nil
}

// Learnset returns the persisted level-up learnset for a Pokémon from the active snapshot.
func (s *Store) Learnset(pokemonID int) ([]dataset.DatasetLearn, bool) {
	ds, err := s.activeSnapshot()
	if err != nil || ds == nil {
		return nil, false
	}
	if err := ds.ensureLearnsets(); err != nil {
		return nil, false
	}
	rows, ok := ds.learnsets[pokemonID]
	return rows, ok
}

// LevelUpMoves resolves level-up moves from the active learnset and move map.
func (s *Store) LevelUpMoves(pokemonID int) ([]domain.LevelUpMove, error) {
	ds, err := s.activeSnapshot()
	if err != nil {
		return nil, err
	}
	if ds == nil {
		return nil, ErrNotFound
	}
	if err := ds.ensureLearnsets(); err != nil {
		return nil, err
	}
	rows, ok := ds.learnsets[pokemonID]
	if !ok {
		return nil, ErrNotFound
	}
	if err := ds.ensureMoves(); err != nil {
		return nil, err
	}
	return dataset.LevelUpMovesFromLearnset(rows, func(id int) (domain.Move, error) {
		move, ok := ds.moves[id]
		if !ok {
			return domain.Move{}, fmt.Errorf("%w: move %d", ErrNotFound, id)
		}
		return move, nil
	}, 0)
}

func (s *Store) lookupPokemon(id int) (domain.Pokemon, bool, error) {
	ds, err := s.activeSnapshot()
	if err != nil || ds == nil {
		return domain.Pokemon{}, false, err
	}
	if err := ds.ensurePokemon(); err != nil {
		return domain.Pokemon{}, false, err
	}
	value, ok := ds.pokemon[id]
	return value, ok, nil
}

func (s *Store) lookupSpecies(id int) (domain.Species, bool, error) {
	ds, err := s.activeSnapshot()
	if err != nil || ds == nil {
		return domain.Species{}, false, err
	}
	if err := ds.ensureSpecies(); err != nil {
		return domain.Species{}, false, err
	}
	value, ok := ds.species[id]
	return value, ok, nil
}

func (s *Store) lookupEvolution(id int) (domain.EvolutionTree, bool, error) {
	ds, err := s.activeSnapshot()
	if err != nil || ds == nil {
		return domain.EvolutionTree{}, false, err
	}
	if err := ds.ensureEvolutions(); err != nil {
		return domain.EvolutionTree{}, false, err
	}
	value, ok := ds.evolutions[id]
	return value, ok, nil
}

func (s *Store) lookupEncounters(id int) ([]domain.Encounter, bool, error) {
	ds, err := s.activeSnapshot()
	if err != nil || ds == nil {
		return nil, false, err
	}
	if err := ds.ensureEncounters(); err != nil {
		return nil, false, err
	}
	value, ok := ds.encounters[id]
	if !ok {
		return nil, false, nil
	}
	return value, true, nil
}

func (s *Store) lookupMove(id int) (domain.Move, bool, error) {
	ds, err := s.activeSnapshot()
	if err != nil || ds == nil {
		return domain.Move{}, false, err
	}
	if err := ds.ensureMoves(); err != nil {
		return domain.Move{}, false, err
	}
	value, ok := ds.moves[id]
	return value, ok, nil
}

// ActiveDatasetSchema returns the schema version of the active snapshot, or 0.
func (s *Store) ActiveDatasetSchema() (int, error) {
	id, err := s.getActiveDatasetID()
	if err != nil {
		return 0, err
	}
	man, err := s.ReadDatasetManifest(id)
	if err != nil {
		return 0, err
	}
	return man.SchemaVersion, nil
}

func supportedDatasetSchema(v int) bool {
	return v == dataset.SchemaVersion
}

// DatasetNotice explains why the active dataset is unavailable without deleting it.
type DatasetNotice struct {
	NeedsUpdate bool   `json:"needs_update"`
	Reason      string `json:"reason,omitempty"`
	DatasetID   string `json:"dataset_id,omitempty"`
	Schema      int    `json:"schema,omitempty"`
}

// EnsureCompatibleDataset validates the active snapshot for the current schema.
// Incompatible or corrupt active pointers are deactivated and explained; dataset
// directories and user prefs are preserved. A replacement must build+validate
// separately before ActivateDataset.
func (s *Store) EnsureCompatibleDataset() error {
	if err := s.EnsureRoot(); err != nil {
		return err
	}
	_ = s.clearDatasetNotice()
	id, err := s.ActiveDatasetID()
	if errors.Is(err, ErrNotFound) {
		return s.noteLegacyMetadataIfPresent()
	}
	if err != nil {
		return s.deactivateIncompatibleDataset("", "corrupt active pointer")
	}
	man, err := s.ReadDatasetManifest(id)
	if err != nil {
		return s.deactivateIncompatibleDataset(id, "unreadable dataset manifest")
	}
	if !supportedDatasetSchema(man.SchemaVersion) {
		return s.deactivateIncompatibleDataset(id, fmt.Sprintf("schema %d requires update (app expects %d)", man.SchemaVersion, dataset.SchemaVersion))
	}
	if _, err := os.Stat(s.AggregatePokemonPath(id)); errors.Is(err, os.ErrNotExist) {
		return s.deactivateIncompatibleDataset(id, "missing aggregate files")
	}
	return s.noteLegacyMetadataIfPresent()
}

func (s *Store) noteLegacyMetadataIfPresent() error {
	legacyDirs := []string{"pokemon", "species", "moves", "evolution", "encounters"}
	for _, dir := range legacyDirs {
		path := filepath.Join(s.root, dir)
		if _, err := os.Stat(path); err == nil {
			// Do not delete; surface as update notice only when nothing is active.
			if !s.HasActiveDataset() {
				return s.writeDatasetNotice(DatasetNotice{
					NeedsUpdate: true,
					Reason:      "legacy metadata tree present; rebuild dataset to continue",
				})
			}
			return nil
		}
	}
	return nil
}

func (s *Store) deactivateIncompatibleDataset(id, reason string) error {
	debuglog.Cachef("dataset.deactivate incompatible: id=%s %s", id, reason)
	s.setActiveDatasetID("")
	_ = os.Remove(s.activePath())
	s.invalidateLoadedSnapshot()
	schema := 0
	if id != "" {
		if man, err := s.ReadDatasetManifest(id); err == nil {
			schema = man.SchemaVersion
		}
	}
	return s.writeDatasetNotice(DatasetNotice{
		NeedsUpdate: true,
		Reason:      reason,
		DatasetID:   id,
		Schema:      schema,
	})
}

func (s *Store) datasetNoticePath() string {
	return filepath.Join(s.root, "dataset_notice.json")
}

func (s *Store) writeDatasetNotice(n DatasetNotice) error {
	data, err := json.Marshal(n)
	if err != nil {
		return err
	}
	return writeAtomic(s.datasetNoticePath(), data, 0o600)
}

func (s *Store) clearDatasetNotice() error {
	err := os.Remove(s.datasetNoticePath())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// ReadDatasetNotice returns a pending update notice, if any.
func (s *Store) ReadDatasetNotice() (DatasetNotice, error) {
	data, err := os.ReadFile(s.datasetNoticePath())
	if errors.Is(err, os.ErrNotExist) {
		return DatasetNotice{}, ErrNotFound
	}
	if err != nil {
		return DatasetNotice{}, err
	}
	var n DatasetNotice
	if err := json.Unmarshal(data, &n); err != nil {
		return DatasetNotice{}, fmt.Errorf("%w: dataset notice: %v", ErrCorrupt, err)
	}
	return n, nil
}

// PreloadDataset loads core aggregate files into memory (optional warm path).
func (s *Store) PreloadDataset() error {
	ds, err := s.activeSnapshot()
	if err != nil || ds == nil {
		return err
	}
	if err := ds.ensurePokemon(); err != nil {
		return err
	}
	if err := ds.ensureSpecies(); err != nil {
		return err
	}
	if err := ds.ensureEvolutions(); err != nil {
		return err
	}
	if err := ds.ensureEncounters(); err != nil {
		return err
	}
	return nil
}
