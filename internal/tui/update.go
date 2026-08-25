package tui

import (
	"context"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		prevShow := m.layout().ShowSprite
		prevSide := m.currentSpriteSide()
		m.width = msg.Width
		m.height = msg.Height
		m.help.SetWidth(msg.Width)
		m.ensureListVisible()
		return m, m.handleSpriteLayoutChange(prevShow, prevSide)

	case coreMsg:
		if msg.gen != m.selectionGen || !m.isSelectedID(msg.speciesID) {
			return m, nil
		}
		r := m.ensureResource(msg.speciesID)
		r.loadingCore = false
		if msg.err != nil {
			r.CoreErr = msg.err
			return m, nil
		}
		r.Pokemon = msg.pokemon
		r.Species = msg.species
		r.HasCore = true
		r.CoreErr = nil
		m.clearSpriteArt(r)
		return m, tea.Batch(m.ensureBrowseLazyLoad(), m.ensureSpriteLoad())

	case evolutionMsg:
		if msg.gen != m.selectionGen || !m.isSelectedID(msg.speciesID) {
			return m, nil
		}
		r := m.ensureResource(msg.speciesID)
		r.loadingEvolution = false
		if msg.err != nil {
			r.EvolutionErr = msg.err
			return m, nil
		}
		r.Evolution = msg.tree
		r.HasEvolution = true
		r.EvolutionErr = nil
		return m, nil

	case encountersMsg:
		if msg.gen != m.selectionGen || !m.isSelectedID(msg.speciesID) {
			return m, nil
		}
		r := m.ensureResource(msg.speciesID)
		r.loadingEncounters = false
		if msg.err != nil {
			r.EncountersErr = msg.err
			return m, nil
		}
		r.Encounters = msg.rows
		r.HasEncounters = true
		r.EncountersErr = nil
		if m.showDialog {
			m.syncDialogViewport()
		}
		return m, nil

	case movesMsg:
		if msg.gen != m.selectionGen || !m.isSelectedID(msg.speciesID) {
			return m, nil
		}
		r := m.ensureResource(msg.speciesID)
		r.loadingMoves = false
		if msg.err != nil {
			r.MovesErr = msg.err
			return m, nil
		}
		r.Moves = msg.rows
		r.HasMoves = true
		r.MovesErr = nil
		if m.showDialog {
			m.syncDialogViewport()
		}
		return m, nil

	case spriteMsg:
		if msg.reqID != m.spriteReqID || msg.gen != m.selectionGen || !m.isSelectedID(msg.speciesID) {
			return m, nil
		}
		if msg.cells != m.currentSpriteCells() {
			return m, nil
		}
		r := m.ensureResource(msg.speciesID)
		r.loadingSprite = false
		r.SpriteCells = msg.cells
		r.HasSprite = true
		if msg.placeholder || msg.ansi == "" {
			r.SpriteANSI = ""
			r.SpritePlaceholder = true
		} else {
			r.SpriteANSI = msg.ansi
			r.SpritePlaceholder = false
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.ForceQuit) {
		m.quitting = true
		return m, tea.Quit
	}

	if m.showHelp {
		if key.Matches(msg, m.keys.Help, m.keys.Clear) {
			m.showHelp = false
		}
		return m, nil
	}

	if m.showDialog {
		return m.handleDialogKey(msg)
	}

	if key.Matches(msg, m.keys.Unbound) {
		return m, nil
	}

	if key.Matches(msg, m.keys.Help) {
		m.showHelp = true
		return m, nil
	}

	if key.Matches(msg, m.keys.Enter) {
		return m.openDialog()
	}

	if key.Matches(msg, m.keys.Info) {
		if m.queryValue() == "" && m.hasSelectedCore() {
			return m.openDialog()
		}
		return m.updateSearch(msg)
	}

	if key.Matches(msg, m.keys.Quit) {
		if m.layout().TooSmall || m.canQuitWithQ() {
			m.quitting = true
			return m, tea.Quit
		}
		return m.updateSearch(msg)
	}

	if key.Matches(msg, m.keys.Clear) {
		m.clearQuery()
		return m, m.syncSelectionLoads()
	}

	if key.Matches(msg, m.keys.Up) {
		m.moveCursor(-1)
		return m, m.syncSelectionLoads()
	}
	if key.Matches(msg, m.keys.Down) {
		m.moveCursor(1)
		return m, m.syncSelectionLoads()
	}
	if key.Matches(msg, m.keys.TabNext) {
		m.browseView = m.browseView.Next()
		return m, m.ensureBrowseLazyLoad()
	}
	if key.Matches(msg, m.keys.TabPrev) {
		m.browseView = m.browseView.Prev()
		return m, m.ensureBrowseLazyLoad()
	}

	return m.updateSearch(msg)
}

