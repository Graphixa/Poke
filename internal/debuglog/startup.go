package debuglog

import (
	"os"
	"sync"
	"time"
)

var (
	startupMu     sync.Mutex
	startupOrigin time.Time
	startupMarks  []startupMark
)

type startupMark struct {
	label string
	at    time.Time
}

// StartupEnabled reports whether startup timing trace is active.
func StartupEnabled() bool {
	return os.Getenv("POKEDEX_DEBUG_STARTUP") == "1"
}

// StartupMark records a monotonic startup checkpoint when debug startup is enabled.
func StartupMark(label string) {
	if !StartupEnabled() {
		return
	}
	startupMu.Lock()
	defer startupMu.Unlock()
	if startupOrigin.IsZero() {
		startupOrigin = time.Now()
	}
	startupMarks = append(startupMarks, startupMark{label: label, at: time.Now()})
}

// StartupReport returns formatted startup timing lines.
func StartupReport() []string {
	if !StartupEnabled() {
		return nil
	}
	startupMu.Lock()
	defer startupMu.Unlock()
	if startupOrigin.IsZero() {
		return nil
	}
	out := make([]string, 0, len(startupMarks))
	prev := startupOrigin
	for _, m := range startupMarks {
		out = append(out, m.label+" "+m.at.Sub(prev).Round(time.Millisecond).String())
		prev = m.at
	}
	return out
}
