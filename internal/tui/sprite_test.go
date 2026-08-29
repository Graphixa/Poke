package tui_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/search"
	"github.com/Graphixa/Pokedex/internal/tui"
)

type fakeSprites struct {
	calls   atomic.Int32
	block   chan struct{}
	ansi    string
	cached  string
	native  bool
	started chan struct{}
}

func (f *fakeSprites) LookupCached(context.Context, domain.Pokemon, int) (sprite.RenderResult, bool) {
	if f == nil || f.cached == "" {
		return sprite.RenderResult{}, false
	}
	return sprite.RenderResult{ANSI: f.cached, Native: f.native, AssetKind: "artwork"}, true
}

func (f *fakeSprites) RenderCold(ctx context.Context, pokemon domain.Pokemon, cells int) sprite.RenderResult {
	ansi, ph, native, kind := f.Render(ctx, pokemon, cells)
	return sprite.RenderResult{ANSI: ansi, UsePlaceholder: ph, Native: native, AssetKind: kind}
}

func (f *fakeSprites) Render(ctx context.Context, _ domain.Pokemon, _ int) (string, bool, bool, string) {
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
			return "", true, false, ""
		case <-f.block:
		}
	}
	if f.ansi == "" {
		return "SPRITE", false, false, "artwork"
	}
	return f.ansi, false, false, "artwork"
}

func (f *fakeSprites) ClearNative() {}

func (f *fakeSprites) ClearNativeRegion(cellsW, cellsH int) string { return "" }

func (f *fakeSprites) InvalidateSpecies(int) {}

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
	cmd = m.ForceSpriteLoadForTest()
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

func TestWarmProgressPopsInSelectedSprite(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{ansi: "POP"}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)

	id, ok := m.SelectedID()
	if !ok {
		t.Fatal("expected selection")
	}
	cells := m.CurrentSpriteCells()
	// Sticky placeholder as if the first encode ran before artwork landed.
	ph := tui.SpriteMsgForTest(m.SpriteReqID(), m.SelectionGen(), id, cells, "", true)
	updated, _ = m.Update(ph)
	m = updated.(tui.Model)
	if !m.SelectedSpritePlaceholderForTest() {
		t.Fatal("expected placeholder")
	}
	if m.HasSpriteANSI() {
		t.Fatal("placeholder should not count as art")
	}

	before := m.SpriteReqID()
	updated, cmd := m.Update(tui.WarmProgressSpeciesMsgForTest(1, 10, id))
	m = updated.(tui.Model)
	if cmd == nil {
		t.Fatal("expected reload cmd when warm finishes selected species")
	}
	if m.SpriteReqID() <= before && !m.SelectedSpriteLoadingForTest() {
		t.Fatal("expected sprite reload after warm for selected species")
	}

	// Other species finishing must not thrash the selected sprite.
	before = m.SpriteReqID()
	loading := m.SelectedSpriteLoadingForTest()
	updated, _ = m.Update(tui.WarmProgressSpeciesMsgForTest(2, 10, id+99))
	m = updated.(tui.Model)
	if !loading && m.SpriteReqID() != before {
		t.Fatal("warm for other species should not bump selected sprite req")
	}
}

func TestWarmProgressUpgradesNonPlaceholderPixel(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{ansi: "HD"}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)

	id, _ := m.SelectedID()
	cells := m.CurrentSpriteCells()
	// First paint succeeded with pixel-tier art (non-placeholder).
	px := tui.SpriteMsgKindForTest(m.SpriteReqID(), m.SelectionGen(), id, cells, "PIXEL", "pixel", false)
	updated, _ = m.Update(px)
	m = updated.(tui.Model)
	if !m.HasSpriteANSI() {
		t.Fatal("expected pixel art showing")
	}

	before := m.SpriteReqID()
	updated, cmd := m.Update(tui.WarmProgressSpeciesMsgForTest(3, 10, id))
	m = updated.(tui.Model)
	if cmd == nil {
		t.Fatal("artwork warm must upgrade pixel frame without scrolling")
	}
	if m.SpriteReqID() <= before && !m.SelectedSpriteLoadingForTest() {
		t.Fatal("expected reload after artwork warm for pixel-showing selection")
	}

	// Already artwork: warm should no-op.
	art := tui.SpriteMsgKindForTest(m.SpriteReqID(), m.SelectionGen(), id, cells, "ART", "artwork", false)
	updated, _ = m.Update(art)
	m = updated.(tui.Model)
	before = m.SpriteReqID()
	updated, cmd = m.Update(tui.WarmProgressSpeciesMsgForTest(4, 10, id))
	m = updated.(tui.Model)
	if m.SpriteReqID() != before {
		t.Fatal("artwork frame should not reload on artwork warm")
	}
	// cmd may still be listenWarmProgress batch with nil reload
	_ = cmd
}

