// Package debuglog provides optional file-backed diagnostics for request,
// cache, and render paths. Logging never writes to the TUI view or stdout.
package debuglog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Category labels diagnostics without dumping secrets or screen contents.
type Category string

const (
	Request Category = "request"
	Cache   Category = "cache"
	Render  Category = "render"
)

// Logger writes structured one-line diagnostics to a file (or discards).
type Logger struct {
	mu   sync.Mutex
	out  io.Writer
	file *os.File
}

var (
	globalMu sync.RWMutex
	global   = &Logger{out: io.Discard}
)

// Global returns the process-wide logger (discard until Enable).
func Global() *Logger {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return global
}

// Enable opens path for append logging. Empty path uses a default under
// the OS temp directory. Calling Enable again replaces the previous logger.
func Enable(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		path = filepath.Join(os.TempDir(), "poke-debug.log")
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("debug log directory: %w", err)
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return "", fmt.Errorf("open debug log: %w", err)
	}
	l := &Logger{out: f, file: f}
	globalMu.Lock()
	prev := global
	global = l
	globalMu.Unlock()
	if prev != nil {
		_ = prev.Close()
	}
	l.Log(Request, "debug logging enabled path=%s", path)
	return path, nil
}

// Close releases the underlying file if owned.
func (l *Logger) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	l.out = io.Discard
	return err
}

// Enabled reports whether logging is writing somewhere other than Discard.
func (l *Logger) Enabled() bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.out != nil && l.out != io.Discard
}

// Log writes a single sanitized diagnostic line.
func (l *Logger) Log(cat Category, format string, args ...any) {
	if l == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	msg = sanitize(msg)
	line := fmt.Sprintf("%s [%s] %s\n", time.Now().UTC().Format(time.RFC3339Nano), cat, msg)
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.out == nil {
		return
	}
	_, _ = io.WriteString(l.out, line)
}

// Requestf logs an HTTP/API diagnostic.
func Requestf(format string, args ...any) { Global().Log(Request, format, args...) }

// Cachef logs a cache diagnostic.
func Cachef(format string, args ...any) { Global().Log(Cache, format, args...) }

// Renderf logs a sprite/render diagnostic.
func Renderf(format string, args ...any) { Global().Log(Render, format, args...) }

func sanitize(msg string) string {
	// Strip control characters that could corrupt a terminal if the log is
	// accidentally printed; never log Authorization-like headers.
	msg = strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\t' {
			return -1
		}
		if r == 0x7f {
			return -1
		}
		return r
	}, msg)
	lower := strings.ToLower(msg)
	for _, secret := range []string{"authorization=", "api_key=", "api-key=", "token=", "password="} {
		if i := strings.Index(lower, secret); i >= 0 {
			msg = msg[:i] + secret + "<redacted>"
			break
		}
	}
	return msg
}
