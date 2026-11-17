package cache_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Graphixa/Poke/internal/cache"
	"github.com/Graphixa/Poke/internal/domain"
)

func TestBootstrapCancelDuringRecords(t *testing.T) {
	testBootstrapCancel(t, "records", func(ctx context.Context, svc *cache.Service) error {
		return svc.BootstrapDataset(ctx, nil)
	})
}

func TestBootstrapCancelDuringEvolution(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	const n = 6
	list := make([]domain.SpeciesSummary, n)
	pokemon := map[int]domain.Pokemon{}
	species := map[int]domain.Species{}
	evolution := map[int]domain.EvolutionTree{}
	for i := 0; i < n; i++ {
		id := i + 1
		evoID := 100 + id
		list[i] = domain.SpeciesSummary{ID: id, Slug: "mon", DisplayName: "Mon"}
		pokemon[id] = domain.Pokemon{ID: id, Stats: domain.Stats{Speed: 1}}
		species[id] = domain.Species{ID: id, DefaultPokemonID: id, EvolutionChainID: evoID}
		evolution[evoID] = domain.EvolutionTree{Root: domain.EvolutionNode{SpeciesID: id}}
	}
	api := &cancelAfterRecordsAPI{
		fakeAPI:     fakeAPI{list: list, pokemon: pokemon, species: species, evolution: evolution, delay: 30 * time.Millisecond},
		cancelAfter: 2,
	}
	svc := cache.NewService(store, api, testConfig(), clock)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		time.Sleep(120 * time.Millisecond)
		cancel()
	}()
	err := svc.BootstrapDataset(ctx, nil)
	if err == nil {
		t.Fatal("expected cancellation during evolution")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v want context.Canceled", err)
	}
	if svc.HasActiveDataset() {
		t.Fatal("cancelled bootstrap must not activate")
	}
}

func testBootstrapCancel(t *testing.T, stage string, run func(context.Context, *cache.Service) error) {
	t.Helper()
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	goodAPI := &fakeAPI{
		list: []domain.SpeciesSummary{{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}},
		pokemon: map[int]domain.Pokemon{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", Stats: domain.Stats{Speed: 90}},
		},
		species: map[int]domain.Species{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", DefaultPokemonID: 25},
		},
	}
	goodSvc := cache.NewService(store, goodAPI, testConfig(), clock)
	if err := goodSvc.BootstrapDataset(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	goodID, _ := store.ActiveDatasetID()

	later := fixedClock{t: clock.t.Add(time.Hour)}
	list := []domain.SpeciesSummary{
		{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"},
		{ID: 26, Slug: "raichu", DisplayName: "Raichu"},
		{ID: 27, Slug: "sandshrew", DisplayName: "Sandshrew"},
	}
	api := &cancelAfterRecordsAPI{
		fakeAPI: fakeAPI{
			list: list,
			pokemon: map[int]domain.Pokemon{
				25: {ID: 25, Stats: domain.Stats{Speed: 90}},
				26: {ID: 26, Stats: domain.Stats{Speed: 80}},
				27: {ID: 27, Stats: domain.Stats{Speed: 70}},
			},
			species: map[int]domain.Species{
				25: {ID: 25, DefaultPokemonID: 25},
				26: {ID: 26, DefaultPokemonID: 26},
				27: {ID: 27, DefaultPokemonID: 27},
			},
			delay: 40 * time.Millisecond,
		},
		cancelAfter: 1,
	}
	svc := cache.NewService(store, api, testConfig(), later)
	if err := store.StoreSpeciesIndex(context.Background(), list, later.t); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		time.Sleep(60 * time.Millisecond)
		cancel()
	}()
	err := run(ctx, svc)
	if err == nil {
		t.Fatalf("expected cancellation during %s", stage)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("stage=%s err=%v want context.Canceled", stage, err)
	}
	got, aerr := store.ActiveDatasetID()
	if aerr != nil || got != goodID {
		t.Fatalf("active=%q err=%v want preserved %s", got, aerr, goodID)
	}
}

type cancelAfterRecordsAPI struct {
	fakeAPI
	cancelAfter int
	records     atomic.Int32
}

func (c *cancelAfterRecordsAPI) GetPokemon(ctx context.Context, id int) (domain.Pokemon, error) {
	if c.records.Add(1) > int32(c.cancelAfter) {
		select {
		case <-ctx.Done():
			return domain.Pokemon{}, ctx.Err()
		case <-time.After(c.delay):
		}
	}
	return c.fakeAPI.GetPokemon(ctx, id)
}
