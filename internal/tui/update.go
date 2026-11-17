package tui

import (
	"context"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Poke/internal/debuglog"
	"github.com/Graphixa/Poke/internal/domain"
	"github.com/Graphixa/Poke/internal/render/sprite"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Raw overlay writes land here after the cell frame flush; do not re-paint
	// or we loop forever. viewEquals then skips the next renderer flush.
	if _, ok := msg.(tea.RawMsg); ok {
		return m, nil
	}
	if _, ok := msg.(nativeCompositeFlushMsg); ok {
		if m.compositor != nil {
			m.compositor.FlushPending()
		}
		return m, nil
	}
	next, cmd := m.update(msg)
	n := next.(Model)
	n.maybeCommitGlanceHold()
	n.captureGlanceAbilities()
	n, overlay := n.scheduleNativeOverlay()
	return n, tea.Batch(cmd, overlay)
}

func (m Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case favouriteSavedMsg:
		return m.finishFavouriteSave(msg)
	case compareArtMsg:
		return m.finishCompareArt(msg)
	case nativeReassertMsg:
		if m.showCompare {
			if m.compareNativeReady() {
				m.markNativeOverlayDirty()
			}
			return m, nil
		}
		if m.nativeOverlayReady() {
			m.markNativeOverlayDirty()
		}
		return m, nil

	case birthdayTickMsg:
		return m.onBirthdayTick(msg)

	case tea.WindowSizeMsg:
		// Wipe sixel at the pre-resize art origin before layout moves — WT does
		// not erase old placements when CUP shifts.
		var wipe tea.Cmd
		if msg.Width != m.width || msg.Height != m.height {
			wipe = m.wipeNativeGraphics()
		}
		prevShow := m.layout().ShowSprite
		prevSide := m.currentSpriteSide()
		m.width = msg.Width
		m.height = msg.Height
		m.ensureListVisible()
		if m.showDialog {
			m.syncDialogViewport()
		}
		m.resizeBirthday()
		m.invalidateNativePaintGeometry()
		m.markNativeOverlayDirty()
		layoutCmd := m.handleSpriteLayoutChange(prevShow, prevSide)
		m.notifyWarmer()
		return m, tea.Batch(wipe, m.noteFirstLayout(layoutCmd))

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
			m.initDialogContext(msg.speciesID, msg.dossier)
			m.syncDialogViewport()
		} else if m.nativeSpriteActive() {
			// Core only adds genus cells outside the art box; the Sixel
			// payload is unchanged. Repaint once, in place (no space-clear, no
			// delayed ~60ms insurance) so the art re-lays if the cell flush
			// grazed it — without the second, delayed redraw that read as lag
			// on WT while browsing new species. origin/main did not repaint the
			// overlay on core arrival at all; this is the minimal safe version.
			m.markNativeOverlayDirty()
		}
		var extrasCmd tea.Cmd
		// Extras only while the dossier overlay is open — browse highlight loads core alone.
		if m.showDialog && !msg.complete && !r.extrasReady() {
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
			m.initDialogContext(msg.speciesID, msg.dossier)
			m.syncDialogViewport()
		}
		return m, m.ensureSpriteLoad()

	case coldFrameMsg:
		return m, m.handleColdFrame(msg)

	case spriteMsg:
		return m, m.handleSpriteMsg(msg)

	case warmProgressMsg:
		if msg.closed {
			m.warmActive = false
			m.nativeWarmActive = false
			m.warmRight = ""
			m.warmStage = ""
			m.warmCells = 0
			m.nativeWarmCells = 0
			var clear tea.Cmd
			if m.nativeSpriteActive() {
				clear = clearWarmFooterRawCmd(m)
			}
			if sel, ok := m.selected(); ok {
				return m, tea.Batch(clear, m.refreshSpriteAfterWarm(sel.ID, WarmStageArtwork), listenWarmProgress(m.warmEvents))
			}
			return m, tea.Batch(clear, listenWarmProgress(m.warmEvents))
		}
		if msg.Stage == WarmStageNative {
			m.nativeWarmDone = msg.Done
			m.nativeWarmTotal = msg.Total
			m.nativeWarmActive = msg.Total > 0 && msg.Done < msg.Total
			if msg.Cells > 0 {
				m.nativeWarmCells = msg.Cells
			}
		} else {
			m.warmDone = msg.Done
			m.warmTotal = msg.Total
			m.warmActive = msg.Total > 0 && msg.Done < msg.Total
			if msg.Stage != "" {
				m.warmStage = msg.Stage
			}
			if msg.Cells > 0 {
				m.warmCells = msg.Cells
			}
		}
		right := m.effectiveWarmFooterRight()
		rightChanged := right != m.warmRight
		now := time.Now()
		footerComplete := (m.nativeWarmActive && m.nativeWarmDone == m.nativeWarmTotal) ||
			(!m.nativeWarmActive && m.warmActive && m.warmDone == m.warmTotal)
		shouldShow := rightChanged && (m.warmRight == "" || right == "" ||
			now.Sub(m.warmHintAt) >= warmHintMinInterval || footerComplete)
		if right == "" {
			if m.warmRight != "" {
				m.warmRight = ""
				m.warmHintAt = now
			}
		} else if shouldShow {
			m.warmRight = right
			m.warmHintAt = now
		}
		var reload tea.Cmd
		if msg.SpeciesID != 0 {
			reload = m.refreshSpriteAfterWarm(msg.SpeciesID, msg.Stage)
		}
		var patch tea.Cmd
		nativeComplete := msg.Stage == WarmStageNative && msg.Total > 0 && msg.Done >= msg.Total
		if !m.composite && nativeComplete && m.nativeSpriteActive() {
			patch = clearWarmFooterRawCmd(m)
		} else if !m.composite && m.nativeSpriteActive() {
			switch {
			case right != "":
				patch = patchWarmFooterCmd(m)
			case rightChanged:
				// Drop stale tea.Raw footer when non-native stages hide progress.
				patch = clearWarmFooterRawCmd(m)
			}
		}
		return m, tea.Batch(reload, patch, listenWarmProgress(m.warmEvents))

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.ForceQuit) {
		return m.requestQuit()
	}

	if m.showBirthday {
		return m.handleBirthdayKey(msg)
	}

	if m.showFilters || m.showCompare || m.featureError != "" {
		return m.handleFeatureKey(msg)
	}

	if m.showDialog {
		return m.handleDialogKey(msg)
	}

	if key.Matches(msg, m.keys.Filters) {
		m.showFilters = true
		m.draftOptions = m.viewOptions
		m.filterStep = 0
		m.glanceHoldID = 0
		return m.coverNativeOverlay()
	}
	if key.Matches(msg, m.keys.Compare) {
		return m.pinOrCompare()
	}
	if key.Matches(msg, m.keys.Favourite) {
		return m.toggleFavourite()
	}

	if key.Matches(msg, m.keys.Enter) {
		return m.handleBirthdayEnter()
	}

	if key.Matches(msg, m.keys.ViewCycle) {
		return m.setBrowseView(m.browseView.Next())
	}

	if key.Matches(msg, m.keys.Clear) {
		if m.layout().TooSmall {
			return m.requestQuit()
		}
		if m.queryValue() != "" {
			m.clearQuery()
			return m, m.syncSelectionLoads()
		}
		if m.viewOptions != (viewOptions{}) {
			m.viewOptions = viewOptions{}
			m.refilter()
			return m, m.syncSelectionLoads()
		}
		return m.requestQuit()
	}

	if key.Matches(msg, m.keys.Up) {
		m.moveCursor(-1)
		return m, m.syncSelectionLoads()
	}
	if key.Matches(msg, m.keys.Down) {
		m.moveCursor(1)
		return m, m.syncSelectionLoads()
	}
	if key.Matches(msg, m.keys.PageUp) {
		m.moveCursor(-listPageJump)
		return m, m.syncSelectionLoads()
	}
	if key.Matches(msg, m.keys.PageDown) {
		m.moveCursor(listPageJump)
		return m, m.syncSelectionLoads()
	}
	if key.Matches(msg, m.keys.Home) {
		m.jumpCursor(0)
		return m, m.syncSelectionLoads()
	}
	if key.Matches(msg, m.keys.End) {
		if n := len(m.filtered); n > 0 {
			m.jumpCursor(n - 1)
		}
		return m, m.syncSelectionLoads()
	}

	return m.updateSearch(msg)
}

