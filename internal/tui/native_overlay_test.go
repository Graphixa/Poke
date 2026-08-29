package tui_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/tui"
	"github.com/Graphixa/Pokedex/internal/tui/components"
	"github.com/Graphixa/Pokedex/internal/tui/views"
)

func TestNativeOverlayDebounceDoesNotStarve(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{ansi: "ART"}
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
	msg := tui.SpriteMsgNativeForTest(tui.SpriteMsgForTest(
		m.SpriteReqID(), m.SelectionGen(), id, cells,
		"\x1bP0;0;8q#1~~~\x1b\\", false,
	))
	updated, cmd := m.Update(msg)
	m = updated.(tui.Model)
	if cmd == nil {
		t.Fatal("expected overlay schedule cmd after native sprite load")
	}
	if !m.NativePaintPendingForTest() {
		t.Fatal("expected nativePaintPending after schedule")
	}

	// Flood updates (warm progress style). Overlay must stay pending through follow-up window.
	for i := 0; i < 20; i++ {
		updated, _ = m.Update(tui.WarmProgressMsgForTest(i, 100))
		m = updated.(tui.Model)
	}
	if !m.NativePaintPendingForTest() {
		t.Fatal("overlay pending flag cleared by update flood — paint would starve")
	}
}

func TestNativeOverlayClearsOnSelectionWithoutArt(t *testing.T) {
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
	msg := tui.SpriteMsgNativeForTest(tui.SpriteMsgForTest(
		m.SpriteReqID(), m.SelectionGen(), id, cells,
		"\x1bP0;0;8q#1~~~\x1b\\", false,
	))
	updated, _ = m.Update(msg)
	m = updated.(tui.Model)
	// Complete follow-up so paint state is committed.
	updated, _ = m.Update(tui.NativeOverlayFollowUpForTest())
	m = updated.(tui.Model)
	if m.NativePaintANSIForTest() == "" {
		t.Fatal("expected painted native ANSI tracked")
	}

	updated, cmd := m.Update(key("down"))
	m = updated.(tui.Model)
	if cmd == nil {
		t.Fatal("expected clear/load cmds on selection change")
	}
}

func TestNativeOverlayNotPerpetualLoop(t *testing.T) {
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
	msg := tui.SpriteMsgNativeForTest(tui.SpriteMsgForTest(
		m.SpriteReqID(), m.SelectionGen(), id, cells,
		"\x1bP0;0;8q#1~~~\x1b\\", false,
	))
	updated, _ = m.Update(msg)
	m = updated.(tui.Model)
	if !m.NativePaintPendingForTest() {
		t.Fatal("expected pending after dirty paint")
	}
	// Follow-up re-paint clears pending — no perpetual 100ms loop.
	updated, cmd := m.Update(tui.NativeOverlayFollowUpForTest())
	m = updated.(tui.Model)
	if m.NativePaintPendingForTest() {
		t.Fatal("follow-up should clear nativePaintPending")
	}
	_ = cmd
}

func TestSpriteArtOriginTwoPane(t *testing.T) {
	d := views.BrowseData{
		Width: 120, Height: 52,
		ShowSprite: true, SpriteNative: true,
		LeftWidth: 40, RightWidth: 79,
		BodySize:    40,
		HasSelected: true,
		Selected:    domain.SpeciesSummary{ID: 25, DisplayName: "Pikachu"},
	}
	row, col, artW, artH, ok := views.SpriteArtOrigin(d)
	if !ok {
		t.Fatal("expected origin")
	}
	if row < 2 || col < 2 {
		t.Fatalf("origin row=%d col=%d looks invalid", row, col)
	}
	if artW < 1 || artH < 1 {
		t.Fatalf("art size %dx%d", artW, artH)
	}
	t.Logf("origin row=%d col=%d %dx%d", row, col, artW, artH)

	d.SpriteNative = false
	row2, col2, artW2, artH2, ok2 := views.SpriteArtRegion(d)
	if !ok2 {
		t.Fatal("expected region for placeholder layout")
	}
	if row != row2 || col != col2 || artW != artW2 || artH != artH2 {
		t.Fatalf("origin vs region mismatch: %d,%d %dx%d vs %d,%d %dx%d",
			row, col, artW, artH, row2, col2, artW2, artH2)
	}
	_, _, _, _, ok3 := views.SpriteArtOrigin(d)
	if ok3 {
		t.Fatal("origin should require native")
	}
}

