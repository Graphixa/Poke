package tui

import (
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
	return warmProgressMsg{Done: done, Total: total, Message: "warming", SpeciesID: speciesID, Stage: "artwork"}
}

// WarmProgressSpeciesStageMsgForTest builds a warm tick with an explicit stage.
func WarmProgressSpeciesStageMsgForTest(done, total, speciesID int, stage string) warmProgressMsg {
	return warmProgressMsg{Done: done, Total: total, Message: "warming", SpeciesID: speciesID, Stage: stage}
}

// NativePaintPendingForTest reports whether a native overlay tick is queued.
func (m Model) NativePaintPendingForTest() bool { return m.nativePaintPending }

// NativePaintANSIForTest returns the last painted native sequence (test helper).
func (m Model) NativePaintANSIForTest() string { return m.nativePaintANSI }

// NativeOverlayDirtyForTest reports whether a native re-paint is requested.
func (m Model) NativeOverlayDirtyForTest() bool { return m.nativeOverlayDirty }

// NativeOverlayFollowUpForTest builds the single delayed re-paint message.
func NativeOverlayFollowUpForTest() nativeOverlayMsg {
	return nativeOverlayMsg{followUp: true}
}

// ClearCellRectForTest exports clearCellRect for overlay tests.
func ClearCellRectForTest(row, col, w, h int) string {
	return clearCellRect(row, col, w, h)
}

// BrowseWarmHintForTest returns the warm status shown in the browse footer (right segment).
func (m Model) BrowseWarmHintForTest() string {
	return m.warmRight
}

// SpriteReqID exposes the current sprite request id for tests.
func (m Model) SpriteReqID() int { return m.spriteReqID }

// HasSpriteANSI reports whether the selected species has rendered sprite art.
func (m Model) HasSpriteANSI() bool {
	sel, ok := m.selected()
	if !ok {
		return false
	}
	r := m.resources[sel.ID]
	return r != nil && r.HasSprite && r.SpriteANSI != ""
}

// SelectedSpritePlaceholderForTest reports sticky placeholder state.
func (m Model) SelectedSpritePlaceholderForTest() bool {
	sel, ok := m.selected()
	if !ok {
		return false
	}
	r := m.resources[sel.ID]
	return r != nil && r.SpritePlaceholder
}

// SelectedSpriteLoadingForTest reports whether a sprite encode is in flight.
func (m Model) SelectedSpriteLoadingForTest() bool {
	sel, ok := m.selected()
	if !ok {
		return false
	}
	r := m.resources[sel.ID]
	return r != nil && r.loadingSprite
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

// EnsureSelectionLoadsForTest runs the selected-species sprite load only
// (prefetch is tested separately).
func (m *Model) EnsureSelectionLoadsForTest() tea.Cmd { return m.ensureSpriteLoad() }

// ScheduleSpritePrefetchForTest starts neighbor sprite encodes.
func (m *Model) ScheduleSpritePrefetchForTest() tea.Cmd { return m.scheduleSpritePrefetch() }

// PrefetchGenForTest exposes the prefetch generation counter.
func (m Model) PrefetchGenForTest() int { return m.prefetchGen }

// BumpPrefetchGenForTest cancels in-flight prefetch and advances the generation.
func (m *Model) BumpPrefetchGenForTest() {
	m.cancelPrefetch()
	m.prefetchGen++
}
