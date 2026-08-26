package cache_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/pokeapi"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type fakeAPI struct {
	list      []domain.SpeciesSummary
	listErr   error
	pokemon   map[int]domain.Pokemon
	pokemonFn func(int) (domain.Pokemon, error)
	species   map[int]domain.Species
	inFlight  atomic.Int64
	maxFlight atomic.Int64
	delay     time.Duration
	calls     atomic.Int64
}

func (f *fakeAPI) ListSpecies(context.Context) ([]domain.SpeciesSummary, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := append([]domain.SpeciesSummary(nil), f.list...)
	return out, nil
}

func (f *fakeAPI) GetPokemon(ctx context.Context, id int) (domain.Pokemon, error) {
	f.calls.Add(1)
	cur := f.inFlight.Add(1)
	for {
		prev := f.maxFlight.Load()
		if cur <= prev || f.maxFlight.CompareAndSwap(prev, cur) {
			break
		}
	}
	defer f.inFlight.Add(-1)
	if f.delay > 0 {
		select {
		case <-ctx.Done():
			return domain.Pokemon{}, ctx.Err()
		case <-time.After(f.delay):
		}
	}
	if f.pokemonFn != nil {
		return f.pokemonFn(id)
	}
	p, ok := f.pokemon[id]
	if !ok {
		return domain.Pokemon{}, pokeapi.ErrNotFound
	}
	return p, nil
}

func (f *fakeAPI) GetSpecies(context.Context, int) (domain.Species, error) {
	return domain.Species{}, pokeapi.ErrNotFound
}
func (f *fakeAPI) GetEvolutionChain(context.Context, int) (domain.EvolutionTree, error) {
	return domain.EvolutionTree{}, pokeapi.ErrNotFound
}
func (f *fakeAPI) GetEncounters(context.Context, int) ([]domain.Encounter, error) {
	return nil, pokeapi.ErrNotFound
}
func (f *fakeAPI) GetMove(context.Context, int) (domain.Move, error) {
	return domain.Move{}, pokeapi.ErrNotFound
}

func testConfig() config.Config {
	cfg := config.Default()
	cfg.Network.MaxConcurrency = 2
	cfg.Cache.IndexTTL = config.Duration{Duration: time.Hour}
	cfg.Cache.DetailTTL = config.Duration{Duration: time.Hour}
	return cfg
}

