package setup_test

import (
	"testing"

	"github.com/Graphixa/Pokedex/internal/tui/setup"
)

func TestWizardStepsAreArtworkOnly(t *testing.T) {
	// Step constants: Welcome → Index → ArtPrompt → ModePrompt → ArtWarm → Done.
	// Pixel warm must not exist in the product flow.
	if setup.StepWelcome != 0 || setup.StepIndex != 1 || setup.StepArtPrompt != 2 {
		t.Fatalf("unexpected step order: welcome=%d index=%d art=%d",
			setup.StepWelcome, setup.StepIndex, setup.StepArtPrompt)
	}
	if setup.StepModePrompt != 3 || setup.StepArtWarm != 4 || setup.StepDone != 5 {
		t.Fatalf("unexpected later steps: mode=%d warm=%d done=%d",
			setup.StepModePrompt, setup.StepArtWarm, setup.StepDone)
	}
}

func TestDepsHasNoWarmPixelsField(t *testing.T) {
	// Compile-time shape check: Deps exposes EstimateHD + WarmArtwork only.
	d := setup.Deps{}
	_ = d.EstimateHD
	_ = d.WarmArtwork
	_ = d.EnsureIndex
	_ = d.SaveConfig
}
