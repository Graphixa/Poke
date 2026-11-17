package tui_test

import (
	"context"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Poke/internal/domain"
	"github.com/Graphixa/Poke/internal/render/sprite"
	"github.com/Graphixa/Poke/internal/tui"
)

type e2eSprites struct {
	lookupHit   bool
	lookupCalls atomic.Int32
	coldCalls   atomic.Int32
	block       chan struct{}
	ansi        string
}

func (e *e2eSprites) Render(_ context.Context, _ domain.Pokemon, _ int) (string, bool, bool, string) {
	return e.ansi, false, e.lookupHit, "pixel"
}

func (e *e2eSprites) LookupCached(_ context.Context, _ domain.Pokemon, _ int) (sprite.RenderResult, bool) {
	e.lookupCalls.Add(1)
	if e.lookupHit {
		return sprite.RenderResult{ANSI: e.ansi, Native: true}, true
	}
	return sprite.RenderResult{}, false
}

func (e *e2eSprites) RenderCold(_ context.Context, _ domain.Pokemon, _ int) sprite.RenderResult {
	if e.block != nil {
		<-e.block
	}
	e.coldCalls.Add(1)
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
	_ = updated.(tui.Model)
	if sprites.coldCalls.Load() != 0 {
		t.Fatalf("cached selection must not cold-render, calls=%d", sprites.coldCalls.Load())
	}
}

func TestE2ESelectionColdUsesLookupFirst(t *testing.T) {
	block := make(chan struct{})
	sprites := &e2eSprites{lookupHit: false, ansi: "COLD", block: block}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: &fakeDetails{},
		Sprites: sprites,
		Context: context.Background(),
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	_ = updated.(tui.Model)
	if sprites.lookupCalls.Load() == 0 {
		t.Fatal("initial layout must lookup cache before cold render")
	}
	if sprites.coldCalls.Load() != 0 {
		t.Fatalf("initial layout must not cold-render before blocked release, calls=%d", sprites.coldCalls.Load())
	}
	close(block)
}