func (m Model) handleDialogKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Topmost overlay owns all keys.
	if m.gamePickerOpen {
		m.handleGamePickerKey(msg.String(), []rune(msg.Text))
		return m, nil
	}
	if m.formPickerOpen {
		m.handleFormPickerKey(msg.String(), []rune(msg.Text))
		return m, nil
	}
	if m.methodPickerOpen {
		m.handleMethodPickerKey(msg.String())
		return m, nil
	}

	if m.moveDetailOpen {
		if key.Matches(msg, m.keys.Clear) {
			m.exitMoveDetail()
			m.syncDialogViewport()
			m.ensureMoveFocusVisible()
			return m, nil
		}
		if key.Matches(msg, m.keys.Up) {
			m.nudgeMoveFocus(-1)
			m.syncDialogViewport()
			return m, nil
		}
		if key.Matches(msg, m.keys.Down) {
			m.nudgeMoveFocus(1)
			m.syncDialogViewport()
			return m, nil
		}
		return m, nil
	}

	if key.Matches(msg, m.keys.Clear) {
		m.showDialog = false
		m.gamePickerOpen = false
		m.formPickerOpen = false
		m.methodPickerOpen = false
		m.moveDetailOpen = false
		m.invalidateNativePaintGeometry()
		m.markNativeOverlayDirty()
		return m, nil
	}

	letter := msg.Text
	if letter == "" {
		letter = msg.String()
	}
	if key.Matches(msg, m.keys.Game) || letter == "g" || letter == "G" {
		m.openGamePicker()
		m.syncDialogViewport()
		return m, nil
	}
	if (letter == "f" || letter == "F") && m.dialogHasForms() {
		m.openFormPicker()
		m.syncDialogViewport()
		return m, nil
	}
	if (letter == "m" || letter == "M") && m.dialogSection == DialogMoves {
		m.openMethodPicker()
		m.syncDialogViewport()
		return m, nil
	}

	// ←/→ change section; ↑/↓ / PgUp/PgDn navigate content (Moves: row select).
	if key.Matches(msg, m.keys.Right) {
		m.dialogNotice = ""
		m.changeDialogSection(m.dialogSection.Next())
		return m, nil
	}
	if key.Matches(msg, m.keys.Left) {
		m.dialogNotice = ""
		m.changeDialogSection(m.dialogSection.Prev())
		return m, nil
	}

	if m.dialogSection == DialogMoves {
		if key.Matches(msg, m.keys.Up) {
			m.nudgeMoveFocus(-1)
			m.syncDialogViewport()
			m.ensureMoveFocusVisible()
			return m, nil
		}
		if key.Matches(msg, m.keys.Down) {
			m.nudgeMoveFocus(1)
			m.syncDialogViewport()
			m.ensureMoveFocusVisible()
			return m, nil
		}
		if key.Matches(msg, m.keys.Enter) {
			if len(m.dialogMoveRows()) > 0 {
				m.moveDetailOpen = true
				m.syncDialogViewport()
			}
			return m, nil
		}
		if key.Matches(msg, m.keys.PageUp) {
			m.nudgeMoveFocus(-m.dialogPageSize())
			m.syncDialogViewport()
			m.ensureMoveFocusVisible()
			return m, nil
		}
		if key.Matches(msg, m.keys.PageDown) {
			m.nudgeMoveFocus(m.dialogPageSize())
			m.syncDialogViewport()
			m.ensureMoveFocusVisible()
			return m, nil
		}
		if key.Matches(msg, m.keys.Home) {
			m.moveFocusIdx = 0
			m.syncDialogViewport()
			m.ensureMoveFocusVisible()
			return m, nil
		}
		if key.Matches(msg, m.keys.End) {
			if n := len(m.dialogMoveRows()); n > 0 {
				m.moveFocusIdx = n - 1
			}
			m.syncDialogViewport()
			m.ensureMoveFocusVisible()
			return m, nil
		}
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
	m.glanceHoldID = 0
	m.dialogSection = DialogOverview
	m.dialogScroll = map[DialogSection]int{}
	m.moveDetailOpen = false
	m.gamePickerOpen = false
	m.formPickerOpen = false
	m.methodPickerOpen = false
	m.dialogNotice = ""
	if r := m.resources[sel.ID]; r != nil && r.HasCore {
		m.initDialogContext(sel.ID, domain.Dossier{Pokemon: r.Pokemon, Species: r.Species})
	} else {
		m.dialogForms = nil
		m.dialogFormIdx = 0
		m.dialogPokemon = domain.Pokemon{}
		m.dialogSpeciesID = sel.ID
	}
	m.dialogVP.GotoTop()
	m.syncDialogViewport()
	covered, wipe := m.coverNativeOverlay()
	return covered, tea.Batch(wipe, covered.ensureDossierLoad())
}

