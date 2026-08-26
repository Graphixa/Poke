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
		if m.warmer != nil {
			if cells := m.currentSpriteCells(); cells >= sprite.MinFrameSize {
				m.warmer.SetCells(cells)
			}
		}
		return m, m.handleSpriteLayoutChange(prevShow, prevSide)

	case dossierMsg:
		if msg.gen != m.selectionGen || !m.isSelectedID(msg.speciesID) {
			m.clearStaleFlag(msg.speciesID, func(r *resourceCache) {
				if !r.HasCore {
					r.loadingCore = false
				}
			})
			if m.isSelectedID(msg.speciesID) && m.showDialog {
				return m, m.ensureDossierLoad()
			}
			return m, nil
		}
		r := m.ensureResource(msg.speciesID)
		r.loadingCore = false
		if msg.err != nil {
			r.CoreErr = msg.err
			if m.showDialog {
				m.syncDialogViewport()
			}
			return m, nil
		}
		r.applyDossier(msg.dossier)
		if m.showDialog {
			m.syncDialogViewport()
		}
		return m, m.ensureSpriteLoad()

	case spriteMsg:
		if msg.reqID != m.spriteReqID || msg.gen != m.selectionGen || !m.isSelectedID(msg.speciesID) {
			m.clearStaleFlag(msg.speciesID, func(r *resourceCache) {
				if r.loadingSprite && (msg.cells == 0 || r.SpriteCells == msg.cells) {
					r.loadingSprite = false
				}
			})
			if m.isSelectedID(msg.speciesID) {
				return m, m.ensureSpriteLoad()
			}
			return m, nil
		}
		if msg.cells != m.currentSpriteCells() {
			m.clearStaleFlag(msg.speciesID, func(r *resourceCache) {
				if r.loadingSprite && r.SpriteCells == msg.cells {
					r.loadingSprite = false
				}
			})
			if m.isSelectedID(msg.speciesID) {
				return m, m.ensureSpriteLoad()
			}
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

	case warmProgressMsg:
		if msg.closed {
			m.warmActive = false
			return m, nil
		}
		m.warmDone = msg.Done
		m.warmTotal = msg.Total
		m.warmActive = msg.Total > 0 && msg.Done < msg.Total
		return m, listenWarmProgress(m.warmEvents)

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
		if m.queryValue() == "" {
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
	// Tab on browse reserved for future sprite variants — no-op for now.
	// TODO(sprite-variants): cycle pixel / official-artwork when assets exist.
	if key.Matches(msg, m.keys.TabNext, m.keys.TabPrev) {
		return m, nil
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
		return m, nil
	}
	if key.Matches(msg, m.keys.TabPrev) {
		m.dialogSection = m.dialogSection.Prev()
		m.dialogVP.GotoTop()
		m.syncDialogViewport()
		return m, nil
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
	sel, ok := m.selected()
	if !ok {
		return m, nil
	}
	m.showDialog = true
	m.dialogSection = DialogSummary
	m.dialogVP.GotoTop()
	m.syncDialogViewport()
	_ = sel
	return m, m.ensureDossierLoad()
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
	m.notifyWarmer()
	return m.ensureSelectionLoads()
}

func (m *Model) notifyWarmer() {
	sel, ok := m.selected()
	if !ok {
		return
	}
	if m.warmer != nil {
		cells := m.currentSpriteCells()
		if cells >= sprite.MinFrameSize {
			m.warmer.SetCells(cells)
		}
		m.warmer.Prioritize(sel.ID)
		m.warmer.SetNeighborsFromList(m.filtered, m.cursor, 5)
	}
	if m.dossierWarmer != nil {
		m.dossierWarmer.Prioritize(sel.ID)
		m.dossierWarmer.SetNeighborsFromList(m.filtered, m.cursor, 5)
	}
}

func (m *Model) ensureSelectionLoads() tea.Cmd {
	// Browse: sprite only — never GetDossier on scroll.
	return m.ensureSpriteLoad()
}

func (m *Model) ensureDossierLoad() tea.Cmd {
	sel, ok := m.selected()
	if !ok {
		return nil
	}
	r := m.ensureResource(sel.ID)
	if r.HasCore || r.loadingCore {
		return nil
	}
	r.loadingCore = true
	r.CoreErr = nil
	return loadDossierCmd(m.ctx, m.details, m.selectionGen, sel.ID)
}

func (m *Model) cancelSprite() {
	if m.spriteCancel != nil {
		m.spriteCancel()
		m.spriteCancel = nil
	}
	m.spriteReqID++
	if m.spriteLoadSpecies != 0 {
		if r := m.resources[m.spriteLoadSpecies]; r != nil {
			r.loadingSprite = false
		}
		m.spriteLoadSpecies = 0
	}
}

func (m *Model) clearStaleFlag(speciesID int, clear func(*resourceCache)) {
	if speciesID < 1 || clear == nil {
		return
	}
	r := m.resources[speciesID]
	if r == nil {
		return
	}
	clear(r)
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
	m.spriteLoadSpecies = sel.ID
	r.loadingSprite = true
	r.SpriteCells = cells
	r.HasSprite = false
	r.SpriteANSI = ""
	r.SpritePlaceholder = false

	pokemon := r.Pokemon
	if pokemon.ID == 0 {
		pokemon = domain.Pokemon{ID: sel.ID, Slug: sel.Slug, DisplayName: sel.DisplayName}
	}
	return loadSpriteCmd(ctx, m.sprites, reqID, m.selectionGen, sel.ID, cells, pokemon)
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
