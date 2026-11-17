package app

import (
	"context"
	"path/filepath"

	"github.com/Graphixa/Poke/internal/config"
	"github.com/Graphixa/Poke/internal/theme"
	"github.com/Graphixa/Poke/internal/tui"
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
		current = "dracula"
	}
	err = tui.RunThemePicker(tui.ThemePickerOptions{
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
	// Bubble Tea mode reports (DECRPM ?2026/?2027) can land after the picker
	// exits; leave them for the next poke and they type into browse search.
	drainPendingStdin()
	return err
}
