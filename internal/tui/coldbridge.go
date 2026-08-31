package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/render/sprite/coldsched"
)

// coldFrameMsg delivers a scheduler-ready frame to the Bubble Tea loop.
type coldFrameMsg struct {
	gen         int
	speciesID   int
	cells       int
	ansi        string
	placeholder bool
	native      bool
	assetKind   string
}

type coldSchedBridge struct {
	sprites    SpriteRenderer
	pokemonSrc PokemonSource
	displayCh  chan coldFrameMsg
	lastRes    sprite.RenderResult
}

func newColdSchedBridge(sprites SpriteRenderer, pokemonSrc PokemonSource, ch chan coldFrameMsg) *coldSchedBridge {
	return &coldSchedBridge{sprites: sprites, pokemonSrc: pokemonSrc, displayCh: ch}
}

func (b *coldSchedBridge) enrichPokemon(ctx context.Context, pokemon domain.Pokemon, speciesID int) domain.Pokemon {
	if b.pokemonSrc != nil && pokemon.Sprites.FrontDefault == "" && pokemon.Sprites.OfficialArtwork == "" && speciesID > 0 {
		if p, err := b.pokemonSrc.PokemonForSprite(ctx, speciesID); err == nil {
			return p
		}
	}
	return pokemon
}

func (b *coldSchedBridge) LookupCached(ctx context.Context, pokemon domain.Pokemon, cells int) (coldsched.ColdResult, bool) {
	if b.sprites == nil || cells < 1 {
		return coldsched.ColdResult{}, false
	}
	pokemon = b.enrichPokemon(ctx, pokemon, pokemon.ID)
	res, hit := b.sprites.LookupCached(ctx, pokemon, cells)
	if !hit || res.UsePlaceholder || res.ANSI == "" {
		return coldsched.ColdResult{}, false
	}
	b.lastRes = res
	return coldsched.ColdResult{
		Frame:      res.ANSI,
		CacheKey:   res.CacheKey,
		CacheState: "cache",
	}, true
}

func (b *coldSchedBridge) RenderCold(ctx context.Context, pokemon domain.Pokemon, cells int) coldsched.ColdResult {
	if b.sprites == nil || cells < 1 {
		return coldsched.ColdResult{Err: context.Canceled}
	}
	start := time.Now()
	pokemon = b.enrichPokemon(ctx, pokemon, pokemon.ID)
	res := b.sprites.RenderCold(ctx, pokemon, cells)
	elapsed := time.Since(start)
	b.lastRes = res
	if res.UsePlaceholder || res.ANSI == "" {
		return coldsched.ColdResult{Err: context.Canceled, Timings: coldsched.RenderTimings{Total: elapsed}}
	}
	return coldsched.ColdResult{
		Frame:      res.ANSI,
		CacheKey:   res.CacheKey,
		CacheState: "cold",
		Timings:    coldsched.RenderTimings{SixelEncode: elapsed, Total: elapsed},
	}
}

func (b *coldSchedBridge) Persist(ctx context.Context, cacheKey, frame string) error {
	if b.sprites == nil || cacheKey == "" || frame == "" {
		return nil
	}
	return b.sprites.PersistFrame(ctx, cacheKey, frame)
}

func (m *Model) initColdScheduler() {
	if m.coldSched != nil || m.sprites == nil {
		return
	}
	m.coldFrameCh = make(chan coldFrameMsg, 2)
	bridge := newColdSchedBridge(m.sprites, m.pokemonSrc, m.coldFrameCh)
	m.coldSched = coldsched.NewScheduler(bridge, func(req coldsched.Request, frame string) {
		r := bridge.lastRes
		if r.ANSI == "" {
			r.ANSI = frame
		}
		msg := coldFrameMsg{
			gen:         req.Gen,
			speciesID:   req.SpeciesID,
			cells:       req.Cells,
			ansi:        frame,
			native:      r.Native && !r.UsePlaceholder && frame != "",
			assetKind:   r.AssetKind,
			placeholder: r.UsePlaceholder || frame == "",
		}
		select {
		case m.coldFrameCh <- msg:
		default:
		}
	})
}

func listenColdFrame(ch <-chan coldFrameMsg) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		return <-ch
	}
}
