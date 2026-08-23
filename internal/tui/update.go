package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/tui/views"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		prevShow := m.layout().ShowSprite
		prevSide := m.currentSpriteSide()
		m.width = msg.Width
		m.height = msg.Height
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
		r.clearSprite()
		return m, tea.Batch(m.ensureTabLoadLocked(), m.ensureSpriteLoad())

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
	key := msg.String()

	if key == "ctrl+c" {
		m.quitting = true
		return m, tea.Quit
	}

	if m.showHelp {
		switch key {
		case "esc", "?":
			m.showHelp = false
		}
		return m, nil
	}

	switch key {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "?":
		m.showHelp = true
		return m, nil
	case "esc":
		m.clearQuery()
		return m, m.syncSelectionLoads()
	case "q":
		if m.layout().TooSmall || m.canQuitWithQ() {
			m.quitting = true
			return m, tea.Quit
		}
		m.appendRune('q')
		return m, m.syncSelectionLoads()
	case "up":
		m.moveCursor(-1)
		return m, m.syncSelectionLoads()
	case "down":
		m.moveCursor(1)
		return m, m.syncSelectionLoads()
	case "tab":
		m.activeTab = m.activeTab.Next()
		m.detailOff = 0
		return m, m.ensureTabLoadLocked()
	case "shift+tab":
		m.activeTab = m.activeTab.Prev()
		m.detailOff = 0
		return m, m.ensureTabLoadLocked()
	case "pgup":
		m.scrollDetail(-m.detailPageSize())
		return m, nil
	case "pgdown":
		m.scrollDetail(m.detailPageSize())
		return m, nil
	case "home":
		m.detailOff = 0
		return m, nil
	case "end":
		m.scrollDetail(1 << 20)
		return m, nil
	case "backspace":
		m.backspace()
		return m, m.syncSelectionLoads()
	case "enter", "r", "/":
		// Reserved / unbound in Phase 4.
		return m, nil
	case "space":
		m.appendRune(' ')
		return m, m.syncSelectionLoads()
	}

	if msg.Text != "" && !msg.Mod.Contains(tea.ModCtrl) && !msg.Mod.Contains(tea.ModAlt) {
		for _, r := range msg.Text {
			m.appendRune(r)
		}
		return m, m.syncSelectionLoads()
	}
	return m, nil
}

func (m *Model) isSelectedID(id int) bool {
	sel, ok := m.selected()
	return ok && sel.ID == id
}

// syncSelectionLoads bumps the request generation only when the selected
// species changes, so filter keystrokes that keep the same row do not cancel
// in-flight loads.
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
	if cmd := m.ensureTabLoadLocked(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if cmd := m.ensureSpriteLoad(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

// ensureTabLoadLocked starts a lazy tab fetch when needed; no-ops if cached/in-flight.
func (m *Model) ensureTabLoadLocked() tea.Cmd {
	sel, ok := m.selected()
	if !ok {
		return nil
	}
	r := m.ensureResource(sel.ID)
	if !r.HasCore {
		return nil
	}
	switch m.activeTab {
	case TabEvolution:
		if r.HasEvolution || r.loadingEvolution {
			return nil
		}
		r.loadingEvolution = true
		return loadEvolutionCmd(m.ctx, m.details, m.selectionGen, sel.ID, r.Species)
	case TabLocations:
		if r.HasEncounters || r.loadingEncounters {
			return nil
		}
		r.loadingEncounters = true
		pokemonID := r.Pokemon.ID
		if pokemonID == 0 {
			pokemonID = r.Species.DefaultPokemonID
		}
		return loadEncountersCmd(m.ctx, m.details, m.selectionGen, sel.ID, pokemonID)
	case TabMoves:
		if r.HasMoves || r.loadingMoves {
			return nil
		}
		r.loadingMoves = true
		return loadMovesCmd(m.ctx, m.details, m.selectionGen, sel.ID, r.Pokemon)
	default:
		return nil
	}
}

func (m *Model) cancelSprite() {
	if m.spriteCancel != nil {
		m.spriteCancel()
		m.spriteCancel = nil
	}
	m.spriteReqID++
}

func (m *Model) currentSpriteSide() int {
	lay := m.layout()
	if !lay.ShowSprite {
		return 0
	}
	bodyH := m.height - 2
	if bodyH < 5 {
		bodyH = 5
	}
	return views.SpriteSideFor(lay.leftWidth(), bodyH)
}

func (m *Model) currentSpriteCells() int {
	side := m.currentSpriteSide()
	if side < 5 {
		return 0
	}
	return side - 2
}

func (m *Model) handleSpriteLayoutChange(prevShow bool, prevSide int) tea.Cmd {
	lay := m.layout()
	if !lay.ShowSprite {
		m.cancelSprite()
		if sel, ok := m.selected(); ok {
			if r := m.resources[sel.ID]; r != nil {
				r.clearSprite()
			}
		}
		return nil
	}
	side := m.currentSpriteSide()
	if !prevShow || side != prevSide {
		if sel, ok := m.selected(); ok {
			if r := m.resources[sel.ID]; r != nil {
				r.clearSprite()
			}
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

func (m *Model) detailPageSize() int {
	h := 8
	if m.height > 20 {
		h = m.height / 4
	}
	if h < 3 {
		h = 3
	}
	return h
}

func (m *Model) scrollDetail(delta int) {
	lines := m.detailContentLines()
	m.detailLines = lines
	maxOff := len(lines) - 1
	if maxOff < 0 {
		maxOff = 0
	}
	m.detailOff += delta
	if m.detailOff < 0 {
		m.detailOff = 0
	}
	if m.detailOff > maxOff {
		m.detailOff = maxOff
	}
}

func (m Model) selectedSummary() (domain.SpeciesSummary, bool) {
	return m.selected()
}
