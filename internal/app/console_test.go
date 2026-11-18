package app

import "testing"

func TestConsoleRestoreDoesNotPanic(t *testing.T) {
	restoreConsole()
	endConsoleLine()
}
