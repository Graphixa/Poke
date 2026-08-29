package debuglog

import (
	"os"
	"strings"
	"testing"
	"time"
)

func resetStartupTrace() {
	startupMu.Lock()
	defer startupMu.Unlock()
	startupOrigin = time.Time{}
	startupMarks = nil
}

func TestStartupTrace(t *testing.T) {
	resetStartupTrace()
	t.Setenv("POKEDEX_DEBUG_STARTUP", "1")

	StartupMark("startup.services")
	StartupMark("startup.dataset")
	StartupMark("startup.browse")
	StartupMark("startup.tui")

	lines := StartupReport()
	if len(lines) != 4 {
		t.Fatalf("lines=%d want 4: %v", len(lines), lines)
	}
	if !strings.HasPrefix(lines[0], "startup.services ") {
		t.Fatalf("first line=%q", lines[0])
	}
	if !strings.HasPrefix(lines[3], "startup.tui ") {
		t.Fatalf("last line=%q", lines[3])
	}
}

func TestStartupDisabledByDefault(t *testing.T) {
	_ = os.Unsetenv("POKEDEX_DEBUG_STARTUP")
	if StartupEnabled() {
		t.Fatal("startup trace disabled unless POKEDEX_DEBUG_STARTUP=1")
	}
	if got := StartupReport(); got != nil {
		t.Fatalf("disabled report=%v want nil", got)
	}
}
