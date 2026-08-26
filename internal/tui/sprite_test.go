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
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	if cmd == nil {
		cmd = m.EnsureSelectionLoadsForTest()
	}
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
	updated, cmd = m.Update(tea.WindowSizeMsg{Width: 140, Height: 56})
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
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	if cmd == nil {
		cmd = m.EnsureSelectionLoadsForTest()
	}
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

func TestStaleDossierClearsLoadingSoReselectRetries(t *testing.T) {
	details := &fakeDetails{blockLoad: make(chan struct{})}
	m := tui.NewModel(tui.Options{
		Index:    fixtureIndex(),
		Details:  details,
		Selected: &domain.SpeciesSummary{ID: 4, Slug: "charmander", DisplayName: "Charmander"},
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)

	updated, cmd := m.Update(key("enter"))
	m = updated.(tui.Model)
	if cmd == nil {
		cmd = m.EnsureDossierLoadForTest()
	}
	if cmd == nil {
		t.Fatal("expected dossier load cmd")
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()

	// Close dialog and move away before load finishes.
	updated, _ = m.Update(key("esc"))
	m = updated.(tui.Model)
	updated, _ = m.Update(key("down"))
	m = updated.(tui.Model)
	if id, _ := m.SelectedID(); id == 4 {
		t.Fatal("expected selection to move off charmander")
	}

	close(details.blockLoad)
	msg := <-done
	updated, _ = m.Update(msg)
	m = updated.(tui.Model)
	if m.CoreLoading(4) {
		t.Fatal("stale dossier must clear loadingCore")
	}

	updated, _ = m.Update(key("up"))
	m = updated.(tui.Model)
	updated, cmd = m.Update(key("enter"))
	m = updated.(tui.Model)
	if cmd == nil {
		cmd = m.EnsureDossierLoadForTest()
	}
	if cmd == nil && m.ResourceName(4) == "" && !m.CoreLoading(4) {
		t.Fatal("reselect enter must retry dossier load")
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
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)

	updated, _ = m.Update(key("p"))
	m = updated.(tui.Model)
	if m.Query() != "p" {
		t.Fatalf("query = %q", m.Query())
	}
	close(sprites.block)
}
