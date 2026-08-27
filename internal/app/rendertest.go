package app

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	spritechafa "github.com/Graphixa/Pokedex/internal/render/sprite/chafa"
	"github.com/Graphixa/Pokedex/internal/render/sprite/halfblock"
	spriteimage "github.com/Graphixa/Pokedex/internal/render/sprite/image"
	"github.com/Graphixa/Pokedex/internal/tui/style"
)

// RenderTestRequest configures poke test (renderer drag race + visual compare).
type RenderTestRequest struct {
	// Target is a Pokémon name or national dex number (default pikachu/25).
	Target string
	// Iterations for the timed drag race (default 30).
	Iterations int
	// SkipVisual suppresses ANSI dumps (bench-only).
	SkipVisual bool
}

var renderTestSizes = []int{20, 28, 40}

// RunRenderTest prints side-by-side chafa vs halfblock (imgcat-style) at three
// sizes, times each encoder, and reports termimg/native encode cost when available.
func (h *Handler) RunRenderTest(ctx context.Context, req RenderTestRequest) error {
	svc, err := h.Services()
	if err != nil {
		return err
	}
	out := h.Stdout
	if out == nil {
		out = os.Stdout
	}

	iters := req.Iterations
	if iters < 1 {
		iters = 30
	}
	target := strings.TrimSpace(req.Target)
	if target == "" {
		target = "25"
	}

	poke, pngBytes, srcLabel, err := loadRenderTestPNG(ctx, svc, target)
	if err != nil {
		return err
	}
	img, _, err := image.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return fmt.Errorf("decode png: %w", err)
	}

	fmt.Fprintf(out, "poke test — renderer compare\n")
	fmt.Fprintf(out, "subject: %s (#%d)  source: %s  iterations: %d\n\n",
		poke.DisplayName, poke.ID, srcLabel, iters)

	bg := style.BaseRGBA()
	chafaR := spritechafa.Renderer{}
	halfR := halfblock.Renderer{}
	nativeR := spriteimage.New()

	type timed struct {
		name string
		ansi string
		d    time.Duration
		err  error
	}

	bench := func(name string, fn func() (string, error)) timed {
		var last string
		var lastErr error
		start := time.Now()
		for i := 0; i < iters; i++ {
			last, lastErr = fn()
			if lastErr != nil {
				break
			}
		}
		return timed{name: name, ansi: last, d: time.Since(start), err: lastErr}
	}

	var rows []string
	rows = append(rows, fmt.Sprintf("%-8s  %-12s  %10s  %10s  %10s", "size", "backend", "total", "per-call", "bytes"))

	for _, side := range renderTestSizes {
		w, h := sprite.FrameSizeFor(side)
		opts := sprite.RenderOptions{
			Width:           w,
			Height:          h,
			ColorMode:       sprite.ColorTrueColor,
			BackgroundColor: bg,
			Source:          sprite.SourceArtwork,
			AlphaThreshold:  16,
			Quality:         sprite.QualityInteractive,
		}

		ch := bench("chafa", func() (string, error) {
			f, err := chafaR.Render(ctx, img, opts)
			return f.Text, err
		})
		hb := bench("halfblock", func() (string, error) {
			f, err := halfR.Render(ctx, img, opts)
			return f.Text, err
		})
		nt := bench("termimg", func() (string, error) {
			if !nativeR.Available() {
				return "", fmt.Errorf("native protocol unavailable")
			}
			f, err := nativeR.Render(ctx, img, opts)
			return f.Text, err
		})

		for _, t := range []timed{ch, hb, nt} {
			if t.err != nil {
				rows = append(rows, fmt.Sprintf("%dx%-4d  %-12s  ERROR: %v", w, h, t.name, t.err))
				continue
			}
			per := t.d / time.Duration(iters)
			rows = append(rows, fmt.Sprintf("%dx%-4d  %-12s  %10s  %10s  %10d",
				w, h, t.name, t.d.Round(time.Microsecond), per.Round(time.Microsecond), len(t.ansi)))
		}
		rows = append(rows, "")

		if req.SkipVisual {
			continue
		}

		fmt.Fprintf(out, "═══ %dx%d cells (side=%d) ═══\n", w, h, side)
		if ch.err == nil && hb.err == nil {
			printSideBySide(out, "chafa", ch.ansi, "halfblock (imgcat-style)", hb.ansi, w)
		} else {
			if ch.err == nil {
				fmt.Fprintf(out, "-- chafa --\n%s\n", ch.ansi)
			} else {
				fmt.Fprintf(out, "-- chafa ERR: %v\n", ch.err)
			}
			if hb.err == nil {
				fmt.Fprintf(out, "-- halfblock --\n%s\n", hb.ansi)
			} else {
				fmt.Fprintf(out, "-- halfblock ERR: %v\n", hb.err)
			}
		}
		if nt.err == nil && nt.ansi != "" {
			fmt.Fprintf(out, "-- termimg/%s (native; not tiled — shown alone) --\n", nativeR.ProtocolName())
			// Native graphics need a clear line + raw sequence.
			fmt.Fprint(out, nt.ansi)
			if !strings.HasSuffix(nt.ansi, "\n") {
				fmt.Fprintln(out)
			}
			fmt.Fprintln(out)
		} else if nt.err != nil {
			fmt.Fprintf(out, "-- termimg skipped: %v\n\n", nt.err)
		}
	}

	fmt.Fprintln(out, "── drag race summary ──")
	for _, line := range rows {
		fmt.Fprintln(out, line)
	}
	fmt.Fprintln(out, "Notes:")
	fmt.Fprintln(out, "  halfblock = imgcat algorithm (▄ + truecolor BG/FG), not the imgcat CLI binary.")
	fmt.Fprintln(out, "  chafa     = current mosaic fallback (ploMP4/chafa-go).")
	fmt.Fprintln(out, "  termimg   = Kitty/iTerm/Sixel encode via go-termimg (when detected).")
	return nil
}

