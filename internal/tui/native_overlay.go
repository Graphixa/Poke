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
		m.nativeOverlayDirty = false
		// Wipe leftover Sixel when selection has no native frame (placeholder /
		// loading). Without this, fast scroll leaves the previous species' art.
		if m.nativePaintANSI != "" {
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
	row, col, artW, artH, ok := views.SpriteArtOrigin(m.browseData())
	m.nativePaintANSI = ""
	m.nativePaintSpecies = 0
	m.nativePaintCells = 0
	m.nativePaintPending = false
	m.nativeNeedsFollowUp = false
	if !ok {
		return m, nil
	}
	debuglog.Renderf("native overlay clear row=%d col=%d %dx%d", row, col, artW, artH)
	seq := clearCellRect(row, col, artW, artH)
	if m.sprites != nil {
		if region := m.sprites.ClearNativeRegion(artW, artH); region != "" {
			seq = fmt.Sprintf("\x1b[%d;%dH%s", row, col, region)
		}
	}
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
	r := m.resources[sel.ID]
	if r == nil || !r.HasSprite || r.SpritePlaceholder || !r.SpriteNative || r.SpriteANSI == "" {
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
	if !m.nativeOverlayReady() {
		m.nativePaintPending = false
		debuglog.Renderf("native overlay stop")
		return m, nil
	}
	sel, _ := m.selected()
	r := m.resources[sel.ID]
	row, col, artW, artH, ok := views.SpriteArtOrigin(m.browseData())
	if !ok {
		m.nativePaintPending = false
		return m, nil
	}
	// Clear character cells under the graphics region first. Warm-footer redraws
	// and differential cell updates otherwise leave leftover glyphs ("warming N/M")
	// visible through/around translucent Sixel pixels.
	seq := clearCellRect(row, col, artW, artH) + fmt.Sprintf("\x1b[%d;%dH%s", row, col, r.SpriteANSI)
	debuglog.Renderf("native overlay emit row=%d col=%d %dx%d bytes=%d followUp=%v",
		row, col, artW, artH, len(r.SpriteANSI), followUp)
	m.nativePaintSpecies = sel.ID
	m.nativePaintCells = r.SpriteCells
	m.nativePaintANSI = r.SpriteANSI
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
