package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/pokeapi"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/render/sprite/assetwarm"
)

// Ensure the file store satisfies the application cache contract.
var _ Cache = (*cache.Store)(nil)

// Services holds runtime dependencies constructed from config and paths.
type Services struct {
	Config config.Config
	Paths  config.Paths
	Cache  *cache.Service
	API    *pokeapi.Client
}

// BuildServices loads config and wires the PokeAPI client and cache service.
func BuildServices(paths config.Paths, cfg config.Config, httpClient *http.Client, clock cache.Clock) (*Services, error) {
	if clock == nil {
		clock = SystemClock{}
	}
	client := pokeapi.NewClient(pokeapi.Options{
		Timeout:    cfg.Network.Timeout.Duration,
		Retries:    cfg.Network.Retries,
		UserAgent:  cfg.Network.UserAgent,
		HTTPClient: httpClient,
	})
	store := cache.NewStore(paths.CacheDir, clock)
	svc := cache.NewService(store, client, cfg, clock)
	return &Services{Config: cfg, Paths: paths, Cache: svc, API: client}, nil
}

func LoadServices(paths config.Paths) (*Services, error) {
	cfg, err := config.Load(paths.ConfigFile)
	if err != nil {
		return nil, err
	}
	return BuildServices(paths, cfg, nil, nil)
}

type cacheCommands struct {
	services func() (*Services, error)
	stdout   io.Writer
	stderr   io.Writer
}

func (c cacheCommands) Refresh(ctx context.Context) error {
	svc, err := c.services()
	if err != nil {
		return err
	}
	progress := func(p cache.Progress) {
		w := c.stderr
		if w == nil {
			return
		}
		switch p.Stage {
		case "index":
			if p.Message != "" {
				fmt.Fprintf(w, "%s\n", p.Message)
			}
		case "enrich":
			if p.Total > 0 {
				fmt.Fprintf(w, "Enriching types %d/%d %s\n", p.Done, p.Total, p.Message)
			}
		}
	}
	if err := svc.Cache.Refresh(ctx, progress); err != nil {
		return err
	}
	if c.stdout != nil {
		fmt.Fprintln(c.stdout, "Cache update complete")
	}
	return nil
}

func (c cacheCommands) Clear() error {
	svc, err := c.services()
	if err != nil {
		return err
	}
	if err := svc.Cache.Clear(); err != nil {
		return err
	}
	if c.stdout != nil {
		fmt.Fprintf(c.stdout, "Cleared cache at %s\n", svc.Cache.Path())
	}
	return nil
}

func (c cacheCommands) WarmSprites(ctx context.Context) error {
	svc, err := c.services()
	if err != nil {
		return err
	}
	index, _, err := svc.Cache.EnsureIndex(ctx, nil)
	if err != nil {
		return err
	}
	details := NewDetailService(svc.Cache)
	progress := func(p cache.Progress) {
		if c.stderr == nil || p.Total <= 0 {
			return
		}
		if p.Done == p.Total || p.Done%25 == 0 {
			fmt.Fprintf(c.stderr, "Warming artwork %d/%d\n", p.Done, p.Total)
		}
	}
	w := assetwarm.New(assetwarm.Config{
		Mode:     assetwarm.ModeArtwork,
		Workers:  assetwarm.ArtworkWorkers(svc.Cache.MaxConcurrency()),
		Loader:   warmPokemonLoader{details},
		Resolver: &sprite.SourceResolver{Store: svc.Cache, API: svc.API},
		Index:    index,
		Progress: progress,
	})
	w.Start(ctx)
	err = w.Wait(ctx)
	w.Stop()
	if err != nil {
		return err
	}
	if c.stdout != nil {
		fmt.Fprintln(c.stdout, "Artwork warm complete")
	}
	return nil
}

// Reset clears cache and deletes the config file so first-run setup runs again.
// DEV: remove before release.
func (c cacheCommands) Reset() error {
	svc, err := c.services()
	if err != nil {
		return err
	}
	cachePath := svc.Cache.Path()
	configPath := svc.Paths.ConfigFile
	if err := svc.Cache.Clear(); err != nil {
		return fmt.Errorf("clear cache: %w", err)
	}
	if err := os.Remove(configPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove config: %w", err)
	}
	if c.stdout != nil {
		fmt.Fprintf(c.stdout, "Cleared cache at %s\n", cachePath)
		fmt.Fprintf(c.stdout, "Removed config at %s\n", configPath)
		fmt.Fprintln(c.stdout, "Run poke to start setup")
	}
	return nil
}

// FixedClock is a deterministic clock for tests.
type FixedClock struct {
	Instant time.Time
}

func (c FixedClock) Now() time.Time { return c.Instant }
