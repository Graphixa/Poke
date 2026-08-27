package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Graphixa/Pokedex/internal/config"
)

func TestResetClearsCacheAndConfig(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	cacheDir := filepath.Join(root, "cache")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.MarkSetupComplete()
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(cacheDir, "keep-me.txt")
	if err := os.WriteFile(marker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	paths := config.Paths{ConfigFile: cfgPath, CacheDir: cacheDir}
	var stdout strings.Builder
	cmds := cacheCommands{
		services: func() (*Services, error) {
			return BuildServices(paths, cfg, nil, nil)
		},
		stdout: &stdout,
	}
	if err := cmds.Reset(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Fatalf("config should be removed, err=%v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("cache marker should be gone, err=%v", err)
	}
	if !strings.Contains(stdout.String(), "Run poke to start setup") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
