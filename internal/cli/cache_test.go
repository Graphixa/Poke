package cli_test

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Graphixa/Pokedex/internal/cli"
	"github.com/Graphixa/Pokedex/internal/config"
)

type stubCacheCommands struct {
	clear  func() error
	update func(context.Context) error
}

func (s stubCacheCommands) Clear() error {
	if s.clear != nil {
		return s.clear()
	}
	return nil
}

func (s stubCacheCommands) Update(ctx context.Context) error {
	if s.update != nil {
		return s.update(ctx)
	}
	return nil
}

func TestUpdateAndCacheCommands(t *testing.T) {
	var updated, cleared int
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root := cli.NewRoot(cli.Options{
		InvocationPath: "poke",
		Paths: config.Paths{
			ConfigFile: filepath.Join(t.TempDir(), "config.yaml"),
			CacheDir:   filepath.Join(t.TempDir(), "cache"),
		},
		Stdout: stdout,
		Stderr: stderr,
		Cache: stubCacheCommands{
			clear:  func() error { cleared++; return nil },
			update: func(context.Context) error { updated++; return nil },
		},
	})

	root.SetArgs([]string{"update"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"cache", "clear"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"cache", "path"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if updated != 1 || cleared != 1 {
		t.Fatalf("updated=%d cleared=%d want 1,1", updated, cleared)
	}
	if !strings.Contains(stdout.String(), "cache") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
