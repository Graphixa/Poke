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
	clear   func() error
	refresh func(context.Context) error
}

func (s stubCacheCommands) Clear() error {
	if s.clear != nil {
		return s.clear()
	}
	return nil
}

func (s stubCacheCommands) Refresh(ctx context.Context) error {
	if s.refresh != nil {
		return s.refresh(ctx)
	}
	return nil
}

type stubUpdater struct {
	check  func(context.Context) (string, string, bool, error)
	apply  func(context.Context) error
	applyV func(context.Context, string) error
	record func(string, string) error
	hours  int
}

func (s stubUpdater) Check(ctx context.Context) (string, string, bool, error) {
	if s.check != nil {
		return s.check(ctx)
	}
	return "1.0.0", "1.0.0", false, nil
}
func (s stubUpdater) ApplyLatest(ctx context.Context) error {
	if s.apply != nil {
		return s.apply(ctx)
	}
	return nil
}
func (s stubUpdater) ApplyVersion(ctx context.Context, version string) error {
	if s.applyV != nil {
		return s.applyV(ctx, version)
	}
	return nil
}
func (s stubUpdater) RecordCheck(last, next string) error {
	if s.record != nil {
		return s.record(last, next)
	}
	return nil
}
func (s stubUpdater) CheckIntervalHours() int {
	if s.hours > 0 {
		return s.hours
	}
	return 24
}

func TestRefreshAndCacheCommands(t *testing.T) {
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
			clear:   func() error { cleared++; return nil },
			refresh: func(context.Context) error { refreshed++; return nil },
		},
	})

	root.SetArgs([]string{"refresh"})
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
	if refreshed != 1 || cleared != 1 {
		t.Fatalf("refreshed=%d cleared=%d want 1,1", refreshed, cleared)
	}
	if !strings.Contains(stdout.String(), "cache") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestUpdateCheckOnly(t *testing.T) {
	stdout := &bytes.Buffer{}
	root := cli.NewRoot(cli.Options{
		InvocationPath: "poke",
		Paths: config.Paths{
			ConfigFile: filepath.Join(t.TempDir(), "config.yaml"),
			CacheDir:   filepath.Join(t.TempDir(), "cache"),
		},
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
		Updater: stubUpdater{
			check: func(context.Context) (string, string, bool, error) {
				return "1.0.0", "1.1.0", true, nil
			},
		},
	})
	root.SetArgs([]string{"update", "--check"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Available: v1.1.0") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
