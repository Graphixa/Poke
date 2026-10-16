package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

func (m *Model) foregroundColdBusy() bool {
	if m.cold != nil {
		return m.cold.ForegroundBusy()
	}
	return m.imageState == ImageLoading
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
	m.initColdRunner()
	if m.cold == nil || cells < 1 {
		return nil
	}
	// Do not ClearNative here — that flashes Kitty and is unnecessary for
	// Sixel; themed reservation + deferred overlay handle transitions.
	m.spriteLoadSpecies = pokemon.ID

	if res, hit := m.sprites.LookupCached(m.ctx, pokemon, cells); hit && !res.UsePlaceholder && res.ANSI != "" {
		m.applySpriteFrame(pokemon.ID, cells, res)
		m.syncWarmerBusy()
		return nil
	}

	m.setImageLoading()
	m.cold.Select(m.selectionGen, pokemon, cells, force)
	m.syncWarmerBusy()
	return m.coldListenCmd()
}

func (m *Model) coldListenCmd() tea.Cmd {
	if m.cold == nil {
		return nil
	}
	return listenColdFrame(m.cold.ch)
}

// coldListenIfBusy re-arms the frame listener while foreground work remains.
func (m *Model) coldListenIfBusy() tea.Cmd {
	if m.cold != nil && m.cold.ForegroundBusy() {
		return m.coldListenCmd()
	}
	return nil
}

func (m *Model) finishColdSelection(msg coldFrameMsg) tea.Cmd {
	if follow := m.applyColdFrame(msg); follow != nil {
		return follow
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
		return m.coldListenIfBusy()
	}
	m.imageLoading = false
	m.applySpriteFrame(msg.speciesID, msg.cells, sprite.RenderResult{
		ANSI:           msg.ansi,
		UsePlaceholder: msg.placeholder,
		Native:         msg.native,
		AssetKind:      msg.assetKind,
	})
	m.syncWarmerBusy()
	return m.coldListenIfBusy()
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
