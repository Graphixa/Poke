package app

import (
	"context"
	"path/filepath"

	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/theme"
	"github.com/Graphixa/Pokedex/internal/themeui"
)

// Theme launches the interactive theme selector (FontGet-style).
func (h *Handler) Theme(ctx context.Context) error {
	_ = ctx
	svc, err := h.Services()
	if err != nil {
		return err
	}
	ApplyTheme(h.Paths, svc.Config)
	themesDir := theme.ThemesDir(filepath.Dir(h.Paths.ConfigFile))
	current := svc.Config.Theme.Name
	if current == "" {
		current = "catppuccin"
	}
	return themeui.Run(themeui.Options{
		Current:   current,
		ThemesDir: themesDir,
		Save: func(name string) error {
			if err := config.SetThemeName(h.Paths.ConfigFile, name); err != nil {
				return err
			}
			// Refresh lazy services cache by reloading next time; update in-memory if possible.
			cfg, _, loadErr := config.Load(h.Paths.ConfigFile)
			if loadErr == nil {
				ApplyTheme(h.Paths, cfg)
			}
			return nil
		},
	})
}
