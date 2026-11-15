package config

import (
	"os"
	"path/filepath"
	"reflect"
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
	if cfg.Network.MaxConcurrency != 8 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.Cache.DossierPrewarm != "window" {
		t.Fatalf("dossier_prewarm = %q want window", cfg.Cache.DossierPrewarm)
	}
	if cfg.Render.SpritePrewarm != "window" {
		t.Fatalf("sprite_prewarm = %q", cfg.Render.SpritePrewarm)
	}
	if cfg.Render.SpriteSource != "artwork" {
		t.Fatalf("sprite_source = %q want artwork", cfg.Render.SpriteSource)
	}
	if cfg.Render.SixelColors != DefaultSixelColors {
		t.Fatalf("sixel_colors = %d want %d", cfg.Render.SixelColors, DefaultSixelColors)
	}
	if cfg.EffectiveSixelColors() != DefaultSixelColors {
		t.Fatalf("effective sixel_colors = %d want %d", cfg.EffectiveSixelColors(), DefaultSixelColors)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config is invalid: %v", err)
	}
}

func TestEffectiveSixelColors(t *testing.T) {
	cases := map[int]int{
		0: 64, 64: 64, 128: 128, 256: 256,
		254: 64, 65: 64, 512: 64, -1: 64,
	}
	for in, want := range cases {
		cfg := Default()
		cfg.Render.SixelColors = in
		if got := cfg.EffectiveSixelColors(); got != want {
			t.Fatalf("SixelColors=%d => %d want %d", in, got, want)
		}
	}
}

func TestLoadInvalidSixelColorsStillSucceeds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte("render:\n  sixel_colors: 999\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(path)
	if err != nil {
		t.Fatalf("Load() err=%v want success with fallback", err)
	}
	if cfg.EffectiveSixelColors() != DefaultSixelColors {
		t.Fatalf("effective=%d want %d", cfg.EffectiveSixelColors(), DefaultSixelColors)
	}
}

func TestLoadOverlaysDefaultsAndIgnoresUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte("network:\n  retries: 1\nunknown_section:\n  enabled: true\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := Load(path)
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
			if _, _, err := Load(path); err == nil {
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
	if _, _, err := Load(path); err != nil {
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

func TestEditorWaits(t *testing.T) {
	if !editorWaits([]string{"vim"}) {
		t.Fatal("vim should wait")
	}
	if !editorWaits([]string{"code", "--wait"}) {
		t.Fatal("code --wait should wait")
	}
	if editorWaits([]string{"notepad"}) {
		t.Fatal("notepad should not wait")
	}
	if editorWaits([]string{"code"}) {
		t.Fatal("code without --wait should not wait")
	}
	if editorWaits([]string{"cursor.exe"}) {
		t.Fatal("cursor should not wait")
	}
}

func TestLoadRejectsLegacySpriteModes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("render:\n  sprite_mode: braille\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(path); err == nil {
		t.Fatal("expected reject of legacy sprite_mode")
	}
}

func TestEditInitializesConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if _, err := Initialize(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config was not initialized: %v", err)
	}
}