func TestNativeOverlayClearsOnPlaceholder(t *testing.T) {
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
	native := tui.SpriteMsgNativeForTest(tui.SpriteMsgForTest(
		m.SpriteReqID(), m.SelectionGen(), id, cells,
		"\x1bP0;0;8q#1~~~\x1b\\", false,
	))
	updated, _ = m.Update(native)
	m = updated.(tui.Model)
	updated, _ = m.Update(tui.NativeOverlayFollowUpForTest())
	m = updated.(tui.Model)
	if m.NativePaintANSIForTest() == "" {
		t.Fatal("expected painted native ANSI tracked")
	}

	ph := tui.SpriteMsgForTest(
		m.SpriteReqID(), m.SelectionGen(), id, cells, "", true,
	)
	updated, cmd := m.Update(ph)
	m = updated.(tui.Model)
	if cmd == nil {
		t.Fatal("expected clear cmd when native art falls back to placeholder")
	}
}

func TestNativeOverlayWrapsSixelInDECSC(t *testing.T) {
	const dcs = "\x1bP0;0;8q#1~~~\x1b\\"
	seq := tui.NativeOverlayPaintSeqForTest(5, 10, 20, 16, dcs)
	assertNativeOverlayPayload(t, seq, dcs)
}

func TestNativeOverlayFollowUpPayloadHasNoFooter(t *testing.T) {
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
	const dcs = "\x1bP0;0;8q#1~~~\x1b\\"
	msg := tui.SpriteMsgNativeForTest(tui.SpriteMsgForTest(
		m.SpriteReqID(), m.SelectionGen(), id, cells, dcs, false,
	))
	updated, _ = m.Update(msg)
	m = updated.(tui.Model)

	_, cmd := m.Update(tui.NativeOverlayFollowUpForTest())
	payloads := collectRawPayloads(cmd)
	if len(payloads) == 0 {
		t.Fatal("expected Raw overlay payload")
	}
	joined := strings.Join(payloads, "")
	assertNativeOverlayPayload(t, joined, dcs)
}

func assertNativeOverlayPayload(t *testing.T, seq, dcs string) {
	t.Helper()
	if seq == "" {
		t.Fatal("empty overlay seq")
	}
	if strings.Contains(seq, "Cache") {
		t.Fatalf("overlay must not contain Cache (footer must not reprint after DCS): %q", seq)
	}
	dcsAt := strings.Index(seq, "\x1bP")
	if dcsAt < 0 {
		t.Fatal("expected DCS in overlay")
	}
	decscAt := strings.Index(seq, "\x1b7")
	if decscAt < 0 || decscAt > dcsAt {
		t.Fatalf("DECSC (ESC 7) must precede DCS")
	}
	decrcAt := strings.LastIndex(seq, "\x1b8")
	if decrcAt < 0 || decrcAt < dcsAt {
		t.Fatalf("DECRC (ESC 8) must follow DCS")
	}
	st := "\x1b\\"
	stAt := strings.LastIndex(seq, st)
	if stAt < 0 {
		t.Fatal("expected ST terminator after Sixel")
	}
	after := seq[stAt+len(st):]
	if strings.Contains(after, "\x1bP") {
		t.Fatalf("extra Sixel after image ST: %q", after)
	}
	if !strings.Contains(seq, dcs) {
		t.Fatalf("missing image DCS")
	}
}

func collectRawPayloads(cmd tea.Cmd) []string {
	var out []string
	var walk func(tea.Cmd)
	walk = func(c tea.Cmd) {
		if c == nil {
			return
		}
		msg := c()
		switch v := msg.(type) {
		case tea.BatchMsg:
			for _, inner := range v {
				walk(inner)
			}
		case tea.RawMsg:
			out = append(out, fmt.Sprint(v.Msg))
		}
	}
	walk(cmd)
	return out
}

func TestClearCellRectCoversBox(t *testing.T) {
	seq := tui.ClearCellRectForTest(5, 10, 3, 2)
	if seq == "" {
		t.Fatal("expected clear sequence")
	}
	// Two CUP+spaces rows at (5,10) and (6,10).
	if got := countSubstr(seq, "\x1b[5;10H"); got != 1 {
		t.Fatalf("row5 cups=%d", got)
	}
	if got := countSubstr(seq, "\x1b[6;10H"); got != 1 {
		t.Fatalf("row6 cups=%d", got)
	}
	if !containsSpaces(seq, 3) {
		t.Fatal("expected space run of width 3")
	}
}

