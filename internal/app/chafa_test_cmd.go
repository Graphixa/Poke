package app

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/search"
	"github.com/Graphixa/Pokedex/internal/tui/style"
	"github.com/Graphixa/Pokedex/internal/uiimage"
	"github.com/ploMP4/chafa-go"
)

// chafaPreview is one named Chafa render pass for poke chafa.
type chafaPreview struct {
	Label   string
	Width   int
	Height  int
	Mode    chafa.CanvasMode
	Quality uiimage.QualityPreset
}

// TestChafaSprites prints Charizard and Blastoise through Chafa quality
// presets so fidelity options can be compared visually.
func (h *Handler) TestChafaSprites(ctx context.Context) error {
	out := h.Stdout
	if out == nil {
		out = io.Discard
	}
	index, err := h.ensureIndex(ctx)
	if err != nil {
		return err
	}
	idx := search.Build(index)
	svc, err := h.Services()
	if err != nil {
		return err
	}
	details := NewDetailService(svc.Cache)
	source := &sprite.SourceResolver{Store: svc.Cache, API: svc.API}
	bg := style.BaseRGBA()

	targets := []string{"charizard", "blastoise"}
	previews := buildChafaPreviews()

	var failures []string
	for _, target := range targets {
		sum, ok := idx.Resolve(target)
		if !ok {
			failures = append(failures, fmt.Sprintf("%s: not in index", target))
			fmt.Fprintf(out, "\n=== %s (MISSING) ===\n", target)
			continue
		}
		detail, err := details.Load(ctx, sum.ID)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: load: %v", target, err))
			fmt.Fprintf(out, "\n=== %s #%d (LOAD FAILED) ===\n", sum.DisplayName, sum.ID)
			continue
		}
		res := source.Resolve(ctx, detail.Pokemon)
		if res.UsePlaceholder || len(res.PNG) == 0 {
			failures = append(failures, fmt.Sprintf("%s: no sprite PNG", target))
			fmt.Fprintf(out, "\n=== %s #%d (NO SPRITE) ===\n", detail.Pokemon.DisplayName, detail.Pokemon.ID)
			continue
		}
		data, err := uiimage.FromPNG(res.PNG)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: decode: %v", target, err))
			continue
		}

		fmt.Fprintf(out, "\n%s\n=== %s #%d (%s) [chafa] ===\n%s\n",
			strings.Repeat("═", 48),
			detail.Pokemon.DisplayName,
			detail.Pokemon.ID,
			strings.Join(typeNames(detail.Pokemon), "/"),
			strings.Repeat("═", 48),
		)

		for _, p := range previews {
			fmt.Fprintf(out, "\n--- %s ---\n", p.Label)
			ansi := uiimage.GenerateANSIBlockOpts(data, uiimage.RenderOpts{
				Width:      p.Width,
				Height:     p.Height,
				Mode:       p.Mode,
				Quality:    p.Quality,
				Background: bg,
			})
			if strings.TrimSpace(ansi) == "" {
				failures = append(failures, fmt.Sprintf("%s/%s: empty output", target, p.Label))
				fmt.Fprintf(out, "[EMPTY OUTPUT]\n")
				continue
			}
			fmt.Fprintln(out, ansi)
			fmt.Fprintf(out, "\n(backend=chafa cells≤%dx%d mode=%s quality=%s)\n",
				p.Width, p.Height, canvasModeName(p.Mode), qualityName(p.Quality))
		}
	}

	fmt.Fprintln(out)
	if len(failures) > 0 {
		fmt.Fprintf(out, "FAILED %d preview(s):\n", len(failures))
		for _, f := range failures {
			fmt.Fprintf(out, "  - %s\n", f)
		}
		return fmt.Errorf("chafa sprite test: %d preview(s) failed", len(failures))
	}
	fmt.Fprintln(out, "All chafa sprite previews rendered successfully.")
	return nil
}

func buildChafaPreviews() []chafaPreview {
	return []chafaPreview{
		{
			Label: "baseline (all symbols) 40x40",
			Width: 40, Height: 40,
			Mode: chafa.CHAFA_CANVAS_MODE_TRUECOLOR, Quality: uiimage.QualityBaseline,
		},
		{
			Label: "mosaic sextant/octant 40x40 (max quality)",
			Width: 40, Height: 40,
			Mode: chafa.CHAFA_CANVAS_MODE_TRUECOLOR, Quality: uiimage.QualityMosaic,
		},
		{
			Label: "mosaic-fast 40x40 (TUI browse path)",
			Width: 40, Height: 40,
			Mode: chafa.CHAFA_CANVAS_MODE_TRUECOLOR, Quality: uiimage.QualityMosaicFast,
		},
		{
			Label: "mosaic-fast 24x24 (compact)",
			Width: 24, Height: 24,
			Mode: chafa.CHAFA_CANVAS_MODE_TRUECOLOR, Quality: uiimage.QualityMosaicFast,
		},
		{
			Label: "sharp half+quad 40x40",
			Width: 40, Height: 40,
			Mode: chafa.CHAFA_CANVAS_MODE_TRUECOLOR, Quality: uiimage.QualitySharp,
		},
	}
}

func canvasModeName(mode chafa.CanvasMode) string {
	switch mode {
	case chafa.CHAFA_CANVAS_MODE_TRUECOLOR:
		return "truecolor"
	case chafa.CHAFA_CANVAS_MODE_INDEXED_256:
		return "indexed-256"
	case chafa.CHAFA_CANVAS_MODE_INDEXED_240:
		return "indexed-240"
	case chafa.CHAFA_CANVAS_MODE_INDEXED_16:
		return "indexed-16"
	case chafa.CHAFA_CANVAS_MODE_FGBG_BGFG:
		return "fgbg-bgfg"
	case chafa.CHAFA_CANVAS_MODE_FGBG:
		return "fgbg"
	default:
		return fmt.Sprintf("%d", mode)
	}
}

func qualityName(q uiimage.QualityPreset) string {
	switch q {
	case uiimage.QualityBaseline:
		return "baseline"
	case uiimage.QualitySharp:
		return "sharp"
	case uiimage.QualityMosaic:
		return "mosaic"
	case uiimage.QualityMosaicFast:
		return "mosaic-fast"
	default:
		return fmt.Sprintf("%d", q)
	}
}

func typeNames(p domain.Pokemon) []string {
	out := make([]string, 0, len(p.Types))
	for _, t := range p.Types {
		out = append(out, t.Name)
	}
	return out
}