func (m Model) setBrowseView(v BrowseView) (tea.Model, tea.Cmd) {
	if m.browseView == v {
		return m, nil
	}
	prevShow := m.layout().ShowSprite
	prevSide := m.currentSpriteSide()
	// Wipe sixel at the pre-switch art origin before layout moves — same as
	// resize. Clearing only the new (often smaller) box leaves hero spill under
	// glance / between cards.
	wipe := m.wipeNativeGraphics()
	m.browseView = v
	m.markNativeOverlayDirty()
	m.notifyWarmer()
	return m, tea.Batch(wipe, m.handleSpriteLayoutChange(prevShow, prevSide))
}

func (m Model) updateSearch(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	prev := m.search.Value()
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	if m.search.Value() != prev {
		if !isBirthdayQuery(m.search.Value()) {
			m.declanEnterCount = 0
		}
		m.refilter()
		m.dialogVP.GotoTop()
		return m, tea.Batch(cmd, m.syncSelectionLoads())
	}
	return m, cmd
}

func (m *Model) isSelectedID(id int) bool {
	sel, ok := m.selected()
	return ok && sel.ID == id
}

func (m *Model) effectiveWarmFooterRight() string {
	if !m.nativeWarmActive {
		return ""
	}
	return formatWarmFooterRight(WarmStageNative, m.nativeWarmDone, m.nativeWarmTotal)
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
	displayed := m.glanceHoldID
	if displayed == 0 {
		displayed = m.trackedID
	}
	m.trackedID = id
	m.bumpSelection()
	if displayed > 0 && displayed != id {
		m.glanceHoldID = displayed
	}
	m.notifyWarmer()
	return m.ensureSelectionLoads()
}

