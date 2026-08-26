package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/tui/views"
)

// nativeOverlayMsg signals a native-graphics paint pass via tea.Raw.
type nativeOverlayMsg struct{}

const nativeOverlayInterval = 100 * time.Millisecond

// scheduleNativeOverlay starts a refresh loop that re-paints Kitty/iTerm/Sixel
// on top of Bubble Tea's cell frame.
//
// Why a loop: BT v2's StyledString drops DCS/graphics from View content, so we
// reserve blank cells and paint via tea.Raw. Any later cell redraw (list move,
// warm footer, resize) wipes the image — so we keep re-emitting while the
// native sprite is visible.
func (m Model) scheduleNativeOverlay() (Model, tea.Cmd) {
	if m.nativePaintPending {
		return m, nil
	}
	if !m.nativeOverlayReady() {
		return m, nil
	}
	m.nativePaintPending = true
	debuglog.Renderf("native overlay loop start")
	return m, tea.Tick(nativeOverlayInterval, func(time.Time) tea.Msg {
		return nativeOverlayMsg{}
	})
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
	_, _, ok = views.SpriteArtOrigin(m.browseData())
	return ok
}

func (m Model) emitNativeOverlay() (Model, tea.Cmd) {
	if !m.nativeOverlayReady() {
		m.nativePaintPending = false
		debuglog.Renderf("native overlay loop stop")
		return m, nil
	}
	sel, _ := m.selected()
	r := m.resources[sel.ID]
	row, col, ok := views.SpriteArtOrigin(m.browseData())
	if !ok {
		m.nativePaintPending = false
		return m, nil
	}
	seq := fmt.Sprintf("\x1b[%d;%dH%s", row, col, r.SpriteANSI)
	debuglog.Renderf("native overlay emit row=%d col=%d bytes=%d", row, col, len(r.SpriteANSI))
	// Keep pending=true and reschedule so the next cell redraw cannot leave a blank hole.
	return m, tea.Batch(
		tea.Raw(seq),
		tea.Tick(nativeOverlayInterval, func(time.Time) tea.Msg {
			return nativeOverlayMsg{}
		}),
	)
}
