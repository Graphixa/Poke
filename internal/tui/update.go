package tui

import (
	"context"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
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
		m, cmd := m.emitNativeOverlay(msg.followUp)
		return m, tea.Batch(cmd, m.warmFooterPatchIfNeeded())

	case spinner.TickMsg:
		if m.imageState != ImageLoading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case spritePrefetchMsg:
		return m.applySpritePrefetch(msg)

	case tea.WindowSizeMsg:
		prevShow := m.layout().ShowSprite
		prevSide := m.currentSpriteSide()
		m.width = msg.Width
		m.height = msg.Height
		m.help.SetWidth(msg.Width)
		m.ensureListVisible()
		m.notifyWarmer()
		return m, m.handleSpriteLayoutChange(prevShow, prevSide)

	case dossierCoreMsg:
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
		r.applyCore(msg.dossier, msg.complete)
		if m.showDialog {
			m.syncDialogViewport()
		}
		var extrasCmd tea.Cmd
		if !msg.complete && !r.extrasReady() {
			extrasCmd = m.ensureDossierExtras(msg.slug)
		}
		return m, tea.Batch(extrasCmd, m.ensureSpriteLoad())

	case dossierExtrasMsg:
		if msg.gen != m.selectionGen || !m.isSelectedID(msg.speciesID) {
			m.clearStaleFlag(msg.speciesID, func(r *resourceCache) {
				r.loadingExtras = false
			})
			return m, nil
		}
		r := m.ensureResource(msg.speciesID)
		r.loadingExtras = false
		if msg.err != nil {
			r.EvolutionErr = msg.err
			r.EncountersErr = msg.err
			r.MovesErr = msg.err
			if m.showDialog {
				m.syncDialogViewport()
			}
			return m, nil
		}
		r.applyExtras(msg.dossier)
		if m.showDialog {
			m.syncDialogViewport()
		}
		return m, nil

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
		return m, m.handleSpriteMsg(msg)

	case warmProgressMsg:
		if msg.closed {
			m.warmActive = false
			m.warmRight = ""
			m.warmStage = ""
			m.warmCells = 0
			var clear tea.Cmd
			if m.nativeSpriteActive() {
				clear = clearWarmFooterRawCmd(m)
			}
			if sel, ok := m.selected(); ok {
				return m, tea.Batch(clear, m.refreshSpriteAfterWarm(sel.ID, "artwork"), listenWarmProgress(m.warmEvents))
			}
			return m, tea.Batch(clear, listenWarmProgress(m.warmEvents))
		}
		m.warmDone = msg.Done
		m.warmTotal = msg.Total
		m.warmActive = msg.Total > 0 && msg.Done < msg.Total
		if msg.Stage != "" {
			m.warmStage = msg.Stage
		}
		if msg.Cells > 0 {
			m.warmCells = msg.Cells
		}
		right := ""
		if m.warmActive {
			right = formatWarmFooterRight(m.warmStage, m.warmDone, m.warmTotal, m.warmCells)
		}
		rightChanged := right != m.warmRight
		now := time.Now()
		shouldShow := rightChanged && (m.warmRight == "" || right == "" ||
			now.Sub(m.warmHintAt) >= warmHintMinInterval || m.warmDone == m.warmTotal)
		if shouldShow {
			m.warmRight = right
			m.warmHintAt = now
		}
		var reload tea.Cmd
		if msg.SpeciesID != 0 {
			reload = m.refreshSpriteAfterWarm(msg.SpeciesID, msg.Stage)
		}
		var patch tea.Cmd
		if msg.Total > 0 && msg.Done >= msg.Total && m.nativeSpriteActive() {
			patch = clearWarmFooterRawCmd(m)
		} else if m.warmActive && m.warmStage == "native" && m.nativeSpriteActive() {
			patch = patchWarmFooterCmd(m)
		}
		return m, tea.Batch(reload, patch, listenWarmProgress(m.warmEvents))

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
	if m.dossierWarmer != nil {
		m.dossierWarmer.Prioritize(sel.ID)
		m.dossierWarmer.SetNeighborsFromList(m.filtered, m.cursor, warmerNeighborRadius)
	}
	if m.warmer != nil {
		cells := m.currentSpriteCells()
		if cells >= sprite.MinFrameSize {
			m.warmer.SetCells(cells)
		}
		m.warmer.Prioritize(sel.ID)
		m.warmer.SetNeighborsFromList(m.filtered, m.cursor, warmerNeighborRadius)
		m.warmer.NoteActivity()
	}
	if m.artworkWarmer != nil {
		m.artworkWarmer.Prioritize(sel.ID)
		m.artworkWarmer.SetNeighborsFromList(m.filtered, m.cursor, warmerNeighborRadius)
	}
	if m.nativeWarmer != nil {
		if cells := m.currentSpriteCells(); cells >= sprite.MinFrameSize {
			m.nativeWarmer.EnsureStarted(m.ctx, cells)
		}
		m.nativeWarmer.Prioritize(sel.ID)
		m.nativeWarmer.SetNeighborsFromList(m.filtered, m.cursor, warmerNeighborRadius)
		m.nativeWarmer.NoteActivity()
	}
}

