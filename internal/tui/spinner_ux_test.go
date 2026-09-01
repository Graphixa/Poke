package tui_test

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/tui"
)

func newSpinnerModel(sprites tui.SpriteRenderer) tui.Model {
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: &fakeDetails{},
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	return updated.(tui.Model)
}

func TestSpinnerOnlyOnTrueColdMiss(t *testing.T) {
	cached := &fakeSprites{cached: "WARM", native: true, ansi: "COLD"}
	m := newSpinnerModel(cached)
	_ = m.EnsureSelectionLoadsForTest()
	if m.ImageSpinnerForTest() {
		t.Fatal("cache hit must not show spinner")
	}

	updated, _ := m.Update(tui.WarmProgressNativeMsgForTest(3, 10, 32))
	m = updated.(tui.Model)
	if m.ImageSpinnerForTest() {
		t.Fatal("background native warm progress alone must not show spinner")
	}

	cold := &fakeSprites{ansi: "COLD", block: make(chan struct{})}
	m = newSpinnerModel(cold)
	_ = m.ForceSpriteLoadForTest()
	if !m.ImageSpinnerForTest() {
		t.Fatal("true cold miss must show spinner")
	}
	if m.ImageStateForTest() != int(tui.ImageLoading) {
		t.Fatalf("image state=%d want loading on cold miss", m.ImageStateForTest())
	}

	updated, _ = m.Update(tui.WarmProgressNativeMsgForTest(4, 10, 32))
	m = updated.(tui.Model)
	if !m.ImageSpinnerForTest() {
		t.Fatal("spinner must remain for active cold miss while background warms")
	}
	close(cold.block)
}

func TestSpinnerNotShownForNativeMemoryHit(t *testing.T) {
	sprites := &memHitSprites{fakeSprites: fakeSprites{ansi: "NATIVE", native: true}}
	m := newSpinnerModel(sprites)
	cmd := m.ForceSpriteLoadForTest()
	if cmd == nil {
		t.Fatal("expected cold render to populate mem cache")
	}
	if !m.ImageSpinnerForTest() {
		t.Fatal("cold miss must show spinner before mem is warm")
	}
	var model tea.Model = m
	drainCmds(t, &model, cmd)
	m = model.(tui.Model)
	if m.ImageSpinnerForTest() {
		t.Fatal("completed render must clear spinner")
	}

	updated, _ := m.Update(key("down"))
	m = updated.(tui.Model)
	if !m.ImageSpinnerForTest() {
		t.Fatal("cold neighbor must show spinner")
	}

	updated, _ = m.Update(key("up"))
	m = updated.(tui.Model)
	_ = m.EnsureSelectionLoadsForTest()
	if m.ImageSpinnerForTest() {
		t.Fatal("native memory hit must not show spinner")
	}
	if m.ImageStateForTest() != int(tui.ImageReady) {
		t.Fatalf("image state=%d want ready on mem hit", m.ImageStateForTest())
	}
}

// memHitSprites simulates process-local mem cache: miss until RenderCold stores frame.
type memHitSprites struct {
	fakeSprites
	mem map[int]string
}

func (m *memHitSprites) LookupCached(ctx context.Context, pokemon domain.Pokemon, cells int) (sprite.RenderResult, bool) {
	if s, ok := m.mem[pokemon.ID]; ok && s != "" {
		return sprite.RenderResult{ANSI: s, Native: m.native, AssetKind: "artwork"}, true
	}
	return m.fakeSprites.LookupCached(ctx, pokemon, cells)
}

func (m *memHitSprites) RenderCold(ctx context.Context, pokemon domain.Pokemon, cells int) sprite.RenderResult {
	res := m.fakeSprites.RenderCold(ctx, pokemon, cells)
	if !res.UsePlaceholder && res.ANSI != "" {
		if m.mem == nil {
			m.mem = map[int]string{}
		}
		m.mem[pokemon.ID] = res.ANSI
	}
	return res
}

