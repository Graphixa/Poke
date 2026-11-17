package cache

import (
	"fmt"

	"github.com/Graphixa/Poke/internal/dataset"
	"github.com/Graphixa/Poke/internal/domain"
)

// ResolveDossierContext loads catalog maps and applies shared form/game resolution.
func (s *Store) ResolveDossierContext(speciesID, pokemonID, formID, gameVersionID int) (domain.ResolvedDossierContext, error) {
	ds, err := s.activeSnapshot()
	if err != nil || ds == nil {
		return domain.ResolvedDossierContext{}, err
	}
	if err := ds.ensureSpecies(); err != nil {
		return domain.ResolvedDossierContext{}, err
	}
	sp, ok := ds.species[speciesID]
	if !ok {
		return domain.ResolvedDossierContext{}, fmt.Errorf("%w: species %d", ErrNotFound, speciesID)
	}
	compactSp := dataset.SpeciesFromDomain(sp)

	if err := ds.ensureVersions(); err != nil {
		return domain.ResolvedDossierContext{}, err
	}
	if err := ds.ensureForms(); err != nil {
		return domain.ResolvedDossierContext{}, err
	}
	if err := ds.ensurePokemon(); err != nil {
		return domain.ResolvedDossierContext{}, err
	}

	versions := make(map[int]dataset.DatasetVersion, len(ds.versions))
	for id, v := range ds.versions {
		versions[id] = dataset.VersionFromDomain(v)
	}
	groups, err := s.LoadAggregateVersionGroups(ds.id)
	if err != nil && err != ErrNotFound {
		return domain.ResolvedDossierContext{}, err
	}
	if groups == nil {
		groups = map[int]dataset.DatasetVersionGroup{}
	}
	forms := make(map[int]dataset.DatasetForm, len(ds.forms))
	for id, f := range ds.forms {
		forms[id] = dataset.FormFromDomain(f)
	}
	pokemon := make(map[int]dataset.DatasetPokemon, len(ds.pokemon))
	for id, p := range ds.pokemon {
		pokemon[id] = dataset.PokemonFromDomain(p)
	}

	return dataset.ResolveDossierContext(dataset.ResolveInput{
		Species:       compactSp,
		PokemonID:     pokemonID,
		FormID:        formID,
		GameVersionID: gameVersionID,
		Versions:      versions,
		VersionGroups: groups,
		Forms:         forms,
		Pokemon:       pokemon,
	}), nil
}

// FormOptionsForGame returns picker choices with compatibility for the game.
func (s *Store) FormOptionsForGame(speciesID, gameVersionID int) ([]domain.FormOption, error) {
	ds, err := s.activeSnapshot()
	if err != nil || ds == nil {
		return nil, err
	}
	if err := ds.ensureSpecies(); err != nil {
		return nil, err
	}
	sp, ok := ds.species[speciesID]
	if !ok {
		return nil, fmt.Errorf("%w: species %d", ErrNotFound, speciesID)
	}
	if err := ds.ensureVersions(); err != nil {
		return nil, err
	}
	if err := ds.ensureForms(); err != nil {
		return nil, err
	}
	if err := ds.ensurePokemon(); err != nil {
		return nil, err
	}
	versions := make(map[int]dataset.DatasetVersion, len(ds.versions))
	for id, v := range ds.versions {
		versions[id] = dataset.VersionFromDomain(v)
	}
	groups, err := s.LoadAggregateVersionGroups(ds.id)
	if err != nil && err != ErrNotFound {
		return nil, err
	}
	if groups == nil {
		groups = map[int]dataset.DatasetVersionGroup{}
	}
	forms := make(map[int]dataset.DatasetForm, len(ds.forms))
	for id, f := range ds.forms {
		forms[id] = dataset.FormFromDomain(f)
	}
	pokemon := make(map[int]dataset.DatasetPokemon, len(ds.pokemon))
	for id, p := range ds.pokemon {
		pokemon[id] = dataset.PokemonFromDomain(p)
	}
	return dataset.FormOptionsForGame(dataset.ResolveInput{
		Species:       dataset.SpeciesFromDomain(sp),
		GameVersionID: gameVersionID,
		Versions:      versions,
		VersionGroups: groups,
		Forms:         forms,
		Pokemon:       pokemon,
	}), nil
}

func (s *Service) ResolveDossierContext(speciesID, pokemonID, formID, gameVersionID int) (domain.ResolvedDossierContext, error) {
	return s.store.ResolveDossierContext(speciesID, pokemonID, formID, gameVersionID)
}

func (s *Service) FormOptionsForGame(speciesID, gameVersionID int) ([]domain.FormOption, error) {
	return s.store.FormOptionsForGame(speciesID, gameVersionID)
}

func (s *Service) ReadDatasetNotice() (DatasetNotice, error) {
	return s.store.ReadDatasetNotice()
}
