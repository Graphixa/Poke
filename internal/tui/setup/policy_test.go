package setup

import "testing"

func TestNeedsModePrompt(t *testing.T) {
	if needsModePrompt(artworkSizeThreshold - 1) {
		t.Fatal("under threshold should not prompt")
	}
	if !needsModePrompt(artworkSizeThreshold) {
		t.Fatal("at threshold should prompt")
	}
}

func TestFormatBytes(t *testing.T) {
	if got := formatBytes(100 * 1024 * 1024); got != "100.0 MB" {
		t.Fatalf("formatBytes() = %q, want %q", got, "100.0 MB")
	}
}