func printSideBySide(w io.Writer, leftTitle, left, rightTitle, right string, cellW int) {
	leftLines := strings.Split(strings.TrimRight(left, "\n"), "\n")
	rightLines := strings.Split(strings.TrimRight(right, "\n"), "\n")
	n := len(leftLines)
	if len(rightLines) > n {
		n = len(rightLines)
	}
	gap := "  │  "
	fmt.Fprintf(w, "%-*s%s%s\n", cellW, leftTitle, gap, rightTitle)
	fmt.Fprintf(w, "%s%s%s\n", strings.Repeat("─", cellW), gap, strings.Repeat("─", cellW))
	for i := 0; i < n; i++ {
		l, r := "", ""
		if i < len(leftLines) {
			l = leftLines[i]
		}
		if i < len(rightLines) {
			r = rightLines[i]
		}
		// Pad visually with spaces only when line has no ANSI — for ANSI we
		// just juxtapose; titles already mark columns.
		fmt.Fprintf(w, "%s%s%s\n", l, gap, r)
	}
	fmt.Fprintln(w)
}

func loadRenderTestPNG(ctx context.Context, svc *Services, target string) (domain.Pokemon, []byte, string, error) {
	details := NewDetailService(svc.Cache)
	id, slug, err := resolveTestTarget(ctx, svc, target)
	if err != nil {
		return domain.Pokemon{}, nil, "", err
	}
	poke, err := details.PokemonForSprite(ctx, id)
	if err != nil {
		poke = domain.Pokemon{ID: id, Slug: slug, DisplayName: slug}
	}
	if poke.Slug == "" {
		poke.Slug = slug
	}
	if poke.DisplayName == "" {
		poke.DisplayName = slug
	}

	// Prefer named official artwork, then pixel, then CDN download.
	if data, err := svc.Cache.NamedSprite(ctx, poke.ID, poke.Slug, cache.SpriteKindArtwork); err == nil && len(data) > 0 {
		return poke, data, "cache:official-artwork", nil
	}
	if data, err := svc.Cache.NamedSprite(ctx, poke.ID, poke.Slug, cache.SpriteKindPixel); err == nil && len(data) > 0 {
		return poke, data, "cache:pixel", nil
	}

	url := sprite.URLForTier(poke, sprite.AssetTierHD)
	if url == "" {
		url = sprite.CDNArtworkURL(poke.ID)
	}
	data, err := httpGetPNG(ctx, url)
	if err != nil {
		// Last resort: pixel CDN.
		url = sprite.CDNPixelURL(poke.ID)
		data, err = httpGetPNG(ctx, url)
		if err != nil {
			return domain.Pokemon{}, nil, "", fmt.Errorf("download sprite: %w", err)
		}
		return poke, data, "cdn:pixel", nil
	}
	return poke, data, "cdn:official-artwork", nil
}

func resolveTestTarget(ctx context.Context, svc *Services, target string) (id int, slug string, err error) {
	index, _, err := svc.Cache.EnsureIndex(ctx, nil)
	if err != nil {
		// Offline-ish: numeric only.
		var n int
		if _, scanErr := fmt.Sscanf(target, "%d", &n); scanErr == nil && n > 0 {
			return n, fmt.Sprintf("%d", n), nil
		}
		return 0, "", err
	}
	lower := strings.ToLower(target)
	for _, sum := range index {
		if fmt.Sprintf("%d", sum.ID) == target || strings.EqualFold(sum.Slug, lower) || strings.EqualFold(sum.DisplayName, target) {
			return sum.ID, sum.Slug, nil
		}
	}
	var n int
	if _, scanErr := fmt.Sscanf(target, "%d", &n); scanErr == nil && n > 0 {
		return n, fmt.Sprintf("%d", n), nil
	}
	return 0, "", fmt.Errorf("unknown Pokémon %q", target)
}

func httpGetPNG(ctx context.Context, url string) ([]byte, error) {
	if url == "" {
		return nil, fmt.Errorf("empty url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "poke-cli/test")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}