func TestStoreAtomicCorruptAndClear(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}
	store := cache.NewStore(root, clock)

	index := []domain.SpeciesSummary{{ID: 1, Slug: "bulbasaur", DisplayName: "Bulbasaur", Types: []string{"grass", "poison"}}}
	if err := store.StoreSpeciesIndex(context.Background(), index, clock.Now()); err != nil {
		t.Fatal(err)
	}
	got, err := store.SpeciesIndex(context.Background())
	if err != nil || len(got) != 1 || got[0].Slug != "bulbasaur" {
		t.Fatalf("index = %#v err=%v", got, err)
	}

	indexPath := filepath.Join(root, "index", "species.json")
	if err := os.WriteFile(indexPath, []byte("{not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = store.SpeciesIndex(context.Background())
	if !errors.Is(err, cache.ErrCorrupt) {
		t.Fatalf("err = %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "corrupt"))
	if err != nil || len(entries) == 0 {
		t.Fatalf("corrupt dir entries = %v err=%v", entries, err)
	}
	if _, err := os.Stat(indexPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("corrupt index should be removed")
	}

	if err := store.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatal(err)
	}
	_, err = store.SpeciesIndex(context.Background())
	if !errors.Is(err, cache.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestRefreshConcurrencyAndTypes(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	api := &fakeAPI{
		list: []domain.SpeciesSummary{
			{ID: 1, Slug: "bulbasaur", DisplayName: "Bulbasaur"},
			{ID: 4, Slug: "charmander", DisplayName: "Charmander"},
			{ID: 7, Slug: "squirtle", DisplayName: "Squirtle"},
		},
		pokemon: map[int]domain.Pokemon{
			1: {ID: 1, Slug: "bulbasaur", Types: []domain.TypeSlot{{Slot: 1, Name: "grass"}, {Slot: 2, Name: "poison"}}},
			4: {ID: 4, Slug: "charmander", Types: []domain.TypeSlot{{Slot: 1, Name: "fire"}}},
			7: {ID: 7, Slug: "squirtle", Types: []domain.TypeSlot{{Slot: 1, Name: "water"}}},
		},
		delay: 20 * time.Millisecond,
	}
	svc := cache.NewService(store, api, testConfig(), clock)
	if err := svc.Refresh(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if api.maxFlight.Load() > 2 {
		t.Fatalf("max in-flight = %d", api.maxFlight.Load())
	}
	index, err := svc.SpeciesIndex(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(index) != 3 || len(index[0].Types) == 0 || index[1].Types[0] != "fire" {
		t.Fatalf("index = %#v", index)
	}
}

func TestEnsureIndexStartupPolicy(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	clock := fixedClock{t: now}
	store := cache.NewStore(root, clock)
	cfg := testConfig()
	cfg.Cache.AutoRefresh = true

	api := &fakeAPI{listErr: pokeapi.ErrOffline}
	svc := cache.NewService(store, api, cfg, clock)
	_, _, err := svc.EnsureIndex(context.Background(), nil)
	var offline *cache.OfflineError
	if !errors.As(err, &offline) {
		t.Fatalf("err = %v", err)
	}

	api = &fakeAPI{
		list: []domain.SpeciesSummary{{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}},
		pokemon: map[int]domain.Pokemon{
			25: {ID: 25, Slug: "pikachu", Types: []domain.TypeSlot{{Slot: 1, Name: "electric"}}},
		},
	}
	svc = cache.NewService(store, api, cfg, clock)
	index, stale, err := svc.EnsureIndex(context.Background(), nil)
	if err != nil || stale || len(index) != 1 {
		t.Fatalf("index=%v stale=%v err=%v", index, stale, err)
	}
	// Thin index returns before type enrich; wait for enrich to finish.
	if err := svc.EnrichTypes(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	index, err = svc.SpeciesIndex(context.Background())
	if err != nil || len(index) != 1 || len(index[0].Types) == 0 || index[0].Types[0] != "electric" {
		t.Fatalf("enriched index=%#v err=%v", index, err)
	}

	// Fresh cache: EnsureIndex must not hit the network again.
	api.listErr = errors.New("network should not be used")
	api.pokemonFn = func(int) (domain.Pokemon, error) { return domain.Pokemon{}, errors.New("no") }
	index, stale, err = svc.EnsureIndex(context.Background(), nil)
	if err != nil || stale || index[0].Types[0] != "electric" {
		t.Fatalf("fresh ensure index=%v stale=%v err=%v", index, stale, err)
	}

	// Advance clock beyond TTL → stale but still usable offline.
	clock.t = now.Add(2 * time.Hour)
	svc = cache.NewService(store, api, cfg, clock)
	index, stale, err = svc.EnsureIndex(context.Background(), nil)
	if err != nil || !stale || len(index) != 1 {
		t.Fatalf("stale ensure index=%v stale=%v err=%v", index, stale, err)
	}
	if !svc.ShouldBackgroundRefresh(stale) {
		t.Fatal("expected background refresh")
	}
}

func TestInterruptedRefreshKeepsPartialIndex(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	var started atomic.Int64
	release := make(chan struct{})
	api := &fakeAPI{
		list: []domain.SpeciesSummary{
			{ID: 1, Slug: "bulbasaur", DisplayName: "Bulbasaur"},
			{ID: 4, Slug: "charmander", DisplayName: "Charmander"},
		},
		pokemonFn: func(id int) (domain.Pokemon, error) {
			if started.Add(1) == 1 {
				<-release
			}
			return domain.Pokemon{
				ID: id, Slug: "x", Types: []domain.TypeSlot{{Slot: 1, Name: "normal"}},
			}, nil
		},
	}
	cfg := testConfig()
	cfg.Network.MaxConcurrency = 1
	svc := cache.NewService(store, api, cfg, clock)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- svc.Refresh(ctx, nil) }()

	// Wait until index is written (ListSpecies + Store) and enrichment starts.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if store.HasSpeciesIndex() && started.Load() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !store.HasSpeciesIndex() {
		t.Fatal("expected index to be persisted before enrichment finished")
	}
	cancel()
	close(release)
	<-done

	index, err := store.SpeciesIndex(context.Background())
	if err != nil || len(index) != 2 {
		t.Fatalf("partial index = %#v err=%v", index, err)
	}
}

func TestWarmOfflineDetailRead(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	pokemon := domain.Pokemon{ID: 25, Slug: "pikachu", DisplayName: "Pikachu", Stats: domain.Stats{Speed: 90}}
	if err := store.StorePokemon(context.Background(), pokemon, clock.Now()); err != nil {
		t.Fatal(err)
	}
	api := &fakeAPI{pokemonFn: func(int) (domain.Pokemon, error) {
		return domain.Pokemon{}, pokeapi.ErrOffline
	}}
	svc := cache.NewService(store, api, testConfig(), clock)
	got, err := svc.Pokemon(context.Background(), 25)
	if err != nil || got.Stats.Speed != 90 {
		t.Fatalf("got=%#v err=%v", got, err)
	}
}

func TestEnvelopeRoundTripJSON(t *testing.T) {
	root := t.TempDir()
	store := cache.NewStore(root, nil)
	move := domain.Move{ID: 1, Slug: "pound", DisplayName: "Pound", PP: 35}
	if err := store.StoreMove(context.Background(), move, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "moves", "1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	if env["schema_version"].(float64) != 2 || env["source"] != "pokeapi" {
		t.Fatalf("envelope = %#v", env)
	}
}
