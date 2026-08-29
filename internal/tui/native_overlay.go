package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/tui/views"
)

// nativeOverlayMsg signals a native-graphics paint pass via tea.Raw.
type nativeOverlayMsg struct {
	// followUp is true for the single delayed re-paint after a dirty paint.
	followUp bool
}

// nativeOverlayRepaintDelay lets Bubble Tea finish a cell flush that would wipe
// Sixel before we re-emit once. Not a perpetual refresh loop.
const nativeOverlayRepaintDelay = 60 * time.Millisecond

const (
	decSaveCursor    = "\x1b7" // DECSC
	decRestoreCursor = "\x1b8" // DECRC
	// Windows Terminal: leave cursor at last sixel position, not the cell below.
	wtSixelCursorHereOn  = "\x1b[?8452h"
	wtSixelCursorHereOff = "\x1b[?8452l"
)

// markNativeOverlayDirty requests a paint on the next scheduleNativeOverlay pass.
func (m *Model) markNativeOverlayDirty() {
	m.nativeOverlayDirty = true
}

// markNativeOverlayDirtyFollowUp also schedules the single delayed re-paint
// (used after selection/content changes that BT may wipe).
func (m *Model) markNativeOverlayDirtyFollowUp() {
	m.nativeOverlayDirty = true
	m.nativeNeedsFollowUp = true
}

// dirtyNativeIfPaintChanged dirties only when the painted payload changes.
func (m *Model) dirtyNativeIfPaintChanged(speciesID, cells int, ansi string) {
	if speciesID == m.nativePaintSpecies && cells == m.nativePaintCells && ansi == m.nativePaintANSI {
		return
	}
	m.markNativeOverlayDirtyFollowUp()
}

// scheduleNativeOverlay paints Kitty/iTerm/Sixel via tea.Raw when dirty.
//
// BT v2's StyledString drops DCS/graphics from View content, so we reserve blank
// cells and paint via tea.Raw. Any later cell redraw (list move, warm footer,
// resize) wipes the image — clear the art rect then re-emit when dirty.
func (m Model) scheduleNativeOverlay() (Model, tea.Cmd) {
	if !m.nativeOverlayDirty {
		return m, nil
	}
	if m.nativePaintPending {
		if m.nativeOverlayReady() {
			return m, nil
		}
		m.nativePaintPending = false
		m.nativeNeedsFollowUp = false
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
		// Wipe leftover Sixel when selection has no native frame (placeholder /
		// loading). Without this, fast scroll leaves the previous species' art.
		if m.nativePaintANSI != "" || m.nativePaintSpecies != 0 {
			return m.emitNativeClear()
		}
		return m, nil
	}
	m.nativeOverlayDirty = false
	m.nativePaintPending = true
	debuglog.Renderf("native overlay paint")
	return m.emitNativeOverlay(false)
}

func (m Model) emitNativeClear() (Model, tea.Cmd) {
	row, col, artW, artH, ok := views.SpriteArtRegion(m.browseData())
	m.nativePaintANSI = ""
	m.nativePaintSpecies = 0
	m.nativePaintCells = 0
	m.nativePaintPending = false
	m.nativeNeedsFollowUp = false
	if !ok {
		return m, nil
	}
	debuglog.Renderf("native overlay clear row=%d col=%d %dx%d", row, col, artW, artH)
	seq := nativeOverlayClearSeq(row, col, artW, artH)
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
	if !lay.ShowSprite || lay.SpriteSize < sprite.MinFrameSize {
		return false
	}
	_, _, _, _, ok = views.SpriteArtOrigin(m.browseData())
	return ok
}

func (m Model) emitNativeOverlay(followUp bool) (Model, tea.Cmd) {
	end := debuglog.Trace(debuglog.Render, "frame.paint")
	defer end()
	if !m.nativeOverlayReady() {
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
	debuglog.Renderf("native overlay emit row=%d col=%d %dx%d bytes=%d followUp=%v",
		row, col, artW, artH, len(m.imageANSI), followUp)
	m.nativePaintSpecies = sel.ID
	m.nativePaintCells = m.imageCells
	m.nativePaintANSI = m.imageANSI
	cmds := []tea.Cmd{tea.Raw(seq)}
	if !followUp {
		if m.nativeNeedsFollowUp {
			m.nativeNeedsFollowUp = false
			cmds = append(cmds, tea.Tick(nativeOverlayRepaintDelay, func(time.Time) tea.Msg {
				return nativeOverlayMsg{followUp: true}
			}))
		} else {
			m.nativePaintPending = false
		}
	} else {
		m.nativePaintPending = false
	}
	return m, tea.Batch(cmds...)
}

// nativeOverlayPaintSeq space-clears the art box, then paints DCS between
// DECSC/DECRC so the cursor is not left below the image. No footer, no extra
// Sixel bands after the image ST.
func nativeOverlayPaintSeq(row, col, artW, artH int, ansi string) string {
	var b strings.Builder
	b.WriteString(decSaveCursor)
	b.WriteString(wtSixelCursorHereOn)
	b.WriteString(clearCellRect(row, col, artW, artH))
	if ansi != "" && row >= 1 && col >= 1 {
		fmt.Fprintf(&b, "\x1b[%d;%dH%s", row, col, ansi)
	}
	b.WriteString(wtSixelCursorHereOff)
	b.WriteString(decRestoreCursor)
	return b.String()
}

func nativeOverlayClearSeq(row, col, artW, artH int) string {
	return decSaveCursor + clearCellRect(row, col, artW, artH) + decRestoreCursor
}

// clearCellRect writes spaces over a WxH cell box via CUP (1-indexed origin).
func clearCellRect(row, col, w, h int) string {
	if w < 1 || h < 1 || row < 1 || col < 1 {
		return ""
	}
	blank := strings.Repeat(" ", w)
	var b strings.Builder
	b.Grow(h * (16 + w))
	for i := 0; i < h; i++ {
		fmt.Fprintf(&b, "\x1b[%d;%dH%s", row+i, col, blank)
	}
	return b.String()
}
