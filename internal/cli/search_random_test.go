package cli_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/Graphixa/Poke/internal/cli"
	"github.com/Graphixa/Poke/internal/config"
)

type searchRandomStub struct {
	searchQ    string
	randomMode cli.InlineMode
}

func (s *searchRandomStub) RunRoot(context.Context, cli.RootRequest) error { return nil }

func (s *searchRandomStub) Search(_ context.Context, q string) error {
	s.searchQ = q
	return nil
}

func (s *searchRandomStub) Random(_ context.Context, mode cli.InlineMode) error {
	s.randomMode = mode
	return nil
}

func (s *searchRandomStub) Theme(context.Context) error { return nil }

func TestSearchAndRandomRouting(t *testing.T) {
	stub := &searchRandomStub{}
	stdout := &bytes.Buffer{}
	root := cli.NewRoot(cli.Options{
		InvocationPath: "poke",
		Paths: config.Paths{
			ConfigFile: filepath.Join(t.TempDir(), "config.yaml"),
			CacheDir:   filepath.Join(t.TempDir(), "cache"),
		},
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
		App:    stub,
	})

	root.SetArgs([]string{"search", "char"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if stub.searchQ != "char" {
		t.Fatalf("search query = %q", stub.searchQ)
	}

	root.SetArgs([]string{"random", "--stats"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if stub.randomMode != cli.InlineStats {
		t.Fatalf("random mode = %q", stub.randomMode)
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
		App:    &searchRandomStub{},
	})
	root.SetArgs([]string{"random", "--info", "--stats"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected mutually exclusive error")
	}
}