func (m *Model) ensureSelectionLoads() tea.Cmd {
	load := m.ensureSpriteLoad()
	if m.foregroundColdBusy() {
		m.cancelPrefetch()
		return load
	}
	return tea.Batch(load, m.scheduleSpritePrefetch())
}

func (m *Model) ensureDossierLoad() tea.Cmd {
	sel, ok := m.selected()
	if !ok {
		return nil
	}
	r := m.ensureResource(sel.ID)
	if r.extrasReady() {
		return nil
	}
	if r.HasCore {
		return m.ensureDossierExtras(sel.Slug)
	}
	if r.loadingCore {
		return nil
	}
	r.loadingCore = true
	r.CoreErr = nil
	return loadDossierCoreCmd(m.beginDossierLoad(sel.ID), m.details, m.selectionGen, sel.ID, sel.Slug)
}

func (m *Model) ensureDossierExtras(slug string) tea.Cmd {
	sel, ok := m.selected()
	if !ok {
		return nil
	}
	r := m.ensureResource(sel.ID)
	if r.extrasReady() || r.loadingExtras || !r.HasCore {
		return nil
	}
	r.loadingExtras = true
	core := domain.Dossier{Pokemon: r.Pokemon, Species: r.Species}
	return loadDossierExtrasCmd(m.dossierCtxForLoad(sel.ID), m.details, m.selectionGen, sel.ID, slug, core)
}

func (m *Model) cancelDossier() {
	if m.dossierCancel != nil {
		m.dossierCancel()
		m.dossierCancel = nil
	}
	m.dossierLoadCtx = nil
	if m.dossierLoadSpecies != 0 {
		if r := m.resources[m.dossierLoadSpecies]; r != nil {
			r.loadingCore = false
			r.loadingExtras = false
		}
		m.dossierLoadSpecies = 0
	}
}

func (m *Model) beginDossierLoad(speciesID int) context.Context {
	m.cancelDossier()
	ctx, cancel := context.WithCancel(m.ctx)
	m.dossierCancel = cancel
	m.dossierLoadCtx = ctx
	m.dossierLoadSpecies = speciesID
	return ctx
}

func (m *Model) dossierCtxForLoad(speciesID int) context.Context {
	if m.dossierLoadSpecies == speciesID && m.dossierLoadCtx != nil {
		return m.dossierLoadCtx
	}
	return m.beginDossierLoad(speciesID)
}

