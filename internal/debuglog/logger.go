// Package debuglog provides optional file-backed diagnostics for request,
// cache, and render paths. Logging never writes to the TUI view or stdout.
package debuglog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
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
	mu      sync.Mutex
	out     io.Writer
	file    *os.File
	enabled bool
	log     *slog.Logger
}

var (
	globalMu sync.RWMutex
	global   = newDiscardLogger()
)

func newDiscardLogger() *Logger {
	return &Logger{out: io.Discard, log: slog.New(discardHandler{})}
}

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
	h := &fileHandler{w: f}
	l := &Logger{
		out:     f,
		file:    f,
		enabled: true,
		log:     slog.New(h),
	}
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
	l.enabled = false
	l.log = slog.New(discardHandler{})
	return err
}

// Enabled reports whether logging is writing somewhere other than Discard.
func (l *Logger) Enabled() bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.enabled
}

// Log writes a single sanitized diagnostic line.
func (l *Logger) Log(cat Category, format string, args ...any) {
	if l == nil {
		return
	}
	l.mu.Lock()
	log := l.log
	enabled := l.enabled
	l.mu.Unlock()
	if !enabled || log == nil {
		return
	}
	msg := sanitize(fmt.Sprintf(format, args...))
	log.LogAttrs(context.Background(), slog.LevelInfo, msg, slog.String("category", string(cat)))
}

// Requestf logs an HTTP/API diagnostic.
func Requestf(format string, args ...any) { Global().Log(Request, format, args...) }

// Cachef logs a cache diagnostic.
func Cachef(format string, args ...any) { Global().Log(Cache, format, args...) }

// Renderf logs a sprite/render diagnostic.
func Renderf(format string, args ...any) { Global().Log(Render, format, args...) }

type fileHandler struct {
	w io.Writer
}

func (h *fileHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *fileHandler) Handle(_ context.Context, r slog.Record) error {
	cat := ""
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "category" {
			cat = a.Value.String()
			return false
		}
		return true
	})
	line := fmt.Sprintf("%s [%s] %s\n", r.Time.UTC().Format(time.RFC3339Nano), cat, r.Message)
	_, err := io.WriteString(h.w, line)
	return err
}

func (h *fileHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *fileHandler) WithGroup(string) slog.Handler { return h }

type discardHandler struct{}

func (discardHandler) Enabled(context.Context, slog.Level) bool { return false }

func (discardHandler) Handle(context.Context, slog.Record) error { return nil }

func (discardHandler) WithAttrs([]slog.Attr) slog.Handler { return discardHandler{} }

func (discardHandler) WithGroup(string) slog.Handler { return discardHandler{} }

func sanitize(msg string) string {
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
