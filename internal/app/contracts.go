package app

import (
	"context"
	"io"
	"time"

	"github.com/Graphixa/Pokedex/internal/domain"
)

// PokeAPI is the transport-independent API boundary consumed by services.
// Concrete HTTP DTOs remain private to internal/pokeapi.
type PokeAPI interface {
	ListSpecies(context.Context) ([]domain.SpeciesSummary, error)
	GetPokemon(context.Context, int) (domain.Pokemon, error)
	GetSpecies(context.Context, int) (domain.Species, error)
	GetEvolutionChain(context.Context, int) (domain.EvolutionTree, error)
	GetEncounters(context.Context, int) ([]domain.Encounter, error)
	GetMove(context.Context, int) (domain.Move, error)
	GetSprite(context.Context, string) (io.ReadCloser, error)
}

// Cache is the application-facing persistence boundary. Implementations decide
// freshness and storage details without exposing files or API payloads.
type Cache interface {
	SpeciesIndex(context.Context) ([]domain.SpeciesSummary, error)
	Pokemon(context.Context, int) (domain.Pokemon, error)
	Species(context.Context, int) (domain.Species, error)
	EvolutionChain(context.Context, int) (domain.EvolutionTree, error)
	Encounters(context.Context, int) ([]domain.Encounter, error)
	Move(context.Context, int) (domain.Move, error)
	Sprite(context.Context, string) ([]byte, error)

	StoreSpeciesIndex(context.Context, []domain.SpeciesSummary, time.Time) error
	StoreSprite(context.Context, string, []byte) error
}

type Clock interface {
	Now() time.Time
}

type SystemClock struct{}

func (SystemClock) Now() time.Time {
	return time.Now()
}
