package cache

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/Graphixa/Pokedex/internal/dataset"
	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/domain"
)

type v3Dataset struct {
	store *Store
	id    string

	mu sync.Mutex

	pokemon    map[int]domain.Pokemon
	pokemonErr error
	species    map[int]domain.Species
	speciesErr error
	moves      map[int]domain.Move
	movesErr   error
	evolutions map[int]domain.EvolutionTree
	evolutionsErr error
	encounters map[int][]domain.Encounter
	encountersErr error
	learnsets  map[int][]dataset.DatasetLearn
	learnsetsErr error
}

func (s *Store) invalidateV3Dataset() {
	s.v3Mu.Lock()
	defer s.v3Mu.Unlock()
	s.v3 = nil
	s.v3LoadErr = nil
}

func (s *Store) v3DatasetLoaded() (*v3Dataset, error) {
	s.v3Mu.RLock()
	if s.v3 != nil || s.v3LoadErr != nil {
		ds, err := s.v3, s.v3LoadErr
		s.v3Mu.RUnlock()
		return ds, err
	}
	s.v3Mu.RUnlock()

	s.v3Mu.Lock()
	defer s.v3Mu.Unlock()
	if s.v3 != nil || s.v3LoadErr != nil {
		return s.v3, s.v3LoadErr
	}
	ds, err := s.openV3DatasetLocked()
	s.v3 = ds
	s.v3LoadErr = err
	return ds, err
}

func (s *Store) openV3DatasetLocked() (*v3Dataset, error) {
	id, err := s.getActiveDatasetID()
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	man, err := s.ReadDatasetManifest(id)
	if err != nil {
		return nil, err
	}
	if man.SchemaVersion != dataset.SchemaVersion {
		return nil, nil
	}
	return &v3Dataset{store: s, id: id}, nil
}

func (d *v3Dataset) ensurePokemon() error {
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
	if d.learnsets == nil && d.learnsetsErr == nil {
		d.learnsets, d.learnsetsErr = d.store.LoadAggregateLearnsets(d.id)
	}
	if d.learnsetsErr != nil {
		d.pokemonErr = d.learnsetsErr
		return d.learnsetsErr
	}
	if d.moves == nil && d.movesErr == nil {
		if err := d.loadMovesLocked(); err != nil {
			d.pokemonErr = err
			return err
		}
	}
	if d.movesErr != nil {
		d.pokemonErr = d.movesErr
		return d.movesErr
	}
	moveSlugs := make(map[int]string, len(d.moves))
	for moveID, move := range d.moves {
		moveSlugs[moveID] = move.Slug
	}
	pokemon := make(map[int]domain.Pokemon, len(pokemonFile))
	for pokemonID, compact := range pokemonFile {
		rows := d.learnsets[pokemonID]
		stubs := dataset.LearnedMovesFromLearnset(rows, moveSlugs)
		pokemon[pokemonID] = dataset.PokemonToDomain(compact, stubs)
	}
	d.pokemon = pokemon
	return nil
}

func (d *v3Dataset) ensureSpecies() error {
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

func (d *v3Dataset) loadMovesLocked() error {
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

func (d *v3Dataset) ensureMoves() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.moves != nil || d.movesErr != nil {
		return d.movesErr
	}
	return d.loadMovesLocked()
}

func (d *v3Dataset) ensureEvolutions() error {
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

func (d *v3Dataset) ensureEncounters() error {
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

func (d *v3Dataset) ensureLearnsets() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.learnsets != nil || d.learnsetsErr != nil {
		return d.learnsetsErr
	}
	d.learnsets, d.learnsetsErr = d.store.LoadAggregateLearnsets(d.id)
	return d.learnsetsErr
}

func (s *Store) activeV3() (*v3Dataset, error) {
	ds, err := s.v3DatasetLoaded()
	if err != nil {
		return nil, err
	}
	return ds, nil
}

// Learnset returns the persisted level-up learnset for a Pokémon when v3 is active.
func (s *Store) Learnset(pokemonID int) ([]dataset.DatasetLearn, bool) {
	ds, err := s.activeV3()
	if err != nil || ds == nil {
		return nil, false
	}
	if err := ds.ensureLearnsets(); err != nil {
		return nil, false
	}
	rows, ok := ds.learnsets[pokemonID]
	return rows, ok
}

// LevelUpMoves resolves level-up moves from the v3 learnset and move map.
func (s *Store) LevelUpMoves(pokemonID int) ([]domain.LevelUpMove, error) {
	ds, err := s.activeV3()
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
	})
}

func (s *Store) pokemonV3(id int) (domain.Pokemon, bool, error) {
	ds, err := s.activeV3()
	if err != nil || ds == nil {
		return domain.Pokemon{}, false, err
	}
	if err := ds.ensurePokemon(); err != nil {
		return domain.Pokemon{}, false, err
	}
	value, ok := ds.pokemon[id]
	return value, ok, nil
}

func (s *Store) speciesV3(id int) (domain.Species, bool, error) {
	ds, err := s.activeV3()
	if err != nil || ds == nil {
		return domain.Species{}, false, err
	}
	if err := ds.ensureSpecies(); err != nil {
		return domain.Species{}, false, err
	}
	value, ok := ds.species[id]
	return value, ok, nil
}

func (s *Store) evolutionV3(id int) (domain.EvolutionTree, bool, error) {
	ds, err := s.activeV3()
	if err != nil || ds == nil {
		return domain.EvolutionTree{}, false, err
	}
	if err := ds.ensureEvolutions(); err != nil {
		return domain.EvolutionTree{}, false, err
	}
	value, ok := ds.evolutions[id]
	return value, ok, nil
}

func (s *Store) encountersV3(id int) ([]domain.Encounter, bool, error) {
	ds, err := s.activeV3()
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

func (s *Store) moveV3(id int) (domain.Move, bool, error) {
	ds, err := s.activeV3()
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

// EnsureCompatibleDataset removes an active pointer when the snapshot uses an
// unsupported schema. Pre-release policy: discard incompatible data and rebuild
// rather than migrate v2→v3.
func (s *Store) EnsureCompatibleDataset() error {
	if err := s.EnsureRoot(); err != nil {
		return err
	}
	id, err := s.ActiveDatasetID()
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return s.resetIncompatibleDataset("corrupt active pointer")
	}
	man, err := s.ReadDatasetManifest(id)
	if err != nil || !supportedDatasetSchema(man.SchemaVersion) {
		return s.resetIncompatibleDataset(fmt.Sprintf("schema=%d id=%s", man.SchemaVersion, id))
	}
	if _, err := os.Stat(s.AggregatePokemonPath(id)); errors.Is(err, os.ErrNotExist) {
		return s.resetIncompatibleDataset(fmt.Sprintf("missing v3 aggregates id=%s", id))
	}
	return nil
}

func (s *Store) resetIncompatibleDataset(reason string) error {
	debuglog.Cachef("dataset.reset incompatible: %s", reason)
	if err := os.Remove(s.activePath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	s.invalidateActiveDatasetCache()
	return nil
}

// PreloadV3Dataset loads all v3 aggregate files into memory (optional warm path).
func (s *Store) PreloadV3Dataset() error {
	ds, err := s.activeV3()
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
