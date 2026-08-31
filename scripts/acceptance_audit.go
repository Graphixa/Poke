//go:build ignore

package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	spritechafa "github.com/Graphixa/Pokedex/internal/render/sprite/chafa"
	"github.com/Graphixa/Pokedex/internal/render/sprite/placeholder"
	"github.com/Graphixa/Pokedex/internal/tui/style"
)

type engineStub struct {
	encodes int
}

func (s *engineStub) Name() string         { return "image" }
func (s *engineStub) ProtocolName() string { return "sixel" }
func (s *engineStub) Supported(sprite.Capabilities) bool {
	return true
}
func (s *engineStub) Render(_ context.Context, _ image.Image, _ sprite.RenderOptions) (sprite.Frame, error) {
	s.encodes++
	return sprite.Frame{Text: "COLD", Backend: "image", Native: true}, nil
}

func main() {
	start := time.Now()
	cacheDir := os.Getenv("POKEDEX_CACHE")
	if cacheDir == "" {
		home, err := os.UserCacheDir()
		if err != nil {
			fatalf("user cache dir: %v", err)
		}
		cacheDir = filepath.Join(home, "pokedex")
	}
	fmt.Printf("cache=%s\n", cacheDir)
	store := cache.NewStore(cacheDir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	ctx := context.Background()

	id, err := store.ActiveDatasetID()
	if err != nil {
		fatalf("active dataset: %v", err)
	}
	man, err := store.ReadDatasetManifest(id)
	if err != nil {
		fatalf("dataset manifest: %v", err)
	}
	entries, err := store.LoadBrowse()
	if err != nil {
		fatalf("browse: %v", err)
	}
	fmt.Printf("dataset.id=%s complete=%v pokemon=%d species=%d browse_file=%d encounters=%d evo=%d moves=%d\n",
		id, man.Complete, man.PokemonCount, man.SpeciesCount, len(entries), man.EncounterCount, man.EvolutionCount, man.MoveCount)
	if rep, repErr := store.ReadBootstrapReport(id); repErr == nil {
		fmt.Printf("bootstrap ready=%v pokemon=%d/%d species=%d/%d browse=%d/%d encounters=%d/%d evo=%d/%d moves=%d/%d storage_fail=%d\n",
			rep.Ready(),
			rep.PokemonOK, rep.Expected,
			rep.SpeciesOK, rep.Expected,
			rep.BrowseOK, rep.Expected,
			rep.EncounterOK, rep.PokemonOK,
			rep.EvolutionOK, rep.EvolutionWanted,
			rep.MoveOK, rep.MoveWanted,
			rep.StorageFailures)
	}

	bg := style.BaseRGBA()
	fmt.Printf("theme.base=%02x%02x%02x algorithm=%s sizes=%v\n", bg.R, bg.G, bg.B, sprite.AlgorithmVersion, sprite.FrameSteps())

	artworkReady := 0
	namedHash, shaHash := 0, 0
	for _, e := range entries {
		hash, path, ready, merr := store.NamedAssetMeta(ctx, e.ID, e.Slug, cache.SpriteKindArtwork)
		if merr != nil || !ready || hash == "" {
			continue
		}
		if _, statErr := os.Stat(path); statErr != nil {
			continue
		}
		artworkReady++
		if strings.HasPrefix(hash, "named-") {
			namedHash++
		} else {
			shaHash++
		}
	}
	fmt.Printf("artwork.ready=%d/%d meta.named_hash=%d meta.sha256_hash=%d\n", artworkReady, len(entries), namedHash, shaHash)

	nativeDir := filepath.Join(cacheDir, "native")
	files, _ := os.ReadDir(nativeDir)
	namedN, shaN := 0, 0
	dimCount := map[string]int{}
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".frame") {
			continue
		}
		if strings.HasPrefix(f.Name(), "named-") {
			namedN++
		} else {
			shaN++
		}
		key := strings.TrimSuffix(f.Name(), ".frame")
		_, _, w, h, _, _, _, perr := sprite.ParseNativeCacheKey(key)
		if perr == nil {
			dimCount[fmt.Sprintf("%dx%d", w, h)]++
		}
	}
	fmt.Printf("native.files named=%d sha256=%d dims=%v\n", namedN, shaN, dimCount)

	nman, err := store.ReadNativeManifest()
	if err != nil {
		fmt.Printf("native.manifest err=%v\n", err)
	} else {
		fmt.Printf("native.manifest version=%s protocol=%s\n", nman.Version, nman.Protocol)
		for _, cells := range sprite.FrameSteps() {
			counted, _ := store.CountNativeSpeciesForCells(cells, nman.Protocol)
			fmt.Printf("  cells=%d filesystem_unique=%d\n", cells, counted)
		}
	}

	stub := &engineStub{}
	engine := newEngine(svc, stub, bg)

	byID := map[int]domain.BrowseEntry{}
	for _, e := range entries {
		byID[e.ID] = e
	}
	boundary := []int{1, 25, 395, 396, 397, 398, 399, 400, 401, 402, 403, 404, 405}
	if len(entries) > 0 {
		boundary = append(boundary, entries[len(entries)-1].ID)
	}

	fmt.Println("--- boundary lookup ---")
	for _, cells := range sprite.FrameSteps() {
		w, h := sprite.FrameSizeFor(cells)
		pxW, pxH := sprite.PixelSize(w, h)
		for _, bid := range boundary {
			e, ok := byID[bid]
			if !ok {
				fmt.Printf("id=%d cells=%d MISSING_FROM_INDEX\n", bid, cells)
				continue
			}
			hash, _, ready, _ := store.NamedAssetMeta(ctx, e.ID, e.Slug, cache.SpriteKindArtwork)
			key := sprite.NativeCacheKey(hash, "sixel", w, h, bg, 64)
			nativePath := filepath.Join(nativeDir, key+".frame")
			_, statErr := os.Stat(nativePath)
			exists := statErr == nil
			poke := domain.Pokemon{ID: e.ID, Slug: e.Slug, DisplayName: e.Name}
			res, hit := engine.LookupCached(ctx, poke, cells)
			kind := "sha256"
			if strings.HasPrefix(hash, "named-") {
				kind = "named"
			}
			fmt.Printf("id=%d slug=%s cells=%d px=%dx%d backend=sixel ready=%v hash_kind=%s hash=%s key=%s exists=%v lookup.hit=%v cachekey=%s\n",
				bid, e.Slug, cells, pxW, pxH, ready, kind, hash, key, exists, hit, res.CacheKey)
		}
	}

	fmt.Println("--- full size walk (LookupCached only) ---")
	sprite.ResetStats()
	walkStart := time.Now()
	for _, cells := range sprite.FrameSteps() {
		hits, misses := 0, 0
		for _, e := range entries {
			poke := domain.Pokemon{ID: e.ID, Slug: e.Slug, DisplayName: e.Name}
			_, hit := engine.LookupCached(ctx, poke, cells)
			if hit {
				hits++
			} else {
				misses++
			}
		}
		fmt.Printf("size=%d hits=%d misses=%d expected=%d\n", cells, hits, misses, len(entries))
	}
	st := sprite.Stats()
	fmt.Printf("walk.elapsed=%s mem_hits=%d disk_hits=%d cold=%d png_reads=%d png_decodes=%d sixel_encodes=%d http_artwork=%d stub_encodes=%d\n",
		time.Since(walkStart).Round(time.Millisecond),
		st.MemHits.Load(), st.DiskHits.Load(), st.ColdRenders.Load(),
		st.PNGReads.Load(), st.PNGDecodes.Load(), st.SixelEncodes.Load(), st.HTTPArtwork.Load(),
		stub.encodes)

	fmt.Println("--- single-file miss (401, restore) ---")
	if e401, ok401 := byID[401]; ok401 {
		w, h := sprite.FrameSizeFor(sprite.MaxFrameSize)
		hash, _, _, _ := store.NamedAssetMeta(ctx, e401.ID, e401.Slug, cache.SpriteKindArtwork)
		key := sprite.NativeCacheKey(hash, "sixel", w, h, bg, 64)
		p := filepath.Join(nativeDir, key+".frame")
		if data, err := os.ReadFile(p); err == nil {
			if err := os.Remove(p); err != nil {
				fatalf("delete 401: %v", err)
			}
			fresh := newEngine(svc, &engineStub{}, bg)
			check := func(id int) bool {
				e := byID[id]
				_, hit := fresh.LookupCached(ctx, domain.Pokemon{ID: e.ID, Slug: e.Slug, DisplayName: e.Name}, sprite.MaxFrameSize)
				return hit
			}
			fmt.Printf("after_delete 399=%v 400=%v 401=%v 402=%v\n", check(399), check(400), check(401), check(402))
			if err := os.WriteFile(p, data, 0o600); err != nil {
				fatalf("restore 401: %v", err)
			}
			fmt.Printf("after_restore 401=%v\n", check(401))
		} else {
			fmt.Printf("401 frame missing at %s — skip delete test\n", p)
		}
	}

	fmt.Printf("audit.elapsed=%s\n", time.Since(start).Round(time.Millisecond))
}

func newEngine(svc *cache.Service, stub *engineStub, bg color.RGBA) *sprite.Engine {
	return &sprite.Engine{
		Source:     &sprite.SourceResolver{Store: svc},
		Modes:      sprite.NewModeResolver(stub, spritechafa.Renderer{}, placeholder.Renderer{}),
		Artefacts:  svc,
		Caps:       sprite.Capabilities{NativeImage: true, TrueColor: true, ANSI256: true},
		Preference: sprite.ModeAuto,
		Color:      sprite.ColorTrueColor,
		Background: bg,
		HDOff:      false,
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
