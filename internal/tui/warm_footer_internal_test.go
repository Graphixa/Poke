package tui

import "testing"

func TestWarmFooterShowsNativeCacheProgress(t *testing.T) {
	got := formatWarmFooterRight(WarmStageNative, 304, 1025, 40)
	if got == "" {
		t.Fatal("native cache progress should be visible")
	}
}

func TestWarmFooterHidesArtworkDownloadProgress(t *testing.T) {
	got := formatWarmFooterRight(WarmStageArtwork, 304, 1025, 0)
	if got != "" {
		t.Fatalf("artwork progress should be hidden, got %q", got)
	}
}

func TestWarmFooterHidesNonCacheWarmStages(t *testing.T) {
	stages := []WarmStage{WarmStageSpritesRender, WarmStageDossier, "", "warming"}

	for _, stage := range stages {
		if got := formatWarmFooterRight(stage, 1, 10, 0); got != "" {
			t.Errorf("stage %q should be hidden, got %q", stage, got)
		}
	}
}

func TestWarmFooterHidesCompletedNativeCache(t *testing.T) {
	if got := formatWarmFooterRight(WarmStageNative, 1025, 1025, 40); got != "" {
		t.Fatalf("completed progress should be hidden, got %q", got)
	}
}
