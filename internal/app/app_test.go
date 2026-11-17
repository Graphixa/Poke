package app

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Graphixa/Poke/internal/config"
)

func testRuntime(t *testing.T, args ...string) (Runtime, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	runtime := Runtime{
		InvocationPath: "poke",
		Args:           args,
		Getenv:         func(string) string { return "" },
		Directories: config.DirectoryResolver{
			UserConfigDir: func() (string, error) { return filepath.Join(t.TempDir(), "config"), nil },
			UserCacheDir:  func() (string, error) { return filepath.Join(t.TempDir(), "cache"), nil },
		},
		Stdout: stdout,
		Stderr: stderr,
	}
	return runtime, stdout, stderr
}

func TestMainVersion(t *testing.T) {
	runtime, stdout, _ := testRuntime(t, "version")
	if code := Main(context.Background(), runtime); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stdout.String(), "poke dev") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestMainReportsOneStableError(t *testing.T) {
	runtime, _, stderr := testRuntime(t, "--stats")
	if code := Main(context.Background(), runtime); code != 1 {
		t.Fatalf("exit code = %d", code)
	}
	if got := stderr.String(); !strings.HasPrefix(got, "Error: --stats requires") {
		t.Fatalf("stderr = %q", got)
	}
}

func TestExitCodeCancellation(t *testing.T) {
	if got := ExitCode(context.Canceled); got != 130 {
		t.Fatalf("ExitCode(context.Canceled) = %d", got)
	}
	if got := ExitCode(errors.New("failure")); got != 1 {
		t.Fatalf("ExitCode(failure) = %d", got)
	}
}