func (m Model) handleDialogKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Clear, m.keys.Quit) {
		m.showDialog = false
		return m, nil
	}
	if key.Matches(msg, m.keys.TabNext) {
		m.dialogSection = m.dialogSection.Next()
		m.dialogVP.GotoTop()
		m.syncDialogViewport()
		return m, m.ensureDialogLazyLoad()
	}
	if key.Matches(msg, m.keys.TabPrev) {
		m.dialogSection = m.dialogSection.Prev()
		m.dialogVP.GotoTop()
		m.syncDialogViewport()
		return m, m.ensureDialogLazyLoad()
	}
	if key.Matches(msg, m.keys.Up) {
		m.scrollDialog(-1)
		return m, nil
	}
	if key.Matches(msg, m.keys.Down) {
		m.scrollDialog(1)
		return m, nil
	}
	if key.Matches(msg, m.keys.PageUp) {
		m.scrollDialog(-m.dialogPageSize())
		return m, nil
	}
	if key.Matches(msg, m.keys.PageDown) {
		m.scrollDialog(m.dialogPageSize())
		return m, nil
	}
	if key.Matches(msg, m.keys.Home) {
		m.syncDialogViewport()
		m.dialogVP.GotoTop()
		return m, nil
	}
	if key.Matches(msg, m.keys.End) {
		m.syncDialogViewport()
		m.dialogVP.GotoBottom()
		return m, nil
	}
	return m, nil
}

func (m Model) openDialog() (tea.Model, tea.Cmd) {
	if !m.hasSelectedCore() {
		return m, nil
	}
	m.showDialog = true
	m.dialogSection = DialogMoves
	m.dialogVP.GotoTop()
	m.syncDialogViewport()
	return m, m.ensureDialogLazyLoad()
}

func (m Model) updateSearch(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	prev := m.search.Value()
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	if m.search.Value() != prev {
		m.refilter()
		m.dialogVP.GotoTop()
		return m, tea.Batch(cmd, m.syncSelectionLoads())
	}
	return m, cmd
}

func (m Model) hasSelectedCore() bool {
	sel, ok := m.selected()
	if !ok {
		return false
	}
	r := m.resources[sel.ID]
	return r != nil && r.HasCore
}

func (m *Model) isSelectedID(id int) bool {
	sel, ok := m.selected()
	return ok && sel.ID == id
}

func (m *Model) syncSelectionLoads() tea.Cmd {
	sel, ok := m.selected()
	id := 0
	if ok {
		id = sel.ID
	}
	if id == m.trackedID {
		return nil
	}
	m.trackedID = id
	m.bumpSelection()
	return m.ensureSelectionLoads()
}

