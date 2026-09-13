package tui

import (
	"bytes"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
)

// Exported accessors for model/layout tests.

func (m Model) Query() string                      { return m.queryValue() }
func (m Model) CanQuitWithQ() bool                 { return m.canQuitWithQ() }
func (m Model) HelpVisible() bool                  { return m.showHelp }
func (m Model) DialogVisible() bool                { return m.showDialog }
func (m Model) Quitting() bool                     { return m.quitting }
func (m Model) ActiveBrowseView() BrowseView       { return m.browseView }
func (m Model) ActiveDialogSection() DialogSection { return m.dialogSection }
func (m Model) DialogOffset() int                  { return m.dialogVP.YOffset() }
func (m Model) SelectionGen() int                  { return m.selectionGen }

func (m Model) SelectedID() (int, bool) {
	sel, ok := m.selected()
	if !ok {
		return 0, false
	}
	return sel.ID, true
}

func (m Model) ResourceName(speciesID int) string {
	r := m.resources[speciesID]
	if r == nil || !r.HasCore {
		return ""
	}
	return r.Pokemon.DisplayName
}

func (m Model) LayoutFlags() LayoutFlags { return m.layout() }

// DossierCoreMsgForTest builds a core-loaded message for race tests.
func DossierCoreMsgForTest(gen, speciesID int, dossier domain.Dossier, complete bool, err error) dossierCoreMsg {
	return dossierCoreMsg{gen: gen, speciesID: speciesID, dossier: dossier, complete: complete, err: err}
}

// DossierExtrasMsgForTest builds an extras-loaded message for tests.
func DossierExtrasMsgForTest(gen, speciesID int, dossier domain.Dossier, err error) dossierExtrasMsg {
	return dossierExtrasMsg{gen: gen, speciesID: speciesID, dossier: dossier, err: err}
}

// ExtrasReadyForTest reports whether speciesID has all dossier extras loaded.
func (m Model) ExtrasReadyForTest(speciesID int) bool {
	r := m.resources[speciesID]
	return r != nil && r.extrasReady()
}

// ExtrasLoadingForTest reports whether extras are in flight.
func (m Model) ExtrasLoadingForTest(speciesID int) bool {
	r := m.resources[speciesID]
	return r != nil && r.loadingExtras
}

// DossierMsgForTest builds a dossier-loaded message for race tests.
func DossierMsgForTest(gen, speciesID int, dossier domain.Dossier, err error) dossierMsg {
	return dossierMsg{gen: gen, speciesID: speciesID, dossier: dossier, err: err}
}

// SpriteMsgForTest builds a sprite-loaded message for race tests.
func SpriteMsgForTest(reqID, gen, speciesID, cells int, ansi string, placeholder bool) spriteMsg {
	return spriteMsg{
		reqID: reqID, gen: gen, speciesID: speciesID, cells: cells,
		ansi: ansi, placeholder: placeholder, assetKind: "artwork",
	}
}

// SpriteMsgKindForTest builds a sprite message with an explicit asset kind.
func SpriteMsgKindForTest(reqID, gen, speciesID, cells int, ansi, kind string, placeholder bool) spriteMsg {
	return spriteMsg{
		reqID: reqID, gen: gen, speciesID: speciesID, cells: cells,
		ansi: ansi, placeholder: placeholder, assetKind: kind,
	}
}

// SpriteMsgNativeForTest marks a sprite message as native graphics.
func SpriteMsgNativeForTest(msg spriteMsg) spriteMsg {
	msg.native = true
	return msg
}

// WarmProgressMsgForTest builds a warm-progress update for overlay debounce tests.
func WarmProgressMsgForTest(done, total int) warmProgressMsg {
	return warmProgressMsg{Done: done, Total: total, Message: "warming"}
}

// WarmProgressSpeciesMsgForTest builds a warm tick for a finished species.
func WarmProgressSpeciesMsgForTest(done, total, speciesID int) warmProgressMsg {
	return warmProgressMsg{Done: done, Total: total, Message: "warming", SpeciesID: speciesID, Stage: WarmStageArtwork}
}

// WarmProgressSpeciesStageMsgForTest builds a warm tick with an explicit stage.
func WarmProgressSpeciesStageMsgForTest(done, total, speciesID int, stage WarmStage) warmProgressMsg {
	return warmProgressMsg{Done: done, Total: total, Message: "warming", SpeciesID: speciesID, Stage: stage}
}

// WarmProgressNativeMsgForTest builds a native warm tick with optional size bucket.
func WarmProgressNativeMsgForTest(done, total, cells int) warmProgressMsg {
	return warmProgressMsg{Done: done, Total: total, Message: "Building image cache", Stage: WarmStageNative, Cells: cells}
}

// NativePaintANSIForTest returns the last painted native sequence (test helper).
func (m Model) NativePaintANSIForTest() string { return m.nativePaintANSI }

// GlanceHoldIDForTest returns the right-pane hold species (0 if none).
func (m Model) GlanceHoldIDForTest() int { return m.glanceHoldID }

// DropResourceForTest removes a species from the detail LRU (simulates eviction).
func (m *Model) DropResourceForTest(id int) {
	delete(m.resources, id)
	out := m.lruIDs[:0]
	for _, existing := range m.lruIDs {
		if existing != id {
			out = append(out, existing)
		}
	}
	m.lruIDs = out
}

// BrowseSelectedNameForTest returns the name shown in the glance header.
func (m Model) BrowseSelectedNameForTest() string {
	return m.browseData().Selected.DisplayName
}

// BrowseAboutHasCoreForTest reports whether browseData currently exposes About.
func (m Model) BrowseAboutHasCoreForTest() bool {
	return m.browseData().HasCore
}

