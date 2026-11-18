package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/Graphixa/Poke/internal/debuglog"
	"github.com/Graphixa/Poke/internal/domain"
)

// SpeciesIndex returns the cached national dex index without network access.
func (s *Service) SpeciesIndex(ctx context.Context) ([]domain.SpeciesSummary, error) {
	index, err := s.store.SpeciesIndex(ctx)
	if err != nil {
		return nil, err
	}
	if len(index) == 0 {
		return nil, ErrEmpty
	}
	return index, nil
}

func (s *Service) Pokemon(ctx context.Context, id int) (domain.Pokemon, error) {
	v, err := s.group.Do(fmt.Sprintf("pokemon:%d", id), func() (any, error) {
		return s.getPokemon(ctx, id)
	})
	if err != nil {
		return domain.Pokemon{}, err
	}
	return v.(domain.Pokemon), nil
}

func (s *Service) getPokemon(ctx context.Context, id int) (domain.Pokemon, error) {
	if s.datasetReadsOnly() {
		p, err := s.store.Pokemon(ctx, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return domain.Pokemon{}, fmt.Errorf("%w: pokemon %d missing from active dataset", ErrCorrupt, id)
			}
			return domain.Pokemon{}, err
		}
		debuglog.Cachef("detail.cache.hit pokemon id=%d", id)
		return p, nil
	}
	if err := s.acquire(ctx); err != nil {
		return domain.Pokemon{}, err
	}
	fresh, fetchErr := s.api.GetPokemon(ctx, id)
	s.release()
	if fetchErr != nil {
		debuglog.Cachef("detail.cache.miss pokemon id=%d err=%v", id, fetchErr)
		return domain.Pokemon{}, fetchErr
	}
	return fresh, nil
}

func (s *Service) Species(ctx context.Context, id int) (domain.Species, error) {
	v, err := s.group.Do(fmt.Sprintf("species:%d", id), func() (any, error) {
		return s.getSpecies(ctx, id)
	})
	if err != nil {
		return domain.Species{}, err
	}
	return v.(domain.Species), nil
}

func (s *Service) getSpecies(ctx context.Context, id int) (domain.Species, error) {
	if s.datasetReadsOnly() {
		sp, err := s.store.Species(ctx, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return domain.Species{}, fmt.Errorf("%w: species %d missing from active dataset", ErrCorrupt, id)
			}
			return domain.Species{}, err
		}
		return sp, nil
	}
	if err := s.acquire(ctx); err != nil {
		return domain.Species{}, err
	}
	fresh, fetchErr := s.api.GetSpecies(ctx, id)
	s.release()
	if fetchErr != nil {
		return domain.Species{}, fetchErr
	}
	return fresh, nil
}

func (s *Service) EvolutionChain(ctx context.Context, id int) (domain.EvolutionTree, error) {
	v, err := s.group.Do(fmt.Sprintf("evolution:%d", id), func() (any, error) {
		return s.getEvolution(ctx, id)
	})
	if err != nil {
		return domain.EvolutionTree{}, err
	}
	return v.(domain.EvolutionTree), nil
}

func (s *Service) getEvolution(ctx context.Context, id int) (domain.EvolutionTree, error) {
	if s.datasetReadsOnly() {
		tree, err := s.store.EvolutionChain(ctx, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return domain.EvolutionTree{}, fmt.Errorf("%w: evolution %d missing from active dataset", ErrCorrupt, id)
			}
			return domain.EvolutionTree{}, err
		}
		return tree, nil
	}
	if err := s.acquire(ctx); err != nil {
		return domain.EvolutionTree{}, err
	}
	fresh, fetchErr := s.api.GetEvolutionChain(ctx, id)
	s.release()
	if fetchErr != nil {
		return domain.EvolutionTree{}, fetchErr
	}
	return fresh, nil
}

func (s *Service) Encounters(ctx context.Context, pokemonID int) ([]domain.Encounter, error) {
	v, err := s.group.Do(fmt.Sprintf("encounters:%d", pokemonID), func() (any, error) {
		return s.getEncounters(ctx, pokemonID)
	})
	if err != nil {
		return nil, err
	}
	return v.([]domain.Encounter), nil
}

func (s *Service) getEncounters(ctx context.Context, pokemonID int) ([]domain.Encounter, error) {
	if s.datasetReadsOnly() {
		enc, err := s.store.Encounters(ctx, pokemonID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil, fmt.Errorf("%w: encounters for pokemon %d missing from active dataset", ErrCorrupt, pokemonID)
			}
			return nil, err
		}
		return enc, nil
	}
	if err := s.acquire(ctx); err != nil {
		return nil, err
	}
	fresh, fetchErr := s.api.GetEncounters(ctx, pokemonID)
	s.release()
	if fetchErr != nil {
		return nil, fetchErr
	}
	return fresh, nil
}

func (s *Service) Move(ctx context.Context, id int) (domain.Move, error) {
	v, err := s.group.Do(fmt.Sprintf("move:%d", id), func() (any, error) {
		return s.getMove(ctx, id)
	})
	if err != nil {
		return domain.Move{}, err
	}
	return v.(domain.Move), nil
}

func (s *Service) getMove(ctx context.Context, id int) (domain.Move, error) {
	if s.datasetReadsOnly() {
		mv, err := s.store.Move(ctx, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return domain.Move{}, fmt.Errorf("%w: move %d missing from active dataset", ErrCorrupt, id)
			}
			return domain.Move{}, err
		}
		return mv, nil
	}
	if err := s.acquire(ctx); err != nil {
		return domain.Move{}, err
	}
	fresh, fetchErr := s.api.GetMove(ctx, id)
	s.release()
	if fetchErr != nil {
		return domain.Move{}, fetchErr
	}
	return fresh, nil
}

func (s *Service) datasetReadsOnly() bool {
	return s.localOnly.Load() || s.store.HasActiveDataset()
}
