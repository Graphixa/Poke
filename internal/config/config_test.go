package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Cache.IndexTTL.Duration != 24*time.Hour {
		t.Fatalf("index TTL = %v", cfg.Cache.IndexTTL.Duration)
	}
	if cfg.Network.Timeout.Duration != 10*time.Second {
		t.Fatalf("network timeout = %v", cfg.Network.Timeout.Duration)
	}
	if cfg.Network.MaxConcurrency != 4 || cfg.Data.Language != "en" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config is invalid: %v", err)
	}
}

func TestLoadOverlaysDefaultsAndIgnoresUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte("network:\n  retries: 1\nunknown_section:\n  enabled: true\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Network.Retries != 1 {
		t.Fatalf("retries = %d", cfg.Network.Retries)
	}
	if cfg.Network.Timeout.Duration != 10*time.Second {
		t.Fatalf("unspecified timeout was not preserved: %v", cfg.Network.Timeout.Duration)
	}
}

func TestLoadRejectsInvalidYAMLAndValues(t *testing.T) {
	tests := map[string]string{
		"yaml":     "network: [",
		"duration": "network:\n  timeout: someday\n",
		"value":    "network:\n  max_concurrency: 0\n",
	}
	for name, contents := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("Load() error = nil")
			}
		})
	}
}

func TestResolvePaths(t *testing.T) {
	resolver := DirectoryResolver{
		UserConfigDir: func() (string, error) { return filepath.Join("home", "config"), nil },
		UserCacheDir:  func() (string, error) { return filepath.Join("home", "cache"), nil },
	}
	paths, err := resolver.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("home", "config", "pokedex", "config.yaml"); paths.ConfigFile != want {
		t.Fatalf("config path = %q, want %q", paths.ConfigFile, want)
	}
	if want := filepath.Join("home", "cache", "pokedex"); paths.CacheDir != want {
		t.Fatalf("cache path = %q, want %q", paths.CacheDir, want)
	}
}

func TestInitializeIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	created, err := Initialize(path)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("first Initialize() did not create the file")
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("initialized config cannot be loaded: %v", err)
	}
	created, err = Initialize(path)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("second Initialize() reported creation")
	}
	again, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, again) {
		t.Fatal("idempotent initialization changed the file")
	}
}

func TestSelectEditor(t *testing.T) {
	values := map[string]string{"VISUAL": `"C:\Program Files\Code\code.exe" --wait`, "EDITOR": "vim"}
	getenv := func(name string) string { return values[name] }
	if got, want := SelectEditor(getenv, "windows"), []string{`C:\Program Files\Code\code.exe`, "--wait"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("SelectEditor() = %#v, want %#v", got, want)
	}
	delete(values, "VISUAL")
	if got := SelectEditor(getenv, "linux"); !reflect.DeepEqual(got, []string{"vim"}) {
		t.Fatalf("EDITOR selection = %#v", got)
	}
	delete(values, "EDITOR")
	if got := SelectEditor(getenv, "windows"); !reflect.DeepEqual(got, []string{"notepad"}) {
		t.Fatalf("Windows fallback = %#v", got)
	}
}

type recordingRunner struct {
	name string
	args []string
}

func (r *recordingRunner) Run(name string, args ...string) error {
	r.name = name
	r.args = append([]string(nil), args...)
	return nil
}

func TestEditInitializesAndLaunchesSelectedEditor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	runner := &recordingRunner{}
	getenv := func(name string) string {
		if name == "VISUAL" {
			return "code --wait"
		}
		return ""
	}
	if err := Edit(path, getenv, runner); err != nil {
		t.Fatal(err)
	}
	if runner.name != "code" || strings.Join(runner.args, "|") != "--wait|"+path {
		t.Fatalf("runner call = %q %#v", runner.name, runner.args)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config was not initialized: %v", err)
	}
}