func (m *Model) ensureSelectionLoads() tea.Cmd {
	sel, ok := m.selected()
	if !ok {
		return nil
	}
	r := m.ensureResource(sel.ID)
	var cmds []tea.Cmd
	if !r.HasCore && !r.loadingCore {
		r.loadingCore = true
		cmds = append(cmds, loadCoreCmd(m.ctx, m.details, m.selectionGen, sel.ID))
	}
	if cmd := m.ensureBrowseLazyLoad(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if cmd := m.ensureSpriteLoad(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

func (m *Model) ensureBrowseLazyLoad() tea.Cmd {
	if m.browseView != BrowseEvolution {
		return nil
	}
	return m.ensureEvolutionLoad()
}

func (m *Model) ensureDialogLazyLoad() tea.Cmd {
	if !m.showDialog {
		return nil
	}
	switch m.dialogSection {
	case DialogMoves:
		return m.ensureMovesLoad()
	case DialogLocations:
		return m.ensureEncountersLoad()
	default:
		return nil
	}
}

func (m *Model) ensureEvolutionLoad() tea.Cmd {
	sel, ok := m.selected()
	if !ok {
		return nil
	}
	r := m.ensureResource(sel.ID)
	if !r.HasCore || r.HasEvolution || r.loadingEvolution {
		return nil
	}
	r.loadingEvolution = true
	return loadEvolutionCmd(m.ctx, m.details, m.selectionGen, sel.ID, r.Species)
}

func (m *Model) ensureMovesLoad() tea.Cmd {
	sel, ok := m.selected()
	if !ok {
		return nil
	}
	r := m.ensureResource(sel.ID)
	if !r.HasCore || r.HasMoves || r.loadingMoves {
		return nil
	}
	r.loadingMoves = true
	return loadMovesCmd(m.ctx, m.details, m.selectionGen, sel.ID, r.Pokemon)
}

func (m *Model) ensureEncountersLoad() tea.Cmd {
	sel, ok := m.selected()
	if !ok {
		return nil
	}
	r := m.ensureResource(sel.ID)
	if !r.HasCore || r.HasEncounters || r.loadingEncounters {
		return nil
	}
	r.loadingEncounters = true
	pokemonID := r.Pokemon.ID
	if pokemonID == 0 {
		pokemonID = r.Species.DefaultPokemonID
	}
	return loadEncountersCmd(m.ctx, m.details, m.selectionGen, sel.ID, pokemonID)
}

func (m *Model) cancelSprite() {
	if m.spriteCancel != nil {
		m.spriteCancel()
		m.spriteCancel = nil
	}
	m.spriteReqID++
}

func (m *Model) clearSpriteArt(r *resourceCache) {
	if m.sprites != nil {
		m.sprites.ClearNative()
	}
	if r != nil {
		r.clearSprite()
	}
}

func (m *Model) currentSpriteSide() int {
	lay := m.layout()
	if !lay.ShowSprite || lay.SpriteSize < sprite.MinFrameSize {
		return 0
	}
	return lay.SpriteSize
}

func (m *Model) currentSpriteCells() int {
	return m.currentSpriteSide()
}

func (m *Model) handleSpriteLayoutChange(prevShow bool, prevSide int) tea.Cmd {
	lay := m.layout()
	if !lay.ShowSprite {
		m.cancelSprite()
		if sel, ok := m.selected(); ok {
			m.clearSpriteArt(m.resources[sel.ID])
		}
		return nil
	}
	side := m.currentSpriteSide()
	if !prevShow || prevSide != side {
		if sel, ok := m.selected(); ok {
			m.clearSpriteArt(m.resources[sel.ID])
		}
		return m.ensureSpriteLoad()
	}
	return nil
}

func (m *Model) ensureSpriteLoad() tea.Cmd {
	if m.sprites == nil || !m.layout().ShowSprite {
		return nil
	}
	sel, ok := m.selected()
	if !ok {
		return nil
	}
	r := m.ensureResource(sel.ID)
	if !r.HasCore {
		return nil
	}
	cells := m.currentSpriteCells()
	if cells < 1 {
		return nil
	}
	if r.HasSprite && r.SpriteCells == cells && !r.loadingSprite {
		return nil
	}
	if r.loadingSprite && r.SpriteCells == cells {
		return nil
	}
	m.cancelSprite()
	ctx, cancel := context.WithCancel(m.ctx)
	m.spriteCancel = cancel
	reqID := m.spriteReqID
	r.loadingSprite = true
	r.SpriteCells = cells
	r.HasSprite = false
	r.SpriteANSI = ""
	r.SpritePlaceholder = false
	return loadSpriteCmd(ctx, m.sprites, reqID, m.selectionGen, sel.ID, cells, r.Pokemon)
}

func (m *Model) scrollDialog(delta int) {
	m.syncDialogViewport()
	if delta > 0 {
		m.dialogVP.ScrollDown(delta)
	} else if delta < 0 {
		m.dialogVP.ScrollUp(-delta)
	}
}

func (m Model) selectedSummary() (domain.SpeciesSummary, bool) {
	return m.selected()
}
