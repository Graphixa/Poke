package tui

import (
	"context"
	"fmt"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Raw overlay writes land here after the cell frame flush; do not re-paint
	// or we loop forever. viewEquals then skips the next renderer flush.
	if _, ok := msg.(tea.RawMsg); ok {
		return m, nil
	}
	next, cmd := m.update(msg)
	n := next.(Model)
	// Always drain a dirty overlay after updates — including follow-up paints,
	// where a warm/list tick may have set dirty while paint was pending.
	n, overlay := n.scheduleNativeOverlay()
	return n, tea.Batch(cmd, overlay)
}

func (m Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case nativeOverlayMsg:
		return m.emitNativeOverlay(msg.followUp)

	case spritePrefetchMsg:
		return m.applySpritePrefetch(msg)

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
		if msg.placeholder || msg.ansi == "" {
			// Match prefetch: placeholders are not "done" for skip logic.
			r.HasSprite = false
			r.SpriteANSI = ""
			r.SpritePlaceholder = true
			r.SpriteNative = false
			r.SpriteKind = ""
		} else {
			r.HasSprite = true
			r.SpriteANSI = msg.ansi
			r.SpritePlaceholder = false
			r.SpriteNative = msg.native
			r.SpriteKind = msg.assetKind
			if msg.native {
				m.markNativeOverlayDirtyFollowUp()
			}
		}
		return m, nil

	case warmProgressMsg:
		if msg.closed {
			m.warmActive = false
			m.warmHint = ""
			if sel, ok := m.selected(); ok {
				return m, m.refreshSpriteAfterWarm(sel.ID, "artwork")
			}
			return m, nil
		}
		m.warmDone = msg.Done
		m.warmTotal = msg.Total
		m.warmActive = msg.Total > 0 && msg.Done < msg.Total
		hint := ""
		if m.warmActive {
			hint = fmt.Sprintf("warming %d/%d", m.warmDone, m.warmTotal)
		}
		// Throttle model updates so browseData/footer does not churn every tick.
		// While a native sprite is visible the hint is omitted from View entirely
		// (showsWarmHintInView) so these updates never wipe Sixel.
		hintChanged := hint != m.warmHint
		now := time.Now()
		shouldShow := hintChanged && (m.warmHint == "" || hint == "" ||
			now.Sub(m.warmHintAt) >= warmHintMinInterval || m.warmDone == m.warmTotal)
		if shouldShow {
			m.warmHint = hint
			m.warmHintAt = now
		}
		var reload tea.Cmd
		if msg.SpeciesID != 0 {
			reload = m.refreshSpriteAfterWarm(msg.SpeciesID, msg.Stage)
		}
		return m, tea.Batch(reload, listenWarmProgress(m.warmEvents))

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
			m.markNativeOverlayDirtyFollowUp()
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
		// Dialog wiped native graphics from the cell grid; re-paint cached art.
		m.markNativeOverlayDirtyFollowUp()
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

// warmerNeighborRadius aligns background prioritize windows with TUI prefetch.
const warmerNeighborRadius = spritePrefetchRadius

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
		m.warmer.SetNeighborsFromList(m.filtered, m.cursor, warmerNeighborRadius)
	}
	if m.artworkWarmer != nil {
		m.artworkWarmer.Prioritize(sel.ID)
		m.artworkWarmer.SetNeighborsFromList(m.filtered, m.cursor, warmerNeighborRadius)
	}
	if m.dossierWarmer != nil {
		m.dossierWarmer.Prioritize(sel.ID)
		m.dossierWarmer.SetNeighborsFromList(m.filtered, m.cursor, warmerNeighborRadius)
	}
}