// warmerNeighborRadius is the P1 window around the cursor for background warmers.
const warmerNeighborRadius = 1

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
		if cells >= sprite.SmallestFrameBucket() {
			m.warmer.SetBuckets(sprite.BrowseWarmBuckets())
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
		if cells := m.currentSpriteCells(); cells >= sprite.SmallestFrameBucket() {
			m.nativeWarmer.SetBuckets(sprite.BrowseWarmBuckets())
			m.nativeWarmer.EnsureStarted(m.ctx, cells)
		}
		m.nativeWarmer.Prioritize(sel.ID)
		m.nativeWarmer.SetNeighborsFromList(m.filtered, m.cursor, warmerNeighborRadius)
		m.nativeWarmer.NoteActivity()
	}
}

func (m *Model) ensureSelectionLoads() tea.Cmd {
	return tea.Batch(m.ensureDossierCore(), m.ensureSpriteLoad())
}

// ensureDossierCore loads pokemon+species for glance genus / Enter dossier without extras.
func (m *Model) ensureDossierCore() tea.Cmd {
	sel, ok := m.selected()
	if !ok || m.details == nil {
		return nil
	}
	r := m.ensureResource(sel.ID)
	if r.HasCore {
		return nil
	}
	if r.loadingCore {
		return nil
	}
	r.loadingCore = true
	r.CoreErr = nil
	return loadDossierCoreCmd(m.beginDossierLoad(sel.ID), m.details, m.selectionGen, sel.ID, sel.Slug)
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
	if !lay.ShowSprite || lay.SpriteSize < sprite.SmallestFrameBucket() {
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
		m.resetSpriteLayoutState()
		return nil
	}
	side := m.currentSpriteSide()
	if !prevShow || prevSide != side {
		m.resetSpriteLayoutState()
		return tea.Batch(m.ensureDossierCore(), m.ensureSpriteLoad())
	}
	return nil
}

