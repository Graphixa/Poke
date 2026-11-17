package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Graphixa/Poke/internal/debuglog"
	"github.com/Graphixa/Poke/internal/domain"
)

// DossierCoreResult is the fast Enter path: species+pokemon, optionally complete.
type DossierCoreResult struct {
	Dossier  domain.Dossier
	Complete bool // true when evolution, encounters, and moves are included
}

// Dossier returns a species dossier assembled from the active dataset.
func (s *Service) Dossier(ctx context.Context, speciesID int) (domain.Dossier, error) {
	v, err := s.group.Do(fmt.Sprintf("dossier:%d", speciesID), func() (any, error) {
		return s.getDossier(ctx, speciesID, "")
	})
	if err != nil {
		return domain.Dossier{}, err
	}
	return v.(domain.Dossier), nil
}

// DossierWithSlug is like Dossier but accepts a known slug for logging.
func (s *Service) DossierWithSlug(ctx context.Context, speciesID int, slug string) (domain.Dossier, error) {
	v, err := s.group.Do(fmt.Sprintf("dossier:%d", speciesID), func() (any, error) {
		return s.getDossier(ctx, speciesID, slug)
	})
	if err != nil {
		return domain.Dossier{}, err
	}
	return v.(domain.Dossier), nil
}

// DossierCore loads species+pokemon quickly for the dialog Summary tab.
func (s *Service) DossierCore(ctx context.Context, speciesID int, slug string) (DossierCoreResult, error) {
	v, err := s.group.Do(fmt.Sprintf("dossier-core:%d", speciesID), func() (any, error) {
		return s.getDossierCore(ctx, speciesID, slug)
	})
	if err != nil {
		return DossierCoreResult{}, err
	}
	return v.(DossierCoreResult), nil
}

// DossierExtras fills evolution, encounters, and level-up moves for a core dossier.
func (s *Service) DossierExtras(ctx context.Context, speciesID int, slug string, core domain.Dossier) (domain.Dossier, error) {
	v, err := s.group.Do(fmt.Sprintf("dossier-extras:%d", speciesID), func() (any, error) {
		return s.getDossierExtras(ctx, slug, core)
	})
	if err != nil {
		return domain.Dossier{}, err
	}
	return v.(domain.Dossier), nil
}

func (s *Service) getDossier(ctx context.Context, speciesID int, slug string) (domain.Dossier, error) {
	core, err := s.getDossierCore(ctx, speciesID, slug)
	if err != nil {
		return domain.Dossier{}, err
	}
	if core.Complete {
		return core.Dossier, nil
	}
	return s.getDossierExtras(ctx, slug, core.Dossier)
}

func (s *Service) getDossierCore(ctx context.Context, speciesID int, slug string) (DossierCoreResult, error) {
	_ = slug
	if s.store.HasActiveDataset() || s.localOnly.Load() {
		d, err := s.assembleCore(ctx, speciesID)
		if err != nil {
			return DossierCoreResult{}, err
		}
		return DossierCoreResult{Dossier: d, Complete: false}, nil
	}
	d, err := s.assembleCore(ctx, speciesID)
	if err != nil {
		return DossierCoreResult{}, err
	}
	return DossierCoreResult{Dossier: d, Complete: false}, nil
}

func (s *Service) getDossierExtras(ctx context.Context, slug string, core domain.Dossier) (domain.Dossier, error) {
	_ = slug
	complete, err := s.dossierComplete(core)
	if err != nil {
		return domain.Dossier{}, err
	}
	if complete {
		return core, nil
	}
	return s.assembleExtras(ctx, core)
}

func (s *Service) dossierComplete(d domain.Dossier) (bool, error) {
	evoDone := d.Evolution != nil || d.Species.EvolutionChainID == 0
	encDone := d.Encounters != nil
	if !evoDone || !encDone {
		return false, nil
	}
	pokemonID := d.Pokemon.ID
	if pokemonID == 0 {
		pokemonID = d.Species.DefaultPokemonID
	}
	if pokemonID == 0 {
		pokemonID = d.Species.ID
	}
	if len(d.LevelUpMoves) > 0 {
		return true, nil
	}
	rows, ok := s.store.Learnset(pokemonID)
	if !ok {
		return true, nil
	}
	if len(rows) == 0 {
		return true, nil
	}
	moves, err := s.store.LevelUpMoves(pokemonID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	_ = moves
	return false, nil
}

// WarmCore fetches and caches pokemon+species only (no evolution/encounters/moves).
func (s *Service) WarmCore(ctx context.Context, speciesID int) error {
	if speciesID < 1 {
		return fmt.Errorf("cache: invalid species id")
	}
	species, err := s.Species(ctx, speciesID)
	if err != nil {
		return err
	}
	pokemonID := species.DefaultPokemonID
	if pokemonID == 0 {
		pokemonID = speciesID
	}
	_, err = s.Pokemon(ctx, pokemonID)
	return err
}

func (s *Service) assembleCore(ctx context.Context, speciesID int) (domain.Dossier, error) {
	var (
		species    domain.Species
		pokemon    domain.Pokemon
		speciesErr error
		pokemonErr error
		wg         sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		species, speciesErr = s.Species(ctx, speciesID)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		pokemon, pokemonErr = s.Pokemon(ctx, speciesID)
	}()
	wg.Wait()
	if speciesErr != nil {
		return domain.Dossier{}, speciesErr
	}
	pokemonID := species.DefaultPokemonID
	if pokemonID == 0 {
		pokemonID = speciesID
	}
	if pokemonErr != nil || pokemon.ID != pokemonID {
		var err error
		pokemon, err = s.Pokemon(ctx, pokemonID)
		if err != nil {
			return domain.Dossier{}, err
		}
	}
	if species.DisplayName != "" {
		pokemon.DisplayName = species.DisplayName
	}
	debuglog.Cachef("detail.cache.hit dossier-core id=%d", speciesID)
	return domain.Dossier{Pokemon: pokemon, Species: species}, nil
}

func (s *Service) assembleExtras(ctx context.Context, d domain.Dossier) (domain.Dossier, error) {
	species := d.Species
	pokemon := d.Pokemon
	pokemonID := pokemon.ID
	if pokemonID == 0 {
		pokemonID = species.DefaultPokemonID
	}
	if pokemonID == 0 {
		pokemonID = species.ID
	}

	var (
		wg         sync.WaitGroup
		evoTree    domain.EvolutionTree
		evoOK      bool
		encounters []domain.Encounter
		encOK      bool
		moves      []domain.LevelUpMove
		movesOK    bool
	)
	if species.EvolutionChainID != 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tree, evoErr := s.EvolutionChain(ctx, species.EvolutionChainID)
			if evoErr == nil {
				evoTree = tree
				evoOK = true
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		enc, encErr := s.Encounters(ctx, pokemonID)
		if encErr == nil {
			encounters = enc
			encOK = true
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		rows, err := s.levelUpMovesForPokemon(pokemonID)
		if err == nil {
			moves = rows
			movesOK = true
		}
	}()
	wg.Wait()

	if evoOK {
		d.Evolution = &evoTree
	}
	if encOK {
		d.Encounters = encounters
	}
	if movesOK {
		d.LevelUpMoves = moves
	}
	return d, nil
}

func (s *Service) levelUpMovesForPokemon(pokemonID int) ([]domain.LevelUpMove, error) {
	if pokemonID != 0 {
		if rows, err := s.store.LevelUpMoves(pokemonID); err == nil {
			return rows, nil
		} else if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}
	return nil, ErrNotFound
}
