package tui_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Poke/internal/domain"
	"github.com/Graphixa/Poke/internal/render/sprite"
	"github.com/Graphixa/Poke/internal/search"
	"github.com/Graphixa/Poke/internal/tui"
)

type fakeSprites struct {
	calls       atomic.Int32
	inFlight    atomic.Int32
	maxInFlight atomic.Int32
	block       chan struct{}
	ansi        string
	cached      string
	cachedByID  map[int]string
	native      bool
	fail        bool
	started     chan struct{}
}

func (f *fakeSprites) LookupCached(_ context.Context, pokemon domain.Pokemon, _ int) (sprite.RenderResult, bool) {
	if f == nil {
		return sprite.RenderResult{}, false
	}
	if f.cachedByID != nil {
		s, ok := f.cachedByID[pokemon.ID]
		if !ok || s == "" {
			return sprite.RenderResult{}, false
		}
		return sprite.RenderResult{ANSI: s, Native: f.native, AssetKind: "artwork"}, true
	}
	if f.cached == "" {
		return sprite.RenderResult{}, false
	}
	return sprite.RenderResult{ANSI: f.cached, Native: f.native, AssetKind: "artwork"}, true
}

func (f *fakeSprites) RenderCold(ctx context.Context, pokemon domain.Pokemon, cells int) sprite.RenderResult {
	ansi, ph, native, kind := f.Render(ctx, pokemon, cells)
	return sprite.RenderResult{ANSI: ansi, UsePlaceholder: ph, Native: native, AssetKind: kind}
}

func (f *fakeSprites) Render(ctx context.Context, _ domain.Pokemon, _ int) (string, bool, bool, string) {
	n := f.inFlight.Add(1)
	for {
		old := f.maxInFlight.Load()
		if n <= old || f.maxInFlight.CompareAndSwap(old, n) {
			break
		}
	}
	defer f.inFlight.Add(-1)
	f.calls.Add(1)
	if f.started != nil {
		select {
		case f.started <- struct{}{}:
		default:
		}
	}
	if f.block != nil {
		<-f.block
		select {
		case <-ctx.Done():
			return "", true, false, ""
		default:
		}
	}
	if f.fail {
		return "", true, false, ""
	}
	if f.ansi == "" {
		return "SPRITE", false, false, "artwork"
	}
	return f.ansi, false, false, "artwork"
}

func (f *fakeSprites) ClearNative() {}

func (f *fakeSprites) ClearNativeRegion(int, int) string { return "" }

func (f *fakeSprites) InvalidateSpecies(int) {}

func (f *fakeSprites) PersistFrame(_ context.Context, _, _ string) error { return nil }

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
	if cmd == nil && !m.HasSpriteANSI() {
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
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 17})
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
	if m.HasSpriteANSI() && m.ImageCellsForTest() == cells && cmd == nil {
		t.Fatal("reflow must not keep stale ansi without scheduling reload")
	}
	if cmd == nil {
		t.Fatal("expected reflow sprite load")
	}
	wrongCells := tui.SpriteMsgForTest(m.SpriteReqID(), m.SelectionGen(), 4, cells+3, "OLDSIZE", false)
	updated, _ = m.Update(wrongCells)
	m = updated.(tui.Model)
	if m.ImageCellsForTest() == cells+3 {
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
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	cmd := m.ForceSpriteLoadForTest()
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
	// WindowSize schedules core load; this test discards that cmd and starts a
	// blocking load from Enter instead — clear the stuck loadingCore flag.
	m.CancelDossierForTest()

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
	if !m.HasSpriteANSI() || m.ImageANSILenForTest() != len("ART") {
		t.Fatal("expected current selection art on image pane")
	}
	updated, _ = m.Update(key("down"))
	m = updated.(tui.Model)
	if m.HasSpriteANSI() {
		t.Fatal("image pane must not keep prior species after selection change")
	}
	if m.ImageStateForTest() != int(tui.ImageLoading) {
		t.Fatalf("image state=%d want loading on cold miss", m.ImageStateForTest())
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

func TestForegroundLatestSelectionWins(t *testing.T) {
	details := &fakeDetails{}
	started := make(chan struct{}, 8)
	block := make(chan struct{})
	sprites := &fakeSprites{ansi: "ART", block: block, started: started}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)

	cmd := m.ForceSpriteLoadForTest()
	if cmd == nil {
		t.Fatal("expected first cold render")
	}
	firstID, _ := m.SelectedID()
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("first render did not start")
	}
	if sprites.calls.Load() != 1 {
		t.Fatalf("calls=%d want 1 active encode", sprites.calls.Load())
	}

	for i := 0; i < 3; i++ {
		updated, _ = m.Update(key("down"))
		m = updated.(tui.Model)
	}
	lastID, _ := m.SelectedID()
	if lastID == firstID {
		t.Fatal("expected cursor to move")
	}
	if sprites.calls.Load() != 1 {
		t.Fatalf("fast scroll must not start extra encodes, calls=%d", sprites.calls.Load())
	}
	if sprites.maxInFlight.Load() != 1 {
		t.Fatalf("max in-flight=%d want 1", sprites.maxInFlight.Load())
	}
	if m.FgPendingSpeciesForTest() != lastID {
		t.Fatalf("pending=%d want latest %d", m.FgPendingSpeciesForTest(), lastID)
	}
	if m.FgActiveSpeciesForTest() != firstID {
		t.Fatalf("active=%d want first %d", m.FgActiveSpeciesForTest(), firstID)
	}

	close(block)
	var msg tea.Msg
	select {
	case msg = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("blocked render did not finish")
	}
	updated, follow := m.Update(msg)
	m = updated.(tui.Model)
	if m.ImageSpeciesIDForTest() == firstID && m.HasSpriteANSI() {
		t.Fatal("stale first encode must not paint after scroll")
	}
	settled := m.FgActiveSpeciesForTest() == lastID ||
		m.FgPendingSpeciesForTest() == lastID ||
		m.SelectedSpriteLoadingForTest() ||
		follow != nil ||
		(m.HasSpriteANSI() && m.ImageSpeciesIDForTest() == lastID)
	if !settled {
		t.Fatal("expected pending latest selection to start after active exits")
	}
	if m.FgActiveSpeciesForTest() != lastID && m.FgPendingSpeciesForTest() != lastID &&
		!m.SelectedSpriteLoadingForTest() && !(m.HasSpriteANSI() && m.ImageSpeciesIDForTest() == lastID) {
		t.Fatalf("active after drain=%d want %d", m.FgActiveSpeciesForTest(), lastID)
	}
	if sprites.maxInFlight.Load() > 1 {
		t.Fatalf("max in-flight=%d want 1", sprites.maxInFlight.Load())
	}
}

