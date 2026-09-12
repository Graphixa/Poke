package tui_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/tui"
	"github.com/Graphixa/Pokedex/internal/tui/views"
)

// Init cannot mutate the live model. First WindowSize must paint native art
// for the initially selected species (Bulbasaur) without a list move.
func TestStartupPaintsFirstNativeSprite(t *testing.T) {
	const dcs = "\x1bP0;0;8q#1~~~\x1b\\"
	details := &fakeDetails{}
	sprites := &fakeSprites{cached: dcs, native: true}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: details,
		Sprites: sprites,
	})
	_ = m.Init()

	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	if !m.HasSpriteANSI() {
		if cmd == nil {
			t.Fatal("first WindowSize produced no sprite and no load cmd")
		}
		var model tea.Model = m
		drainCmds(t, &model, cmd)
		m = model.(tui.Model)
	}
	if !m.HasSpriteANSI() {
		t.Fatal("expected first species native sprite after WindowSize")
	}
	if !m.NativePaintPendingForTest() && m.NativePaintANSIForTest() == "" {
		t.Fatal("expected native overlay paint for the first species")
	}
	payloads := collectRawPayloads(cmd)
	if m.NativePaintANSIForTest() == "" && len(payloads) == 0 {
		t.Fatal("expected tea.Raw overlay payload")
	}
}

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
		t.Fatal("expected nativePaintPending after schedule (insurance)")
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
	if m.NativePaintANSIForTest() != "" {
		t.Fatal("selection change must clear paint tracking")
	}
}

func TestNativeOverlayKeepsTextHoldUntilReady(t *testing.T) {
	details := &fakeDetails{}
	sprites := &fakeSprites{ansi: "ART", block: make(chan struct{})}
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
	updated, _ = m.Update(tui.NativeOverlayFollowUpForTest())
	m = updated.(tui.Model)
	if m.NativePaintANSIForTest() == "" {
		t.Fatal("expected painted native ANSI tracked")
	}

	updated, cmd := m.Update(key("down"))
	m = updated.(tui.Model)
	if cmd != nil {
		c := cmd
		go func() { _ = c() }()
	}
	// Clear→paint: tracking cleared immediately; text hold may remain.
	if m.NativePaintANSIForTest() != "" {
		t.Fatalf("selection change must clear paint tracking, got %q", m.NativePaintANSIForTest())
	}
	if m.HasSpriteANSI() {
		t.Fatal("image pane must not keep prior species after selection change")
	}
	if m.GlanceHoldIDForTest() == 0 {
		t.Fatal("expected text glance hold while loading")
	}
	close(sprites.block)
}

func TestGlanceHoldKeepsPreviousUntilReady(t *testing.T) {
	block := make(chan struct{})
	sprites := &fakeSprites{ansi: "COLD", native: true, block: block}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: &fakeDetails{},
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	id, _ := m.SelectedID()
	cells := m.CurrentSpriteCells()
	const dcs = "\x1bP0;0;8q#A~~~\x1b\\"
	updated, _ = m.Update(tui.SpriteMsgNativeForTest(tui.SpriteMsgForTest(
		m.SpriteReqID(), m.SelectionGen(), id, cells, dcs, false,
	)))
	m = updated.(tui.Model)
	if m.BrowseSelectedNameForTest() != "Charmander" {
		t.Fatalf("name=%q", m.BrowseSelectedNameForTest())
	}

	updated, cmd := m.Update(key("down"))
	m = updated.(tui.Model)
	if cmd != nil {
		c := cmd
		go func() { _ = c() }()
	}
	if m.GlanceHoldIDForTest() != id {
		t.Fatalf("hold=%d want %d", m.GlanceHoldIDForTest(), id)
	}
	if m.BrowseSelectedNameForTest() != "Charmander" {
		t.Fatalf("held glance name=%q want Charmander", m.BrowseSelectedNameForTest())
	}
	nextID, _ := m.SelectedID()
	if nextID == id {
		t.Fatal("list selection should have moved")
	}

	gen := m.SelectionGen()
	cells = m.CurrentSpriteCells()
	updated, _ = m.Update(tui.ColdFrameMsgForTest(gen, nextID, cells, "\x1bP0;0;8q#B~~~\x1b\\", false, true, "artwork"))
	m = updated.(tui.Model)
	close(block)
	if m.GlanceHoldIDForTest() != 0 {
		t.Fatal("hold should clear once new frame is ready")
	}
	if m.BrowseSelectedNameForTest() != "Charmeleon" {
		t.Fatalf("committed name=%q want Charmeleon", m.BrowseSelectedNameForTest())
	}
}

