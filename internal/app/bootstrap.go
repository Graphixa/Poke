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
)

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
	if err := cache.EnsureCompatibleCache(store); err != nil {
		return nil, fmt.Errorf("cache compatibility: %w", err)
	}
	svc := cache.NewService(store, client, cfg, clock)
	return &Services{Config: cfg, Paths: paths, Cache: svc, API: client}, nil
}

func LoadServices(paths config.Paths) (*Services, error) {
	cfg, warns, err := config.Load(paths.ConfigFile)
	if err != nil {
		return nil, err
	}
	for _, w := range warns {
		fmt.Fprintf(os.Stderr, "config: %s\n", w)
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
	run := func(ctx context.Context, progress cache.DatasetProgressFunc) error {
		return svc.Cache.RefreshDataset(ctx, progress)
	}
	if err := cacheDatasetProgress(ctx, c.stderr, run); err != nil {
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
	progress := func(p cache.ArtworkSyncProgress) {
		if c.stderr == nil || p.Total <= 0 {
			return
		}
		if p.Done == p.Total || p.Done%25 == 0 {
			fmt.Fprintf(c.stderr, "Downloading artwork %d/%d\n", p.Done, p.Total)
		}
	}
	result, err := svc.Cache.SyncArtwork(ctx, index, progress)
	if err != nil {
		return err
	}
	if c.stdout != nil {
		fmt.Fprintf(c.stdout, "Artwork sync complete: new=%d updated=%d unchanged=%d failed=%d\n",
			result.New, result.Updated, result.Unchanged, result.Failed)
	}
	return nil
}

func (c cacheCommands) Update(ctx context.Context) error {
	svc, err := c.services()
	if err != nil {
		return err
	}
	run := func(ctx context.Context, progress cache.DatasetProgressFunc) error {
		if err := svc.Cache.RefreshDataset(ctx, progress); err != nil {
			return err
		}
		index, _, err := svc.Cache.EnsureIndex(ctx, nil)
		if err != nil {
			return err
		}
		syncResult, err := svc.Cache.SyncArtwork(ctx, index, func(p cache.ArtworkSyncProgress) {
			if progress == nil {
				return
			}
			// Preserve the historical update progress label while keeping artwork
			// stages out of cache's dataset-stage API.
			progress(cache.DatasetProgress{
				Stage:   cache.DatasetStage("artwork"),
				Done:    p.Done,
				Total:   p.Total,
				Message: p.Message,
			})
		})
		if err != nil {
			return err
		}
		if c.stdout != nil {
			fmt.Fprintf(c.stdout, "Artwork: new=%d updated=%d unchanged=%d failed=%d\n",
				syncResult.New, syncResult.Updated, syncResult.Unchanged, syncResult.Failed)
			fmt.Fprintln(c.stdout, "Update complete")
		}
		return nil
	}
	if err := cacheDatasetProgress(ctx, c.stderr, run); err != nil {
		return err
	}
	return nil
}

// Reset clears cache and deletes the config file so first-run setup runs again.
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
