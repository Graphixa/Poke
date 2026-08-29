package tui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/tui"
)

func TestSpritePrefetchFillsNeighbors(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{ansi: "PRE"}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)

	cmd := m.ScheduleSpritePrefetchForTest()
	if cmd == nil {
		t.Fatal("expected prefetch cmds for neighbors")
	}
	var apply func(tea.Cmd)
	apply = func(c tea.Cmd) {
		if c == nil {
			return
		}
		msg := c()
		switch v := msg.(type) {
		case tea.BatchMsg:
			for _, inner := range v {
				apply(inner)
			}
		default:
			updated, follow := m.Update(msg)
			m = updated.(tui.Model)
			_ = follow
		}
	}
	apply(cmd)

	if sprites.calls.Load() < 1 {
		t.Fatal("expected prefetch to call Render")
	}
}

func TestSpritePrefetchStaleGenDropped(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{ansi: "STALE"}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)

	cmd := m.ScheduleSpritePrefetchForTest()
	if cmd == nil {
		t.Fatal("expected prefetch")
	}
	gen := m.PrefetchGenForTest()
	m.BumpPrefetchGenForTest()
	if m.PrefetchGenForTest() <= gen {
		t.Fatal("expected prefetch gen bump")
	}

	// Deliver results from the cancelled generation — must not panic / must drop.
	msg := cmd()
	switch v := msg.(type) {
	case tea.BatchMsg:
		for _, inner := range v {
			if inner == nil {
				continue
			}
			updated, _ = m.Update(inner())
			m = updated.(tui.Model)
		}
	default:
		updated, _ = m.Update(msg)
		m = updated.(tui.Model)
	}
}

func TestEnsureSpriteLoadTakesOverPrefetchLoading(t *testing.T) {
	// Fast-scroll regression: neighbor had loadingSprite from prefetch; landing
	// on it must start a selection encode (not skip and leave a sticky placeholder).
	details := &fakeDetails{}
	sprites := &fakeSprites{ansi: "ART", block: make(chan struct{})}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)

	firstID, _ := m.SelectedID()
	cmd := m.ScheduleSpritePrefetchForTest()
	if cmd == nil {
		t.Fatal("expected prefetch for neighbors")
	}
	// Drain prefetch batch without waiting on blocked Render (don't run cmds).
	_ = cmd

	// Move to next row — syncSelectionLoads used to skip when loadingSprite was set
	// by prefetch, then cancelPrefetch abandoned that work.
	updated, _ = m.Update(key("down"))
	m = updated.(tui.Model)
	nextID, _ := m.SelectedID()
	if nextID == firstID {
		t.Fatal("expected cursor to move")
	}
	if !m.SelectedSpriteLoadingForTest() {
		t.Fatal("expected selection sprite load in flight after prefetch takeover")
	}
	close(sprites.block)
}
