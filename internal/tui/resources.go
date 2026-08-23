package tui

import (
	"github.com/Graphixa/Pokedex/internal/domain"
)

// resourceCache holds loaded detail for one species selection.
type resourceCache struct {
	Pokemon domain.Pokemon
	Species domain.Species
	HasCore bool
	CoreErr error

	Evolution    domain.EvolutionTree
	HasEvolution bool
	EvolutionErr error

	Encounters    []domain.Encounter
	HasEncounters bool
	EncountersErr error

	Moves    []LevelUpMove
	HasMoves bool
	MovesErr error

	SpriteANSI        string
	SpriteCells       int
	HasSprite         bool
	SpritePlaceholder bool
	loadingSprite     bool

	loadingCore       bool
	loadingEvolution  bool
	loadingEncounters bool
	loadingMoves      bool
}

func (r *resourceCache) clearSprite() {
	if r == nil {
		return
	}
	r.SpriteANSI = ""
	r.SpriteCells = 0
	r.HasSprite = false
	r.SpritePlaceholder = false
	r.loadingSprite = false
}

func (r *resourceCache) coreReady() bool {
	return r != nil && r.HasCore
}

func (r *resourceCache) tabLoading(tab Tab) bool {
	if r == nil {
		return false
	}
	switch tab {
	case TabEvolution:
		return r.loadingEvolution
	case TabLocations:
		return r.loadingEncounters
	case TabMoves:
		return r.loadingMoves
	default:
		return false
	}
}

func (r *resourceCache) tabError(tab Tab) error {
	if r == nil {
		return nil
	}
	switch tab {
	case TabEvolution:
		return r.EvolutionErr
	case TabLocations:
		return r.EncountersErr
	case TabMoves:
		return r.MovesErr
	default:
		if r.CoreErr != nil {
			return r.CoreErr
		}
		return nil
	}
}
