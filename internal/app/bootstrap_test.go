package app

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/domain"
)

type updateStubAPI struct {
	list []domain.SpeciesSummary
}

func (s *updateStubAPI) ListSpecies(context.Context) ([]domain.SpeciesSummary, error) {
	return append([]domain.SpeciesSummary(nil), s.list...), nil
}
func (s *updateStubAPI) GetPokemon(context.Context, int) (domain.Pokemon, error) {
	return domain.Pokemon{}, errors.New("unused")
}
func (s *updateStubAPI) GetSpecies(context.Context, int) (domain.Species, error) {
	return domain.Species{}, errors.New("unused")
}
func (s *updateStubAPI) GetEvolutionChain(context.Context, int) (domain.EvolutionTree, error) {
	return domain.EvolutionTree{}, errors.New("unused")
}
func (s *updateStubAPI) GetEncounters(context.Context, int) ([]domain.Encounter, error) {
	return nil, nil
}
func (s *updateStubAPI) GetMove(context.Context, int) (domain.Move, error) {
	return domain.Move{}, errors.New("unused")
}
func (s *updateStubAPI) GetSprite(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.New("unused")
}

func TestUpdateDataOnlyNoNativeCache(t *testing.T) {
	root := t.TempDir()
	clock := FixedClock{Instant: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	api := &updateStubAPI{
		list: []domain.SpeciesSummary{
			{ID: 25, Slug: "pikachu", DisplayName: "Pikachu", Types: []string{"electric"}},
			{ID: 133, Slug: "eevee", DisplayName: "Eevee", Types: []string{"normal"}},
		},
	}
	cfg := config.Default()
	cfg.Cache.AutoRefresh = false
	cfg.Network.MaxConcurrency = 2
	svc := cache.NewService(store, api, cfg, clock)

	var stdout, stderr strings.Builder
	cmds := cacheCommands{
		services: func() (*Services, error) {
			return &Services{
				Config: cfg,
				Paths:  config.Paths{CacheDir: root},
				Cache:  svc,
			}, nil
		},
		stdout: &stdout,
		stderr: &stderr,
	}
	if err := cmds.Update(context.Background()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stderr.String(), "Building image cache") {
		t.Fatalf("stderr should not report native warm: %q", stderr.String())
	}
	nativeDir := filepath.Join(root, "native")
	entries, err := os.ReadDir(nativeDir)
	if err != nil {
		if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	} else {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".frame") {
				t.Fatalf("unexpected native frame %s", e.Name())
			}
		}
	}
	if !strings.Contains(stdout.String(), "Update complete") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
