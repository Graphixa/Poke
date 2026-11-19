package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Graphixa/Poke/internal/cache"
)

func TestAPIDataFileMapsRESTPaths(t *testing.T) {
	root := t.TempDir()
	got, ok := apiDataFile(root, "/api/v2/pokemon/1/encounters")
	if !ok {
		t.Fatal("expected a file")
	}
	want := filepath.Join(root, "data", "api", "v2", "pokemon", "1", "encounters", "index.json")
	if got != want {
		t.Fatalf("file=%s want %s", got, want)
	}
	if _, ok := apiDataFile(root, "/api/v2/../../secret"); ok {
		t.Fatal("path escaped the checkout")
	}
}

func TestBuildSnapshotFromFixture(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"pokemon-species/index.json": listJSON("bulbasaur", "/api/v2/pokemon-species/1/"),
		"pokemon-species/1/index.json": `{
			"id": 1, "name": "bulbasaur",
			"names": [{"name": "Bulbasaur", "language": {"name": "en"}}],
			"varieties": [{"is_default": true, "pokemon": {"name": "bulbasaur", "url": "/api/v2/pokemon/1/"}}]
		}`,
		"pokemon/1/index.json": `{
			"id": 1, "name": "bulbasaur", "height": 7, "weight": 69, "base_experience": 64,
			"species": {"name": "bulbasaur", "url": "/api/v2/pokemon-species/1/"},
			"types": [{"slot": 1, "type": {"name": "grass", "url": "/api/v2/type/12/"}}],
			"stats": [
				{"base_stat": 45, "stat": {"name": "hp"}},
				{"base_stat": 49, "stat": {"name": "attack"}},
				{"base_stat": 49, "stat": {"name": "defense"}},
				{"base_stat": 65, "stat": {"name": "special-attack"}},
				{"base_stat": 65, "stat": {"name": "special-defense"}},
				{"base_stat": 45, "stat": {"name": "speed"}}
			]
		}`,
		"pokemon/1/encounters/index.json": `[]`,
		"version/index.json":              listJSON("red", "/api/v2/version/1/"),
		"version/1/index.json":            `{"id": 1, "name": "red", "version_group": {"name": "red-blue", "url": "/api/v2/version-group/1/"}}`,
		"version-group/index.json":        listJSON("red-blue", "/api/v2/version-group/1/"),
		"version-group/1/index.json":      `{"id": 1, "name": "red-blue", "order": 1}`,
		"type/index.json":                 listJSON("normal", "/api/v2/type/1/"),
		"type/1/index.json":               `{"id": 1, "name": "normal"}`,
	}
	for name, body := range files {
		path := filepath.Join(root, "data", "api", "v2", filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	dir, err := buildSnapshot(context.Background(), root, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store := cache.NewStore(filepath.Dir(filepath.Dir(dir)), nil)
	id, err := store.ActiveDatasetID()
	if err != nil || id == "" {
		t.Fatalf("active=%q err=%v", id, err)
	}
	man, err := store.ReadDatasetManifest(id)
	if err != nil || !man.Complete || man.PokemonCount != 1 {
		t.Fatalf("manifest complete=%v pokemon=%d err=%v", man.Complete, man.PokemonCount, err)
	}
	srv := httptest.NewServer(apiDataHandler(root))
	defer srv.Close()
	res, err := http.Get(srv.URL + "/api/v2/pokemon-species?limit=200")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("list status=%d", res.StatusCode)
	}
}

func listJSON(name, url string) string {
	return `{"count":1,"next":null,"previous":null,"results":[{"name":"` + name + `","url":"` + url + `"}]}`
}
