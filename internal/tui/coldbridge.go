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

func renderToCold(res sprite.RenderResult, frame, cacheState string) coldsched.ColdResult {
	return coldsched.ColdResult{
		Frame:       frame,
		CacheKey:    res.CacheKey,
		CacheState:  cacheState,
		Native:      res.Native && !res.UsePlaceholder && frame != "",
		AssetKind:   res.AssetKind,
		Placeholder: res.UsePlaceholder || frame == "",
	}
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
	return renderToCold(res, res.ANSI, "cache"), true
}

func (b *coldSchedBridge) RenderCold(ctx context.Context, pokemon domain.Pokemon, cells int) coldsched.ColdResult {
	if b.sprites == nil || cells < 1 {
		return coldsched.ColdResult{Err: context.Canceled}
	}
	start := time.Now()
	pokemon = b.enrichPokemon(ctx, pokemon, pokemon.ID)
	res := b.sprites.RenderCold(ctx, pokemon, cells)
	elapsed := time.Since(start)
	if res.UsePlaceholder || res.ANSI == "" {
		return coldsched.ColdResult{Err: context.Canceled, Timings: coldsched.RenderTimings{Total: elapsed}}
	}
	out := renderToCold(res, res.ANSI, "cold")
	out.Timings = coldsched.RenderTimings{SixelEncode: elapsed, Total: elapsed}
	return out
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
	m.coldSched = coldsched.NewScheduler(bridge, func(req coldsched.Request, res coldsched.ColdResult) {
		msg := coldFrameMsg{
			gen:         req.Gen,
			speciesID:   req.SpeciesID,
			cells:       req.Cells,
			ansi:        res.Frame,
			native:      res.Native,
			assetKind:   res.AssetKind,
			placeholder: res.Placeholder,
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
