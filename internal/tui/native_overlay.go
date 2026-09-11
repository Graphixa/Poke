package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/tui/style"
	"github.com/Graphixa/Pokedex/internal/tui/views"
)

// nativeOverlayMsg signals a deferred native-graphics paint/clear via tea.Raw.
// Paint must run after Bubble Tea's cell flush when View changed: the FPS loop
// writes Raw first, then Clear+Draw, so same-tick Raw is wiped on View churn.
// Never wrap stdout / inject into syncd — that corrupts WT cell apply.
type nativeOverlayMsg struct {
	// followUp is true for the insurance re-paint after a deferred paint.
	followUp bool
}

// nativeOverlayRepaintDelay is one frame at the default BT FPS so the wiping
// cell flush completes before we emit tea.Raw (which then sticks via viewEquals).
const nativeOverlayRepaintDelay = time.Second / 60

// nativeOverlayInsuranceDelay is a second re-paint after the first Raw. WT/BT
// often flush another cell frame (dossier core, list settle) that wipes sixel;
// main used ~60ms — keep that window so the insurance shot lands after churn.
const nativeOverlayInsuranceDelay = 60 * time.Millisecond

const (
	decSaveCursor    = "\x1b7" // DECSC
	decRestoreCursor = "\x1b8" // DECRC
	// Windows Terminal: leave cursor at last sixel position, not the cell below.
	wtSixelCursorHereOn  = "\x1b[?8452h"
	wtSixelCursorHereOff = "\x1b[?8452l"
)

// markNativeOverlayDirty requests a paint eligible for immediate tea.Raw when
// View is already stable (sprite ready after selection settled).
func (m *Model) markNativeOverlayDirty() {
	m.nativeOverlayDirty = true
}

// markNativeOverlayDirtyFollowUp requests a deferred paint after View-changing
// updates (selection, dossier core, layout). Same-tick Raw would be wiped.
func (m *Model) markNativeOverlayDirtyFollowUp() {
	m.nativeOverlayDirty = true
	m.nativeDeferPaint = true
}

// dirtyNativeIfPaintChanged dirties only when the painted payload changes.
// Uses immediate Dirty — payload refresh without selection View churn.
func (m *Model) dirtyNativeIfPaintChanged(speciesID, cells int, ansi string) {
	if speciesID == m.nativePaintSpecies && cells == m.nativePaintCells && ansi == m.nativePaintANSI {
		return
	}
	m.markNativeOverlayDirty()
}

// scheduleNativeOverlay paints Kitty/iTerm/Sixel via tea.Raw when dirty.
//
// BT v2's StyledString drops DCS/graphics from View content, so we reserve blank
// themed cells and paint via tea.Raw. View-changing dirties (nativeDeferPaint)
// wait one frame so Clear+Draw finishes first. Every successful ready paint
// schedules one insurance re-paint (~60ms) so a follow-on cell flush does not
// leave a permanent drop. Stable-View ready paints immediately.
func (m Model) scheduleNativeOverlay() (Model, tea.Cmd) {
	if !m.nativeOverlayDirty {
		return m, nil
	}
	if m.nativePaintPending {
		// A deferred tick is already queued; the handler reads latest model state.
		return m, nil
	}
	if !m.nativeOverlayReady() {
		// Keep dirty while dialog/help is up so Esc restores the paint.
		if m.showHelp || m.showDialog {
			return m, nil
		}
		// Layout unknown: a cache hit before WindowSize must not drop the dirty
		// flag or the first species never paints.
		if m.width < 1 || m.height < 1 {
			return m, nil
		}
		if m.nativeStickyPaintOK() {
			m.nativeOverlayDirty = false
			m.nativeDeferPaint = false
			m.nativePaintPending = true
			m.nativePaintInsurance = true // sticky Raw can also be wiped by list churn
			debuglog.Renderf("native overlay defer sticky")
			return m, tea.Tick(nativeOverlayRepaintDelay, func(time.Time) tea.Msg {
				return nativeOverlayMsg{}
			})
		}
		m.nativeOverlayDirty = false
		m.nativeDeferPaint = false
		m.nativePaintInsurance = false
		// Wipe leftover Sixel when we truly have nothing to show (unavailable /
		// hidden / size change). Avoids ghost art from a prior species.
		if m.nativePaintANSI != "" || m.nativePaintSpecies != 0 {
			m.nativePaintPending = true
			debuglog.Renderf("native overlay defer clear")
			return m, tea.Tick(nativeOverlayRepaintDelay, func(time.Time) tea.Msg {
				return nativeOverlayMsg{}
			})
		}
		return m, nil
	}
	deferPaint := m.nativeDeferPaint
	m.nativeOverlayDirty = false
	m.nativeDeferPaint = false
	// Always arm insurance for ready paints — dossier/list churn drops sixel.
	m.nativePaintInsurance = true
	if deferPaint {
		m.nativePaintPending = true
		debuglog.Renderf("native overlay defer paint")
		return m, tea.Tick(nativeOverlayRepaintDelay, func(time.Time) tea.Msg {
			return nativeOverlayMsg{}
		})
	}
	debuglog.Renderf("native overlay immediate paint")
	return m.emitNativeOverlay(false)
}

