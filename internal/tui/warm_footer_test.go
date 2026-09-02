package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Pokedex/internal/tui"
	"github.com/Graphixa/Pokedex/internal/tui/components"
)

func TestFooterSplitFixedWidthRight(t *testing.T) {
	left := components.FooterSplit(false, 80, "")
	if len(left) < 40 {
		t.Fatalf("footer too short: %q", left)
	}
	withBar := components.FooterSplit(false, 80, "Artwork ██████░░░░ 5/10")
	if withBar == left {
		t.Fatal("expected different footer with artwork progress")
	}
}

func TestFormatNativeCacheBarVisible(t *testing.T) {
	got := tui.FormatNativeCacheBarForTest(36, 5, 10)
	if got == "" {
		t.Fatal("native warm must show progress bar while incomplete")
	}
	if !strings.Contains(got, "Image Cache") || !strings.Contains(got, "5/10") {
		t.Fatalf("unexpected bar: %q", got)
	}
}

func TestImageCacheFooterVisibleForNativeWarm(t *testing.T) {
	got := tui.FormatNativeCacheBarForTest(40, 0, 1025)
	if got == "" {
		t.Fatal("expected visible bar for incomplete native warm")
	}
	if !strings.Contains(got, "0/1025") {
		t.Fatalf("unexpected bar: %q", got)
	}
	if strings.Contains(got, "·") {
		t.Fatalf("bucket size must not appear in footer: %q", got)
	}
}

func TestImageCacheFooterHiddenWhenComplete(t *testing.T) {
	if got := tui.FormatNativeCacheBarForTest(40, 8, 8); got != "" {
		t.Fatalf("expected hidden bar, got %q", got)
	}
}

func TestNativeCacheBarFitsFooterWidth(t *testing.T) {
	got := tui.FormatNativeCacheBarForTest(40, 432, 1025)
	if got == "" {
		t.Fatal("native bar must be visible while incomplete")
	}
	if w := lipgloss.Width(got); w > components.FooterRightWidth {
		t.Fatalf("bar width=%d max=%d", w, components.FooterRightWidth)
	}
}

func TestArtworkWarmDoesNotShowBrowseFooter(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{ansi: "ART"}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)

	updated, _ = m.Update(tui.WarmProgressSpeciesStageMsgForTest(131, 1025, 0, "artwork"))
	m = updated.(tui.Model)
	if hint := m.BrowseWarmHintForTest(); hint != "" {
		t.Fatalf("artwork warm must not populate footer, got %q", hint)
	}
	if vis := m.BrowseVisibleWarmHintForTest(); vis != "" {
		t.Fatalf("artwork warm must not render in browse footer, got %q", vis)
	}
}