func (m *Model) ensureSelectionLoads() tea.Cmd {
	// Browse: sprite only — never GetDossier on scroll.
	return tea.Batch(m.ensureSpriteLoad(), m.scheduleSpritePrefetch())
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

// selectionSpriteOwns reports whether the selection encode path owns speciesID.
func (m Model) selectionSpriteOwns(speciesID int) bool {
	return m.isSelectedID(speciesID) &&
		m.spriteLoadSpecies == speciesID &&
		m.spriteCancel != nil
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
	if r.HasSprite && !r.SpritePlaceholder && r.SpriteCells == cells && !r.loadingSprite {
		// Only treat official artwork as final. Pixel frames must upgrade when
		// background artwork warm finishes for this species.
		if r.SpriteKind == "artwork" {
			if r.SpriteNative && r.SpriteANSI != "" {
				m.dirtyNativeIfPaintChanged(sel.ID, cells, r.SpriteANSI)
			}
			return nil
		}
		if !m.artworkReady(sel.ID) {
			if r.SpriteNative && r.SpriteANSI != "" {
				m.dirtyNativeIfPaintChanged(sel.ID, cells, r.SpriteANSI)
			}
			return nil
		}
		// Artwork is on disk now — fall through to re-encode.
		m.sprites.InvalidateSpecies(sel.ID)
		m.clearSpriteArt(r)
	}
	// Prefetch sets loadingSprite on neighbors; that is not a selection encode.
	// Only skip when this model already owns an in-flight selection load.
	if r.loadingSprite && r.SpriteCells == cells &&
		m.spriteLoadSpecies == sel.ID && m.spriteCancel != nil {
		return nil
	}
	m.cancelSprite()
	ctx, cancel := context.WithCancel(m.ctx)
	m.spriteCancel = cancel
	m.spriteReqID++
	reqID := m.spriteReqID
	m.spriteLoadSpecies = sel.ID
	r.loadingSprite = true
	r.SpriteCells = cells
	r.HasSprite = false
	r.SpriteANSI = ""
	r.SpritePlaceholder = false
	r.SpriteNative = false
	r.SpriteKind = ""

	pokemon := r.Pokemon
	if pokemon.ID == 0 {
		pokemon = domain.Pokemon{ID: sel.ID, Slug: sel.Slug, DisplayName: sel.DisplayName}
	}
	return loadSpriteCmd(ctx, m.sprites, m.pokemonSrc, reqID, m.selectionGen, sel.ID, cells, pokemon)
}

// artworkReady reports whether background artwork warm has stored official art
// for speciesID (so a pixel frame can be upgraded).
func (m *Model) artworkReady(speciesID int) bool {
	if m.artworkWarmer == nil || speciesID < 1 {
		return false
	}
	return m.artworkWarmer.Ready(speciesID)
}

// refreshSpriteAfterWarm reloads the selected sprite when background warm just
// finished that species. Artwork stage always upgrades (even if ANSI present);
// other stages only reload placeholders / non-artwork frames.
func (m *Model) refreshSpriteAfterWarm(speciesID int, stage string) tea.Cmd {
	if speciesID == 0 || m.sprites == nil || !m.layout().ShowSprite {
		return nil
	}
	sel, ok := m.selected()
	if !ok || sel.ID != speciesID {
		return nil
	}
	r := m.resources[sel.ID]
	artworkDone := stage == "artwork" || stage == ""
	if r != nil && r.HasSprite && !r.SpritePlaceholder && r.SpriteANSI != "" && r.SpriteKind == "artwork" {
		// Already showing official artwork — no upgrade needed.
		return nil
	}
	if !artworkDone {
		// Dossier / chafa warm: only retry if still waiting on art.
		if r != nil && r.HasSprite && !r.SpritePlaceholder && r.SpriteANSI != "" {
			return nil
		}
	}
	if m.sprites != nil {
		m.sprites.InvalidateSpecies(speciesID)
	}
	if r != nil {
		m.clearSpriteArt(r)
	}
	return m.ensureSpriteLoad()
}

// scheduleSpritePrefetch encodes nearby list rows into the resource cache so
// scrolling hits warm frames instead of placeholders.
func (m *Model) scheduleSpritePrefetch() tea.Cmd {
	if m.sprites == nil || !m.layout().ShowSprite || len(m.filtered) == 0 {
		return nil
	}
	cells := m.currentSpriteCells()
	if cells < 1 {
		return nil
	}
	m.pruneSpriteResources()
	lo := m.cursor - spritePrefetchRadius
	if lo < 0 {
		lo = 0
	}
	hi := m.cursor + spritePrefetchRadius
	if hi >= len(m.filtered) {
		hi = len(m.filtered) - 1
	}
	need := 0
	for dist := 1; dist <= spritePrefetchRadius; dist++ {
		for _, idx := range []int{m.cursor - dist, m.cursor + dist} {
			if idx < lo || idx > hi || idx == m.cursor {
				continue
			}
			sum := m.filtered[idx]
			r := m.ensureResource(sum.ID)
			if r.HasSprite && r.SpriteCells == cells {
				continue
			}
			if r.loadingSprite && r.SpriteCells == cells {
				continue
			}
			need++
		}
	}
	if need == 0 {
		return nil
	}

	m.cancelPrefetch()
	ctx, cancel := context.WithCancel(m.ctx)
	m.prefetchCancel = cancel
	m.prefetchGen++
	gen := m.prefetchGen

	var cmds []tea.Cmd
	// Prefer neighbors closest to the cursor (walk outward).
	for dist := 1; dist <= spritePrefetchRadius && len(cmds) < spritePrefetchBurst; dist++ {
		for _, idx := range []int{m.cursor - dist, m.cursor + dist} {
			if idx < lo || idx > hi || idx == m.cursor {
				continue
			}
			sum := m.filtered[idx]
			r := m.ensureResource(sum.ID)
			if r.HasSprite && r.SpriteCells == cells {
				continue
			}
			if r.loadingSprite && r.SpriteCells == cells {
				continue
			}
			r.loadingSprite = true
			r.SpriteCells = cells
			pokemon := r.Pokemon
			if pokemon.ID == 0 {
				pokemon = domain.Pokemon{ID: sum.ID, Slug: sum.Slug, DisplayName: sum.DisplayName}
			}
			cmds = append(cmds, loadSpritePrefetchCmd(ctx, m.sprites, m.pokemonSrc, gen, sum.ID, cells, pokemon))
			if len(cmds) >= spritePrefetchBurst {
				break
			}
		}
	}
	return tea.Batch(cmds...)
}

func (m *Model) cancelPrefetch() {
	if m.prefetchCancel != nil {
		m.prefetchCancel()
		m.prefetchCancel = nil
	}
}

// pruneSpriteResources drops large Sixel/ANSI payloads outside the prefetch
// window so scrolling cannot unbounded-grow RAM.
func (m *Model) pruneSpriteResources() {
	if len(m.filtered) == 0 || m.resources == nil {
		return
	}
	keep := make(map[int]struct{}, spritePrefetchRadius*2+2)
	lo := m.cursor - spritePrefetchRadius*2
	if lo < 0 {
		lo = 0
	}
	hi := m.cursor + spritePrefetchRadius*2
	if hi >= len(m.filtered) {
		hi = len(m.filtered) - 1
	}
	for i := lo; i <= hi; i++ {
		keep[m.filtered[i].ID] = struct{}{}
	}
	if sel, ok := m.selected(); ok {
		keep[sel.ID] = struct{}{}
	}
	for id, r := range m.resources {
		if _, ok := keep[id]; ok {
			continue
		}
		if r == nil {
			delete(m.resources, id)
			continue
		}
		// Keep dossier/core data; free sprite payloads only.
		if r.HasSprite || r.SpriteANSI != "" || r.loadingSprite {
			r.clearSprite()
		}
		if !r.HasCore && !r.HasEvolution && !r.HasEncounters && !r.HasMoves && !r.loadingCore {
			delete(m.resources, id)
		}
	}
}

func (m Model) applySpritePrefetch(msg spritePrefetchMsg) (tea.Model, tea.Cmd) {
	if msg.gen != 0 && msg.gen != m.prefetchGen {
		if r := m.resources[msg.speciesID]; r != nil && r.loadingSprite && r.SpriteCells == msg.cells {
			// Selection encode may have taken over this species after cancel.
			if !m.selectionSpriteOwns(msg.speciesID) {
				r.loadingSprite = false
			}
		}
		return m, nil
	}
	if msg.cells != m.currentSpriteCells() {
		if r := m.resources[msg.speciesID]; r != nil && r.loadingSprite && r.SpriteCells == msg.cells {
			if !m.selectionSpriteOwns(msg.speciesID) {
				r.loadingSprite = false
			}
		}
		return m, nil
	}
	r := m.ensureResource(msg.speciesID)
	// Selection encode in flight for this species — that result is authoritative.
	if m.selectionSpriteOwns(msg.speciesID) {
		return m, m.scheduleSpritePrefetch()
	}
	// Drop results that scrolled out of the sliding window (prune already ran).
	if !m.inPrefetchWindow(msg.speciesID) {
		r.loadingSprite = false
		if !m.isSelectedID(msg.speciesID) {
			r.clearSprite()
		}
		return m, m.scheduleSpritePrefetch()
	}
	r.loadingSprite = false
	r.SpriteCells = msg.cells
	if msg.pokemon.ID != 0 && (r.Pokemon.ID == 0 || r.Pokemon.Sprites.FrontDefault == "") {
		r.Pokemon = msg.pokemon
	}
	if msg.placeholder || msg.ansi == "" {
		r.HasSprite = false
		r.SpriteANSI = ""
		r.SpritePlaceholder = true
		r.SpriteNative = false
		r.SpriteKind = ""
		return m, m.scheduleSpritePrefetch()
	}
	r.HasSprite = true
	r.SpriteANSI = msg.ansi
	r.SpritePlaceholder = false
	r.SpriteNative = msg.native
	r.SpriteKind = msg.assetKind
	if m.isSelectedID(msg.speciesID) && msg.native {
		m.markNativeOverlayDirtyFollowUp()
	}
	return m, m.scheduleSpritePrefetch()
}

// inPrefetchWindow reports whether speciesID is within the keep radius of the cursor.
func (m Model) inPrefetchWindow(speciesID int) bool {
	if len(m.filtered) == 0 {
		return false
	}
	lo := m.cursor - spritePrefetchRadius*2
	if lo < 0 {
		lo = 0
	}
	hi := m.cursor + spritePrefetchRadius*2
	if hi >= len(m.filtered) {
		hi = len(m.filtered) - 1
	}
	for i := lo; i <= hi; i++ {
		if m.filtered[i].ID == speciesID {
			return true
		}
	}
	return m.isSelectedID(speciesID)
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
