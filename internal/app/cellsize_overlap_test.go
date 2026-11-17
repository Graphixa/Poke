package app

import (
	"testing"
	"time"
)

func TestStartQueryCellPixelSizeNonTTYCompletes(t *testing.T) {
	// CI / pipes: probe must finish quickly and always send on the channel.
	ch := startQueryCellPixelSize()
	select {
	case r := <-ch:
		if r.ok {
			t.Fatal("non-TTY probe must not claim success")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("probe channel stalled")
	}
}

func TestCellSizeTimeoutConstant(t *testing.T) {
	if cellSizeQueryTimeout > 60*time.Millisecond {
		t.Fatalf("cellSizeQueryTimeout=%v want ≤50ms for startup budget", cellSizeQueryTimeout)
	}
	if stdinDrainIdle > 15*time.Millisecond {
		t.Fatalf("stdinDrainIdle=%v want ≤10ms", stdinDrainIdle)
	}
}