func TestGlanceHoldClearsOnHelp(t *testing.T) {
	block := make(chan struct{})
	sprites := &fakeSprites{ansi: "COLD", native: true, block: block}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: &fakeDetails{},
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	id, _ := m.SelectedID()
	cells := m.CurrentSpriteCells()
	updated, _ = m.Update(tui.SpriteMsgNativeForTest(tui.SpriteMsgForTest(
		m.SpriteReqID(), m.SelectionGen(), id, cells, "\x1bP0;0;8q#A~~~\x1b\\", false,
	)))
	m = updated.(tui.Model)
	updated, cmd := m.Update(key("down"))
	m = updated.(tui.Model)
	if cmd != nil {
		c := cmd
		go func() { _ = c() }()
	}
	if m.GlanceHoldIDForTest() == 0 {
		t.Fatal("expected glance hold before help")
	}
	updated, _ = m.Update(key("?"))
	m = updated.(tui.Model)
	close(block)
	if m.GlanceHoldIDForTest() != 0 {
		t.Fatal("help should clear glance hold")
	}
}

func TestGlanceHoldSkipsStickyRawUntilCommit(t *testing.T) {
	const dcsA = "\x1bP0;0;8q#A~~~\x1b\\"
	const dcsB = "\x1bP0;0;8q#B~~~\x1b\\"
	block := make(chan struct{})
	sprites := &fakeSprites{ansi: "COLD", native: true, block: block}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: &fakeDetails{},
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	id, _ := m.SelectedID()
	cells := m.CurrentSpriteCells()
	updated, _ = m.Update(tui.SpriteMsgNativeForTest(tui.SpriteMsgForTest(
		m.SpriteReqID(), m.SelectionGen(), id, cells, dcsA, false,
	)))
	m = updated.(tui.Model)
	for m.NativePaintPendingForTest() {
		updated, _ = m.Update(tui.NativeOverlayFollowUpForTest())
		m = updated.(tui.Model)
	}

	updated, cmd := m.Update(key("down"))
	m = updated.(tui.Model)
	nextID, _ := m.SelectedID()
	if cmd != nil {
		c := cmd
		go func() { _ = c() }()
	}
	if m.GlanceHoldIDForTest() != id {
		t.Fatalf("hold=%d want %d", m.GlanceHoldIDForTest(), id)
	}
	// Clear→paint: no sticky Raw of species A while loading B.
	if m.NativePaintANSIForTest() != "" {
		t.Fatal("must clear paint tracking on selection change")
	}

	gen := m.SelectionGen()
	cells = m.CurrentSpriteCells()
	updated, cmd = m.Update(tui.ColdFrameMsgForTest(gen, nextID, cells, dcsB, false, true, "artwork"))
	m = updated.(tui.Model)
	close(block)
	if m.GlanceHoldIDForTest() != 0 {
		t.Fatal("ready frame must commit hold")
	}
	found := false
	for _, p := range collectRawPayloads(cmd) {
		if strings.Contains(p, dcsB) {
			found = true
		}
	}
	if !found {
		t.Fatal("after commit, immediate Raw must paint new sixel")
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
		t.Fatal("expected pending after dirty paint (insurance)")
	}
	// Follow-up re-paint clears pending — no perpetual loop.
	updated, cmd := m.Update(tui.NativeOverlayFollowUpForTest())
	m = updated.(tui.Model)
	if m.NativePaintPendingForTest() {
		t.Fatal("follow-up should clear nativePaintPending")
	}
	_ = cmd
	updated, _ = m.Update(tui.NativeOverlayFollowUpForTest())
	m = updated.(tui.Model)
	if m.NativePaintPendingForTest() {
		t.Fatal("follow-up must not leave pending without dirty")
	}
}

func TestNativeOverlayInsuranceRepaintAfterImmediate(t *testing.T) {
	const dcs = "\x1bP0;0;8q#1~~~\x1b\\"
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: &fakeDetails{},
		Sprites: &fakeSprites{ansi: "ART"},
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	id, _ := m.SelectedID()
	cells := m.CurrentSpriteCells()
	updated, cmd := m.Update(tui.SpriteMsgNativeForTest(tui.SpriteMsgForTest(
		m.SpriteReqID(), m.SelectionGen(), id, cells, dcs, false,
	)))
	m = updated.(tui.Model)
	if !m.NativePaintPendingForTest() {
		t.Fatal("ready paint must arm insurance pending")
	}
	found := false
	for _, p := range collectRawPayloads(cmd) {
		if strings.Contains(p, dcs) {
			found = true
		}
	}
	if !found {
		t.Fatal("expected first Raw with DCS")
	}
	updated, cmd = m.Update(tui.NativeOverlayFollowUpForTest())
	m = updated.(tui.Model)
	found = false
	for _, p := range collectRawPayloads(cmd) {
		if strings.Contains(p, dcs) {
			found = true
		}
	}
	if !found {
		t.Fatal("insurance follow-up must Raw-paint again")
	}
	if m.NativePaintPendingForTest() {
		t.Fatal("insurance must be single-shot")
	}
}

