package app

import (
	"context"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/tui"
)

func TestBootstrapDatasetCompletesSynchronously(t *testing.T) {
	root := t.TempDir()
	clock := FixedClock{Instant: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	list := []domain.SpeciesSummary{{ID: 1, Slug: "bulbasaur", DisplayName: "Bulbasaur", Types: []string{"grass"}}}
	api := &hangDatasetAPI{list: list}
	cfg := config.Default()
	cfg.Cache.AutoRefresh = false
	svc := cache.NewService(store, api, cfg, clock)
	if svc.HasActiveDataset() {
		t.Fatal("fixture must not have an active snapshot")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := svc.BootstrapDataset(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if !svc.HasActiveDataset() {
		t.Fatal("expected active dataset after synchronous bootstrap")
	}
}

type hangDatasetAPI struct {
	list  []domain.SpeciesSummary
	block chan struct{}
}

func (h *hangDatasetAPI) ListSpecies(context.Context) ([]domain.SpeciesSummary, error) {
	return append([]domain.SpeciesSummary(nil), h.list...), nil
}

func (h *hangDatasetAPI) GetPokemon(ctx context.Context, id int) (domain.Pokemon, error) {
	if h.block != nil {
		select {
		case <-h.block:
		case <-ctx.Done():
			return domain.Pokemon{}, ctx.Err()
		}
	}
	return domain.Pokemon{
		ID: id, Slug: "bulbasaur", DisplayName: "Bulbasaur",
		Stats: domain.Stats{HP: 45},
		Types: []domain.TypeSlot{{Name: "grass"}},
	}, nil
}

func (h *hangDatasetAPI) GetSpecies(ctx context.Context, id int) (domain.Species, error) {
	if h.block != nil {
		select {
		case <-h.block:
		case <-ctx.Done():
			return domain.Species{}, ctx.Err()
		}
	}
	return domain.Species{ID: id, DefaultPokemonID: id}, nil
}

func (h *hangDatasetAPI) GetEvolutionChain(ctx context.Context, id int) (domain.EvolutionTree, error) {
	if h.block != nil {
		select {
		case <-h.block:
		case <-ctx.Done():
			return domain.EvolutionTree{}, ctx.Err()
		}
	}
	return domain.EvolutionTree{}, nil
}

func (h *hangDatasetAPI) GetEncounters(ctx context.Context, id int) ([]domain.Encounter, error) {
	if h.block != nil {
		select {
		case <-h.block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, nil
}

func (h *hangDatasetAPI) GetMove(ctx context.Context, id int) (domain.Move, error) {
	if h.block != nil {
		select {
		case <-h.block:
		case <-ctx.Done():
			return domain.Move{}, ctx.Err()
		}
	}
	return domain.Move{ID: id}, nil
}

func TestSendWarmProgressNeverBlocks(t *testing.T) {
	ch := make(chan tui.WarmProgress, 1)
	ch <- tui.WarmProgress{}
	done := make(chan struct{})
	go func() {
		sendWarmProgress(ch, tui.WarmProgress{SpeciesID: 25, Done: 1, Total: 2})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("sendWarmProgress blocked on full channel")
	}
}

func TestImageCacheCompletionEventNotDropped(t *testing.T) {
	ch := make(chan tui.WarmProgress, 1)
	ch <- tui.WarmProgress{Done: 1024, Total: 1025, Stage: "native"}
	sendWarmProgress(ch, tui.WarmProgress{Done: 1025, Total: 1025, Stage: tui.WarmStageNative})
	select {
	case got := <-ch:
		if got.Done != 1025 || got.Total != 1025 {
			t.Fatalf("got %+v want 1025/1025", got)
		}
		if got.Stage != "native" {
			t.Fatalf("stage=%q want native", got.Stage)
		}
	default:
		t.Fatal("completion event dropped")
	}
}

func TestSendWarmProgressPreservesStageAndSpecies(t *testing.T) {
	ch := make(chan tui.WarmProgress, 1)
	sendWarmProgress(ch, tui.WarmProgress{
		Stage: tui.WarmStageArtwork, Done: 3, Total: 10, Message: "Downloading official artwork", SpeciesID: 25, Cells: 40,
	})
	got := <-ch
	if got.Stage != "artwork" || got.Done != 3 || got.Total != 10 || got.SpeciesID != 25 || got.Cells != 40 {
		t.Fatalf("got %+v", got)
	}
	if got.Message != "Downloading official artwork" {
		t.Fatalf("message=%q", got.Message)
	}
}
