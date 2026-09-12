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

	// sixelClearSlackRows extends space-clear / opaque wipe below the art
	// reservation. Encoded frames assume SixelCellPxH (20); if the host cell is
	// shorter, the bitmap spills into the card's bottom pad/border. Clearing only
	// artH leaves a thin remnant strip (often the previous species) at the
	// bottom of the Glance art box. Slack stays inside the card chrome (pad+border).
	sixelClearSlackRows = 2
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
// cells and paint via tea.Raw. Ready paints emit immediately; View-changing
// dirties arm one ~60ms insurance re-paint. Never wrap stdout / WithOutput.
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
		// loading / hold text-only). Prefer immediate clear like main.
		return m.emitNativeClear()
	}
	m.nativeOverlayDirty = false
	m.nativePaintPending = true
	debuglog.Renderf("native overlay paint")
	return m.emitNativeOverlay(false)
}

func (m Model) emitNativeClear() (Model, tea.Cmd) {
	// Only emit a Raw wipe when something was actually painted. On startup / when
	// already clear there is no prior Sixel, so skip the round-trip (matches
	// origin/main's guarded clear).
	hadPaint := m.nativePaintCells > 0 || m.nativePaintANSI != "" || m.nativePaintSpecies != 0
	row, col, artW, artH, ok := views.SpriteArtRegion(m.browseData())
	m.nativePaintANSI = ""
	m.nativePaintSpecies = 0
	m.nativePaintCells = 0
	m.nativePaintPending = false
	m.nativeNeedsFollowUp = false
	if m.sprites != nil {
		m.sprites.ClearNative()
	}
	if !ok || !hadPaint {
		return m, nil
	}
	debuglog.Renderf("native overlay clear row=%d col=%d %dx%d", row, col, artW, artH)
	seq := nativeOverlayClearSeq(row, col, artW, artH)
	return m, tea.Raw(seq)
}

// nativeOverlayReady is true when a native frame can paint for the selection.
// Glance text hold does not block ready sixel — art may paint while About lags.
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
	// Same cell bucket as the last painted/cleared geometry → overwrite in place
	// (no space-clear). First paint, size change, or post-wipe (cells==0) clears.
	clearFirst := m.nativePaintCells < 1 || m.nativePaintCells != m.imageCells
	seq := nativeOverlayPaintSeq(row, col, artW, artH, m.imageANSI, clearFirst)
	debuglog.Renderf("native overlay emit row=%d col=%d %dx%d bytes=%d followUp=%v clear=%v",
		row, col, artW, artH, len(m.imageANSI), followUp, clearFirst)
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

// nativeOverlayPaintSeq paints DCS between DECSC/DECRC. When clearFirst, theme
// space-clears the art box (plus slack) before CUP+DCS — used after real wipes
// or geometry change. Same-geometry replaces skip the clear and overwrite in place
// so WT warm scroll does not flash an empty rect before every DCS.
//
// Space-clear runs without ?8452. Enabling 8452 during the multi-row clear leaves
// the cursor at the bottom of the rect; some WT builds then place the following
// sixel there, which shows up as a bottom sliver inside the Glance art card.
func nativeOverlayPaintSeq(row, col, artW, artH int, ansi string, clearFirst bool) string {
	artW, artH = clampArtCells(artW, artH)
	var b strings.Builder
	b.WriteString(decSaveCursor)
	if clearFirst {
		clearH := artH + sixelClearSlackRows
		b.WriteString(clearCellRect(row, col, artW, clearH))
	}
	if ansi != "" && row >= 1 && col >= 1 {
		fmt.Fprintf(&b, "\x1b[%d;%dH", row, col)
		b.WriteString(wtSixelCursorHereOn)
		b.WriteString(ansi)
		b.WriteString(wtSixelCursorHereOff)
	}
	b.WriteString(decRestoreCursor)
	return b.String()
}

// invalidateNativePaintGeometry forces the next ready paint to space-clear
// (help/dialog dismiss, layout wipe). Does not emit Raw itself.
func (m *Model) invalidateNativePaintGeometry() {
	m.nativePaintANSI = ""
	m.nativePaintSpecies = 0
	m.nativePaintCells = 0
}

// nativeOverlayClearSeq wipes the art box with theme-base spaces (plus slack for
// WT cell-px overflow) between DECSC/DECRC. On WT a cell write erases the
// underlying Sixel, so themed spaces both clear the previous frame and avoid a
// black flash. No solid-Sixel fill is emitted: encoding a full opaque rectangle
// on every uncached selection was pure overhead that read as lag while browsing.
func nativeOverlayClearSeq(row, col, artW, artH int) string {
	artW, artH = clampArtCells(artW, artH)
	clearH := artH + sixelClearSlackRows
	var b strings.Builder
	b.WriteString(decSaveCursor)
	b.WriteString(clearCellRect(row, col, artW, clearH))
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
