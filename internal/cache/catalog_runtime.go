package cache

import (
	"fmt"

	"github.com/Graphixa/Pokedex/internal/dataset"
	"github.com/Graphixa/Pokedex/internal/domain"
)

func (d *loadedSnapshot) ensureVersions() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.versions != nil || d.versionsErr != nil {
		return d.versionsErr
	}
	raw, err := d.store.LoadAggregateVersions(d.id)
	if err != nil {
		d.versionsErr = err
		return err
	}
	out := make(map[int]domain.Version, len(raw))
	for id, v := range raw {
		out[id] = dataset.VersionToDomain(v)
	}
	d.versions = out
	return nil
}

func (d *loadedSnapshot) ensureForms() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.forms != nil || d.formsErr != nil {
		return d.formsErr
	}
	raw, err := d.store.LoadAggregateForms(d.id)
	if err != nil {
		// Forms optional on incomplete/test snapshots.
		if err == ErrNotFound {
			d.forms = map[int]domain.PokemonForm{}
			return nil
		}
		d.formsErr = err
		return err
	}
	out := make(map[int]domain.PokemonForm, len(raw))
	for id, f := range raw {
		out[id] = dataset.FormToDomain(f)
	}
	d.forms = out
	return nil
}

// CatalogVersions returns chronologically ordered game versions from the active dataset.
func (s *Store) CatalogVersions() ([]domain.Version, error) {
	ds, err := s.activeSnapshot()
	if err != nil || ds == nil {
		return nil, err
	}
	if err := ds.ensureVersions(); err != nil {
		if err == ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	raw, err := ds.store.LoadAggregateVersions(ds.id)
	if err != nil {
		if err == ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	ordered := dataset.VersionsChronological(raw)
	out := make([]domain.Version, 0, len(ordered))
	for _, v := range ordered {
		out = append(out, dataset.VersionToDomain(v))
	}
	return out, nil
}

// VersionGroupIDForVersion resolves a version id to its group id.
func (s *Store) VersionGroupIDForVersion(versionID int) (int, bool) {
	ds, err := s.activeSnapshot()
	if err != nil || ds == nil {
		return 0, false
	}
	if err := ds.ensureVersions(); err != nil {
		return 0, false
	}
	v, ok := ds.versions[versionID]
	if !ok || v.VersionGroupID < 1 {
		return 0, false
	}
	return v.VersionGroupID, true
}

// SelectableFormsForSpecies builds dossier form choices for a species id.
func (s *Store) SelectableFormsForSpecies(speciesID int) ([]domain.SelectableForm, error) {
	ds, err := s.activeSnapshot()
	if err != nil || ds == nil {
		return nil, err
	}
	if err := ds.ensureSpecies(); err != nil {
		return nil, err
	}
	if err := ds.ensurePokemon(); err != nil {
		return nil, err
	}
	_ = ds.ensureForms()
	sp, ok := ds.species[speciesID]
	if !ok {
		return nil, fmt.Errorf("%w: species %d", ErrNotFound, speciesID)
	}
	compactSp := dataset.SpeciesFromDomain(sp)
	pokemonFile, err := ds.store.LoadAggregatePokemon(ds.id)
	if err != nil {
		return nil, err
	}
	formsFile, err := ds.store.LoadAggregateForms(ds.id)
	if err != nil && err != ErrNotFound {
		return nil, err
	}
	if formsFile == nil {
		formsFile = map[int]dataset.DatasetForm{}
	}
	return dataset.SelectableForms(compactSp, formsFile, pokemonFile), nil
}

// LevelUpMovesForGroup resolves level-up moves for a Pokémon and version group.
func (s *Store) LevelUpMovesForGroup(pokemonID, versionGroupID int) ([]domain.LevelUpMove, error) {
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
	}, versionGroupID)
}

