package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

// fgRenderReq is one foreground cold-render identity.
// At most one active job and one latest pending request exist.
type fgRenderReq struct {
	reqID     int
	speciesID int
	gen       int
	cells     int
	pokemon   domain.Pokemon
}

func (a fgRenderReq) same(b fgRenderReq) bool {
	return a.speciesID == b.speciesID && a.gen == b.gen && a.cells == b.cells
}

func (m *Model) abortFgContext() {
	if m.spriteCancel != nil {
		m.spriteCancel()
		m.spriteCancel = nil
	}
}

func (m *Model) foregroundColdBusy() bool {
	return m.imageState == ImageLoading
}

func (m *Model) scheduleSpritePrefetchIfIdle() tea.Cmd {
	if m.foregroundColdBusy() {
		return nil
	}
	return m.scheduleSpritePrefetch()
}

func (m *Model) queueOrStartFgRender(req fgRenderReq, force bool) tea.Cmd {
	if m.fgActive != nil {
		if m.fgActive.same(req) && !force {
			return nil
		}
		cp := req
		m.fgPending = &cp
		m.abortFgContext()
		m.clearNativeDisplay()
		m.setImageLoading()
		m.spriteLoadSpecies = req.speciesID
		return nil
	}
	return m.startFgRender(req)
}

func (m *Model) startFgRender(req fgRenderReq) tea.Cmd {
	m.cancelSprite()
	ctx, cancel := context.WithCancel(m.ctx)
	m.spriteCancel = cancel
	m.spriteReqID++
	req.reqID = m.spriteReqID
	cp := req
	m.fgActive = &cp
	m.fgPending = nil
	m.spriteLoadSpecies = req.speciesID
	m.clearNativeDisplay()
	m.setImageLoading()
	return loadSpriteCmd(ctx, m.sprites, m.pokemonSrc, req.reqID, req.gen, req.speciesID, req.cells, req.pokemon)
}

func (m *Model) handleSpriteMsg(msg spriteMsg) tea.Cmd {
	if m.fgActive != nil && m.fgActive.reqID == msg.reqID {
		m.fgActive = nil
		m.spriteCancel = nil
	}

	current := msg.reqID == m.spriteReqID &&
		msg.gen == m.selectionGen &&
		m.isSelectedID(msg.speciesID) &&
		msg.cells == m.currentSpriteCells()
	if current {
		m.imageLoading = false
		m.applySpriteFrame(msg.speciesID, msg.cells, sprite.RenderResult{
			ANSI:           msg.ansi,
			UsePlaceholder: msg.placeholder,
			Native:         msg.native,
			AssetKind:      msg.assetKind,
		})
		m.fgPending = nil
		return m.scheduleSpritePrefetchIfIdle()
	}

	// Stale result: never paint. Drain the latest pending job only.
	return m.startPendingFg()
}

func (m *Model) startPendingFg() tea.Cmd {
	if m.fgActive != nil {
		return nil
	}
	if m.fgPending == nil {
		if m.imageState != ImageReady && m.imageState != ImageUnavailable {
			return m.beginSpriteLoad()
		}
		return m.scheduleSpritePrefetchIfIdle()
	}
	p := *m.fgPending
	m.fgPending = nil
	if !m.isSelectedID(p.speciesID) || p.cells != m.currentSpriteCells() || p.gen != m.selectionGen {
		return m.beginSpriteLoad()
	}
	pokemon := p.pokemon
	if cached, hit := m.sprites.LookupCached(m.ctx, pokemon, p.cells); hit && !cached.UsePlaceholder && cached.ANSI != "" {
		m.applySpriteFrame(p.speciesID, p.cells, cached)
		return m.scheduleSpritePrefetchIfIdle()
	}
	return m.startFgRender(p)
}