// nativeStickyPaintOK is true when we should deferred-repaint the last native
// frame while the current selection's sprite is not ready yet.
func (m Model) nativeStickyPaintOK() bool {
	if m.nativePaintANSI == "" {
		return false
	}
	if m.showHelp || m.showDialog || m.quitting {
		return false
	}
	lay := m.layout()
	if !lay.ShowSprite || lay.SpriteSize < sprite.SmallestFrameBucket() {
		return false
	}
	cells := m.currentSpriteCells()
	if cells < 1 || m.nativePaintCells != cells {
		return false
	}
	switch m.imageState {
	case ImageLoading, ImageBlank:
		return true
	default:
		return false
	}
}

func (m Model) emitNativeClear() (Model, tea.Cmd) {
	row, col, artW, artH, ok := views.SpriteArtRegion(m.browseData())
	m.nativePaintANSI = ""
	m.nativePaintSpecies = 0
	m.nativePaintCells = 0
	m.nativePaintPending = false
	m.nativePaintInsurance = false
	m.nativeDeferPaint = false
	if m.sprites != nil {
		m.sprites.ClearNative()
	}
	if !ok {
		return m, nil
	}
	debuglog.Renderf("native overlay clear row=%d col=%d %dx%d", row, col, artW, artH)
	seq := m.nativeOverlayClearSeq(row, col, artW, artH)
	return m, tea.Raw(seq)
}

func (m Model) nativeOverlayReady() bool {
	if m.showHelp || m.showDialog || m.quitting {
		return false
	}
	sel, ok := m.selected()
	if !ok {
		return false
	}
	if m.imageSpeciesID != sel.ID || m.imagePlaceholder || !m.imageNative || m.imageANSI == "" {
		return false
	}
	lay := m.layout()
	if !lay.ShowSprite || lay.SpriteSize < sprite.SmallestFrameBucket() {
		return false
	}
	_, _, _, _, ok = views.SpriteArtOrigin(m.browseData())
	return ok
}

func (m Model) emitNativeOverlay(followUp bool) (Model, tea.Cmd) {
	end := debuglog.Trace(debuglog.Render, "frame.paint")
	defer end()
	if !m.nativeOverlayReady() {
		if m.showHelp || m.showDialog {
			// Keep dirty so dismissing help/dialog restores the paint.
			m.nativePaintPending = false
			m.nativePaintInsurance = false
			m.nativeOverlayDirty = true
			m.nativeDeferPaint = true
			debuglog.Renderf("native overlay defer while overlay")
			return m, nil
		}
		if m.nativeStickyPaintOK() {
			return m.emitNativeStickyOverlay()
		}
		if m.nativePaintANSI != "" || m.nativePaintSpecies != 0 {
			return m.emitNativeClear()
		}
		m.nativePaintPending = false
		m.nativePaintInsurance = false
		debuglog.Renderf("native overlay stop")
		return m, nil
	}
	sel, _ := m.selected()
	row, col, artW, artH, ok := views.SpriteArtOrigin(m.browseData())
	if !ok {
		m.nativePaintPending = false
		m.nativePaintInsurance = false
		return m, nil
	}
	seq := nativeOverlayPaintSeq(row, col, artW, artH, m.imageANSI)
	debuglog.Renderf("native overlay emit row=%d col=%d %dx%d bytes=%d followUp=%v",
		row, col, artW, artH, len(m.imageANSI), followUp)
	m.nativePaintSpecies = sel.ID
	m.nativePaintCells = m.imageCells
	m.nativePaintANSI = m.imageANSI
	cmds := []tea.Cmd{tea.Raw(seq)}
	// Insurance re-paint: a later cell flush (dossier core, warm footer layout)
	// often wipes the first Raw. One shot at ~60ms matches origin/main recovery.
	if !followUp && m.nativePaintInsurance {
		m.nativePaintInsurance = false
		m.nativePaintPending = true
		cmds = append(cmds, tea.Tick(nativeOverlayInsuranceDelay, func(time.Time) tea.Msg {
			return nativeOverlayMsg{followUp: true}
		}))
	} else {
		m.nativePaintPending = false
		m.nativePaintInsurance = false
	}
	return m, tea.Batch(cmds...)
}

