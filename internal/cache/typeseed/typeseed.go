// Package typeseed embeds national-dex type hints so EnsureIndex does not need
// ~1k GetPokemon calls for a fresh install.
package typeseed

import (
	_ "embed"
	"encoding/json"
	"strconv"
	"sync"

	"github.com/Graphixa/Pokedex/internal/domain"
)

//go:embed types.json
var typesJSON []byte

var (
	loadOnce sync.Once
	byID     map[int][]string
	loadErr  error
)

func load() {
	loadOnce.Do(func() {
		raw := make(map[string][]string)
		if err := json.Unmarshal(typesJSON, &raw); err != nil {
			loadErr = err
			byID = map[int][]string{}
			return
		}
		byID = make(map[int][]string, len(raw))
		for k, types := range raw {
			id, err := strconv.Atoi(k)
			if err != nil || id < 1 {
				continue
			}
			cp := append([]string(nil), types...)
			byID[id] = cp
		}
	})
}

// Lookup returns seeded types for a national dex id, or nil if unknown.
func Lookup(id int) []string {
	load()
	if t, ok := byID[id]; ok && len(t) > 0 {
		return append([]string(nil), t...)
	}
	return nil
}

// Has reports whether the seed knows this id.
func Has(id int) bool {
	load()
	_, ok := byID[id]
	return ok
}

// Apply fills empty Types on index rows from the seed.
// Returns the (possibly mutated) index and IDs still missing types after apply
// (unknown to seed or empty seed entry).
func Apply(index []domain.SpeciesSummary) (updated []domain.SpeciesSummary, missing []int) {
	load()
	updated = index
	for i := range updated {
		if len(updated[i].Types) > 0 {
			continue
		}
		if t := byID[updated[i].ID]; len(t) > 0 {
			updated[i].Types = append([]string(nil), t...)
			continue
		}
		missing = append(missing, updated[i].ID)
	}
	return updated, missing
}

// LoadError reports embed/parse failure (tests).
func LoadError() error {
	load()
	return loadErr
}