func (m *Model) resetSpriteLayoutState() {
	m.selectionGen++
	m.cancelSprite()
	if m.cold != nil {
		m.cold.AbortForeground()
	}
	m.clearImagePane()
	m.clearNativeDisplay()
	m.setImageBlank()
}

func (m *Model) noteFirstLayout(cmd tea.Cmd) tea.Cmd {
	if m.uiReady || m.width < 1 || m.height < 1 {
		return cmd
	}
	m.uiReady = true
	debuglog.StartupMark("startup.first_window_begin")
	debuglog.StartupMark("startup.first_view_or_first_render")
	if cmd != nil {
		debuglog.StartupMark("startup.foreground_scheduled")
	}
	if m.onUIReady != nil {
		debuglog.StartupMark("startup.background_ready_queued")
		m.onUIReady()
	}
	return cmd
}

func (m *Model) ensureSpriteLoad() tea.Cmd {
	load := m.beginSpriteLoad()
	m.syncWarmerBusy()
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
		// Only treat official artwork as final. Pixel frames must upgrade when
		// background artwork warm finishes for this species.
		if m.imageKind == "artwork" {
			if m.imageNative && m.imageANSI != "" &&
				!(sel.ID == m.nativePaintSpecies && cells == m.nativePaintCells && m.imageANSI == m.nativePaintANSI) {
				m.markNativeOverlayDirty()
			}
			return nil
		}
		if !m.artworkReady(sel.ID) {
			if m.imageNative && m.imageANSI != "" &&
				!(sel.ID == m.nativePaintSpecies && cells == m.nativePaintCells && m.imageANSI == m.nativePaintANSI) {
				m.markNativeOverlayDirty()
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
	return m.queueOrStartFgRender(pokemon, cells, force)
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
func (m *Model) refreshSpriteAfterWarm(speciesID int, stage WarmStage) tea.Cmd {
	if speciesID == 0 || m.sprites == nil || !m.layout().ShowSprite {
		return nil
	}
	sel, ok := m.selected()
	if !ok || sel.ID != speciesID {
		return nil
	}
	artworkDone := stage == WarmStageArtwork || stage == ""
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
	r := m.ensureResource(sel.ID)
	pokemon := r.Pokemon
	if pokemon.ID == 0 {
		pokemon = domain.Pokemon{ID: sel.ID, Slug: sel.Slug, DisplayName: sel.DisplayName}
	}
	return m.queueOrStartFgRender(pokemon, m.currentSpriteCells(), true)
}

func (m *Model) scrollDialog(delta int) {
	m.syncDialogViewport()
	if delta > 0 {
		m.dialogVP.ScrollDown(delta)
	} else if delta < 0 {
		m.dialogVP.ScrollUp(-delta)
	}
}
