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

// Native graphics (Sixel/Kitty/iTerm) cannot live in Bubble Tea's cell View —
// StyledString drops DCS. Production Run uses compositorWriter (cells-then-DCS).
// Unit tests without a compositor still use tea.Raw + ~50ms re-assert.

const (
	decSaveCursor    = "\x1b7" // DECSC
	decRestoreCursor = "\x1b8" // DECRC

	// Extra clear rows below the art box for hosts whose cell height is shorter
	// than SixelCellPxH (spill into pad/border). Stays inside card chrome.
	sixelClearSlackRows = 2
)

// Delay before post-flush overwrite. Tests set to 0 so collectRawPayloads can
// walk paint Batches without blocking on tea.Tick.
var nativeReassertDelay = 50 * time.Millisecond

// nativeReassertMsg triggers one overwrite paint after the cell flush that
// follows the initial Raw (see emitNativeOverlay).
type nativeReassertMsg struct{}

func (m *Model) markNativeOverlayDirty() {
	m.nativeOverlayDirty = true
}

func (m Model) scheduleNativeOverlay() (Model, tea.Cmd) {
	if !m.nativeOverlayDirty {
		return m, nil
	}
	// Keep dirty while covered so Esc can restore.
	if m.showHelp || m.showDialog {
		return m, nil
	}
	if m.width < 1 || m.height < 1 {
		return m, nil
	}
	if !m.nativeOverlayReady() {
		m.nativeOverlayDirty = false
		m.nativeFollowUpPending = false
		return m.emitNativeClear()
	}
	m.nativeOverlayDirty = false
	debuglog.Renderf("native overlay paint")
	return m.emitNativeOverlay()
}

func (m Model) emitNativeClear() (Model, tea.Cmd) {
	hadPaint := m.nativePaintCells > 0 || m.nativePaintANSI != "" || m.nativePaintSpecies != 0
	row, col, artW, artH, ok := views.SpriteArtRegion(m.browseData())
	m.nativePaintANSI = ""
	m.nativePaintSpecies = 0
	m.nativePaintCells = 0
	if !ok || !hadPaint {
		if m.composite && m.compositor != nil {
			m.compositor.ClearNative()
		}
		return m, nil
	}
	debuglog.Renderf("native overlay clear row=%d col=%d %dx%d", row, col, artW, artH)
	seq := nativeOverlayClearSeq(row, col, artW, artH)
	if m.composite && m.compositor != nil {
		// Quit has no further cell flush — write clear now. Live clears can wait
		// for the next frame via SetNativeOnce; on exit that would never run and
		// a pending SetNative could re-paint DCS after alt-screen exit.
		if m.quitting {
			m.compositor.WriteDirect(seq)
			return m, nil
		}
		m.compositor.SetNativeOnce(seq)
		return m, nil
	}
	return m, tea.Raw(seq)
}

// wipeNativeGraphics drops pending DCS and erases the art box at the current
// origin. Call before Quit (so alt-screen exit cannot re-tail DCS onto the main
// buffer) and before resize (so the old CUP placement is not left behind).
func (m *Model) wipeNativeGraphics() tea.Cmd {
	if m.sprites != nil {
		m.sprites.ClearNative()
	}
	hadPaint := m.nativePaintCells > 0 || m.nativePaintANSI != "" || m.nativePaintSpecies != 0
	row, col, artW, artH, ok := views.SpriteArtRegion(m.browseData())
	m.nativePaintANSI = ""
	m.nativePaintSpecies = 0
	m.nativePaintCells = 0
	m.nativeFollowUpPending = false
	m.nativeOverlayDirty = false
	if !ok || !hadPaint {
		if m.compositor != nil {
			m.compositor.ClearNative()
		}
		return nil
	}
	seq := nativeOverlayClearSeq(row, col, artW, artH)
	if m.compositor != nil {
		m.compositor.WriteDirect(seq)
		return nil
	}
	return tea.Raw(seq)
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

func (m Model) emitNativeOverlay() (Model, tea.Cmd) {
	end := debuglog.Trace(debuglog.Render, "frame.paint")
	defer end()
	if !m.nativeOverlayReady() {
		return m, nil
	}
	sel, _ := m.selected()
	row, col, artW, artH, ok := views.SpriteArtOrigin(m.browseData())
	if !ok {
		return m, nil
	}
	// Clear only when replacing a different painted bucket. After an explicit
	// wipe (cells==0) the rect is already empty — skip a second space-clear flash.
	clearFirst := m.nativePaintCells > 0 && m.nativePaintCells != m.imageCells
	seq := nativeOverlayPaintSeq(row, col, artW, artH, m.imageANSI, clearFirst)
	debuglog.Renderf("native overlay emit row=%d col=%d %dx%d bytes=%d clear=%v",
		row, col, artW, artH, len(m.imageANSI), clearFirst)
	m.nativePaintSpecies = sel.ID
	m.nativePaintCells = m.imageCells
	m.nativePaintANSI = m.imageANSI
	if m.composite && m.compositor != nil {
		// Paint immediately; also keep pending so later cell flushes re-assert.
		m.compositor.PaintNow(seq)
		m.nativeFollowUpPending = false
		return m, nil
	}
	raw := tea.Raw(seq)
	if m.nativeFollowUpPending {
		// This paint is the post-flush re-assert — do not chain another timer.
		m.nativeFollowUpPending = false
		return m, raw
	}
	m.nativeFollowUpPending = true
	return m, tea.Batch(raw, tea.Tick(nativeReassertDelay, func(time.Time) tea.Msg {
		return nativeReassertMsg{}
	}))
}

// nativeOverlayPaintSeq paints DCS between DECSC/DECRC. clearFirst theme-spaces
// the art box (plus slack) before CUP+DCS. Same-geometry paints overwrite in place.
//
// No DECSET 8452: on some Windows Terminal builds it interacted badly with
// multi-row clears and left gaps through the sprite. Plain CUP+DCS is enough.
func nativeOverlayPaintSeq(row, col, artW, artH int, ansi string, clearFirst bool) string {
	artW, artH = clampArtCells(artW, artH)
	var b strings.Builder
	b.WriteString(decSaveCursor)
	if clearFirst {
		b.WriteString(clearCellRect(row, col, artW, artH+sixelClearSlackRows))
	}
	if ansi != "" && row >= 1 && col >= 1 {
		fmt.Fprintf(&b, "\x1b[%d;%dH%s", row, col, ansi)
	}
	b.WriteString(decRestoreCursor)
	return b.String()
}

// invalidateNativePaintGeometry forces the next ready paint to treat geometry
// as unset (help/dialog dismiss). Does not emit Raw.
func (m *Model) invalidateNativePaintGeometry() {
	m.nativePaintANSI = ""
	m.nativePaintSpecies = 0
	m.nativePaintCells = 0
	m.nativeFollowUpPending = false
	if m.composite && m.compositor != nil {
		m.compositor.ClearNative()
	}
}

func nativeOverlayClearSeq(row, col, artW, artH int) string {
	artW, artH = clampArtCells(artW, artH)
	var b strings.Builder
	b.WriteString(decSaveCursor)
	b.WriteString(clearCellRect(row, col, artW, artH+sixelClearSlackRows))
	b.WriteString(decRestoreCursor)
	return b.String()
}

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

// clearCellRect writes theme-base spaces over a WxH cell box (1-indexed CUP).
// Unstyled spaces use the terminal default background (often black on alt-screen).
func clearCellRect(row, col, w, h int) string {
	if w < 1 || h < 1 || row < 1 || col < 1 {
		return ""
	}
	bg := style.BaseRGBA()
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
