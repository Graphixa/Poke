package tui

import (
	"context"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

// coldStopTimeout bounds how long quit waits for an in-flight encode to unwind.
const coldStopTimeout = 2 * time.Second

// coldFrameMsg delivers a foreground-render frame to the Bubble Tea loop.
type coldFrameMsg struct {
	gen         int
	speciesID   int
	cells       int
	ansi        string
	placeholder bool
	native      bool
	assetKind   string
}

// coldReq identifies one foreground render target.
type coldReq struct {
	gen, speciesID, cells int
	pokemon               domain.Pokemon
}

func (a coldReq) same(b coldReq) bool {
	return a.gen == b.gen && a.speciesID == b.speciesID && a.cells == b.cells
}

type coldJob struct {
	req      coldReq
	ctx      context.Context
	cancel   context.CancelFunc
	canceled bool // guarded by coldRunner.mu
}

// coldRunner is the latest-selection-wins foreground render path: one worker
// goroutine, at most one active render and one pending (the newest selection).
// A cancelled render stays "active" until it unwinds, so renders never overlap.
type coldRunner struct {
	sprites    SpriteRenderer
	pokemonSrc PokemonSource
	ctx        context.Context
	stopCtx    context.CancelFunc
	ch         chan coldFrameMsg

	mu      sync.Mutex
	sel     coldReq
	active  *coldJob
	pending *coldReq
	stopped bool
	wg      sync.WaitGroup
}

func newColdRunner(ctx context.Context, sprites SpriteRenderer, src PokemonSource) *coldRunner {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	return &coldRunner{sprites: sprites, pokemonSrc: src, ctx: ctx, stopCtx: cancel, ch: make(chan coldFrameMsg, 2)}
}

// Select makes pokemon the current target. The caller has already missed the
// cache. force aborts an in-flight render of the same target (artwork upgrade).
func (r *coldRunner) Select(gen int, pokemon domain.Pokemon, cells int, force bool) {
	req := coldReq{gen: gen, speciesID: pokemon.ID, cells: cells, pokemon: pokemon}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return
	}
	r.sel = req
	a := r.active
	if a != nil && !a.canceled && a.req.same(req) {
		if !force {
			r.pending = nil // user scrolled back to the active target
			return
		}
		a.canceled = true
		a.cancel()
	}
	if a != nil {
		r.pending = &req
		return
	}
	r.start(req)
}

// start launches the worker; caller holds mu and active must be nil.
func (r *coldRunner) start(req coldReq) {
	r.active = r.newJob(req)
	r.wg.Add(1)
	go r.run(r.active)
}

func (r *coldRunner) newJob(req coldReq) *coldJob {
	ctx, cancel := context.WithCancel(r.ctx)
	return &coldJob{req: req, ctx: ctx, cancel: cancel}
}

func (r *coldRunner) run(job *coldJob) {
	defer r.wg.Done()
	for lookup := false; job != nil; lookup = true {
		res, cached := r.render(job, lookup)
		ok := usableFrame(res)

		r.mu.Lock()
		current := !job.canceled && r.sel.same(job.req)
		var next *coldJob
		r.active = nil
		if r.pending != nil && !r.stopped {
			next = r.newJob(*r.pending)
			r.pending = nil
			r.active = next
		}
		r.mu.Unlock()
		job.cancel()

		// Failed (non-cancelled) renders surface as a placeholder so the
		// spinner never hangs.
		if current && (ok || r.ctx.Err() == nil) {
			msg := coldFrameMsg{gen: job.req.gen, speciesID: job.req.speciesID, cells: job.req.cells, placeholder: !ok}
			if ok {
				msg.ansi, msg.native, msg.assetKind = res.ANSI, res.Native, res.AssetKind
			}
			select {
			case r.ch <- msg:
			case <-r.ctx.Done():
			}
		}
		if ok && !cached && res.CacheKey != "" {
			_ = r.sprites.PersistFrame(r.ctx, res.CacheKey, res.ANSI)
		}
		job = next
	}
}

// render probes the cache first when lookup is set (a queued job may have been
// warmed meanwhile), else encodes cold.
func (r *coldRunner) render(job *coldJob, lookup bool) (res sprite.RenderResult, cached bool) {
	p := r.enrich(job.ctx, job.req.pokemon)
	if lookup {
		if res, hit := r.sprites.LookupCached(job.ctx, p, job.req.cells); hit && usableFrame(res) {
			return res, true
		}
	}
	return r.sprites.RenderCold(job.ctx, p, job.req.cells), false
}

func usableFrame(res sprite.RenderResult) bool { return !res.UsePlaceholder && res.ANSI != "" }

func (r *coldRunner) enrich(ctx context.Context, p domain.Pokemon) domain.Pokemon {
	if r.pokemonSrc != nil && p.Sprites.FrontDefault == "" && p.Sprites.OfficialArtwork == "" && p.ID > 0 {
		if full, err := r.pokemonSrc.PokemonForSprite(ctx, p.ID); err == nil {
			return full
		}
	}
	return p
}

// AbortForeground cancels active and pending work; the runner stays usable.
func (r *coldRunner) AbortForeground() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active != nil {
		r.active.canceled = true
		r.active.cancel()
	}
	r.pending = nil
	r.sel = coldReq{}
}

// Stop cancels all work and waits for the worker to exit (or ctx to expire).
func (r *coldRunner) Stop(ctx context.Context) error {
	r.mu.Lock()
	r.stopped = true
	r.pending = nil
	r.mu.Unlock()
	r.stopCtx()
	done := make(chan struct{})
	go func() { r.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ForegroundBusy reports whether a live render is active or pending.
func (r *coldRunner) ForegroundBusy() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return (r.active != nil && !r.active.canceled) || r.pending != nil
}

// ActiveSpecies returns the species of the live active render, or 0.
func (r *coldRunner) ActiveSpecies() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active != nil && !r.active.canceled {
		return r.active.req.speciesID
	}
	return 0
}

// PendingSpecies returns the species of the pending render, or 0.
func (r *coldRunner) PendingSpecies() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pending != nil {
		return r.pending.speciesID
	}
	return 0
}

func (m *Model) initColdRunner() {
	if m.cold == nil && m.sprites != nil {
		m.cold = newColdRunner(m.ctx, m.sprites, m.pokemonSrc)
	}
}

func (m *Model) shutdown() {
	// Drop any pending DCS so teardown Writes cannot re-paint on the main screen.
	if m.compositor != nil {
		m.compositor.ClearNative()
	}
	if m.sprites != nil {
		m.sprites.ClearNative()
	}
	if m.cold != nil {
		ctx, cancel := context.WithTimeout(context.Background(), coldStopTimeout)
		defer cancel()
		_ = m.cold.Stop(ctx)
	}
}

func listenColdFrame(ch <-chan coldFrameMsg) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		return <-ch
	}
}