func TestSpinnerNotShownForNativeDiskHit(t *testing.T) {
	sprites := &fakeSprites{
		cached:     "DISK",
		native:     true,
		cachedByID: map[int]string{4: "DISK"},
		ansi:       "COLD",
	}
	m := newSpinnerModel(sprites)
	_ = m.EnsureSelectionLoadsForTest()
	if m.ImageSpinnerForTest() {
		t.Fatal("native disk hit must not show spinner")
	}
	if m.ImageStateForTest() != int(tui.ImageReady) {
		t.Fatalf("image state=%d want ready on disk hit", m.ImageStateForTest())
	}
	if !m.HasSpriteANSI() || m.ImageANSILenForTest() != len("DISK") {
		t.Fatal("disk hit must paint cached frame immediately")
	}
	if sprites.calls.Load() != 0 {
		t.Fatalf("disk hit must not cold-render, calls=%d", sprites.calls.Load())
	}
}

func TestStaleCompletionDoesNotClearNewerSpinner(t *testing.T) {
	block := make(chan struct{})
	sprites := &fakeSprites{ansi: "ART", block: block, native: true}
	m := newSpinnerModel(sprites)
	cmd := m.ForceSpriteLoadForTest()
	if cmd == nil {
		t.Fatal("expected cold load")
	}
	go func() { _ = cmd() }()

	firstID, _ := m.SelectedID()
	updated, _ := m.Update(key("down"))
	m = updated.(tui.Model)
	nextID, _ := m.SelectedID()
	if nextID == firstID {
		t.Fatal("expected selection to move")
	}
	if !m.ImageSpinnerForTest() {
		t.Fatal("new selection cold miss must show spinner")
	}

	cells := m.CurrentSpriteCells()
	gen := m.SelectionGen()
	stale := tui.ColdFrameMsgForTest(gen, firstID, cells, "STALE", false, true, "artwork")
	updated, _ = m.Update(stale)
	m = updated.(tui.Model)
	if !m.ImageSpinnerForTest() {
		t.Fatal("stale completion must not clear newer selection spinner")
	}
	if m.HasSpriteANSI() && m.ImageSpeciesIDForTest() == firstID {
		t.Fatal("stale frame must not paint for prior selection")
	}

	close(block)
	current := tui.ColdFrameMsgForTest(gen, nextID, cells, "FRESH", false, true, "artwork")
	updated, _ = m.Update(current)
	m = updated.(tui.Model)
	if m.ImageSpinnerForTest() {
		t.Fatal("current completion must clear spinner")
	}
	if !m.HasSpriteANSI() {
		t.Fatal("expected current frame to paint")
	}
}

func TestCurrentCompletionClearsSpinner(t *testing.T) {
	block := make(chan struct{})
	started := make(chan struct{}, 1)
	sprites := &fakeSprites{ansi: "ART", block: block, started: started, native: true}
	m := newSpinnerModel(sprites)
	cmd := m.ForceSpriteLoadForTest()
	if cmd == nil {
		t.Fatal("expected cold load")
	}
	if !m.ImageSpinnerForTest() {
		t.Fatal("cold miss must show spinner immediately")
	}

	go func() { _ = cmd() }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("render did not start")
	}
	if !m.ImageSpinnerForTest() {
		t.Fatal("spinner must remain while encode is in flight")
	}

	id, _ := m.SelectedID()
	cells := m.CurrentSpriteCells()
	gen := m.SelectionGen()
	updated, _ := m.Update(tui.ColdFrameMsgForTest(gen, id, cells, "ART", false, true, "artwork"))
	m = updated.(tui.Model)
	close(block)
	if m.ImageSpinnerForTest() {
		t.Fatal("current completion must clear spinner")
	}
	if !m.HasSpriteANSI() {
		t.Fatal("expected frame to paint after completion")
	}
	if m.ImageStateForTest() != int(tui.ImageReady) {
		t.Fatalf("image state=%d want ready after completion", m.ImageStateForTest())
	}
}