func TestWarmSelectionBypassesUnrelatedColdRender(t *testing.T) {
	details := &fakeDetails{}
	started := make(chan struct{}, 1)
	block := make(chan struct{})
	sprites := &fakeSprites{
		cachedByID: map[int]string{5: "WARM5"},
		ansi:       "COLD",
		block:      block,
		started:    started,
	}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	cmd := m.ForceSpriteLoadForTest()
	if cmd == nil {
		t.Fatal("expected cold load for first species")
	}
	go func() { _ = cmd() }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("cold render did not start")
	}
	updated, _ = m.Update(key("down"))
	m = updated.(tui.Model)
	id, _ := m.SelectedID()
	if id != 5 {
		t.Fatalf("selected=%d want 5", id)
	}
	if m.ImageStateForTest() != int(tui.ImageReady) {
		t.Fatalf("warm neighbor state=%d want ready", m.ImageStateForTest())
	}

	if m.ImageANSILenForTest() != len("WARM5") {
		t.Fatal("warm frame must paint immediately")
	}
	close(block)
}

// A failed cold render must end the spinner (placeholder), not hang it.
func TestColdRenderFailureEndsLoading(t *testing.T) {
	m := tui.NewModel(tui.Options{Index: fixtureIndex(), Details: &fakeDetails{}, Sprites: &fakeSprites{fail: true}})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	cmd := m.EnsureSelectionLoadsForTest() // listener for the load WindowSize started
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		updated, _ = m.Update(msg)
		m = updated.(tui.Model)
	case <-time.After(2 * time.Second):
		t.Fatal("failed render never delivered a frame")
	}
	if m.ImageStateForTest() != int(tui.ImageUnavailable) {
		t.Fatalf("state=%d want unavailable", m.ImageStateForTest())
	}
}

func TestImageUnavailableClearsPreviousArt(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{cached: "ART"}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	_ = m.EnsureSelectionLoadsForTest()
	if m.ImageStateForTest() != int(tui.ImageReady) {
		t.Fatal("expected ready art")
	}
	id, _ := m.SelectedID()
	cells := m.CurrentSpriteCells()
	updated, _ = m.Update(tui.SpriteMsgForTest(
		m.SpriteReqID(), m.SelectionGen(), id, cells, "", true,
	))
	m = updated.(tui.Model)
	if m.ImageStateForTest() != int(tui.ImageUnavailable) {
		t.Fatalf("state=%d want unavailable", m.ImageStateForTest())
	}
	if m.HasSpriteANSI() {
		t.Fatal("unavailable must clear previous image")
	}
}

