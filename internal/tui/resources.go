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