func (d *loadedSnapshot) ensureTypes() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.types != nil || d.typesErr != nil {
		return d.typesErr
	}
	raw, err := d.store.LoadAggregateTypes(d.id)
	if err != nil {
		if err == ErrNotFound {
			d.types = map[string]domain.TypeInfo{}
			return nil
		}
		d.typesErr = err
		return err
	}
	out := make(map[string]domain.TypeInfo, len(raw))
	for _, t := range raw {
		ti := dataset.TypeToDomain(t)
		out[ti.Slug] = ti
	}
	d.types = out
	return nil
}

func (d *loadedSnapshot) ensureAbilities() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.abilities != nil || d.abilitiesErr != nil {
		return d.abilitiesErr
	}
	raw, err := d.store.LoadAggregateAbilities(d.id)
	if err != nil {
		if err == ErrNotFound {
			d.abilities = map[int]domain.Ability{}
			return nil
		}
		d.abilitiesErr = err
		return err
	}
	out := make(map[int]domain.Ability, len(raw))
	for id, a := range raw {
		out[id] = dataset.AbilityToDomain(a)
	}
	d.abilities = out
	return nil
}

// TypeChart returns defensive type relations keyed by type slug.
func (s *Store) TypeChart() (map[string]domain.TypeInfo, error) {
	ds, err := s.activeSnapshot()
	if err != nil || ds == nil {
		return nil, err
	}
	if err := ds.ensureTypes(); err != nil {
		return nil, err
	}
	return ds.types, nil
}

// AbilityByID returns a cached ability.
func (s *Store) AbilityByID(id int) (domain.Ability, bool) {
	ds, err := s.activeSnapshot()
	if err != nil || ds == nil {
		return domain.Ability{}, false
	}
	if err := ds.ensureAbilities(); err != nil {
		return domain.Ability{}, false
	}
	a, ok := ds.abilities[id]
	return a, ok
}

// AbilityBySlug returns a cached ability by slug.
func (s *Store) AbilityBySlug(slug string) (domain.Ability, bool) {
	if slug == "" {
		return domain.Ability{}, false
	}
	ds, err := s.activeSnapshot()
	if err != nil || ds == nil {
		return domain.Ability{}, false
	}
	if err := ds.ensureAbilities(); err != nil {
		return domain.Ability{}, false
	}
	for _, a := range ds.abilities {
		if a.Slug == slug {
			return a, true
		}
	}
	return domain.Ability{}, false
}

// MovesForGroup returns all learn methods for a Pokémon in a version group.
func (s *Store) MovesForGroup(pokemonID, versionGroupID int) ([]domain.LevelUpMove, error) {
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
	return dataset.MovesFromLearnset(rows, func(id int) (domain.Move, error) {
		move, ok := ds.moves[id]
		if !ok {
			return domain.Move{}, fmt.Errorf("%w: move %d", ErrNotFound, id)
		}
		return move, nil
	}, versionGroupID)
}

// (Service wrappers)

func (s *Service) CatalogVersions() ([]domain.Version, error) {
	return s.store.CatalogVersions()
}

func (s *Service) VersionGroupIDForVersion(versionID int) (int, bool) {
	return s.store.VersionGroupIDForVersion(versionID)
}

func (s *Service) SelectableFormsForSpecies(speciesID int) ([]domain.SelectableForm, error) {
	return s.store.SelectableFormsForSpecies(speciesID)
}

func (s *Service) LevelUpMovesForGroup(pokemonID, versionGroupID int) ([]domain.LevelUpMove, error) {
	return s.store.LevelUpMovesForGroup(pokemonID, versionGroupID)
}

func (s *Service) MovesForGroup(pokemonID, versionGroupID int) ([]domain.LevelUpMove, error) {
	return s.store.MovesForGroup(pokemonID, versionGroupID)
}

func (s *Service) TypeChart() (map[string]domain.TypeInfo, error) {
	return s.store.TypeChart()
}

func (s *Service) AbilityByID(id int) (domain.Ability, bool) {
	return s.store.AbilityByID(id)
}

func (s *Service) AbilityBySlug(slug string) (domain.Ability, bool) {
	return s.store.AbilityBySlug(slug)
}
