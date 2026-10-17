package cache

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/Graphixa/Pokedex/internal/dataset"
)

func (s *Store) aggregatePath(datasetID, filename string) string {
	return filepath.Join(s.datasetDir(datasetID), filename)
}

func (s *Store) AggregatePokemonPath(datasetID string) string {
	return s.aggregatePath(datasetID, dataset.FilePokemon)
}

func (s *Store) AggregateSpeciesPath(datasetID string) string {
	return s.aggregatePath(datasetID, dataset.FileSpecies)
}

func (s *Store) AggregateLearnsetsPath(datasetID string) string {
	return s.aggregatePath(datasetID, dataset.FileLearnsets)
}

func (s *Store) AggregateMovesPath(datasetID string) string {
	return s.aggregatePath(datasetID, dataset.FileMoves)
}

func (s *Store) AggregateEvolutionsPath(datasetID string) string {
	return s.aggregatePath(datasetID, dataset.FileEvolutions)
}

func (s *Store) AggregateEncountersPath(datasetID string) string {
	return s.aggregatePath(datasetID, dataset.FileEncounters)
}

func (s *Store) AggregateVersionsPath(datasetID string) string {
	return s.aggregatePath(datasetID, dataset.FileVersions)
}

func (s *Store) AggregateVersionGroupsPath(datasetID string) string {
	return s.aggregatePath(datasetID, dataset.FileVersionGroups)
}

func (s *Store) AggregateTypesPath(datasetID string) string {
	return s.aggregatePath(datasetID, dataset.FileTypes)
}

func (s *Store) AggregateAbilitiesPath(datasetID string) string {
	return s.aggregatePath(datasetID, dataset.FileAbilities)
}

func (s *Store) AggregateFormsPath(datasetID string) string {
	return s.aggregatePath(datasetID, dataset.FileForms)
}

func writeAggregateJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return writeAtomicRename(path, data, 0o600)
}

func readAggregateJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, value); err != nil {
		return err
	}
	return nil
}

// StoreAggregatePokemon writes pokemon.json for a staging snapshot.
func (s *Store) StoreAggregatePokemon(datasetID string, records map[int]dataset.DatasetPokemon) error {
	return writeAggregateJSON(s.AggregatePokemonPath(datasetID), records)
}

// StoreAggregateSpecies writes species.json for a staging snapshot.
func (s *Store) StoreAggregateSpecies(datasetID string, records map[int]dataset.DatasetSpecies) error {
	return writeAggregateJSON(s.AggregateSpeciesPath(datasetID), records)
}

// StoreAggregateLearnsets writes learnsets.json for a staging snapshot.
func (s *Store) StoreAggregateLearnsets(datasetID string, records map[int][]dataset.DatasetLearn) error {
	return writeAggregateJSON(s.AggregateLearnsetsPath(datasetID), records)
}

// StoreAggregateMoves writes moves.json for a staging snapshot.
func (s *Store) StoreAggregateMoves(datasetID string, records map[int]dataset.DatasetMove) error {
	return writeAggregateJSON(s.AggregateMovesPath(datasetID), records)
}

// StoreAggregateEvolutions writes evolutions.json for a staging snapshot.
func (s *Store) StoreAggregateEvolutions(datasetID string, records map[int]dataset.DatasetEvolutionTree) error {
	return writeAggregateJSON(s.AggregateEvolutionsPath(datasetID), records)
}

// StoreAggregateEncounters writes encounters.json for a staging snapshot.
func (s *Store) StoreAggregateEncounters(datasetID string, records map[int][]dataset.DatasetEncounter) error {
	return writeAggregateJSON(s.AggregateEncountersPath(datasetID), records)
}

// LoadAggregatePokemon reads pokemon.json from a snapshot.
func (s *Store) LoadAggregatePokemon(datasetID string) (map[int]dataset.DatasetPokemon, error) {
	var records map[int]dataset.DatasetPokemon
	if err := readAggregateJSON(s.AggregatePokemonPath(datasetID), &records); err != nil {
		return nil, err
	}
	return records, nil
}

// LoadAggregateSpecies reads species.json from a snapshot.
func (s *Store) LoadAggregateSpecies(datasetID string) (map[int]dataset.DatasetSpecies, error) {
	var records map[int]dataset.DatasetSpecies
	if err := readAggregateJSON(s.AggregateSpeciesPath(datasetID), &records); err != nil {
		return nil, err
	}
	return records, nil
}

// LoadAggregateLearnsets reads learnsets.json from a snapshot.
func (s *Store) LoadAggregateLearnsets(datasetID string) (map[int][]dataset.DatasetLearn, error) {
	var records map[int][]dataset.DatasetLearn
	if err := readAggregateJSON(s.AggregateLearnsetsPath(datasetID), &records); err != nil {
		return nil, err
	}
	return records, nil
}

