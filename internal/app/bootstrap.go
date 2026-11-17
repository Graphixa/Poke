package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Graphixa/Poke/internal/cache"
	"github.com/Graphixa/Poke/internal/config"
	"github.com/Graphixa/Poke/internal/pokeapi"
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
	// nil clock → cache uses its system clock.
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

type updateProgressKind uint8

const (
	updateProgressDataset updateProgressKind = iota
	updateProgressArtwork
)

// updateProgress is app-owned because update composes dataset refresh and
// artwork sync, whose progress types remain owned by their subsystems.
type updateProgress struct {
	kind         updateProgressKind
	datasetStage cache.DatasetStage
	done         int
	total        int
	message      string
}

type updateProgressFunc func(updateProgress)

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

func (c cacheCommands) Refresh(ctx context.Context) error {
	svc, err := c.services()
	if err != nil {
		return err
	}
	run := func(ctx context.Context, progress updateProgressFunc) error {
		datasetProgress := func(p cache.DatasetProgress) {
			if progress != nil {
				progress(updateProgress{
					kind:         updateProgressDataset,
					datasetStage: p.Stage,
					done:         p.Done,
					total:        p.Total,
					message:      p.Message,
				})
			}
		}
		if err := svc.Cache.RefreshDataset(ctx, datasetProgress); err != nil {
			return err
		}
		index, _, err := svc.Cache.EnsureIndex(ctx, nil)
		if err != nil {
			return err
		}
		syncResult, err := svc.Cache.SyncArtwork(ctx, index, func(p cache.ArtworkSyncProgress) {
			if progress != nil {
				progress(updateProgress{
					kind:    updateProgressArtwork,
					done:    p.Done,
					total:   p.Total,
					message: p.Message,
				})
			}
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
	if err := cacheUpdateProgress(ctx, c.stderr, run); err != nil {
		return err
	}
	return nil
}

// FixedClock is a deterministic clock for tests.
type FixedClock struct {
	Instant time.Time
}

func (c FixedClock) Now() time.Time { return c.Instant }
