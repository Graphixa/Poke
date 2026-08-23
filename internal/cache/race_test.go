package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/domain"
)

func TestEnrichCancellationDoesNotLeak(t *testing.T) {
	api := &fakeAPI{
		list: []domain.SpeciesSummary{
			{ID: 1, Slug: "a", DisplayName: "A"},
			{ID: 2, Slug: "b", DisplayName: "B"},
			{ID: 3, Slug: "c", DisplayName: "C"},
			{ID: 4, Slug: "d", DisplayName: "D"},
		},
		pokemonFn: func(id int) (domain.Pokemon, error) {
			return domain.Pokemon{ID: id, Slug: "x", Types: []domain.TypeSlot{{Slot: 1, Name: "normal"}}}, nil
		},
		delay: 80 * time.Millisecond,
	}
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	cfg := config.Default()
	cfg.Network.MaxConcurrency = 2
	svc := cache.NewService(store, api, cfg, clock)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- svc.Refresh(ctx, nil)
	}()
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected cancellation error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("refresh did not respect cancellation")
	}
}

func TestBackgroundRefreshSingleFlight(t *testing.T) {
	api := &fakeAPI{
		list: []domain.SpeciesSummary{
			{ID: 1, Slug: "a", DisplayName: "A", Types: []string{"normal"}},
		},
		delay: 100 * time.Millisecond,
	}
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	cfg := config.Default()
	svc := cache.NewService(store, api, cfg, clock)

	// Seed index so BackgroundRefresh's Refresh is the only work.
	if err := store.EnsureRoot(); err != nil {
		t.Fatal(err)
	}
	if err := store.StoreSpeciesIndex(context.Background(), api.list, clock.Now()); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		svc.BackgroundRefresh(ctx, nil)
		svc.BackgroundRefresh(ctx, nil)
		// Wait for single-flight work to finish.
		time.Sleep(300 * time.Millisecond)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("background refresh hung")
	}
}
