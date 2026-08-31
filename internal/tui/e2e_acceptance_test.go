package tui_test

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/tui"
)

type e2eSprites struct {
	lookupHit bool
	coldCalls int
	ansi      string
}

func (e *e2eSprites) Render(_ context.Context, _ domain.Pokemon, _ int) (string, bool, bool, string) {
	return e.ansi, false, e.lookupHit, "pixel"
}

func (e *e2eSprites) LookupCached(_ context.Context, _ domain.Pokemon, _ int) (sprite.RenderResult, bool) {
	if e.lookupHit {
		return sprite.RenderResult{ANSI: e.ansi, Native: true}, true
	}
	return sprite.RenderResult{}, false
}

func (e *e2eSprites) RenderCold(_ context.Context, _ domain.Pokemon, _ int) sprite.RenderResult {
	e.coldCalls++
	return sprite.RenderResult{ANSI: e.ansi, Native: true}
}

func (e *e2eSprites) ClearNative()                                       {}
func (e *e2eSprites) InvalidateSpecies(int)                              {}
func (e *e2eSprites) ClearNativeRegion(int, int) string                  { return "" }
func (e *e2eSprites) PersistFrame(context.Context, string, string) error { return nil }

func TestE2ECachedSelectionNoColdRender(t *testing.T) {
	sprites := &e2eSprites{lookupHit: true, ansi: "CACHED"}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: &fakeDetails{},
		Sprites: sprites,
		Context: context.Background(),
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(tui.Model)
	if sprites.coldCalls != 0 {
		t.Fatalf("cached selection must not cold-render, calls=%d", sprites.coldCalls)
	}
}

func TestE2ESelectionColdUsesLookupFirst(t *testing.T) {
	sprites := &e2eSprites{lookupHit: false, ansi: "COLD"}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: &fakeDetails{},
		Sprites: sprites,
		Context: context.Background(),
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	if sprites.coldCalls != 0 {
		t.Fatalf("initial layout must lookup before cold, calls=%d", sprites.coldCalls)
	}
}
