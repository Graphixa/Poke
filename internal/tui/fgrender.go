package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

func (m *Model) foregroundColdBusy() bool {
	if m.coldSched != nil {
		return m.coldSched.ForegroundBusy()
	}
	return m.imageState == ImageLoading
}

func (m *Model) scheduleSpritePrefetchIfIdle() tea.Cmd {
	// Foreground ±1 speculative prewarm removed; background warmer handles this (phase 8).
	return nil
}

func (m *Model) syncWarmerBusy() {
	busy := m.foregroundColdBusy()
	if m.nativeWarmer != nil {
		m.nativeWarmer.SetForegroundBusy(busy)
	}
	if m.warmer != nil {
		m.warmer.SetForegroundBusy(busy)
	}
}

func (m *Model) runColdSelection(pokemon domain.Pokemon, cells int, force bool) tea.Cmd {
	m.initColdScheduler()
	if m.coldSched == nil || cells < 1 {
		return nil
	}
	m.clearNativeDisplay()
	m.spriteLoadSpecies = pokemon.ID

	if res, hit := m.sprites.LookupCached(m.ctx, pokemon, cells); hit && !res.UsePlaceholder && res.ANSI != "" {
		m.applySpriteFrame(pokemon.ID, cells, res)
		m.syncWarmerBusy()
		return nil
	}

	m.setImageLoading()
	m.coldSched.Select(m.ctx, m.selectionGen, pokemon, cells, force, true)
	m.syncWarmerBusy()
	return m.coldListenCmd()
}

func (m *Model) coldListenCmd() tea.Cmd {
	var spin tea.Cmd
	if m.imageState == ImageLoading {
		spin = func() tea.Msg { return m.spinner.Tick() }
	}
	return tea.Batch(listenColdFrame(m.coldFrameCh), spin)
}

func (m *Model) finishColdSelection(msg coldFrameMsg) tea.Cmd {
	follow := m.applyColdFrame(msg)
	if follow != nil {
		var spin tea.Cmd
		if m.imageState == ImageLoading {
			spin = func() tea.Msg { return m.spinner.Tick() }
		}
		return tea.Batch(follow, spin)
	}
	if m.imageState == ImageLoading {
		return m.coldListenCmd()
	}
	return nil
}

func (m *Model) applyColdFrame(msg coldFrameMsg) tea.Cmd {
	current := msg.gen == m.selectionGen &&
		m.isSelectedID(msg.speciesID) &&
		msg.cells == m.currentSpriteCells()
	if !current {
		sprite.NoteStale()
		m.syncWarmerBusy()
		if m.coldSched != nil && m.coldSched.ForegroundBusy() {
			return listenColdFrame(m.coldFrameCh)
		}
		return nil
	}
	m.imageLoading = false
	m.applySpriteFrame(msg.speciesID, msg.cells, sprite.RenderResult{
		ANSI:           msg.ansi,
		UsePlaceholder: msg.placeholder,
		Native:         msg.native,
		AssetKind:      msg.assetKind,
	})
	m.syncWarmerBusy()
	if m.coldSched != nil && m.coldSched.ForegroundBusy() {
		return listenColdFrame(m.coldFrameCh)
	}
	return nil
}

func (m *Model) handleColdFrame(msg coldFrameMsg) tea.Cmd {
	return m.finishColdSelection(msg)
}

func (m *Model) queueOrStartFgRender(pokemon domain.Pokemon, cells int, force bool) tea.Cmd {
	return m.runColdSelection(pokemon, cells, force)
}

func (m *Model) handleSpriteMsg(msg spriteMsg) tea.Cmd {
	// Legacy spriteMsg path retained for tests; production uses coldFrameMsg.
	current := msg.reqID == m.spriteReqID &&
		msg.gen == m.selectionGen &&
		m.isSelectedID(msg.speciesID) &&
		msg.cells == m.currentSpriteCells()
	if !current {
		sprite.NoteStale()
		m.syncWarmerBusy()
		return nil
	}
	m.imageLoading = false
	m.applySpriteFrame(msg.speciesID, msg.cells, sprite.RenderResult{
		ANSI:           msg.ansi,
		UsePlaceholder: msg.placeholder,
		Native:         msg.native,
		AssetKind:      msg.assetKind,
	})
	m.syncWarmerBusy()
	return nil
}
