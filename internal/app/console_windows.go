package app

import (
	"os"

	"golang.org/x/sys/windows"
)

// lipgloss and Bubble Tea call os.Stdout.Fd and, while a TUI is up, set
// DISABLE_NEWLINE_AUTO_RETURN. That flag lives on the shared console, so a
// later `\n` (cache clear, version, the next prompt) moves down but stays
// in the column where the text ended.

func restoreConsole() {
	for _, f := range []*os.File{os.Stdout, os.Stderr} {
		clearStickyNewline(f)
	}
}

func endConsoleLine() {
	restoreConsole()
	parkCursor(os.Stdout)
	parkCursor(os.Stderr)
}

func clearStickyNewline(f *os.File) {
	h := windows.Handle(f.Fd())
	var mode uint32
	if windows.GetConsoleMode(h, &mode) != nil {
		return
	}
	if mode&windows.DISABLE_NEWLINE_AUTO_RETURN == 0 {
		return
	}
	_ = windows.SetConsoleMode(h, mode&^windows.DISABLE_NEWLINE_AUTO_RETURN)
}

func parkCursor(f *os.File) {
	var info windows.ConsoleScreenBufferInfo
	if windows.GetConsoleScreenBufferInfo(windows.Handle(f.Fd()), &info) != nil {
		return
	}
	if info.CursorPosition.X == 0 {
		return
	}
	_, _ = f.Write([]byte("\r"))
}
