package app

import (
	"context"
	"fmt"
	"image"
	"io"
	"strings"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/render/sprite/ascii"
	"github.com/Graphixa/Pokedex/internal/render/sprite/braille"
	"github.com/Graphixa/Pokedex/internal/render/sprite/halfblock"
	"github.com/Graphixa/Pokedex/internal/render/sprite/placeholder"
	"github.com/Graphixa/Pokedex/internal/search"
	"github.com/Graphixa/Pokedex/internal/tui/style"
)

// spritePreview is one named render pass for poke test.
type spritePreview struct {
	Label string
	Mode  string
	Width int
	Height int
	Color sprite.ColorMode
	Render func(ctx context.Context, img image.Image, opts sprite.RenderOptions) (sprite.Frame, error)
}

// TestSprites prints Charizard and Blastoise through every text renderer so
// modes can be compared visually. Temporary assessment command.
func (h *Handler) TestSprites(ctx context.Context) error {
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
	previews := buildSpritePreviews()

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
		img, err := sprite.DecodeImage(res.PNG)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: decode: %v", target, err))
			continue
		}

		fmt.Fprintf(out, "\n%s\n=== %s #%d (%s) ===\n%s\n",
			strings.Repeat("═", 48),
			detail.Pokemon.DisplayName,
			detail.Pokemon.ID,
			strings.Join(typeNames(detail.Pokemon), "/"),
			strings.Repeat("═", 48),
		)

		for _, p := range previews {
			opts := sprite.RenderOptions{
				Width:           p.Width,
				Height:          p.Height,
				PreserveAR:      true,
				ColorMode:       p.Color,
				BackgroundColor: bg,
				AlphaThreshold:  16,
				Source:          sprite.SourcePixel,
				Label:           detail.Pokemon.DisplayName,
				ContentHash:     res.ContentHash,
			}
			fmt.Fprintf(out, "\n--- %s ---\n", p.Label)
			frame, err := p.Render(ctx, img, opts)
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s/%s: %v", target, p.Label, err))
				fmt.Fprintf(out, "[RENDER ERROR] %v\n", err)
				continue
			}
			if strings.TrimSpace(frame.Text) == "" {
				failures = append(failures, fmt.Sprintf("%s/%s: empty output", target, p.Label))
				fmt.Fprintf(out, "[EMPTY OUTPUT]\n")
				continue
			}
			fmt.Fprintln(out, frame.Text)
			fmt.Fprintf(out, "\n(backend=%s cells=%dx%d)\n", frame.Backend, p.Width, p.Height)
		}
	}

	fmt.Fprintln(out)
	if len(failures) > 0 {
		fmt.Fprintf(out, "FAILED %d preview(s):\n", len(failures))
		for _, f := range failures {
			fmt.Fprintf(out, "  - %s\n", f)
		}
		return fmt.Errorf("sprite test: %d preview(s) failed", len(failures))
	}
	fmt.Fprintln(out, "All sprite previews rendered successfully.")
	return nil
}

func buildSpritePreviews() []spritePreview {
	hb := halfblock.Renderer{}
	br := braille.Renderer{}
	as := ascii.Renderer{}
	ph := placeholder.Renderer{}

	return []spritePreview{
		{
			Label: "halfblock truecolor 24x24 (TUI size)",
			Mode: "halfblock", Width: 24, Height: 24, Color: sprite.ColorTrueColor,
			Render: hb.Render,
		},
		{
			Label: "halfblock truecolor 40x40 (larger)",
			Mode: "halfblock", Width: 40, Height: 40, Color: sprite.ColorTrueColor,
			Render: hb.Render,
		},
		{
			Label: "halfblock truecolor 48x48 (largest)",
			Mode: "halfblock", Width: 48, Height: 48, Color: sprite.ColorTrueColor,
			Render: hb.Render,
		},
		{
			Label: "halfblock ansi-256 40x40",
			Mode: "halfblock", Width: 40, Height: 40, Color: sprite.Color256,
			Render: hb.Render,
		},
		{
			Label: "braille 24x24 (TUI size)",
			Mode: "braille", Width: 24, Height: 24, Color: sprite.ColorMono,
			Render: br.Render,
		},
		{
			Label: "braille 40x40 (larger)",
			Mode: "braille", Width: 40, Height: 40, Color: sprite.ColorMono,
			Render: br.Render,
		},
		{
			Label: "braille 48x48 (largest)",
			Mode: "braille", Width: 48, Height: 48, Color: sprite.ColorMono,
			Render: br.Render,
		},
		{
			Label: "ascii 24x24 (TUI size)",
			Mode: "ascii", Width: 24, Height: 24, Color: sprite.ColorMono,
			Render: as.Render,
		},
		{
			Label: "ascii 40x40 (larger)",
			Mode: "ascii", Width: 40, Height: 40, Color: sprite.ColorMono,
			Render: as.Render,
		},
		{
			Label: "ascii 48x48 (largest)",
			Mode: "ascii", Width: 48, Height: 48, Color: sprite.ColorMono,
			Render: as.Render,
		},
		{
			Label: "placeholder 24x24",
			Mode: "placeholder", Width: 24, Height: 24, Color: sprite.ColorMono,
			Render: ph.Render,
		},
	}
}

func typeNames(p domain.Pokemon) []string {
	out := make([]string, 0, len(p.Types))
	for _, t := range p.Types {
		out = append(out, t.Name)
	}
	return out
}
