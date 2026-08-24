package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Graphixa/Pokedex/internal/config"
)

type recordingHandler struct {
	calls []RootRequest
}

func (h *recordingHandler) RunRoot(_ context.Context, request RootRequest) error {
	h.calls = append(h.calls, request)
	return nil
}

func commandForTest(t *testing.T, invocation string, handler RootHandler) (*bytes.Buffer, *bytes.Buffer, func(...string) error) {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root := NewRoot(Options{
		InvocationPath: invocation,
		Paths: config.Paths{
			ConfigFile: filepath.Join(t.TempDir(), "config.yaml"),
			CacheDir:   filepath.Join(t.TempDir(), "cache"),
		},
		Getenv:  func(string) string { return "" },
		Editor:  &recordingRunner{},
		Handler: handler,
		Stdout:  stdout,
		Stderr:  stderr,
	})
	execute := func(args ...string) error {
		root.SetArgs(args)
		return root.Execute()
	}
	return stdout, stderr, execute
}

func TestVersionCommandAndFlag(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"--version"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			stdout, _, execute := commandForTest(t, "poke", nil)
			if err := execute(args...); err != nil {
				t.Fatal(err)
			}
			if got := stdout.String(); !strings.HasPrefix(got, "poke dev (commit none, built unknown)") {
				t.Fatalf("version output = %q", got)
			}
		})
	}
}

func TestAliasInvocationRoutesIdentically(t *testing.T) {
	handler := &recordingHandler{}
	_, _, execute := commandForTest(t, `C:\bin\pokedex.exe`, handler)
	if err := execute("pikachu", "--stats"); err != nil {
		t.Fatal(err)
	}
	if len(handler.calls) != 1 {
		t.Fatalf("handler calls = %d", len(handler.calls))
	}
	want := RootRequest{Target: "pikachu", InlineMode: InlineStats, Invocation: "pokedex"}
	if handler.calls[0] != want {
		t.Fatalf("request = %#v, want %#v", handler.calls[0], want)
	}
}

func TestRootFlagRouting(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want RootRequest
	}{
		{name: "browse", args: nil, want: RootRequest{Invocation: "poke"}},
		{name: "target", args: []string{"25"}, want: RootRequest{Target: "25", Invocation: "poke"}},
		{name: "info", args: []string{"eevee", "--info"}, want: RootRequest{Target: "eevee", InlineMode: InlineInfo, Invocation: "poke"}},
		{name: "evolution", args: []string{"eevee", "--evolution"}, want: RootRequest{Target: "eevee", InlineMode: InlineEvolution, Invocation: "poke"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &recordingHandler{}
			_, _, execute := commandForTest(t, "poke", handler)
			if err := execute(tt.args...); err != nil {
				t.Fatal(err)
			}
			if len(handler.calls) != 1 || handler.calls[0] != tt.want {
				t.Fatalf("calls = %#v, want %#v", handler.calls, tt.want)
			}
		})
	}
}

func TestRootRejectsInvalidInlineRouting(t *testing.T) {
	_, _, execute := commandForTest(t, "poke", &recordingHandler{})
	if err := execute("--stats"); err == nil {
		t.Fatal("--stats without a target succeeded")
	}

	_, _, execute = commandForTest(t, "poke", &recordingHandler{})
	if err := execute("pikachu", "--stats", "--info"); err == nil {
		t.Fatal("mutually exclusive flags succeeded")
	}
}

func TestConfigAndCachePathAreSubcommands(t *testing.T) {
	handler := &recordingHandler{}
	stdout, _, execute := commandForTest(t, "poke", handler)
	if err := execute("config", "path"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(strings.TrimSpace(stdout.String()), "config.yaml") {
		t.Fatalf("config path output = %q", stdout.String())
	}
	if len(handler.calls) != 0 {
		t.Fatalf("root handler called for subcommand: %#v", handler.calls)
	}
}

func TestDebugFlagInvokesCallback(t *testing.T) {
	var got string
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root := NewRoot(Options{
		InvocationPath: "poke",
		Paths: config.Paths{
			ConfigFile: filepath.Join(t.TempDir(), "config.yaml"),
			CacheDir:   filepath.Join(t.TempDir(), "cache"),
		},
		Getenv:  func(string) string { return "" },
		Handler: &recordingHandler{},
		Stdout:  stdout,
		Stderr:  stderr,
		OnDebug: func(path string) error {
			got = path
			return nil
		},
	})
	root.SetArgs([]string{"--debug=" + filepath.Join(t.TempDir(), "x.log"), "version"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, "x.log") {
		t.Fatalf("debug path = %q", got)
	}
}

func TestTestCommandRoutes(t *testing.T) {
	called := false
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root := NewRoot(Options{
		InvocationPath: "poke",
		Paths: config.Paths{
			ConfigFile: filepath.Join(t.TempDir(), "config.yaml"),
			CacheDir:   filepath.Join(t.TempDir(), "cache"),
		},
		Test: TestHandlerFunc(func(context.Context) error {
			called = true
			return nil
		}),
		Stdout: stdout,
		Stderr: stderr,
	})
	root.SetArgs([]string{"test"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("TestSprites was not called")
	}
}

type recordingRunner struct{}

func (*recordingRunner) Run(string, ...string) error {
	return nil
}
