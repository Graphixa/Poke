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

func TestUpdateAndCacheCommands(t *testing.T) {
	var refreshed, cleared int
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
		Cache: cli.CacheCommandsFuncs{
			RefreshFunc:     func(context.Context) error { refreshed++; return nil },
			ClearFunc:       func() error { cleared++; return nil },
			WarmSpritesFunc: func(context.Context) error { return nil },
		},
	})

	root.SetArgs([]string{"update"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"cache", "refresh"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"cache", "clear"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"cache", "warm-sprites"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"cache", "path"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if refreshed != 2 || cleared != 1 {
		t.Fatalf("refreshed=%d cleared=%d", refreshed, cleared)
	}
	if !strings.Contains(stdout.String(), "cache") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