// LoadAggregateMoves reads moves.json from a snapshot.
func (s *Store) LoadAggregateMoves(datasetID string) (map[int]dataset.DatasetMove, error) {
	var records map[int]dataset.DatasetMove
	if err := readAggregateJSON(s.AggregateMovesPath(datasetID), &records); err != nil {
		return nil, err
	}
	return records, nil
}

// LoadAggregateEvolutions reads evolutions.json from a snapshot.
func (s *Store) LoadAggregateEvolutions(datasetID string) (map[int]dataset.DatasetEvolutionTree, error) {
	var records map[int]dataset.DatasetEvolutionTree
	if err := readAggregateJSON(s.AggregateEvolutionsPath(datasetID), &records); err != nil {
		return nil, err
	}
	return records, nil
}

// LoadAggregateEncounters reads encounters.json from a snapshot.
func (s *Store) LoadAggregateEncounters(datasetID string) (map[int][]dataset.DatasetEncounter, error) {
	var records map[int][]dataset.DatasetEncounter
	if err := readAggregateJSON(s.AggregateEncountersPath(datasetID), &records); err != nil {
		return nil, err
	}
	return records, nil
}

// StoreAggregateVersions writes versions.json for a staging snapshot.
func (s *Store) StoreAggregateVersions(datasetID string, records map[int]dataset.DatasetVersion) error {
	return writeAggregateJSON(s.AggregateVersionsPath(datasetID), records)
}

func (s *Store) StoreAggregateVersionGroups(datasetID string, records map[int]dataset.DatasetVersionGroup) error {
	return writeAggregateJSON(s.AggregateVersionGroupsPath(datasetID), records)
}

func (s *Store) StoreAggregateTypes(datasetID string, records map[int]dataset.DatasetType) error {
	return writeAggregateJSON(s.AggregateTypesPath(datasetID), records)
}

func (s *Store) StoreAggregateAbilities(datasetID string, records map[int]dataset.DatasetAbility) error {
	return writeAggregateJSON(s.AggregateAbilitiesPath(datasetID), records)
}

func (s *Store) StoreAggregateForms(datasetID string, records map[int]dataset.DatasetForm) error {
	return writeAggregateJSON(s.AggregateFormsPath(datasetID), records)
}

func (s *Store) LoadAggregateVersions(datasetID string) (map[int]dataset.DatasetVersion, error) {
	var records map[int]dataset.DatasetVersion
	if err := readAggregateJSON(s.AggregateVersionsPath(datasetID), &records); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *Store) LoadAggregateVersionGroups(datasetID string) (map[int]dataset.DatasetVersionGroup, error) {
	var records map[int]dataset.DatasetVersionGroup
	if err := readAggregateJSON(s.AggregateVersionGroupsPath(datasetID), &records); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *Store) LoadAggregateTypes(datasetID string) (map[int]dataset.DatasetType, error) {
	var records map[int]dataset.DatasetType
	if err := readAggregateJSON(s.AggregateTypesPath(datasetID), &records); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *Store) LoadAggregateAbilities(datasetID string) (map[int]dataset.DatasetAbility, error) {
	var records map[int]dataset.DatasetAbility
	if err := readAggregateJSON(s.AggregateAbilitiesPath(datasetID), &records); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *Store) LoadAggregateForms(datasetID string) (map[int]dataset.DatasetForm, error) {
	var records map[int]dataset.DatasetForm
	if err := readAggregateJSON(s.AggregateFormsPath(datasetID), &records); err != nil {
		return nil, err
	}
	return records, nil
}

// StoreAggregateBuilder writes all aggregate files from a completed builder.
func (s *Store) StoreAggregateBuilder(datasetID string, b *dataset.Builder) error {
	if b == nil {
		return errors.New("cache: nil dataset builder")
	}
	if err := s.StoreAggregatePokemon(datasetID, b.Pokemon); err != nil {
		return err
	}
	if err := s.StoreAggregateSpecies(datasetID, b.Species); err != nil {
		return err
	}
	if err := s.StoreAggregateLearnsets(datasetID, b.Learnsets); err != nil {
		return err
	}
	if err := s.StoreAggregateMoves(datasetID, b.Moves); err != nil {
		return err
	}
	if err := s.StoreAggregateEvolutions(datasetID, b.Evolutions); err != nil {
		return err
	}
	if err := s.StoreAggregateEncounters(datasetID, b.Encounters); err != nil {
		return err
	}
	if err := s.StoreAggregateVersions(datasetID, b.Versions); err != nil {
		return err
	}
	if err := s.StoreAggregateVersionGroups(datasetID, b.VersionGroups); err != nil {
		return err
	}
	if err := s.StoreAggregateTypes(datasetID, b.Types); err != nil {
		return err
	}
	if err := s.StoreAggregateAbilities(datasetID, b.Abilities); err != nil {
		return err
	}
	if err := s.StoreAggregateForms(datasetID, b.Forms); err != nil {
		return err
	}
	return nil
}