func TestSelectionChangeBlanksImagePane(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{ansi: "ART"}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	id, _ := m.SelectedID()
	cells := m.CurrentSpriteCells()
	fresh := tui.SpriteMsgForTest(m.SpriteReqID(), m.SelectionGen(), id, cells, "ART", false)
	updated, _ = m.Update(fresh)
	m = updated.(tui.Model)
	if !m.HasSpriteANSI() {
		t.Fatal("expected art")
	}
	updated, _ = m.Update(key("down"))
	m = updated.(tui.Model)
	if m.HasSpriteANSI() {
		t.Fatal("image pane must not keep prior species after selection change")
	}
	if m.ImageStateForTest() != int(tui.ImageLoading) {
		t.Fatalf("image state=%d want loading on cold miss", m.ImageStateForTest())
	}
	if !m.ImageSpinnerForTest() {
		t.Fatal("cold miss must show spinner immediately")
	}
}

func TestResourceCacheLRUPlateau(t *testing.T) {
	const n = 100
	rows := make([]domain.SpeciesSummary, n)
	for i := range rows {
		id := i + 1
		rows[i] = domain.SpeciesSummary{ID: id, Slug: "p", DisplayName: "P"}
	}
	m := tui.NewModel(tui.Options{
		Index:   search.Build(rows),
		Details: &fakeDetails{},
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	for i := 0; i < n-1; i++ {
		updated, _ = m.Update(key("down"))
		m = updated.(tui.Model)
	}
	if count := m.ResourceCountForTest(); count > 16 {
		t.Fatalf("resource cache exceeded limit after %d selections: %d > 16", n, count)
	}
}

func TestResourceCacheLRUBounded(t *testing.T) {
	rows := make([]domain.SpeciesSummary, 40)
	for i := range rows {
		id := i + 1
		rows[i] = domain.SpeciesSummary{ID: id, Slug: "p", DisplayName: "P"}
	}
	m := tui.NewModel(tui.Options{
		Index:   search.Build(rows),
		Details: &fakeDetails{},
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	for i := 0; i < 30; i++ {
		updated, _ = m.Update(key("down"))
		m = updated.(tui.Model)
	}
	if n := m.ResourceCountForTest(); n > 16 {
		t.Fatalf("resource cache grew unbounded: %d", n)
	}
}

func TestColdMissShowsSpinnerImmediately(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{ansi: "ART", block: make(chan struct{})}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	_ = m.EnsureSelectionLoadsForTest()
	if m.ImageStateForTest() != int(tui.ImageLoading) {
		t.Fatalf("image state=%d want loading immediately on cold miss", m.ImageStateForTest())
	}
	if !m.ImageSpinnerForTest() {
		t.Fatal("cold miss must show spinner")
	}
	if !m.SelectedSpriteLoadingForTest() {
		t.Fatal("cold miss must start encode")
	}
	close(sprites.block)
}

func TestWarmCacheHitPaintsImmediatelyWithoutSpinner(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{cached: "WARM", ansi: "COLD"}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	_ = m.EnsureSelectionLoadsForTest()
	if m.ImageStateForTest() != int(tui.ImageReady) {
		t.Fatalf("image state=%d want ready on warm hit", m.ImageStateForTest())
	}
	if m.ImageSpinnerForTest() {
		t.Fatal("warm hit must not show spinner")
	}
	if !m.HasSpriteANSI() || m.ImageANSILenForTest() != len("WARM") {
		t.Fatal("warm hit must paint cached frame immediately")
	}
	if sprites.calls.Load() != 0 {
		t.Fatalf("warm hit must not cold-render, calls=%d", sprites.calls.Load())
	}

	updated, _ = m.Update(key("down"))
	m = updated.(tui.Model)
	if m.ImageStateForTest() != int(tui.ImageReady) {
		t.Fatalf("after move image state=%d want ready", m.ImageStateForTest())
	}
	if m.ImageSpinnerForTest() {
		t.Fatal("warm navigation must not show spinner")
	}
	if sprites.calls.Load() != 0 {
		t.Fatalf("warm navigation must not cold-render, calls=%d", sprites.calls.Load())
	}
}

func TestSelectionStartsSpriteLoadImmediately(t *testing.T) {
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
	updated, _ = m.Update(key("down"))
	m = updated.(tui.Model)
	nextID, _ := m.SelectedID()
	if nextID == firstID {
		t.Fatal("expected cursor to move")
	}
	if m.HasSpriteANSI() {
		t.Fatal("selection change must blank image pane")
	}
	if !m.SelectedSpriteLoadingForTest() {
		t.Fatal("selection must start sprite load immediately, not after a debounce")
	}
	if m.ImageStateForTest() != int(tui.ImageLoading) {
		t.Fatalf("image state=%d want loading", m.ImageStateForTest())
	}
	if !m.ImageSpinnerForTest() {
		t.Fatal("cold selection must show spinner immediately")
	}
	close(sprites.block)
}

func TestRepeatedSelectionStartsSpriteLoadImmediately(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{ansi: "ART", block: make(chan struct{})}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)

	for i := 0; i < 3; i++ {
		updated, _ = m.Update(key("down"))
		m = updated.(tui.Model)
		if !m.SelectedSpriteLoadingForTest() {
			t.Fatalf("keydown %d: sprite load must start immediately", i+1)
		}
		if m.HasSpriteANSI() {
			t.Fatal("held navigation must not keep prior image")
		}
	}
	close(sprites.block)
}

func TestSearchSelectionStartsSpriteLoadImmediately(t *testing.T) {
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
	updated, _ = m.Update(key("p"))
	m = updated.(tui.Model)
	if m.Query() != "p" {
		t.Fatalf("query = %q", m.Query())
	}
	nextID, _ := m.SelectedID()
	if nextID == firstID {
		t.Fatal("expected filter to change selection")
	}
	if m.HasSpriteANSI() {
		t.Fatal("search selection change must blank image pane")
	}
	if !m.SelectedSpriteLoadingForTest() {
		t.Fatal("search selection must start sprite load immediately")
	}
	close(sprites.block)
}

func TestPrefetchDoesNotRetainRenderedImage(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{ansi: "NEIGHBOR"}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)

	id, _ := m.SelectedID()
	cells := m.CurrentSpriteCells()
	fresh := tui.SpriteMsgForTest(m.SpriteReqID(), m.SelectionGen(), id, cells, "CURRENT", false)
	updated, _ = m.Update(fresh)
	m = updated.(tui.Model)
	if !m.HasSpriteANSI() || m.ImageANSILenForTest() != len("CURRENT") {
		t.Fatal("expected current selection art on image pane")
	}

	cmd := m.ScheduleSpritePrefetchForTest()
	if cmd != nil {
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
				updated, _ = m.Update(msg)
				m = updated.(tui.Model)
			}
		}
		apply(cmd)
	}
	if m.ImageANSILenForTest() != len("CURRENT") {
		t.Fatalf("prefetch must not replace selection image, len=%d", m.ImageANSILenForTest())
	}

	updated, _ = m.Update(key("down"))
	m = updated.(tui.Model)
	if m.HasSpriteANSI() {
		t.Fatal("selection change must blank image pane")
	}
}
