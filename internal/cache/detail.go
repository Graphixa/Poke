package cache

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/domain"
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
	cached, cacheErr := s.store.Pokemon(ctx, id)
	haveCache := cacheErr == nil
	if haveCache {
		debuglog.Cachef("detail.cache.hit pokemon id=%d", id)
		return cached, nil
	}
	if cacheErr != nil && !errors.Is(cacheErr, ErrNotFound) && !errors.Is(cacheErr, ErrCorrupt) {
		return domain.Pokemon{}, cacheErr
	}
	if s.localOnly.Load() {
		if haveCache {
			return cached, nil
		}
		return domain.Pokemon{}, cacheErr
	}
	if err := s.acquire(ctx); err != nil {
		if haveCache {
			return cached, nil
		}
		return domain.Pokemon{}, err
	}
	fresh, fetchErr := s.api.GetPokemon(ctx, id)
	s.release()
	if fetchErr != nil {
		if haveCache {
			debuglog.Cachef("stale-serve pokemon id=%d err=%v", id, fetchErr)
			return cached, nil
		}
		debuglog.Cachef("detail.cache.miss pokemon id=%d err=%v", id, fetchErr)
		return domain.Pokemon{}, fetchErr
	}
	debuglog.Cachef("store pokemon id=%d", id)
	_ = s.store.StorePokemon(ctx, fresh, s.clock.Now())
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
	cached, cacheErr := s.store.Species(ctx, id)
	haveCache := cacheErr == nil
	if haveCache {
		return cached, nil
	}
	if cacheErr != nil && !errors.Is(cacheErr, ErrNotFound) && !errors.Is(cacheErr, ErrCorrupt) {
		return domain.Species{}, cacheErr
	}
	if s.localOnly.Load() {
		return domain.Species{}, cacheErr
	}
	if err := s.acquire(ctx); err != nil {
		if haveCache {
			return cached, nil
		}
		return domain.Species{}, err
	}
	fresh, fetchErr := s.api.GetSpecies(ctx, id)
	s.release()
	if fetchErr != nil {
		if haveCache {
			return cached, nil
		}
		return domain.Species{}, fetchErr
	}
	_ = s.store.StoreSpecies(ctx, fresh, s.clock.Now())
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
	cached, cacheErr := s.store.EvolutionChain(ctx, id)
	haveCache := cacheErr == nil
	if haveCache {
		return cached, nil
	}
	if cacheErr != nil && !errors.Is(cacheErr, ErrNotFound) && !errors.Is(cacheErr, ErrCorrupt) {
		return domain.EvolutionTree{}, cacheErr
	}
	if s.localOnly.Load() {
		return domain.EvolutionTree{}, cacheErr
	}
	if err := s.acquire(ctx); err != nil {
		return domain.EvolutionTree{}, err
	}
	fresh, fetchErr := s.api.GetEvolutionChain(ctx, id)
	s.release()
	if fetchErr != nil {
		return domain.EvolutionTree{}, fetchErr
	}
	_ = s.store.StoreEvolutionChain(ctx, id, fresh, s.clock.Now())
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
	cached, cacheErr := s.store.Encounters(ctx, pokemonID)
	haveCache := cacheErr == nil
	if haveCache {
		return cached, nil
	}
	if s.localOnly.Load() {
		return nil, cacheErr
	}
	if err := s.acquire(ctx); err != nil {
		return nil, err
	}
	fresh, fetchErr := s.api.GetEncounters(ctx, pokemonID)
	s.release()
	if fetchErr != nil {
		return nil, fetchErr
	}
	_ = s.store.StoreEncounters(ctx, pokemonID, fresh, s.clock.Now())
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
	cached, cacheErr := s.store.Move(ctx, id)
	haveCache := cacheErr == nil
	if haveCache {
		return cached, nil
	}
	if s.localOnly.Load() {
		return domain.Move{}, cacheErr
	}
	if err := s.acquire(ctx); err != nil {
		return domain.Move{}, err
	}
	fresh, fetchErr := s.api.GetMove(ctx, id)
	s.release()
	if fetchErr != nil {
		return domain.Move{}, fetchErr
	}
	_ = s.store.StoreMove(ctx, fresh, s.clock.Now())
	return fresh, nil
}

// ReadSpriteBytes downloads sprite bytes through the API client.
func ReadSpriteBytes(ctx context.Context, api interface {
	GetSprite(context.Context, string) (io.ReadCloser, error)
}, url string) ([]byte, error) {
	rc, err := api.GetSprite(ctx, url)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