func TestNativeOverlayImmediateOnStableReady(t *testing.T) {
	const dcsA = "\x1bP0;0;8q#A~~~\x1b\\"
	const dcsB = "\x1bP0;0;8q#B~~~\x1b\\"
	block := make(chan struct{})
	sprites := &fakeSprites{ansi: "COLD", native: true, block: block}
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: &fakeDetails{},
		Sprites: sprites,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	id, _ := m.SelectedID()
	cells := m.CurrentSpriteCells()
	updated, _ = m.Update(tui.SpriteMsgNativeForTest(tui.SpriteMsgForTest(
		m.SpriteReqID(), m.SelectionGen(), id, cells, dcsA, false,
	)))
	m = updated.(tui.Model)
	for m.NativePaintPendingForTest() {
		updated, _ = m.Update(tui.NativeOverlayFollowUpForTest())
		m = updated.(tui.Model)
	}
	if m.NativePaintANSIForTest() != dcsA {
		t.Fatalf("species A paint=%q", m.NativePaintANSIForTest())
	}

	updated, cmd := m.Update(key("down"))
	m = updated.(tui.Model)
	nextID, _ := m.SelectedID()
	if nextID == id {
		t.Fatal("expected selection to move")
	}
	if cmd != nil {
		c := cmd
		go func() { _ = c() }()
	}
	if m.ImageStateForTest() != int(tui.ImageLoading) {
		_ = m.ForceSpriteLoadForTest()
	}
	if m.GlanceHoldIDForTest() == 0 {
		t.Fatal("expected text hold while loading neighbor")
	}
	gen := m.SelectionGen()
	cells = m.CurrentSpriteCells()
	updated, cmd = m.Update(tui.ColdFrameMsgForTest(gen, nextID, cells, dcsB, false, true, "artwork"))
	m = updated.(tui.Model)
	close(block)
	if m.GlanceHoldIDForTest() != 0 {
		t.Fatal("commit must clear hold before/with native paint")
	}
	found := false
	for _, p := range collectRawPayloads(cmd) {
		if strings.Contains(p, dcsB) {
			found = true
		}
	}
	if !found {
		t.Fatal("commit-ready paint must include immediate tea.Raw with B DCS")
	}
	if m.NativePaintANSIForTest() != dcsB {
		t.Fatalf("tracked=%q want B", m.NativePaintANSIForTest())
	}
}