// BrowseAboutHeightForTest returns the About height string from browseData.
func (m Model) BrowseAboutHeightForTest() string {
	return m.browseData().AboutHeight
}

// NativeOverlayDirtyForTest reports whether a native re-paint is requested.
func (m Model) NativeOverlayDirtyForTest() bool { return m.nativeOverlayDirty }

// NativeFollowUpPendingForTest reports whether a post-flush overwrite is armed.
func (m Model) NativeFollowUpPendingForTest() bool { return m.nativeFollowUpPending }

// NativeReassertMsgForTest is the post-flush overwrite tick message.
func NativeReassertMsgForTest() tea.Msg { return nativeReassertMsg{} }

// InstantNativeReassertForTest sets the post-flush delay to 0 (Tick returns promptly).
func InstantNativeReassertForTest() { nativeReassertDelay = 0 }

// AttachCompositorForTest enables composite paint into buf (cells-then-DCS path).
func (m *Model) AttachCompositorForTest(buf *bytes.Buffer) {
	m.composite = true
	m.compositor = newCompositorWriter(buf)
}

// NativePaintCellsForTest reports the last painted cell bucket token.
func (m Model) NativePaintCellsForTest() int { return m.nativePaintCells }

// ClearCellRectForTest exports clearCellRect for overlay tests.
func ClearCellRectForTest(row, col, w, h int) string {
	return clearCellRect(row, col, w, h)
}

// NativeOverlayPaintSeqForTest is the Raw payload used to paint a native sprite
// with a leading space-clear (geometry change).
func NativeOverlayPaintSeqForTest(row, col, artW, artH int, ansi string) string {
	return nativeOverlayPaintSeq(row, col, artW, artH, ansi, true)
}

// NativeOverlayPaintOnlySeqForTest paints DCS without a leading space-clear
// (same-geometry overwrite path).
func NativeOverlayPaintOnlySeqForTest(row, col, artW, artH int, ansi string) string {
	return nativeOverlayPaintSeq(row, col, artW, artH, ansi, false)
}

// BrowseWarmHintForTest returns the warm status shown in the browse footer (right segment).
func (m Model) BrowseWarmHintForTest() string {
	return m.warmRight
}

// BrowseVisibleWarmHintForTest returns warm hint actually rendered in View (hidden under native art).
func (m Model) BrowseVisibleWarmHintForTest() string {
	return m.browseData().WarmHint
}

// SpriteReqID exposes the current sprite request id for tests.
func (m Model) SpriteReqID() int { return m.spriteReqID }

// HasSpriteANSI reports whether the selected species has rendered sprite art.
func (m Model) HasSpriteANSI() bool {
	sel, ok := m.selected()
	if !ok {
		return false
	}
	return m.imageSpeciesID == sel.ID && !m.imagePlaceholder && m.imageANSI != ""
}

// SelectedSpritePlaceholderForTest reports sticky placeholder state.
func (m Model) SelectedSpritePlaceholderForTest() bool {
	sel, ok := m.selected()
	if !ok {
		return false
	}
	return m.imageSpeciesID == sel.ID && m.imagePlaceholder
}

// SelectedSpriteLoadingForTest reports whether the selected sprite is waiting on a frame.
func (m Model) SelectedSpriteLoadingForTest() bool {
	return m.imageState == ImageLoading || m.foregroundColdBusy()
}

// CurrentSpriteCells exposes the active sprite cell size for tests.
func (m Model) CurrentSpriteCells() int { return m.currentSpriteCells() }

// CoreLoading reports whether speciesID is in-flight/stuck without HasCore.
func (m Model) CoreLoading(speciesID int) bool {
	r := m.resources[speciesID]
	return r != nil && r.loadingCore && !r.HasCore
}

// HeaderLoading is always false on browse (dossier loads only in the dialog).
func (m Model) HeaderLoading() bool {
	return false
}

// EnsureDossierLoadForTest runs ensureDossierLoad (for dialog race tests).
func (m *Model) EnsureDossierLoadForTest() tea.Cmd { return m.ensureDossierLoad() }

// CancelDossierForTest clears an in-flight dossier load (for tests that discard cmds).
func (m *Model) CancelDossierForTest() { m.cancelDossier() }

// EnsureSelectionLoadsForTest runs the selected-species sprite load only.
func (m *Model) EnsureSelectionLoadsForTest() tea.Cmd { return m.beginSpriteLoad() }

func (m Model) ResourceCountForTest() int { return len(m.resources) }

func (m Model) ImageStateForTest() int { return int(m.imageState) }

func (m Model) ImageSpeciesIDForTest() int { return m.imageSpeciesID }

func (m Model) ImageCellsForTest() int { return m.imageCells }

func (m Model) ImageANSILenForTest() int { return len(m.imageANSI) }

// ForceSpriteLoadForTest cancels any in-flight encode and starts a new one.
func (m *Model) ForceSpriteLoadForTest() tea.Cmd {
	m.cancelSprite()
	return m.beginSpriteLoad()
}

func (m Model) FgActiveSpeciesForTest() int {
	if m.coldSched == nil {
		return 0
	}
	return m.coldSched.ActiveSpecies()
}

func (m Model) FgPendingSpeciesForTest() int {
	if m.coldSched == nil {
		return 0
	}
	return m.coldSched.PendingSpecies()
}

// ColdFrameMsgForTest builds a cold-scheduler frame message for spinner tests.
func ColdFrameMsgForTest(gen, speciesID, cells int, ansi string, placeholder, native bool, assetKind string) coldFrameMsg {
	return coldFrameMsg{
		gen: gen, speciesID: speciesID, cells: cells, ansi: ansi,
		placeholder: placeholder, native: native, assetKind: assetKind,
	}
}
