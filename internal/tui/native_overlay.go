package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Poke/internal/debuglog"
	"github.com/Graphixa/Poke/internal/render/sprite"
	"github.com/Graphixa/Poke/internal/tui/style"
	"github.com/Graphixa/Poke/internal/tui/views"
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

// nativeCompositeFlushMsg writes pending CUP+DCS after View has run. PaintNow
// during Update placed sixel before the art reservation cells existed on WT
// (sprite overflowing the art box until relaunch). SetNative + this flush keeps
// cells-then-DCS order while still covering viewEquals skips.
type nativeCompositeFlushMsg struct{}

func (m *Model) markNativeOverlayDirty() {
	m.nativeOverlayDirty = true
}

func (m Model) scheduleNativeOverlay() (Model, tea.Cmd) {
	if !m.nativeOverlayDirty {
		return m, nil
	}
	// Compare paints its own dual sixel via CUP; browse paint stays suppressed.
	if m.showCompare {
		if m.compareNativeReady() {
			return m.emitCompareNativeOverlay()
		}
		// Both sides loading / no native yet — wipe ghost sixels, keep dirty for restore.
		if m.compareNativePainted {
			return m.emitCompareNativeClear()
		}
		if m.compositor != nil {
			m.compositor.ClearNative()
		}
		return m, nil
	}
	// Keep dirty while covered so Esc can restore — but drop pending DCS so
	// cell flushes cannot re-tail the sprite over the dialog/help.
	if m.overlayOpen() {
		if m.compositor != nil {
			m.compositor.ClearNative()
		}
		return m, nil
	}
	if m.width < 1 || m.height < 1 {
		return m, nil
	}
	if !m.nativeOverlayReady() {
		m.nativeOverlayDirty = false
		m.nativeFollowUpPending = false
		// Glance almost always cache-hits and overwrites in place (no clear).
		// Art 40 often misses; the Glance miss path (space-clear then DCS) is
		// what overpaints the hero card. Same bucket: keep last sixel.
		if m.nativePaintCells > 0 && m.nativePaintCells == m.currentSpriteCells() && !m.imagePlaceholder {
			return m, nil
		}
		return m.emitNativeClear()
	}
	m.nativeOverlayDirty = false
	debuglog.Renderf("native overlay paint")
	return m.emitNativeOverlay()
}

