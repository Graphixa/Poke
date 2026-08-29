package cache

import (
	"fmt"
)

const maxBootstrapFailures = 64

// DatasetFailure is one recorded bootstrap error for troubleshooting.
type DatasetFailure struct {
	Resource string `json:"resource"`
	ID       int    `json:"id,omitempty"`
	Error    string `json:"error"`
	Required bool   `json:"required"`
}

// DatasetBootstrapReport is persisted next to a staging snapshot.
type DatasetBootstrapReport struct {
	Expected        int              `json:"expected"`
	PokemonOK       int              `json:"pokemon_ok"`
	SpeciesOK       int              `json:"species_ok"`
	BrowseOK        int              `json:"browse_ok"`
	EncounterOK     int              `json:"encounter_ok"`
	EvolutionOK     int              `json:"evolution_ok"`
	EvolutionWanted int              `json:"evolution_wanted"`
	MoveOK          int              `json:"move_ok"`
	MoveWanted      int              `json:"move_wanted"`
	StorageFailures int              `json:"storage_failures"`
	Failures        []DatasetFailure `json:"failures,omitempty"`
}

func (r *DatasetBootstrapReport) add(resource string, id int, err error, required bool) {
	if r == nil || err == nil {
		return
	}
	if len(r.Failures) >= maxBootstrapFailures {
		return
	}
	r.Failures = append(r.Failures, DatasetFailure{
		Resource: resource,
		ID:       id,
		Error:    err.Error(),
		Required: required,
	})
}

// Ready reports whether the snapshot satisfies the offline contract.
func (r DatasetBootstrapReport) Ready() bool {
	if r.Expected < 1 {
		return false
	}
	if r.PokemonOK != r.Expected || r.SpeciesOK != r.Expected || r.BrowseOK != r.Expected {
		return false
	}
	if r.EncounterOK != r.PokemonOK {
		return false
	}
	if r.EvolutionOK != r.EvolutionWanted || r.MoveOK != r.MoveWanted {
		return false
	}
	if r.StorageFailures > 0 {
		return false
	}
	for _, f := range r.Failures {
		if f.Required {
			return false
		}
	}
	return true
}

func (r DatasetBootstrapReport) summary() string {
	return fmt.Sprintf("pokemon=%d/%d species=%d/%d browse=%d/%d encounters=%d/%d evo=%d/%d moves=%d/%d storage_fail=%d",
		r.PokemonOK, r.Expected,
		r.SpeciesOK, r.Expected,
		r.BrowseOK, r.Expected,
		r.EncounterOK, r.PokemonOK,
		r.EvolutionOK, r.EvolutionWanted,
		r.MoveOK, r.MoveWanted,
		r.StorageFailures,
	)
}
