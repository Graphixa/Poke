package cache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Graphixa/Poke/internal/dataset"
	"github.com/Graphixa/Poke/internal/domain"
)

func TestAggregatePathHelpers(t *testing.T) {
	store := NewStore(t.TempDir(), nil)
	id := "test-id"
	if got := store.AggregatePokemonPath(id); !strings.HasSuffix(got, filepath.Join("datasets", id, dataset.FilePokemon)) {
		t.Fatalf("pokemon path=%q", got)
	}
	if got := store.AggregateSpeciesPath(id); !strings.HasSuffix(got, dataset.FileSpecies) {
		t.Fatalf("species path=%q", got)
	}
}

func TestAggregateJSONUsesNoEnvelope(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root, nil)
	id := "test"
	b := dataset.NewBuilder()
	b.Pokemon[1] = dataset.PokemonFromDomain(domain.Pokemon{ID: 1, Slug: "a", DisplayName: "A"})
	if err := store.StoreAggregatePokemon(id, b.Pokemon); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(store.AggregatePokemonPath(id))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "schema_version") {
		t.Fatalf("aggregate must not use envelope: %s", raw)
	}
}