// emitNativeStickyOverlay re-paints the last native frame after a cell wipe
// while the new selection is still loading. Does not retarget paint identity
// to the new selection until a real ready paint runs.
func (m Model) emitNativeStickyOverlay() (Model, tea.Cmd) {
	row, col, artW, artH, ok := views.SpriteArtRegion(m.browseData())
	if !ok || m.nativePaintANSI == "" {
		m.nativePaintPending = false
		m.nativePaintInsurance = false
		return m, nil
	}
	seq := nativeOverlayPaintSeq(row, col, artW, artH, m.nativePaintANSI)
	debuglog.Renderf("native overlay sticky emit row=%d col=%d %dx%d bytes=%d species=%d",
		row, col, artW, artH, len(m.nativePaintANSI), m.nativePaintSpecies)
	cmds := []tea.Cmd{tea.Raw(seq)}
	if m.nativePaintInsurance {
		m.nativePaintInsurance = false
		m.nativePaintPending = true
		cmds = append(cmds, tea.Tick(nativeOverlayInsuranceDelay, func(time.Time) tea.Msg {
			return nativeOverlayMsg{followUp: true}
		}))
	} else {
		m.nativePaintPending = false
	}
	return m, tea.Batch(cmds...)
}

// nativeOverlayPaintSeq positions and paints DCS between DECSC/DECRC.
// It does not space-clear first: clearing the art box destroys any on-screen
// sixel and flashes a void before the new graphic. Same-geometry sixel
// overwrites in place; emitNativeClear handles real wipes.
func nativeOverlayPaintSeq(row, col, artW, artH int, ansi string) string {
	artW, artH = clampArtCells(artW, artH)
	var b strings.Builder
	b.WriteString(decSaveCursor)
	b.WriteString(wtSixelCursorHereOn)
	if ansi != "" && row >= 1 && col >= 1 {
		fmt.Fprintf(&b, "\x1b[%d;%dH%s", row, col, ansi)
	}
	b.WriteString(wtSixelCursorHereOff)
	b.WriteString(decRestoreCursor)
	return b.String()
}

func (m Model) nativeOverlayClearSeq(row, col, artW, artH int) string {
	artW, artH = clampArtCells(artW, artH)
	var b strings.Builder
	b.WriteString(decSaveCursor)
	b.WriteString(wtSixelCursorHereOn)
	// Theme-coloured spaces first so hosts without a protocol clear don't flash
	// terminal default black.
	b.WriteString(clearCellRect(row, col, artW, artH))
	if m.sprites != nil {
		if fill := m.sprites.ClearNativeRegion(artW, artH); fill != "" && row >= 1 && col >= 1 {
			fmt.Fprintf(&b, "\x1b[%d;%dH%s", row, col, fill)
		}
	}
	b.WriteString(wtSixelCursorHereOff)
	b.WriteString(decRestoreCursor)
	return b.String()
}

// clampArtCells keeps overlay clear/paint inside a single art inner box.
// Rejects non-positive sizes; caps at the max native bucket so a bad caller
// cannot wipe Stats/About under the art card.
func clampArtCells(artW, artH int) (int, int) {
	maxW, maxH := sprite.FrameSizeFor(sprite.MaxFrameSize)
	minW, minH := sprite.FrameSizeFor(sprite.SmallestFrameBucket())
	if artW < 1 {
		artW = minW
	}
	if artH < 1 {
		artH = minH
	}
	if artW > maxW {
		artW = maxW
	}
	if artH > maxH {
		artH = maxH
	}
	return artW, artH
}

// clearCellRect writes theme-base spaces over a WxH cell box via CUP (1-indexed).
// Unstyled spaces use the terminal default background (often pure black on
// alt-screen), which reads as a black flash under Sixel letterboxing.
func clearCellRect(row, col, w, h int) string {
	if w < 1 || h < 1 || row < 1 || col < 1 {
		return ""
	}
	bg := style.BaseRGBA()
	// Truecolor background + reset after the row so Stats/About keep their styles.
	prefix := fmt.Sprintf("\x1b[48;2;%d;%d;%dm", bg.R, bg.G, bg.B)
	const reset = "\x1b[49m"
	blank := prefix + strings.Repeat(" ", w) + reset
	var b strings.Builder
	b.Grow(h * (24 + w))
	for i := 0; i < h; i++ {
		fmt.Fprintf(&b, "\x1b[%d;%dH%s", row+i, col, blank)
	}
	return b.String()
}
