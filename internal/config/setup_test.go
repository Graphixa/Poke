package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Graphixa/Pokedex/internal/config"
)

func TestNeedsSetupAndArtworkConsent(t *testing.T) {
	cfg := config.Default()
	if !cfg.NeedsSetup() {
		t.Fatal("default config should need setup")
	}
	cfg.ApplyArtworkConsent(config.ArtworkConsentAccepted, config.ArtworkModeBlocking)
	if cfg.EffectiveArtworkConsent() != config.ArtworkConsentAccepted {
		t.Fatalf("consent = %q", cfg.EffectiveArtworkConsent())
	}
	if cfg.EffectiveArtworkMode() != config.ArtworkModeBlocking {
		t.Fatalf("mode = %q", cfg.EffectiveArtworkMode())
	}
	if !cfg.ArtworkWarmEnabled() {
		t.Fatal("accepted blocking consent should enable artwork warm")
	}
	cfg.MarkSetupComplete()
	if cfg.NeedsSetup() {
		t.Fatal("completed setup should not need setup")
	}

	cfg.ApplyArtworkConsent(config.ArtworkConsentDeclined, config.ArtworkModeOff)
	if cfg.ArtworkWarmEnabled() {
		t.Fatal("declined consent should disable artwork warm")
	}
	if cfg.EffectiveArtworkMode() != config.ArtworkModeOff {
		t.Fatalf("mode = %q", cfg.EffectiveArtworkMode())
	}
}

func TestSetupRoundTripYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := config.Default()
	cfg.ApplyArtworkConsent(config.ArtworkConsentAccepted, config.ArtworkModeBackground)
	cfg.MarkSetupComplete()
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.NeedsSetup() {
		t.Fatal("loaded config should be completed")
	}
	if loaded.EffectiveArtworkMode() != config.ArtworkModeBackground {
		t.Fatalf("mode = %q", loaded.EffectiveArtworkMode())
	}
	if !loaded.ArtworkWarmEnabled() {
		t.Fatal("background mode should enable artwork warm")
	}
	data, _ := os.ReadFile(path)
	if len(data) == 0 {
		t.Fatal("empty config file")
	}
	raw := string(data)
	if strings.Contains(raw, "sprite_hd") || strings.Contains(raw, "show_sprite") {
		t.Fatalf("legacy knobs should not be written: %s", raw)
	}
}
