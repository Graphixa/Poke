package tui_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/tui"
)

type fakeSprites struct {
	calls   atomic.Int32
	block   chan struct{}
	ansi    string
	started chan struct{}
}

func (f *fakeSprites) Render(ctx context.Context, _ domain.Pokemon, _ int) (string, bool) {
	f.calls.Add(1)
	if f.started != nil {
		select {
		case f.started <- struct{}{}:
		default:
		}
	}
	if f.block != nil {
		select {
		case <-ctx.Done():
			return "", true
		case <-f.block:
		}
	}
	if f.ansi == "" {
		return "SPRITE", false
	}
	return f.ansi, false
}

func (f *fakeSprites) ClearNative() {}

func TestSpriteStaleSuppressedAndResizeRace(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{ansi: "ART"}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(tui.Model)

	core := tui.CoreMsgForTest(m.SelectionGen(), 4, domain.Pokemon{
		ID: 4, DisplayName: "Charmander",
		Sprites: domain.Sprites{FrontDefault: "https://example/c.png"},
	}, domain.Species{ID: 4, DisplayName: "Charmander"}, nil)
	updated, cmd := m.Update(core)
	m = updated.(tui.Model)
	if cmd == nil {
		t.Fatal("expected sprite load cmd")
	}
	reqBefore := m.SpriteReqID()

	stale := tui.SpriteMsgForTest(reqBefore-1, m.SelectionGen(), 4, m.CurrentSpriteCells(), "STALE", false)
	updated, _ = m.Update(stale)
	m = updated.(tui.Model)
	if m.HasSpriteANSI() {
		t.Fatal("stale sprite should be suppressed")
	}

	cells := m.CurrentSpriteCells()
	fresh := tui.SpriteMsgForTest(m.SpriteReqID(), m.SelectionGen(), 4, cells, "ART", false)
	updated, _ = m.Update(fresh)
	m = updated.(tui.Model)
	if !m.HasSpriteANSI() {
		t.Fatal("expected sprite ansi")
	}

	// Hide sprite panel: must clear art and not leave ghost state.
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m = updated.(tui.Model)
	if m.LayoutFlags().ShowSprite {
		t.Fatal("expected sprite hidden below threshold")
	}
	if m.HasSpriteANSI() {
		t.Fatal("hide should clear sprite")
	}

	// Show again at a different size — prior art must not reappear until a matching render.
	updated, cmd = m.Update(tea.WindowSizeMsg{Width: 140, Height: 48})
	m = updated.(tui.Model)
	if !m.LayoutFlags().ShowSprite {
		t.Fatal("expected sprite visible again")
	}
	if m.HasSpriteANSI() {
		t.Fatal("reflow must not keep stale ansi")
	}
	if cmd == nil {
		t.Fatal("expected reflow sprite load")
	}
	wrongCells := tui.SpriteMsgForTest(m.SpriteReqID(), m.SelectionGen(), 4, cells+3, "OLDSIZE", false)
	updated, _ = m.Update(wrongCells)
	m = updated.(tui.Model)
	if m.HasSpriteANSI() {
		t.Fatal("mismatched cell size must be ignored")
	}
}

func TestSpriteCancelOnSelectionChange(t *testing.T) {
	details := &fakeDetails{}
	started := make(chan struct{}, 1)
	block := make(chan struct{})
	sprites := &fakeSprites{block: block, started: started, ansi: "A"}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(tui.Model)

	core := tui.CoreMsgForTest(m.SelectionGen(), 4, domain.Pokemon{
		ID: 4, DisplayName: "Charmander",
		Sprites: domain.Sprites{FrontDefault: "https://example/c.png"},
	}, domain.Species{ID: 4}, nil)
	updated, cmd := m.Update(core)
	m = updated.(tui.Model)
	if cmd == nil {
		t.Fatal("expected sprite cmd")
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("sprite render did not start")
	}

	updated, _ = m.Update(key("down"))
	m = updated.(tui.Model)

	close(block)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("blocked render did not finish after cancel/unblock")
	}

	old := tui.SpriteMsgForTest(0, 0, 4, m.CurrentSpriteCells(), "OLD", false)
	updated, _ = m.Update(old)
	m = updated.(tui.Model)
	if m.HasSpriteANSI() {
		t.Fatal("cancelled/stale sprite must not apply")
	}
}

func TestTypingDoesNotBlockOnSprite(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{block: make(chan struct{})}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(tui.Model)

	updated, _ = m.Update(key("p"))
	m = updated.(tui.Model)
	if m.Query() != "p" {
		t.Fatalf("query = %q", m.Query())
	}
	close(sprites.block)
}