func TestSameSizeResizeDoesNotRerender(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{cached: "ART"}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	_ = m.EnsureSelectionLoadsForTest()
	calls := sprites.calls.Load()
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	if sprites.calls.Load() != calls {
		t.Fatalf("same-size resize rerendered, calls %d -> %d", calls, sprites.calls.Load())
	}
	if m.ImageStateForTest() != int(tui.ImageReady) {
		t.Fatal("same-size resize must keep ready art")
	}
}

func TestDossierLoadDoesNotBlockWarmSprite(t *testing.T) {
	details := &fakeDetails{blockLoad: make(chan struct{})}
	sprites := &fakeSprites{cached: "WARM"}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	_ = m.EnsureSelectionLoadsForTest()
	if m.ImageStateForTest() != int(tui.ImageReady) {
		t.Fatal("warm sprite must be ready before dossier")
	}
	updated, cmd := m.Update(key("enter"))
	m = updated.(tui.Model)
	_ = cmd
	if m.ImageStateForTest() != int(tui.ImageReady) {
		t.Fatal("opening dossier must not drop warm sprite")
	}
	if sprites.calls.Load() != 0 {
		t.Fatalf("dossier must not force cold render, calls=%d", sprites.calls.Load())
	}
	close(details.blockLoad)
}

func TestMixedWarmColdNavigation(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{
		cachedByID: map[int]string{4: "W4", 5: "W5", 25: "W25"},
		ansi:       "COLD",
		block:      make(chan struct{}),
	}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	_ = m.EnsureSelectionLoadsForTest()
	if m.ImageStateForTest() != int(tui.ImageReady) || m.ImageANSILenForTest() != len("W4") {
		t.Fatal("4 must be warm")
	}
	updated, _ = m.Update(key("down"))
	m = updated.(tui.Model)
	if m.ImageStateForTest() != int(tui.ImageReady) || m.ImageANSILenForTest() != len("W5") {
		t.Fatal("5 must paint immediately")
	}
	updated, _ = m.Update(key("down"))
	m = updated.(tui.Model)
	if m.ImageStateForTest() != int(tui.ImageLoading) {
		t.Fatal("6 must be cold loading")
	}
	updated, _ = m.Update(key("down"))
	m = updated.(tui.Model)
	if m.ImageStateForTest() != int(tui.ImageReady) || m.ImageANSILenForTest() != len("W25") {
		t.Fatal("25 warm must not wait for cold 6")
	}
	close(sprites.block)
}

func TestScroll100CoalescesToLatest(t *testing.T) {
	n := 100
	sums := make([]domain.SpeciesSummary, n)
	for i := 0; i < n; i++ {
		id := i + 1
		sums[i] = domain.SpeciesSummary{ID: id, Slug: "s", DisplayName: "P"}
		sums[i].Slug = "s"
		sums[i].DisplayName = "P"
	}
	details := &fakeDetails{}
	block := make(chan struct{})
	started := make(chan struct{}, 8)
	sprites := &fakeSprites{ansi: "ART", block: block, started: started}
	m := tui.NewModel(tui.Options{
		Index:   search.Build(sums),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	cmd := m.ForceSpriteLoadForTest()
	if cmd == nil {
		t.Fatal("expected first cold render")
	}
	go func() { _ = cmd() }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("first render did not start")
	}
	firstID, _ := m.SelectedID()
	var lastID int
	for i := 0; i < n-1; i++ {
		updated, _ = m.Update(key("down"))
		m = updated.(tui.Model)
		lastID, _ = m.SelectedID()
	}
	if lastID == firstID {
		t.Fatal("expected to reach last species")
	}
	if m.FgActiveSpeciesForTest() != firstID {
		t.Fatalf("active=%d want first %d", m.FgActiveSpeciesForTest(), firstID)
	}
	if m.FgPendingSpeciesForTest() != lastID {
		t.Fatalf("pending=%d want latest %d", m.FgPendingSpeciesForTest(), lastID)
	}
	close(block)
}

// Init is a value receiver — mutations (fgActive, spriteReqID, applied cache
// hit) are discarded. The first WindowSize must load the selected sprite.
func TestStartupAppliesCachedSpriteOnFirstWindowSize(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{cached: "ART"}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	_ = m.Init()
	if m.HasSpriteANSI() {
		t.Fatal("Init must not apply sprites on the discarded copy")
	}

	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	if m.HasSpriteANSI() {
		return
	}
	if cmd == nil {
		t.Fatal("first WindowSize must start a sprite load when Init could not")
	}
	var model tea.Model = m
	drainCmds(t, &model, cmd)
	m = model.(tui.Model)
	if !m.HasSpriteANSI() {
		t.Fatal("expected first species sprite after WindowSize")
	}
}
