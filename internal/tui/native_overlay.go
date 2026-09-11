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
// Paint must run after Bubble Tea's cell flush: the FPS loop writes Raw first,
// then Clear+Draw, so same-tick Raw is always wiped when View changed.
type nativeOverlayMsg struct {
	// followUp is kept for test helpers; all deferred paints behave the same.
	followUp bool
}

// nativeOverlayRepaintDelay is one frame at the default BT FPS so the wiping
// cell flush completes before we emit tea.Raw (which then sticks via viewEquals).
const nativeOverlayRepaintDelay = time.Second / 60

const (
	decSaveCursor    = "\x1b7" // DECSC
	decRestoreCursor = "\x1b8" // DECRC
	// Windows Terminal: leave cursor at last sixel position, not the cell below.
	wtSixelCursorHereOn  = "\x1b[?8452h"
	wtSixelCursorHereOff = "\x1b[?8452l"
)

// markNativeOverlayDirty requests a deferred paint/clear on the next schedule pass.
func (m *Model) markNativeOverlayDirty() {
	m.nativeOverlayDirty = true
}

// markNativeOverlayDirtyFollowUp requests a deferred paint after View-changing
// updates (selection, dossier core, layout). Same as markNativeOverlayDirty —
// all native paints are post-flush.
func (m *Model) markNativeOverlayDirtyFollowUp() {
	m.nativeOverlayDirty = true
}

// dirtyNativeIfPaintChanged dirties only when the painted payload changes.
func (m *Model) dirtyNativeIfPaintChanged(speciesID, cells int, ansi string) {
	if speciesID == m.nativePaintSpecies && cells == m.nativePaintCells && ansi == m.nativePaintANSI {
		return
	}
	m.markNativeOverlayDirtyFollowUp()
}

// scheduleNativeOverlay queues a one-frame deferred paint/clear when dirty.
//
// BT v2's StyledString drops DCS/graphics from View content, so we reserve blank
// themed cells and paint via tea.Raw after the cell flush. Emitting Raw on the
// same Update that changes View is always wiped (flush order: Raw then Clear+Draw).
// Loading shows an empty themed reservation until the frame is ready (pop-in).
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
		m.nativeOverlayDirty = false
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
	m.nativeOverlayDirty = false
	m.nativePaintPending = true
	debuglog.Renderf("native overlay defer paint")
	return m, tea.Tick(nativeOverlayRepaintDelay, func(time.Time) tea.Msg {
		return nativeOverlayMsg{}
	})
}

func (m Model) emitNativeClear() (Model, tea.Cmd) {
	row, col, artW, artH, ok := views.SpriteArtRegion(m.browseData())
	m.nativePaintANSI = ""
	m.nativePaintSpecies = 0
	m.nativePaintCells = 0
	m.nativePaintPending = false
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

func (m Model) emitNativeOverlay(_ bool) (Model, tea.Cmd) {
	end := debuglog.Trace(debuglog.Render, "frame.paint")
	defer end()
	if !m.nativeOverlayReady() {
		if m.showHelp || m.showDialog {
			// Keep dirty so dismissing help/dialog restores the paint.
			m.nativePaintPending = false
			m.nativeOverlayDirty = true
			debuglog.Renderf("native overlay defer while overlay")
			return m, nil
		}
		if m.nativePaintANSI != "" || m.nativePaintSpecies != 0 {
			return m.emitNativeClear()
		}
		m.nativePaintPending = false
		debuglog.Renderf("native overlay stop")
		return m, nil
	}
	sel, _ := m.selected()
	row, col, artW, artH, ok := views.SpriteArtOrigin(m.browseData())
	if !ok {
		m.nativePaintPending = false
		return m, nil
	}
	seq := nativeOverlayPaintSeq(row, col, artW, artH, m.imageANSI)
	debuglog.Renderf("native overlay emit row=%d col=%d %dx%d bytes=%d",
		row, col, artW, artH, len(m.imageANSI))
	m.nativePaintSpecies = sel.ID
	m.nativePaintCells = m.imageCells
	m.nativePaintANSI = m.imageANSI
	m.nativePaintPending = false
	return m, tea.Raw(seq)
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
