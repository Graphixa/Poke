package prewarm

import (
	"context"
	"errors"
	"fmt"

	"github.com/Graphixa/Poke/internal/debuglog"
	"github.com/Graphixa/Poke/internal/domain"
	"github.com/Graphixa/Poke/internal/render/sprite"
)

// FallbackBytesPerArtwork is the per-species official-artwork size estimate
// (~120 KiB) used for setup's download-size prompt.
const FallbackBytesPerArtwork int64 = 120 * 1024

// PokemonLoader loads Pokémon detail (including sprite URLs) by species ID.
type PokemonLoader interface {
	Pokemon(ctx context.Context, id int) (domain.Pokemon, error)
}

// ChafaRenderer warms a single Pokémon into the chafa artefact cache.
type ChafaRenderer interface {
	WarmRender(ctx context.Context, pokemon domain.Pokemon, cells int) sprite.RenderResult
}

// NativeRenderer encodes native graphics frames into the artefact cache.
type NativeRenderer interface {
	HasNativeFile(ctx context.Context, pokemon domain.Pokemon, cells int) bool
	NativeWarmRender(ctx context.Context, pokemon domain.Pokemon, cells int) sprite.RenderResult
	PersistNativeFrame(ctx context.Context, cacheKey, frame string) error
	WaitPersistCapacity(ctx context.Context) error
}

// Resolver fetches and caches PNG bytes for a tier.
type Resolver interface {
	Resolve(ctx context.Context, pokemon domain.Pokemon, tier ...sprite.AssetTier) sprite.SourceResult
}

// DossierLoader assembles dossiers (full) or pokemon+species only (core).
type DossierLoader interface {
	LoadDossier(ctx context.Context, speciesID int) (domain.Dossier, error)
	WarmCore(ctx context.Context, speciesID int) error
}

// NewChafa warms chafa artefacts. all scans the whole index; otherwise only hinted
// species run (window mode).
func NewChafa(index []domain.SpeciesSummary, loader PokemonLoader, r ChafaRenderer, all bool, progress func(Progress)) *Service {
	return New(Config{
		Kind:               ChafaArtefact,
		Index:              index,
		Background:         all,
		Workers:            sprite.InteractiveWorkers,
		ThrottleOnActivity: true,
		Message:            "Warming sprites",
		Progress:           progress,
		Warm: func(ctx context.Context, id, cells int) error {
			poke, err := loader.Pokemon(ctx, id)
			if err != nil || poke.Sprites.FrontDefault == "" {
				debuglog.Renderf("prewarm chafa skip species=%d err=%v", id, err)
				return ErrSkip
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if res := r.WarmRender(ctx, poke, cells); res.UsePlaceholder && res.Backend != "chafa" {
				debuglog.Renderf("prewarm chafa placeholder species=%d", id)
			}
			return nil
		},
	})
}

// NewNative pre-encodes native frames (Sixel/Kitty/iTerm) as .sixel.gz files. A
// single opportunistic lane; the disk audit makes Done reflect persisted files.
func NewNative(index []domain.SpeciesSummary, loader PokemonLoader, r NativeRenderer, progress func(Progress)) *Service {
	slugs := make(map[int]string, len(index))
	for _, sum := range index {
		slugs[sum.ID] = sum.Slug
	}
	poke := func(id int) domain.Pokemon { return domain.Pokemon{ID: id, Slug: slugs[id]} }
	return New(Config{
		Kind:       NativeFrame,
		Index:      index,
		Background: true,
		Workers:    1,
		Message:    "Building image cache",
		Progress:   progress,
		Has: func(ctx context.Context, id, cells int) bool {
			return r.HasNativeFile(ctx, poke(id), cells)
		},
		Warm: func(ctx context.Context, id, cells int) error {
			if err := r.WaitPersistCapacity(ctx); err != nil {
				return err
			}
			p, err := loader.Pokemon(ctx, id)
			if err != nil {
				debuglog.Renderf("prewarm native skip species=%d err=%v", id, err)
				return err
			}
			if slug := slugs[id]; slug != "" {
				p.Slug = slug
			}
			if p.ID == 0 {
				p.ID = id
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if r.HasNativeFile(ctx, p, cells) {
				return nil
			}
			res := r.NativeWarmRender(ctx, p, cells)
			if res.UsePlaceholder || !res.Native || res.ANSI == "" || res.CacheKey == "" {
				return fmt.Errorf("native miss species=%d backend=%s", id, res.Backend)
			}
			if err := r.PersistNativeFrame(ctx, res.CacheKey, res.ANSI); err != nil {
				return err
			}
			if !r.HasNativeFile(ctx, p, cells) {
				return fmt.Errorf("native species=%d unverified on disk", id)
			}
			return nil
		},
	})
}

// NewArtwork downloads official-artwork PNGs from the CDN (URLs derived from the
// species ID, no Pokémon JSON needed). A miss counts as done but not Ready;
// Prioritize retries it.
func NewArtwork(index []domain.SpeciesSummary, loader PokemonLoader, res Resolver, workers int, progress func(Progress)) *Service {
	byID := make(map[int]domain.SpeciesSummary, len(index))
	for _, sum := range index {
		byID[sum.ID] = sum
	}
	return New(Config{
		Kind:       ArtworkAsset,
		Index:      index,
		Background: true,
		// CDN-bound: not limited by the JSON max_concurrency.
		Workers:  min(max(workers, 16), 24),
		Message:  "Downloading official artwork",
		Progress: progress,
		Warm: func(ctx context.Context, id, _ int) error {
			var poke domain.Pokemon
			if sum, ok := byID[id]; ok {
				poke = domain.Pokemon{ID: id, Slug: sum.Slug, DisplayName: sum.DisplayName}
				poke.Sprites.OfficialArtwork = sprite.CDNArtworkURL(id)
			} else if loader == nil {
				return ErrSkip
			} else if p, err := loader.Pokemon(ctx, id); err != nil {
				return ErrSkip
			} else {
				poke = p
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			r := res.Resolve(ctx, poke, sprite.AssetTierHD)
			if r.UsePlaceholder || len(r.PNG) == 0 {
				debuglog.Renderf("prewarm artwork miss species=%d", id)
				return ErrSkip
			}
			return nil
		},
	})
}

// NewDossier pre-assembles dossiers. all loads full dossiers across the index;
// otherwise only hinted species run and only pokemon+species core is warmed.
func NewDossier(index []domain.SpeciesSummary, l DossierLoader, all bool, progress func(Progress)) *Service {
	return New(Config{
		Kind:       Dossier,
		Index:      index,
		Background: all,
		Workers:    2,
		Message:    "dossier warm",
		Progress:   progress,
		Warm: func(ctx context.Context, id, _ int) error {
			var err error
			if all {
				_, err = l.LoadDossier(ctx, id)
			} else {
				err = l.WarmCore(ctx, id)
			}
			if err != nil && !errors.Is(err, context.Canceled) {
				debuglog.Cachef("dossier warm fail id=%d err=%v", id, err)
			}
			return err
		},
	})
}