func TestWarmProgressNativeFooter(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{ansi: "ART"}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)

	updated, _ = m.Update(tui.WarmProgressNativeMsgForTest(5, 10, 40))
	m = updated.(tui.Model)
	hint := m.BrowseWarmHintForTest()
	if hint == "" || !strings.Contains(hint, "Building Image Cache") {
		t.Fatalf("warm right=%q", hint)
	}
	if vis := m.BrowseVisibleWarmHintForTest(); vis == "" {
		t.Fatal("expected visible footer progress without native sprite")
	}
}

func TestWarmHintHiddenUnderNativeSprite(t *testing.T) {
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
	msg := tui.SpriteMsgNativeForTest(tui.SpriteMsgForTest(
		m.SpriteReqID(), m.SelectionGen(), id, cells,
		"\x1bP0;0;8q#1~~~\x1b\\", false,
	))
	updated, _ = m.Update(msg)
	m = updated.(tui.Model)
	// Finish initial overlay paint cycle before warm ticks.
	updated, _ = m.Update(tui.NativeOverlayFollowUpForTest())
	m = updated.(tui.Model)

	updated, _ = m.Update(tui.WarmProgressNativeMsgForTest(460, 649, 36))
	m = updated.(tui.Model)
	if m.BrowseWarmHintForTest() == "" {
		t.Fatal("expected internal warm state")
	}
	if vis := m.BrowseVisibleWarmHintForTest(); vis != "" {
		t.Fatalf("warm hint must not render over native art: %q", vis)
	}
	if m.NativeOverlayDirtyForTest() || m.NativePaintPendingForTest() {
		t.Fatal("warm tick must not trigger overlay repaint (causes Sixel flash)")
	}
}

func TestNativeImageCacheProgressUsesRawFooterPatch(t *testing.T) {
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
	msg := tui.SpriteMsgNativeForTest(tui.SpriteMsgForTest(
		m.SpriteReqID(), m.SelectionGen(), id, cells,
		"\x1bP0;0;8q#1~~~\x1b\\", false,
	))
	updated, _ = m.Update(msg)
	m = updated.(tui.Model)
	updated, _ = m.Update(tui.NativeOverlayFollowUpForTest())
	m = updated.(tui.Model)

	updated, cmd := m.Update(tui.WarmProgressNativeMsgForTest(3, 8, 40))
	m = updated.(tui.Model)
	if strings.Contains(m.BrowseWarmHintForTest(), "(40)") {
		t.Fatalf("size bucket leaked: %q", m.BrowseWarmHintForTest())
	}
	raws := collectRawPayloads(cmd)
	if len(raws) == 0 {
		t.Fatal("expected tea.Raw footer patch while native sprite is visible")
	}
}

func TestNativeImageCacheFooterClearsRawOnComplete(t *testing.T) {
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
	msg := tui.SpriteMsgNativeForTest(tui.SpriteMsgForTest(
		m.SpriteReqID(), m.SelectionGen(), id, cells,
		"\x1bP0;0;8q#1~~~\x1b\\", false,
	))
	updated, _ = m.Update(msg)
	m = updated.(tui.Model)
	updated, _ = m.Update(tui.NativeOverlayFollowUpForTest())
	m = updated.(tui.Model)

	updated, _ = m.Update(tui.WarmProgressNativeMsgForTest(1024, 1025, 40))
	m = updated.(tui.Model)
	if m.BrowseWarmHintForTest() == "" {
		t.Fatal("expected in-progress footer state")
	}

	updated, cmd := m.Update(tui.WarmProgressNativeMsgForTest(1025, 1025, 40))
	m = updated.(tui.Model)
	if m.BrowseWarmHintForTest() != "" {
		t.Fatalf("expected cleared hint, got %q", m.BrowseWarmHintForTest())
	}
	if vis := m.BrowseVisibleWarmHintForTest(); vis != "" {
		t.Fatalf("view hint must stay hidden under native art: %q", vis)
	}
	if m.NativeOverlayDirtyForTest() || m.NativePaintPendingForTest() {
		t.Fatal("completion must not trigger overlay repaint")
	}
	raws := collectRawPayloads(cmd)
	blank := strings.Repeat(" ", components.FooterRightWidth)
	found := false
	for _, r := range raws {
		if strings.Contains(r, blank) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected Raw footer clear on complete, payloads=%v", raws)
	}
}