// coverNativeOverlay erases on-screen native graphics and drops compositor
// pending so help/dialog cell frames cannot re-tail DCS.
func (m Model) coverNativeOverlay() (Model, tea.Cmd) {
	if m.compositor != nil {
		m.compositor.ClearNative()
	}
	if m.compareNativePainted {
		return m.emitCompareNativeClear()
	}
	hadPaint := m.nativePaintCells > 0 || m.nativePaintANSI != "" || m.nativePaintSpecies != 0
	if !hadPaint {
		return m, nil
	}
	return m.emitNativeClear()
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
	seq := m.nativeOverlayClearSeq(row, col, artW, artH)
	if m.composite && m.compositor != nil {
		// Clear before a covering screen is drawn, otherwise the old art
		// rectangle erases the new screen's text. Quit also has no next frame.
		if m.quitting || m.overlayOpen() {
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
	if m.compareNativePainted || m.showCompare {
		_, cmd := m.emitCompareNativeClear()
		m.nativePaintANSI = ""
		m.nativePaintSpecies = 0
		m.nativePaintCells = 0
		m.nativeFollowUpPending = false
		m.nativeOverlayDirty = false
		return cmd
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
	seq := m.nativeOverlayClearSeq(row, col, artW, artH)
	if m.compositor != nil {
		m.compositor.WriteDirect(seq)
		return nil
	}
	return tea.Raw(seq)
}

// compareNativeReady is true when compare can CUP-paint at least one sixel.
func (m Model) compareNativeReady() bool {
	if !m.showCompare || m.quitting {
		return false
	}
	_, _, _, _, ok := m.compareArtOrigin(0)
	if !ok {
		return false
	}
	aOK := m.compareArtANative && m.compareArtA != "" && m.compareArtAID == m.compareA.Summary.ID
	bOK := m.compareArtBNative && m.compareArtB != "" && m.compareArtBID == m.compareB.Summary.ID
	return aOK || bOK
}

func (m Model) emitCompareNativeOverlay() (Model, tea.Cmd) {
	if !m.compareNativeReady() {
		return m, nil
	}
	m.nativeOverlayDirty = false
	rowA, colA, artW, artH, okA := m.compareArtOrigin(0)
	rowB, colB, _, _, okB := m.compareArtOrigin(1)
	// Always clear artH only (no sixelClearSlackRows — slack paints grey under the card).
	var b strings.Builder
	b.WriteString(decSaveCursor)
	paint := func(row, col int, ansi string, ok bool) {
		if !ok {
			return
		}
		b.WriteString(clearCellRect(row, col, artW, artH))
		if ansi != "" {
			fmt.Fprintf(&b, "\x1b[%d;%dH%s", row, col, ansi)
		}
	}
	if m.compareArtANative && m.compareArtAID == m.compareA.Summary.ID {
		paint(rowA, colA, m.compareArtA, okA)
	} else if okA {
		paint(rowA, colA, "", true)
	}
	if m.compareArtBNative && m.compareArtBID == m.compareB.Summary.ID {
		paint(rowB, colB, m.compareArtB, okB)
	} else if okB {
		paint(rowB, colB, "", true)
	}
	b.WriteString(decRestoreCursor)
	seq := b.String()
	debuglog.Renderf("compare native overlay A=%d,%d B=%d,%d %dx%d", rowA, colA, rowB, colB, artW, artH)
	m.compareNativePainted = true
	m.nativePaintCells = m.compareArtCells
	m.nativePaintANSI = ""
	m.nativePaintSpecies = 0
	if m.composite && m.compositor != nil {
		m.compositor.SetNative(seq)
		m.nativeFollowUpPending = false
		return m, func() tea.Msg { return nativeCompositeFlushMsg{} }
	}
	raw := tea.Raw(seq)
	if m.nativeFollowUpPending {
		m.nativeFollowUpPending = false
		return m, raw
	}
	m.nativeFollowUpPending = true
	return m, tea.Batch(raw, tea.Tick(nativeReassertDelay, func(time.Time) tea.Msg {
		return nativeReassertMsg{}
	}))
}

func (m Model) emitCompareNativeClear() (Model, tea.Cmd) {
	had := m.compareNativePainted
	seq := m.compareNativeClearSeq()
	m.compareNativePainted = false
	m.nativePaintCells = 0
	m.nativePaintANSI = ""
	m.nativePaintSpecies = 0
	if !had || seq == "" {
		if m.compositor != nil {
			m.compositor.ClearNative()
		}
		return m, nil
	}
	if m.composite && m.compositor != nil {
		if m.quitting || m.overlayOpen() {
			m.compositor.WriteDirect(seq)
			return m, nil
		}
		m.compositor.SetNativeOnce(seq)
		return m, nil
	}
	return m, tea.Raw(seq)
}

// forceCompareNativeWipe clears dual compare sixels synchronously (Esc / unpin).
func (m *Model) forceCompareNativeWipe() {
	seq := m.compareNativeClearSeq()
	m.compareNativePainted = false
	m.nativePaintCells = 0
	m.nativePaintANSI = ""
	m.nativePaintSpecies = 0
	if m.compositor != nil {
		m.compositor.ClearNative()
		if seq != "" {
			m.compositor.WriteDirect(seq)
		}
	}
}

func (m Model) compareNativeClearSeq() string {
	rowA, colA, artW, artH, okA := m.compareArtOrigin(0)
	rowB, colB, _, _, okB := m.compareArtOrigin(1)
	if !okA && !okB {
		return ""
	}
	var b strings.Builder
	b.WriteString(decSaveCursor)
	if okA {
		b.WriteString(clearCellRect(rowA, colA, artW, artH))
	}
	if okB {
		b.WriteString(clearCellRect(rowB, colB, artW, artH))
	}
	b.WriteString(decRestoreCursor)
	return b.String()
}

// wipeCompareSideRect clears one art box synchronously so ↑/↓ does not leave a ghost sixel.
func (m *Model) wipeCompareSideRect(side int) {
	row, col, artW, artH, ok := m.compareArtOrigin(side)
	if !ok {
		return
	}
	seq := decSaveCursor + clearCellRect(row, col, artW, artH) + decRestoreCursor
	if m.compositor != nil {
		m.compositor.WriteDirect(seq)
		return
	}
}

func (m Model) nativeOverlayReady() bool {
	if m.overlayOpen() || m.quitting {
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
	// Clear only when replacing a different painted bucket. Same-bucket species
	// scroll overwrites in place (DCS only — no clear, no flicker); the encode
	// fits the art box because cell pixels are measured at startup, so no spill
	// accumulates. After an explicit wipe (cells==0) the rect is already empty.
	clearFirst := m.nativePaintCells > 0 && m.nativePaintCells != m.imageCells
	seq := m.nativeOverlayPaintSeq(row, col, artW, artH, m.imageANSI, clearFirst)
	debuglog.Renderf("native overlay emit row=%d col=%d %dx%d bytes=%d clear=%v",
		row, col, artW, artH, len(m.imageANSI), clearFirst)
	m.nativePaintSpecies = sel.ID
	m.nativePaintCells = m.imageCells
	m.nativePaintANSI = m.imageANSI
	if m.composite && m.compositor != nil {
		// Defer write until after View (cells-then-DCS). Flush covers viewEquals.
		m.compositor.SetNative(seq)
		m.nativeFollowUpPending = false
		return m, func() tea.Msg { return nativeCompositeFlushMsg{} }
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
// the art box (plus slack) and DECERA-erases ImageSlices before CUP+DCS.
// Same-geometry paints overwrite in place.
//
// No DECSET 8452: on some Windows Terminal builds it interacted badly with
// multi-row clears and left gaps through the sprite. Plain CUP+DCS is enough.
func (m Model) nativeOverlayPaintSeq(row, col, artW, artH int, ansi string, clearFirst bool) string {
	artW, artH = clampArtCells(artW, artH)
	clearH := artH + sixelClearSlackRows
	var b strings.Builder
	b.WriteString(decSaveCursor)
	if clearFirst {
		b.WriteString(clearCellRect(row, col, artW, clearH))
		b.WriteString(eraseNativeRect(row, col, artW, clearH))
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
	m.compareNativePainted = false
	if m.composite && m.compositor != nil {
		m.compositor.ClearNative()
	}
}

// nativeOverlayClearSeq erases an art box: theme spaces (cell attrs) plus DECERA
// (WT ImageSlice drop). Spaces alone often no-op when the reservation is already
// blank; opaque ClearNativeRegion overwrites pixels but leaves ImageSlices that
// then sit on top of Stats / empty rows after Art→Glance.
func (m Model) nativeOverlayClearSeq(row, col, artW, artH int) string {
	artW, artH = clampArtCells(artW, artH)
	clearH := artH + sixelClearSlackRows
	var b strings.Builder
	b.WriteString(decSaveCursor)
	b.WriteString(clearCellRect(row, col, artW, clearH))
	b.WriteString(eraseNativeRect(row, col, artW, clearH))
	b.WriteString(decRestoreCursor)
	return b.String()
}

// eraseNativeRect emits DECERA (CSI Pt;Pl;Pb;Pr $z). On Windows Terminal that
// path FillRect → ImageSlice::EraseCells, which is what actually drops Sixel.
func eraseNativeRect(row, col, w, h int) string {
	if row < 1 || col < 1 || w < 1 || h < 1 {
		return ""
	}
	return fmt.Sprintf("\x1b[%d;%d;%d;%d$z", row, col, row+h-1, col+w-1)
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
