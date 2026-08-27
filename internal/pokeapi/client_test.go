package pokeapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/pokeapi"
)

func TestClientUserAgentAndMapping(t *testing.T) {
	var gotUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		switch {
		case strings.HasSuffix(r.URL.Path, "/pokemon/25"):
			writeJSON(t, w, map[string]any{
				"id": 25, "name": "pikachu", "base_experience": 112, "height": 4, "weight": 60,
				"species": map[string]string{"name": "pikachu", "url": "https://pokeapi.co/api/v2/pokemon-species/25/"},
				"types": []map[string]any{
					{"slot": 1, "type": map[string]string{"name": "electric", "url": "https://pokeapi.co/api/v2/type/13/"}},
				},
				"abilities": []map[string]any{
					{"slot": 1, "is_hidden": false, "ability": map[string]string{"name": "static", "url": "https://pokeapi.co/api/v2/ability/9/"}},
				},
				"stats": []map[string]any{
					{"base_stat": 35, "stat": map[string]string{"name": "hp", "url": "https://pokeapi.co/api/v2/stat/1/"}},
					{"base_stat": 55, "stat": map[string]string{"name": "attack", "url": "https://pokeapi.co/api/v2/stat/2/"}},
					{"base_stat": 40, "stat": map[string]string{"name": "defense", "url": "https://pokeapi.co/api/v2/stat/3/"}},
					{"base_stat": 50, "stat": map[string]string{"name": "special-attack", "url": "https://pokeapi.co/api/v2/stat/4/"}},
					{"base_stat": 50, "stat": map[string]string{"name": "special-defense", "url": "https://pokeapi.co/api/v2/stat/5/"}},
					{"base_stat": 90, "stat": map[string]string{"name": "speed", "url": "https://pokeapi.co/api/v2/stat/6/"}},
				},
				"moves":   []any{},
				"sprites": map[string]any{
					"front_default": "https://example.test/pikachu.png",
					"other": map[string]any{
						"home": map[string]any{
							"front_default": "https://example.test/pikachu-home.png",
						},
						"official-artwork": map[string]any{
							"front_default": "https://example.test/pikachu-art.png",
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := pokeapi.NewClient(pokeapi.Options{
		BaseURL:    server.URL,
		UserAgent:  "poke-cli",
		Retries:    0,
		HTTPClient: server.Client(),
		Sleeper:    func(context.Context, time.Duration) error { return nil },
	})
	pokemon, err := client.GetPokemon(context.Background(), 25)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(gotUA, "poke-cli/") {
		t.Fatalf("User-Agent = %q", gotUA)
	}
	if pokemon.DisplayName != "Pikachu" || pokemon.Stats.Speed != 90 || len(pokemon.Types) != 1 || pokemon.Types[0].Name != "electric" {
		t.Fatalf("mapped pokemon = %#v", pokemon)
	}
	if pokemon.Sprites.FrontDefault == "" {
		t.Fatal("expected sprite url")
	}
	if pokemon.Sprites.HomeFront != "https://example.test/pikachu-home.png" {
		t.Fatalf("HomeFront = %q", pokemon.Sprites.HomeFront)
	}
	if pokemon.Sprites.OfficialArtwork != "https://example.test/pikachu-art.png" {
		t.Fatalf("OfficialArtwork = %q", pokemon.Sprites.OfficialArtwork)
	}
}

func TestClientRetriesServerErrorsAndHonoursRetryAfter(t *testing.T) {
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if n == 2 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeJSON(t, w, map[string]any{
			"count": 1, "next": nil, "previous": nil,
			"results": []map[string]string{
				{"name": "bulbasaur", "url": "https://pokeapi.co/api/v2/pokemon-species/1/"},
			},
		})
	}))
	defer server.Close()

	var sleeps []time.Duration
	client := pokeapi.NewClient(pokeapi.Options{
		BaseURL:    server.URL,
		Retries:    3,
		HTTPClient: server.Client(),
		Sleeper: func(_ context.Context, d time.Duration) error {
			sleeps = append(sleeps, d)
			return nil
		},
		RandFloat: func() float64 { return 0.5 },
	})
	list, err := client.ListSpecies(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != 1 || list[0].Slug != "bulbasaur" {
		t.Fatalf("list = %#v", list)
	}
	if hits.Load() != 3 {
		t.Fatalf("hits = %d", hits.Load())
	}
	if len(sleeps) != 2 {
		t.Fatalf("sleeps = %#v", sleeps)
	}
	if sleeps[1] != time.Second {
		t.Fatalf("expected Retry-After sleep of 1s, got %s", sleeps[1])
	}
}

func TestClientNotFoundAndContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()
	client := pokeapi.NewClient(pokeapi.Options{
		BaseURL: server.URL, Retries: 2, HTTPClient: server.Client(),
		Sleeper: func(context.Context, time.Duration) error { return nil },
	})
	_, err := client.GetMove(context.Background(), 999999)
	if !errors.Is(err, pokeapi.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}

	blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer blocked.Close()
	cancelClient := pokeapi.NewClient(pokeapi.Options{
		BaseURL: blocked.URL, Retries: 5, HTTPClient: blocked.Client(),
		Sleeper: func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = cancelClient.ListSpecies(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestClientOfflineWrap(t *testing.T) {
	client := pokeapi.NewClient(pokeapi.Options{
		BaseURL:    "http://127.0.0.1:1",
		Retries:    0,
		HTTPClient: &http.Client{Timeout: 50 * time.Millisecond},
		Sleeper:    func(context.Context, time.Duration) error { return nil },
	})
	_, err := client.GetSpecies(context.Background(), 1)
	if !errors.Is(err, pokeapi.ErrOffline) {
		t.Fatalf("err = %v", err)
	}
}

func TestSpeciesEvolutionEncounterMoveMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/pokemon-species/4"):
			writeJSON(t, w, map[string]any{
				"id": 4, "name": "charmander", "capture_rate": 45, "base_happiness": 50, "gender_rate": 1,
				"is_legendary": false, "is_mythical": false,
				"growth_rate":     map[string]string{"name": "medium-slow", "url": "https://pokeapi.co/api/v2/growth-rate/4/"},
				"color":           map[string]string{"name": "red", "url": "https://pokeapi.co/api/v2/pokemon-color/8/"},
				"shape":           map[string]string{"name": "upright", "url": "https://pokeapi.co/api/v2/pokemon-shape/6/"},
				"habitat":         map[string]string{"name": "mountain", "url": "https://pokeapi.co/api/v2/pokemon-habitat/4/"},
				"generation":      map[string]string{"name": "generation-i", "url": "https://pokeapi.co/api/v2/generation/1/"},
				"evolution_chain": map[string]string{"url": "https://pokeapi.co/api/v2/evolution-chain/2/"},
				"names":           []map[string]any{{"name": "Charmander", "language": map[string]string{"name": "en", "url": "https://pokeapi.co/api/v2/language/9/"}}},
				"genera":          []map[string]any{{"genus": "Lizard Pokémon", "language": map[string]string{"name": "en", "url": "https://pokeapi.co/api/v2/language/9/"}}},
				"flavor_text_entries": []map[string]any{{
					"flavor_text": "Obviously prefers hot places.",
					"language":    map[string]string{"name": "en", "url": "https://pokeapi.co/api/v2/language/9/"},
					"version":     map[string]string{"name": "red", "url": "https://pokeapi.co/api/v2/version/1/"},
				}},
				"varieties": []map[string]any{{
					"is_default": true,
					"pokemon":    map[string]string{"name": "charmander", "url": "https://pokeapi.co/api/v2/pokemon/4/"},
				}},
			})
		case strings.Contains(r.URL.Path, "/evolution-chain/2"):
			writeJSON(t, w, map[string]any{
				"id": 2,
				"chain": map[string]any{
					"species":           map[string]string{"name": "charmander", "url": "https://pokeapi.co/api/v2/pokemon-species/4/"},
					"evolution_details": []any{},
					"evolves_to": []map[string]any{{
						"species": map[string]string{"name": "charmeleon", "url": "https://pokeapi.co/api/v2/pokemon-species/5/"},
						"evolution_details": []map[string]any{{
							"min_level":   16,
							"trigger":     map[string]string{"name": "level-up", "url": "https://pokeapi.co/api/v2/evolution-trigger/1/"},
							"time_of_day": "", "needs_overworld_rain": false, "turn_upside_down": false,
						}},
						"evolves_to": []any{},
					}},
				},
			})
		case strings.Contains(r.URL.Path, "/encounters"):
			writeJSON(t, w, []map[string]any{{
				"location_area": map[string]string{"name": "kanto-route-3-area", "url": "https://pokeapi.co/api/v2/location-area/1/"},
				"version_details": []map[string]any{{
					"version":    map[string]string{"name": "red", "url": "https://pokeapi.co/api/v2/version/1/"},
					"max_chance": 20,
					"encounter_details": []map[string]any{{
						"min_level": 10, "max_level": 12, "chance": 20,
						"method":           map[string]string{"name": "walk", "url": "https://pokeapi.co/api/v2/encounter-method/1/"},
						"condition_values": []any{},
					}},
				}},
			}})
		case strings.Contains(r.URL.Path, "/move/1"):
			power := 40
			writeJSON(t, w, map[string]any{
				"id": 1, "name": "pound", "power": power, "accuracy": 100, "pp": 35,
				"type":         map[string]string{"name": "normal", "url": "https://pokeapi.co/api/v2/type/1/"},
				"damage_class": map[string]string{"name": "physical", "url": "https://pokeapi.co/api/v2/move-damage-class/2/"},
				"names":        []map[string]any{{"name": "Pound", "language": map[string]string{"name": "en", "url": "https://pokeapi.co/api/v2/language/9/"}}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := pokeapi.NewClient(pokeapi.Options{
		BaseURL: server.URL, Retries: 0, HTTPClient: server.Client(),
		Sleeper: func(context.Context, time.Duration) error { return nil },
	})

	species, err := client.GetSpecies(context.Background(), 4)
	if err != nil {
		t.Fatal(err)
	}
	if species.DisplayName != "Charmander" || species.DefaultPokemonID != 4 || species.EvolutionChainID != 2 {
		t.Fatalf("species = %#v", species)
	}

	tree, err := client.GetEvolutionChain(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Root.SpeciesSlug != "charmander" || len(tree.Root.Children) != 1 {
		t.Fatalf("tree = %#v", tree)
	}
	if tree.Root.Children[0].Conditions[0].MinimumLevel == nil || *tree.Root.Children[0].Conditions[0].MinimumLevel != 16 {
		t.Fatalf("conditions = %#v", tree.Root.Children[0].Conditions)
	}

	encounters, err := client.GetEncounters(context.Background(), 4)
	if err != nil || len(encounters) != 1 || encounters[0].Area != "kanto-route-3-area" {
		t.Fatalf("encounters = %#v err=%v", encounters, err)
	}

	move, err := client.GetMove(context.Background(), 1)
	if err != nil || move.DisplayName != "Pound" || move.Power == nil || *move.Power != 40 {
		t.Fatalf("move = %#v err=%v", move, err)
	}
}

func TestGetSprite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte{0x89, 0x50, 0x4e, 0x47})
	}))
	defer server.Close()
	client := pokeapi.NewClient(pokeapi.Options{
		BaseURL: server.URL, Retries: 0, HTTPClient: server.Client(),
		Sleeper: func(context.Context, time.Duration) error { return nil },
	})
	rc, err := client.GetSprite(context.Background(), server.URL+"/sprite.png")
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil || string(data) != "\x89PNG" {
		t.Fatalf("sprite = %v err=%v", data, err)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultTransportTuned(t *testing.T) {
	tr := pokeapi.DefaultTransport()
	if tr.MaxIdleConns < 64 {
		t.Fatalf("MaxIdleConns=%d", tr.MaxIdleConns)
	}
	if tr.MaxIdleConnsPerHost < 32 {
		t.Fatalf("MaxIdleConnsPerHost=%d", tr.MaxIdleConnsPerHost)
	}
	if !tr.ForceAttemptHTTP2 {
		t.Fatal("expected ForceAttemptHTTP2")
	}
}
