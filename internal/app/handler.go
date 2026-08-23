package app

import (
	"context"
	"fmt"
	"io"
	"math/rand"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/cli"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/output"
	"github.com/Graphixa/Pokedex/internal/search"
)

// IntnRand is a deterministic injectable random source.
type IntnRand interface {
	Intn(n int) int
}

// Handler implements search, random, and root inline/browse routing.
type Handler struct {
	Services func() (*Services, error)
	Stdout   io.Writer
	Stderr   io.Writer
	Rand     IntnRand
	// Browse is invoked for TUI mode (Phase 4). Nil means no-op success.
	Browse func(context.Context, BrowseRequest) error
}

// BrowseRequest carries resolved selection or search prefill for the TUI.
type BrowseRequest struct {
	Target   string
	Selected *domain.SpeciesSummary
	Prefill  string
	Random   bool
}

func (h *Handler) RunRoot(ctx context.Context, request cli.RootRequest) error {
	index, err := h.ensureIndex(ctx)
	if err != nil {
		return err
	}
	idx := search.Build(index)

	target := request.Target
	if request.InlineMode != cli.InlineNone {
		sum, ok := idx.Resolve(target)
		if !ok {
			return &NotFoundError{Query: target}
		}
		return h.printInline(ctx, sum, request.InlineMode)
	}

	browse := BrowseRequest{Target: target}
	if target != "" {
		if sum, ok := idx.Resolve(target); ok {
			browse.Selected = &sum
		} else {
			browse.Prefill = target
		}
	}
	if h.Browse != nil {
		return h.Browse(ctx, browse)
	}
	return nil
}

func (h *Handler) Search(ctx context.Context, query string) error {
	index, err := h.ensureIndex(ctx)
	if err != nil {
		return err
	}
	hits := search.Build(index).Search(query)
	return output.SearchTable(h.Stdout, hits)
}

func (h *Handler) Random(ctx context.Context, mode cli.InlineMode) error {
	index, err := h.ensureIndex(ctx)
	if err != nil {
		return err
	}
	if len(index) == 0 {
		return fmt.Errorf("species index is empty")
	}
	r := h.Rand
	if r == nil {
		r = rand.New(rand.NewSource(rand.Int63()))
	}
	sum := index[r.Intn(len(index))]
	if mode != cli.InlineNone {
		return h.printInline(ctx, sum, mode)
	}
	browse := BrowseRequest{Selected: &sum, Random: true, Target: sum.Slug}
	if h.Browse != nil {
		return h.Browse(ctx, browse)
	}
	return nil
}

func (h *Handler) printInline(ctx context.Context, sum domain.SpeciesSummary, mode cli.InlineMode) error {
	svc, err := h.Services()
	if err != nil {
		return err
	}
	details := NewDetailService(svc.Cache)
	detail, err := details.Load(ctx, sum.ID)
	if err != nil {
		return err
	}
	switch mode {
	case cli.InlineInfo:
		return output.Info(h.Stdout, detail.Pokemon, detail.Species)
	case cli.InlineStats:
		return output.Stats(h.Stdout, detail.Pokemon, detail.Species.DisplayName)
	case cli.InlineEvolution:
		tree, err := details.Evolution(ctx, detail.Species)
		if err != nil {
			return err
		}
		return output.Evolution(h.Stdout, tree)
	default:
		return fmt.Errorf("unknown inline mode %q", mode)
	}
}

func (h *Handler) ensureIndex(ctx context.Context) ([]domain.SpeciesSummary, error) {
	svc, err := h.Services()
	if err != nil {
		return nil, err
	}
	progress := func(p cache.Progress) {
		if h.Stderr == nil {
			return
		}
		switch p.Stage {
		case "index":
			if p.Message != "" {
				fmt.Fprintf(h.Stderr, "%s\n", p.Message)
			}
		case "enrich":
			if p.Total > 0 && (p.Done == p.Total || p.Done%25 == 0) {
				fmt.Fprintf(h.Stderr, "Enriching types %d/%d\n", p.Done, p.Total)
			}
		}
	}
	index, stale, err := svc.Cache.EnsureIndex(ctx, progress)
	if err != nil {
		return nil, err
	}
	if stale {
		if h.Stderr != nil {
			fmt.Fprintln(h.Stderr, "Index is stale; refreshing in the background…")
		}
	}
	if svc.Cache.ShouldBackgroundRefresh(stale) {
		svc.Cache.BackgroundRefresh(ctx, nil)
	}
	return index, nil
}

// FixedRand returns a constant Intn for deterministic tests.
type FixedRand struct {
	N int
}

func (r FixedRand) Intn(n int) int {
	if n <= 0 {
		return 0
	}
	if r.N < 0 {
		return 0
	}
	return r.N % n
}
