package setup_test

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Poke/internal/config"
	"github.com/Graphixa/Poke/internal/tui/components"
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
	if !strings.Contains(view, "Pick a theme") || !strings.Contains(view, "Poke - Initial Setup") || setupIntro(view) {
		t.Fatalf("theme step:\n%s", view)
	}
	m = press(t, m, tea.KeyEnter)
	view = m.View().Content
	if !strings.Contains(view, "Check for updates when you open Poke?") || setupIntro(view) {
		t.Fatalf("updates step:\n%s", view)
	}
	m = press(t, m, tea.KeyEnter)
	view = m.View().Content
	ok := components.ViewSwitcher(0, []string{"OK"}, 0)
	if !setupIntro(view) || !strings.Contains(view, ok) {
		t.Fatalf("intro step:\n%s", view)
	}
	if saved.Setup.Completed {
		t.Fatal("saved before OK")
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
	// Work has not reported done yet. OK is the screen before the preloader.
	m = press(t, m, tea.KeyEnter)
	m = press(t, m, tea.KeyEnter)
	view := m.View().Content
	if !setupIntro(view) || strings.Contains(view, "Check for updates when you open Poke?") {
		t.Fatalf("expected the intro, got:\n%s", view)
	}
	m = press(t, m, tea.KeyEnter)
	view = m.View().Content
	if strings.Contains(view, "Check for updates when you open Poke?") || strings.Contains(view, "Downloading in the background") || setupIntro(view) {
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
	if !strings.Contains(view, "Poke - Initial Setup") {
		t.Fatalf("setup loading frame missing:\n%s", view)
	}
	if strings.Contains(view, " of ") {
		t.Fatalf("count line on setup loading view:\n%s", view)
	}
}

func TestRefreshScreenSkipsQuestions(t *testing.T) {
	m := setup.New(setup.Deps{Config: config.Default(), Ask: false})
	view := m.View().Content
	if !strings.Contains(view, "Refreshing Pokédex") || strings.Contains(view, "Poke - Initial Setup") || setupIntro(view) {
		t.Fatalf("refresh screen:\n%s", view)
	}
	if strings.Contains(view, "Pick a theme") || strings.Contains(view, " of ") {
		t.Fatalf("refresh screen still has questions or a count:\n%s", view)
	}
	var joke bool
	for _, line := range preload.Lines {
		if strings.Contains(view, line) {
			joke = true
			break
		}
	}
	if !joke {
		t.Fatalf("refresh screen missing a loading line:\n%s", view)
	}
}

func TestStageLabelAndYesBeforeNo(t *testing.T) {
	if got := setup.StageLabel("records"); got != "Gathering Pokémon records" {
		t.Fatalf("records label %q", got)
	}
	if got := setup.StageLabel("import"); got != "Importing Pokémon data" {
		t.Fatalf("import label %q", got)
	}
	if got := setup.StageLabel("artwork"); got != "Downloading Pokémon artwork" {
		t.Fatalf("artwork label %q", got)
	}
	m := setup.New(setup.Deps{Config: config.Default(), Ask: true})
	m = press(t, m, tea.KeyEnter)
	view := m.View().Content
	want := components.ViewSwitcher(1, []string{"Yes", "No"}, 0)
	if !strings.Contains(view, want) {
		t.Fatalf("want Yes before No, No selected:\n%s", view)
	}
}

func setupIntro(view string) bool {
	return strings.Contains(view, "import the Pokémon data") && strings.Contains(view, "won't take long")
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
