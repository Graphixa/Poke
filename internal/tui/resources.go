package tui

import (
	"github.com/Graphixa/Pokedex/internal/domain"
)

// resourceCache holds loaded detail for one species (dossier filled on Enter).
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
	SpriteNative      bool
	loadingSprite     bool

	loadingCore bool
}

func (r *resourceCache) clearSprite() {
	if r == nil {
		return
	}
	r.SpriteANSI = ""
	r.SpriteCells = 0
	r.HasSprite = false
	r.SpritePlaceholder = false
	r.SpriteNative = false
	r.loadingSprite = false
}

func (r *resourceCache) coreReady() bool {
	return r != nil && r.HasCore
}

func (r *resourceCache) applyDossier(d domain.Dossier) {
	r.Pokemon = d.Pokemon
	r.Species = d.Species
	r.HasCore = true
	r.CoreErr = nil
	if d.Evolution != nil {
		r.Evolution = *d.Evolution
		r.HasEvolution = true
		r.EvolutionErr = nil
	} else {
		r.HasEvolution = true
		r.Evolution = domain.EvolutionTree{}
	}
	r.Encounters = d.Encounters
	r.HasEncounters = true
	r.EncountersErr = nil
	r.Moves = make([]LevelUpMove, len(d.LevelUpMoves))
	for i, row := range d.LevelUpMoves {
		r.Moves[i] = LevelUpMove{Level: row.Level, Move: row.Move}
	}
	r.HasMoves = true
	r.MovesErr = nil
	r.loadingCore = false
}
