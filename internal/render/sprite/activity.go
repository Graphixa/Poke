package sprite

import (
	"sync/atomic"
	"time"
)

// InteractiveIdle is how long background (P2) warming stays paused after
// selection activity. It must not delay foreground/current-selection renders.
const InteractiveIdle = 400 * time.Millisecond

// InteractiveWorkers is the chafa background encoder count during TUI browse.
// Native sixel warming uses a single opportunistic lane (see nativewarm).
const InteractiveWorkers = 1

// ActivityGate pauses full-dex background warming during navigation.
type ActivityGate struct {
	last atomic.Int64
	fg   atomic.Bool
}

// NoteActivity records a user selection change.
func (g *ActivityGate) NoteActivity() {
	if g == nil {
		return
	}
	g.last.Store(time.Now().UnixNano())
}

// SetForegroundBusy records whether a foreground cold encode is in flight.
func (g *ActivityGate) SetForegroundBusy(busy bool) {
	if g == nil {
		return
	}
	g.fg.Store(busy)
}

// ForegroundBusy reports whether a foreground cold encode is in flight.
func (g *ActivityGate) ForegroundBusy() bool {
	if g == nil {
		return false
	}
	return g.fg.Load()
}

// ThrottleBackground reports whether P2 full-dex work should wait.
func (g *ActivityGate) ThrottleBackground() bool {
	if g == nil {
		return false
	}
	if g.fg.Load() {
		return true
	}
	last := g.last.Load()
	if last == 0 {
		return false
	}
	return time.Since(time.Unix(0, last)) < InteractiveIdle
}
