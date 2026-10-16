package tui

import (
	"github.com/Graphixa/Pokedex/internal/domain"
)

// resourceCache holds loaded detail for one species (dossier filled on Enter).
// Rendered sprite output lives on the model image pane, not here.
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

	Moves    []domain.LevelUpMove
	HasMoves bool
	MovesErr error

	loadingCore   bool
	loadingExtras bool
}

func (r *resourceCache) extrasReady() bool {
	return r != nil && r.HasEvolution && r.HasEncounters && r.HasMoves
}

func (r *resourceCache) applyCore(d domain.Dossier, complete bool) {
	r.Pokemon = d.Pokemon
	r.Species = d.Species
	r.HasCore = true
	r.CoreErr = nil
	r.loadingCore = false
	if complete {
		r.applyExtrasFields(d)
	}
}

func (r *resourceCache) applyExtras(d domain.Dossier) {
	r.applyExtrasFields(d)
	r.loadingExtras = false
}

func (r *resourceCache) applyExtrasFields(d domain.Dossier) {
	if d.Evolution != nil {
		r.Evolution = *d.Evolution
		r.HasEvolution = true
		r.EvolutionErr = nil
	} else if d.Species.EvolutionChainID == 0 {
		r.HasEvolution = true
		r.Evolution = domain.EvolutionTree{}
		r.EvolutionErr = nil
	}
	if d.Encounters != nil {
		r.Encounters = d.Encounters
		r.HasEncounters = true
		r.EncountersErr = nil
	}
	if len(d.LevelUpMoves) > 0 || !pokemonHasLevelUpMoves(d.Pokemon) {
		r.Moves = append([]domain.LevelUpMove(nil), d.LevelUpMoves...)
		r.HasMoves = true
		r.MovesErr = nil
	}
}

func (r *resourceCache) applyDossier(d domain.Dossier) {
	r.applyCore(d, false)
	r.applyExtrasFields(d)
	r.loadingExtras = false
}

func pokemonHasLevelUpMoves(p domain.Pokemon) bool {
	for _, lm := range p.Moves {
		for _, learn := range lm.Learnings {
			if learn.Method == "level-up" {
				return true
			}
		}
	}
	return false
}
