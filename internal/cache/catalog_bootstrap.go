package cache

import (
	"context"
	"strconv"

	"github.com/Graphixa/Pokedex/internal/dataset"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/pokeapi"
)

type versionCatalogAPI interface {
	ListVersions(context.Context) ([]domain.Version, error)
	ListVersionGroups(context.Context) ([]domain.VersionGroup, error)
}

type typeCatalogAPI interface {
	ListTypes(context.Context) ([]domain.TypeInfo, error)
}

type abilityAPI interface {
	GetAbility(context.Context, int) (domain.Ability, error)
}

type formAPI interface {
	GetPokemonForm(context.Context, int) (domain.PokemonForm, error)
}

func (s *Service) fetchCatalogs(ctx context.Context, builder *dataset.Builder) error {
	if api, ok := s.api.(versionCatalogAPI); ok {
		groups, err := api.ListVersionGroups(ctx)
		if err != nil {
			return err
		}
		versions, err := api.ListVersions(ctx)
		if err != nil {
			return err
		}
		pokeapi.ApplyVersionGroupOrder(versions, groups)
		for _, g := range groups {
			builder.VersionGroups[g.ID] = dataset.VersionGroupFromDomain(g)
		}
		for _, v := range versions {
			builder.Versions[v.ID] = dataset.VersionFromDomain(v)
		}
	}
	if api, ok := s.api.(typeCatalogAPI); ok {
		types, err := api.ListTypes(ctx)
		if err != nil {
			return err
		}
		for _, t := range types {
			builder.Types[t.ID] = dataset.TypeFromDomain(t)
		}
	}
	return nil
}

func (s *Service) fetchAbility(ctx context.Context, id int) (domain.Ability, error) {
	api, ok := s.api.(abilityAPI)
	if !ok {
		return domain.Ability{}, nil
	}
	v, err := s.group.Do("ability:"+strconv.Itoa(id), func() (any, error) {
		if err := s.acquire(ctx); err != nil {
			return domain.Ability{}, err
		}
		defer s.release()
		return api.GetAbility(ctx, id)
	})
	if err != nil {
		return domain.Ability{}, err
	}
	a, _ := v.(domain.Ability)
	return a, nil
}

func (s *Service) fetchForm(ctx context.Context, id int) (domain.PokemonForm, error) {
	api, ok := s.api.(formAPI)
	if !ok {
		return domain.PokemonForm{}, nil
	}
	v, err := s.group.Do("form:"+strconv.Itoa(id), func() (any, error) {
		if err := s.acquire(ctx); err != nil {
			return domain.PokemonForm{}, err
		}
		defer s.release()
		return api.GetPokemonForm(ctx, id)
	})
	if err != nil {
		return domain.PokemonForm{}, err
	}
	f, _ := v.(domain.PokemonForm)
	return f, nil
}
