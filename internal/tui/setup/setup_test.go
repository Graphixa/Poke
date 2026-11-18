package setup_test

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Poke/internal/config"
	"github.com/Graphixa/Poke/internal/tui/preload"
	"github.com/Graphixa/Poke/internal/tui/setup"
)

func TestQuestionsSaveThemeAndSkipDownloadWhenIdle(t *testing.T) {
	var saved config.Config
	m := setup.New(setup.Deps{
		Config: config.Default(),
		Ask:    true,
		SaveConfig: func(c config.Config) error {
			saved = c
			return nil
		},
	})
	view := m.View().Content
	if !strings.Contains(view, "Pick a theme") {
		t.Fatalf("theme step missing:\n%s", view)
	}
	m = press(t, m, tea.KeyEnter)
	view = m.View().Content
	if !strings.Contains(view, "Check for updates when you open Poke?") {
		t.Fatalf("updates step missing:\n%s", view)
	}
	m = press(t, m, tea.KeyEnter)
	if !saved.Setup.Completed {
		t.Fatal("setup not marked complete")
	}
	if saved.Update.CheckForUpdates {
		t.Fatal("default answer is no")
	}
	if saved.Theme.Name == "" {
		t.Fatal("theme not saved")
	}
	if saved.Setup.ArtworkMode != config.ArtworkModeBackground {
		t.Fatalf("artwork mode %q", saved.Setup.ArtworkMode)
	}
	if !m.Outcome().StartBackgroundArtwork || m.Outcome().Aborted {
		t.Fatal("expected a finished setup")
	}
}

func TestFastAnswersWaitForDownload(t *testing.T) {
	m := setup.New(setup.Deps{
		Config: config.Default(),
		Ask:    true,
		RunWork: func(context.Context, setup.ProgressFunc) error {
			return nil
		},
	})
	// Work has not reported done yet, so the last Enter shows the preloader.
	m = press(t, m, tea.KeyEnter)
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(setup.Model)
	view := m.View().Content
	if strings.Contains(view, "Check for updates when you open Poke?") || strings.Contains(view, "Downloading in the background") {
		t.Fatalf("expected preloader, got:\n%s", view)
	}
	var joke bool
	for _, line := range preload.Lines {
		if strings.Contains(view, line) {
			joke = true
			break
		}
	}
	if !joke {
		t.Fatalf("expected a loading line, got:\n%s", view)
	}
}

func press(t *testing.T, m setup.Model, code rune) setup.Model {
	t.Helper()
	next, _ := m.Update(tea.KeyPressMsg{Code: code})
	out, ok := next.(setup.Model)
	if !ok {
		t.Fatalf("model type %T", next)
	}
	return out
}