func TestNativeImageCacheFooterClearsWhenDoneEqualsTotal(t *testing.T) {
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
	msg := tui.SpriteMsgNativeForTest(tui.SpriteMsgForTest(
		m.SpriteReqID(), m.SelectionGen(), id, cells,
		"\x1bP0;0;8q#1~~~\x1b\\", false,
	))
	updated, _ = m.Update(msg)
	m = updated.(tui.Model)
	updated, _ = m.Update(tui.NativeOverlayFollowUpForTest())
	m = updated.(tui.Model)

	updated, cmd := m.Update(tui.WarmProgressNativeMsgForTest(8, 8, 40))
	m = updated.(tui.Model)
	if m.BrowseWarmHintForTest() != "" {
		t.Fatalf("expected hidden bar at 8/8, got %q", m.BrowseWarmHintForTest())
	}
	raws := collectRawPayloads(cmd)
	blank := strings.Repeat(" ", components.FooterRightWidth)
	found := false
	for _, r := range raws {
		if strings.Contains(r, blank) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected Raw footer clear at Done==Total, payloads=%v", raws)
	}
}

type fakeNativeWarmer struct{}

func (fakeNativeWarmer) EnsureStarted(context.Context, int) {}
func (fakeNativeWarmer) Prioritize(int)                     {}
func (fakeNativeWarmer) SetNeighborsFromList([]domain.SpeciesSummary, int, int) {
}
func (fakeNativeWarmer) SetCells(int)           {}
func (fakeNativeWarmer) Snapshot() (int, int)   { return 0, 0 }
func (fakeNativeWarmer) NoteActivity()          {}
func (fakeNativeWarmer) SetForegroundBusy(bool) {}

func TestWarmHintVisibleWhenNativeWarmerNoSprite(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{ansi: "ART"}
	m := tui.NewModel(tui.Options{
		Index:        fixtureIndex(),
		Details:      details,
		Sprites:      sprites,
		NativeWarmer: fakeNativeWarmer{},
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)

	updated, _ = m.Update(tui.WarmProgressNativeMsgForTest(5, 10, 40))
	m = updated.(tui.Model)
	if m.BrowseWarmHintForTest() == "" {
		t.Fatal("expected internal warm state")
	}
	if vis := m.BrowseVisibleWarmHintForTest(); vis == "" {
		t.Fatal("expected visible footer progress before native sprite paints")
	}
}

func TestDialogCloseRepaintsNativeOverlay(t *testing.T) {
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
	msg := tui.SpriteMsgNativeForTest(tui.SpriteMsgForTest(
		m.SpriteReqID(), m.SelectionGen(), id, cells,
		"\x1bP0;0;8q#1~~~\x1b\\", false,
	))
	updated, _ = m.Update(msg)
	m = updated.(tui.Model)

	updated, _ = m.Update(key("enter"))
	m = updated.(tui.Model)
	if !m.DialogVisible() {
		t.Fatal("expected dialog")
	}
	updated, _ = m.Update(key("esc"))
	m = updated.(tui.Model)
	if m.DialogVisible() {
		t.Fatal("expected dialog closed")
	}
	if !m.NativePaintPendingForTest() && !m.NativeOverlayDirtyForTest() {
		t.Fatal("expected native overlay dirty/pending after dialog close")
	}
}

func TestNativeOverlayNavigationAndResize(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{ansi: "ART"}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	paint := func() {
		id, _ := m.SelectedID()
		cells := m.CurrentSpriteCells()
		msg := tui.SpriteMsgNativeForTest(tui.SpriteMsgForTest(
			m.SpriteReqID(), m.SelectionGen(), id, cells,
			"\x1bP0;0;8q#1~~~\x1b\\", false,
		))
		updated, _ = m.Update(msg)
		m = updated.(tui.Model)
	}
	paint()
	firstID, _ := m.SelectedID()
	for _, k := range []string{"down", "up", "pgdown", "pgup", "home", "end"} {
		updated, _ = m.Update(key(k))
		m = updated.(tui.Model)
	}
	updated, _ = m.Update(key("p"))
	m = updated.(tui.Model)
	updated, _ = m.Update(key("esc"))
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 140, Height: 60})
	m = updated.(tui.Model)
	updated, _ = m.Update(key("?"))
	m = updated.(tui.Model)
	updated, _ = m.Update(key("esc"))
	m = updated.(tui.Model)
	id, _ := m.SelectedID()
	if id == 0 {
		t.Fatal("expected selection after navigation")
	}
	_ = firstID
	f := m.LayoutFlags()
	if f.LeftWidth+f.RightWidth+1 != f.Width && !f.OnePane {
		t.Fatalf("panel widths left=%d right=%d width=%d", f.LeftWidth, f.RightWidth, f.Width)
	}
}

func countSubstr(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}

func containsSpaces(s string, n int) bool {
	run := make([]byte, n)
	for i := range run {
		run[i] = ' '
	}
	return countSubstr(s, string(run)) > 0
}