func (m *Model) cancelSprite() {
	if m.spriteCancel != nil {
		m.spriteCancel()
		m.spriteCancel = nil
	}
	m.spriteReqID++
	m.imageLoading = false
	m.spriteLoadSpecies = 0
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

func (m *Model) clearNativeDisplay() {
	if m.sprites != nil {
		m.sprites.ClearNative()
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
		m.fgActive = nil
		m.fgPending = nil
		m.cancelSprite()
		m.setImageBlank()
		m.clearNativeDisplay()
		return nil
	}
	side := m.currentSpriteSide()
	if !prevShow || prevSide != side {
		m.clearImagePane()
		m.clearNativeDisplay()
		return m.ensureSpriteLoad()
	}
	return nil
}

func (m *Model) ensureSpriteLoad() tea.Cmd {
	load := m.beginSpriteLoad()
	m.syncWarmerBusy()
	if load == nil {
		return nil
	}
	if m.imageState == ImageLoading {
		tick := m.spinner.Tick()
		return tea.Batch(load, func() tea.Msg { return tick })
	}
	return load
}

func (m *Model) beginSpriteLoad() tea.Cmd {
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
	force := false
	if m.imageSpeciesID == sel.ID && m.imageCells == cells && !m.imagePlaceholder && m.imageANSI != "" && !m.imageLoading {
		m.imageState = ImageReady
		m.imageSpinner = false
		// Only treat official artwork as final. Pixel frames must upgrade when
		// background artwork warm finishes for this species.
		if m.imageKind == "artwork" {
			if m.imageNative && m.imageANSI != "" {
				m.dirtyNativeIfPaintChanged(sel.ID, cells, m.imageANSI)
			}
			return nil
		}
		if !m.artworkReady(sel.ID) {
			if m.imageNative && m.imageANSI != "" {
				m.dirtyNativeIfPaintChanged(sel.ID, cells, m.imageANSI)
			}
			return nil
		}
		// Artwork is on disk now — fall through to re-encode.
		m.sprites.InvalidateSpecies(sel.ID)
		m.clearImagePane()
		force = true
	}

	pokemon := r.Pokemon
	if pokemon.ID == 0 {
		pokemon = domain.Pokemon{ID: sel.ID, Slug: sel.Slug, DisplayName: sel.DisplayName}
	}
	if cached, hit := m.sprites.LookupCached(m.ctx, pokemon, cells); hit && !cached.UsePlaceholder && cached.ANSI != "" {
		m.abortFgContext()
		m.fgActive = nil
		m.fgPending = nil
		m.spriteReqID++
		m.applySpriteFrame(sel.ID, cells, cached)
		return nil
	}

	req := fgRenderReq{
		speciesID: sel.ID,
		gen:       m.selectionGen,
		cells:     cells,
		pokemon:   pokemon,
	}
	return m.queueOrStartFgRender(req, force)
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
	artworkDone := stage == "artwork" || stage == ""
	_ = stage // native warm does not force selected reload when art is already visible
	if m.imageSpeciesID == speciesID && !m.imagePlaceholder && m.imageANSI != "" && m.imageKind == "artwork" {
		return nil
	}
	if !artworkDone {
		if m.imageSpeciesID == speciesID && !m.imagePlaceholder && m.imageANSI != "" {
			return nil
		}
	}
	if m.sprites != nil {
		m.sprites.InvalidateSpecies(speciesID)
	}
	m.clearImagePane()
	return m.ensureSpriteLoad()
}

// scheduleSpritePrefetch warms engine/disk render caches for nearby list rows.
// Rendered output is not retained on resource entries.
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

	m.cancelPrefetch()
	ctx, cancel := context.WithCancel(m.ctx)
	m.prefetchCancel = cancel
	m.prefetchGen++
	gen := m.prefetchGen

	var cmds []tea.Cmd
	for dist := 1; dist <= spritePrefetchRadius && len(cmds) < spritePrefetchBurst; dist++ {
		for _, idx := range []int{m.cursor - dist, m.cursor + dist} {
			if idx < lo || idx > hi || idx == m.cursor {
				continue
			}
			sum := m.filtered[idx]
			r := m.ensureResource(sum.ID)
			pokemon := r.Pokemon
			if pokemon.ID == 0 {
				pokemon = domain.Pokemon{ID: sum.ID, Slug: sum.Slug, DisplayName: sum.DisplayName}
			}
			if _, hit := m.sprites.LookupCached(m.ctx, pokemon, cells); hit {
				continue
			}
			sprite.NoteNeighbour()
			cmds = append(cmds, loadSpritePrefetchCmd(ctx, m.sprites, m.pokemonSrc, gen, sum.ID, cells, pokemon))
			if len(cmds) >= spritePrefetchBurst {
				break
			}
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) cancelPrefetch() {
	if m.prefetchCancel != nil {
		m.prefetchCancel()
		m.prefetchCancel = nil
	}
}

// pruneSpriteResources drops dossier entries outside the prefetch window.
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
		if r == nil || (!r.HasCore && !r.HasEvolution && !r.HasEncounters && !r.HasMoves && !r.loadingCore && !r.loadingExtras) {
			delete(m.resources, id)
		}
	}
}

func (m Model) applySpritePrefetch(msg spritePrefetchMsg) (tea.Model, tea.Cmd) {
	if m.foregroundColdBusy() {
		return m, nil
	}
	if msg.gen != 0 && msg.gen != m.prefetchGen {
		return m, nil
	}
	if msg.cells != m.currentSpriteCells() {
		return m, nil
	}
	if m.selectionSpriteOwns(msg.speciesID) {
		return m, m.scheduleSpritePrefetchIfIdle()
	}
	if !m.inPrefetchWindow(msg.speciesID) {
		return m, m.scheduleSpritePrefetchIfIdle()
	}
	// Warm engine/disk cache only — do not retain rendered output on resources.
	if msg.pokemon.ID != 0 {
		r := m.ensureResource(msg.speciesID)
		if r.Pokemon.ID == 0 || r.Pokemon.Sprites.FrontDefault == "" {
			r.Pokemon = msg.pokemon
		}
	}
	return m, m.scheduleSpritePrefetchIfIdle()
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
