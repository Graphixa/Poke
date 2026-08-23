package cli_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/Graphixa/Pokedex/internal/cli"
	"github.com/Graphixa/Pokedex/internal/config"
)

func TestSearchAndRandomRouting(t *testing.T) {
	var searchQ string
	var randomMode cli.InlineMode
	stdout := &bytes.Buffer{}
	root := cli.NewRoot(cli.Options{
		InvocationPath: "poke",
		Paths: config.Paths{
			ConfigFile: filepath.Join(t.TempDir(), "config.yaml"),
			CacheDir:   filepath.Join(t.TempDir(), "cache"),
		},
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
		Search: cli.SearchHandlerFunc(func(_ context.Context, q string) error {
			searchQ = q
			return nil
		}),
		Random: cli.RandomHandlerFunc(func(_ context.Context, mode cli.InlineMode) error {
			randomMode = mode
			return nil
		}),
	})

	root.SetArgs([]string{"search", "char"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if searchQ != "char" {
		t.Fatalf("search query = %q", searchQ)
	}

	root.SetArgs([]string{"random", "--stats"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if randomMode != cli.InlineStats {
		t.Fatalf("random mode = %q", randomMode)
	}
}

func TestRandomMutuallyExclusiveFlags(t *testing.T) {
	root := cli.NewRoot(cli.Options{
		InvocationPath: "poke",
		Paths: config.Paths{
			ConfigFile: filepath.Join(t.TempDir(), "config.yaml"),
			CacheDir:   filepath.Join(t.TempDir(), "cache"),
		},
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Random: cli.RandomHandlerFunc(func(context.Context, cli.InlineMode) error { return nil }),
	})
	root.SetArgs([]string{"random", "--info", "--stats"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected mutually exclusive error")
	}
}
