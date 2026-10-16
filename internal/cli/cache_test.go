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
	refresh     func(context.Context) error
	clear       func() error
	warmSprites func(context.Context) error
	update      func(context.Context) error
}

func (s stubCacheCommands) Refresh(ctx context.Context) error {
	if s.refresh != nil {
		return s.refresh(ctx)
	}
	return nil
}

func (s stubCacheCommands) Clear() error {
	if s.clear != nil {
		return s.clear()
	}
	return nil
}

func (s stubCacheCommands) WarmSprites(ctx context.Context) error {
	if s.warmSprites != nil {
		return s.warmSprites(ctx)
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
		Cache: stubCacheCommands{
			refresh:     func(context.Context) error { refreshed++; return nil },
			clear:       func() error { cleared++; return nil },
			warmSprites: func(context.Context) error { return nil },
			update:      func(context.Context) error { refreshed++; return nil },
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
		t.Fatalf("refreshed=%d cleared=%d want update+refresh=2", refreshed, cleared)
	}
	if !strings.Contains(stdout.String(), "cache") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