func TestSpriteArtOriginTwoPane(t *testing.T) {
	d := views.BrowseData{
		Width: 120, Height: 52,
		ShowSprite: true, SpriteNative: true,
		LeftWidth: 40, RightWidth: 79,
		BodySize:    24,
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
	if m.NativePaintANSIForTest() != "" {
		t.Fatal("placeholder must clear tracked native paint")
	}
	_ = cmd
}

func TestNativeOverlayWrapsSixelInDECSC(t *testing.T) {
	const dcs = "\x1bP0;0;8q#1~~~\x1b\\"
	seq := tui.NativeOverlayPaintSeqForTest(5, 10, 20, 16, dcs)
	assertNativeOverlayPayload(t, seq, dcs)

	dcsAt := strings.Index(seq, dcs)
	if dcsAt < 0 {
		t.Fatal("expected DCS")
	}
	before := seq[:dcsAt]
	cup := "\x1b[5;10H"
	cupAt := strings.LastIndex(before, cup)
	if cupAt < 0 {
		t.Fatal("expected CUP immediately before DCS")
	}
	between := before[cupAt+len(cup):]
	if strings.Contains(between, "48;2;") || containsSpaces(between, 8) {
		t.Fatalf("after DCS CUP must not re-clear; got %q", between)
	}
	if !strings.Contains(between, "\x1b[?8452h") {
		t.Fatal("expected 8452h after CUP, before DCS")
	}
	if !strings.Contains(before[:cupAt], "48;2;") && !containsSpaces(before[:cupAt], 8) {
		t.Fatal("clear+paint seq must space-clear before the DCS CUP")
	}
	// artH=16 + slack=2 → last clear row = 5+18-1 = 22
	lastClearRow := "\x1b[22;10H"
	if !strings.Contains(before, lastClearRow) {
		t.Fatalf("expected clear slack through %s", lastClearRow)
	}
	on8452 := "\x1b[?8452h"
	onAt := strings.Index(seq, on8452)
	firstClear := strings.Index(seq, "48;2;")
	if firstClear < 0 {
		firstClear = strings.Index(seq, strings.Repeat(" ", 8))
	}
	if onAt < 0 || firstClear < 0 || onAt < firstClear {
		t.Fatalf("8452h must follow space-clear, onAt=%d clearAt=%d", onAt, firstClear)
	}
}

func TestNativeOverlayPaintOnlySkipsSpaceClear(t *testing.T) {
	const dcs = "\x1bP0;0;8q#1~~~\x1b\\"
	seq := tui.NativeOverlayPaintOnlySeqForTest(5, 10, 20, 16, dcs)
	assertNativeOverlayPayload(t, seq, dcs)
	if strings.Contains(seq, "48;2;") || containsSpaces(seq, 8) {
		t.Fatal("same-geometry overwrite must not space-clear before DCS")
	}
	if !strings.Contains(seq, "\x1b[?8452h") || !strings.Contains(seq, "\x1b[?8452l") {
		t.Fatal("overwrite paint still wraps DCS in 8452")
	}
	cup := "\x1b[5;10H"
	if !strings.Contains(seq, cup) {
		t.Fatal("expected CUP before DCS")
	}
}

func TestNativeOverlayInsuranceOverwriteSkipsClear(t *testing.T) {
	const dcs = "\x1bP0;0;8q#1~~~\x1b\\"
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: &fakeDetails{},
		Sprites: &fakeSprites{ansi: "ART"},
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	id, _ := m.SelectedID()
	cells := m.CurrentSpriteCells()
	updated, cmd := m.Update(tui.SpriteMsgNativeForTest(tui.SpriteMsgForTest(
		m.SpriteReqID(), m.SelectionGen(), id, cells, dcs, false,
	)))
	m = updated.(tui.Model)
	first := strings.Join(collectRawPayloads(cmd), "")
	if !strings.Contains(first, dcs) {
		t.Fatal("expected first Raw with DCS")
	}
	if !strings.Contains(first, "48;2;") && !containsSpaces(first, 8) {
		t.Fatal("first paint (no prior geometry) must space-clear")
	}

	updated, cmd = m.Update(tui.NativeOverlayFollowUpForTest())
	m = updated.(tui.Model)
	second := strings.Join(collectRawPayloads(cmd), "")
	if !strings.Contains(second, dcs) {
		t.Fatal("insurance follow-up must Raw-paint again")
	}
	if strings.Contains(second, "48;2;") || containsSpaces(second, 8) {
		t.Fatal("same-geometry insurance paint must overwrite without space-clear")
	}
}

// Browsing a new species loads its dossier core in the background. Its arrival
// changes header genus / About cells (outside the art box) but not the Sixel
// payload. It must repaint once, in place — no space-clear flash and no delayed
// ~60ms insurance shot (that second, late redraw read as lag on WT).
func TestBrowseCoreArrivalRepaintsInPlaceWithoutInsurance(t *testing.T) {
	const dcs = "\x1bP0;0;8q#1~~~\x1b\\"
	m := tui.NewModel(tui.Options{
		Index:   fixtureIndex(),
		Details: &fakeDetails{},
		Sprites: &fakeSprites{ansi: "ART"},
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 52})
	m = updated.(tui.Model)
	id, _ := m.SelectedID()
	cells := m.CurrentSpriteCells()
	updated, _ = m.Update(tui.SpriteMsgNativeForTest(tui.SpriteMsgForTest(
		m.SpriteReqID(), m.SelectionGen(), id, cells, dcs, false,
	)))
	m = updated.(tui.Model)
	// Settle the single insurance shot so the paint is stable.
	for m.NativePaintPendingForTest() {
		updated, _ = m.Update(tui.NativeOverlayFollowUpForTest())
		m = updated.(tui.Model)
	}
	if m.NativePaintANSIForTest() != dcs {
		t.Fatalf("expected settled paint, got %q", m.NativePaintANSIForTest())
	}

	dossier := domain.Dossier{
		Pokemon: domain.Pokemon{ID: id},
		Species: domain.Species{Genus: "Seed"},
	}
	updated, cmd := m.Update(tui.DossierCoreMsgForTest(m.SelectionGen(), id, dossier, false, nil))
	m = updated.(tui.Model)

	if m.NativePaintPendingForTest() {
		t.Fatal("core arrival must not arm the 60ms insurance repaint")
	}
	joined := strings.Join(collectRawPayloads(cmd), "")
	if !strings.Contains(joined, dcs) {
		t.Fatal("core arrival must repaint the Sixel in place")
	}
	if strings.Contains(joined, "48;2;") || containsSpaces(joined, 8) {
		t.Fatal("same-geometry core repaint must not space-clear (flash)")
	}
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

func TestWarmProgressNativeFooterVisibleViaRaw(t *testing.T) {
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

	updated, cmd := m.Update(tui.WarmProgressNativeMsgForTest(5, 10, 40))
	m = updated.(tui.Model)
	if hint := m.BrowseWarmHintForTest(); hint == "" {
		t.Fatal("native warm must populate footer progress state")
	}
	if vis := m.BrowseVisibleWarmHintForTest(); vis != "" {
		t.Fatalf("warm hint must not render over native art: %q", vis)
	}
	if len(collectRawPayloads(cmd)) == 0 {
		t.Fatal("native warm must patch footer via tea.Raw")
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
		t.Fatal("native warm must track footer progress while sprite visible")
	}
	if vis := m.BrowseVisibleWarmHintForTest(); vis != "" {
		t.Fatalf("warm hint must not render over native art: %q", vis)
	}
	if m.NativeOverlayDirtyForTest() || m.NativePaintPendingForTest() {
		t.Fatal("warm tick must not trigger overlay repaint (causes Sixel flash)")
	}
}

func TestNativeWarmDoesNotPatchFooterWhileSpriteVisible(t *testing.T) {
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
	if m.BrowseWarmHintForTest() == "" {
		t.Fatal("native warm must populate footer hint state")
	}
	if raws := collectRawPayloads(cmd); len(raws) == 0 {
		t.Fatalf("native warm must patch footer via tea.Raw, got %v", raws)
	}
}

func TestNativeWarmCompleteClearsFooterViaRaw(t *testing.T) {
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
		t.Fatalf("native warm must show in-progress footer state")
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
	if raws := collectRawPayloads(cmd); len(raws) == 0 {
		t.Fatalf("native complete must clear footer via tea.Raw")
	}
}

func TestArtworkWarmClearsStaleFooterUnderNativeSprite(t *testing.T) {
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

	updated, _ = m.Update(tui.WarmProgressNativeMsgForTest(5, 10, 40))
	m = updated.(tui.Model)
	if m.BrowseWarmHintForTest() == "" {
		t.Fatal("native warm must populate footer progress state")
	}

	updated, cmd := m.Update(tui.WarmProgressNativeMsgForTest(10, 10, 40))
	m = updated.(tui.Model)
	if m.BrowseWarmHintForTest() != "" {
		t.Fatalf("native complete must clear footer state, got %q", m.BrowseWarmHintForTest())
	}
	if raws := collectRawPayloads(cmd); len(raws) == 0 {
		t.Fatal("native complete must clear footer via tea.Raw")
	}

	updated, cmd = m.Update(tui.WarmProgressSpeciesStageMsgForTest(981, 1025, 0, tui.WarmStageArtwork))
	m = updated.(tui.Model)
	if hint := m.BrowseWarmHintForTest(); hint != "" {
		t.Fatalf("artwork warm must hide footer, got %q", hint)
	}
	if vis := m.BrowseVisibleWarmHintForTest(); vis != "" {
		t.Fatalf("artwork warm must not render over native art: %q", vis)
	}
	if raws := collectRawPayloads(cmd); len(raws) != 0 {
		t.Fatalf("artwork warm must not repaint footer, got raw %v", raws)
	}
}

func TestNativeWarmDoneEqualsTotalClearsFooterViaRaw(t *testing.T) {
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
	if raws := collectRawPayloads(cmd); len(raws) == 0 {
		t.Fatalf("native warm must clear footer at completion: %v", raws)
	}
}

type fakeNativeWarmer struct{}

func (fakeNativeWarmer) EnsureStarted(context.Context, int) {}
func (fakeNativeWarmer) Prioritize(int)                     {}
func (fakeNativeWarmer) SetNeighborsFromList([]domain.SpeciesSummary, int, int) {
}
func (fakeNativeWarmer) SetBuckets([]int)       {}
func (fakeNativeWarmer) SetCells(int)           {}
func (fakeNativeWarmer) Snapshot() (int, int)   { return 0, 0 }
func (fakeNativeWarmer) NoteActivity()          {}
func (fakeNativeWarmer) SetForegroundBusy(bool) {}

func TestNativeWarmHintVisibleBeforeSpriteViaRaw(t *testing.T) {
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
		t.Fatal("native warm must track footer progress before sprite paint")
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
