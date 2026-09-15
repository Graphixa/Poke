package tui_test

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/search"
	"github.com/Graphixa/Pokedex/internal/tui"
)

func fixtureIndex() *search.Index {
	return search.Build([]domain.SpeciesSummary{
		{ID: 4, Slug: "charmander", DisplayName: "Charmander", Types: []string{"fire"}},
		{ID: 5, Slug: "charmeleon", DisplayName: "Charmeleon", Types: []string{"fire"}},
		{ID: 6, Slug: "charizard", DisplayName: "Charizard", Types: []string{"fire", "flying"}},
		{ID: 25, Slug: "pikachu", DisplayName: "Pikachu", Types: []string{"electric"}},
		{ID: 133, Slug: "eevee", DisplayName: "Eevee", Types: []string{"normal"}},
	})
}

type fakeDetails struct {
	loadCalls   atomic.Int32
	coreCalls   atomic.Int32
	extrasCalls atomic.Int32
	blockLoad   chan struct{}
	loadErr     error
	coreOnly    bool
}

func (f *fakeDetails) makeDossier(speciesID int) (domain.Dossier, error) {
	if f.blockLoad != nil {
		<-f.blockLoad
	}
	if f.loadErr != nil {
		return domain.Dossier{}, f.loadErr
	}
	lv := 16
	pow := 40
	evo := domain.EvolutionTree{Root: domain.EvolutionNode{
		DisplayName: "Charmander",
		Children: []domain.EvolutionBranch{{
			Conditions: []domain.EvolutionCondition{{Trigger: "level-up", MinimumLevel: &lv}},
			Target:     domain.EvolutionNode{DisplayName: "Charmeleon"},
		}},
	}}
	return domain.Dossier{
		Pokemon: domain.Pokemon{
			ID: speciesID, Slug: "x", DisplayName: "X",
			HeightDecimeter: 6, WeightHectogram: 85, BaseExperience: 62,
			Types: []domain.TypeSlot{{Slot: 1, Name: "fire"}},
			Abilities: []domain.AbilitySlot{
				{Slot: 1, Name: "blaze"},
				{Slot: 3, Name: "solar-power", IsHidden: true},
			},
			Stats: domain.Stats{HP: 39, Attack: 52, Defense: 43, SpecialAttack: 60, SpecialDefense: 50, Speed: 65},
			Moves: []domain.LearnedMove{{
				MoveID: 10, Slug: "scratch",
				Learnings: []domain.MoveLearning{{VersionGroup: "scarlet-violet", VersionGroupOrder: 25, Method: "level-up", Level: 1}},
			}},
		},
		Species: domain.Species{
			ID: speciesID, Slug: "x", DisplayName: "X", Genus: "Lizard Pokémon",
			DefaultPokemonID: speciesID, EvolutionChainID: 2,
			Generation: "generation-i", Habitat: "mountain", Shape: "upright", Color: "red",
			GrowthRate: "medium-slow", CaptureRate: 45, BaseHappiness: 70, GenderRate: 1,
			FlavorTextEntries: []domain.FlavorText{
				{Language: "en", Version: "red", VersionOrder: 1, Text: "Prefers hot places."},
				{Language: "en", Version: "sword", VersionOrder: 40, Text: "Obviously prefers hot places."},
			},
		},
		Evolution:  &evo,
		Encounters: []domain.Encounter{{Version: "red", Area: "route-3", Method: "walk", MinimumLevel: 3, MaximumLevel: 5, Chance: 20}},
		LevelUpMoves: []domain.LevelUpMove{{
			Level: 1,
			Move:  domain.Move{Slug: "scratch", DisplayName: "Scratch", Type: "normal", DamageClass: "physical", Power: &pow},
		}},
	}, nil
}

func (f *fakeDetails) LoadDossierCore(_ context.Context, speciesID int, _ string) (domain.Dossier, bool, error) {
	f.loadCalls.Add(1)
	f.coreCalls.Add(1)
	d, err := f.makeDossier(speciesID)
	if err != nil {
		return domain.Dossier{}, false, err
	}
	if f.coreOnly {
		d.Evolution = nil
		d.Encounters = nil
		d.LevelUpMoves = nil
		return d, false, nil
	}
	return d, true, nil
}

func (f *fakeDetails) LoadDossierExtras(_ context.Context, speciesID int, _ string, core domain.Dossier) (domain.Dossier, error) {
	f.extrasCalls.Add(1)
	if f.loadErr != nil {
		return domain.Dossier{}, f.loadErr
	}
	d, err := f.makeDossier(speciesID)
	if err != nil {
		return core, err
	}
	return d, nil
}

func key(s string) tea.KeyPressMsg {
	msg := tea.KeyPressMsg{}
	switch s {
	case "up":
		msg.Code = tea.KeyUp
	case "down":
		msg.Code = tea.KeyDown
	case "tab":
		msg.Code = tea.KeyTab
	case "shift+tab":
		msg.Code = tea.KeyTab
		msg.Mod = tea.ModShift
	case "esc":
		msg.Code = tea.KeyEscape
	case "backspace":
		msg.Code = tea.KeyBackspace
	case "pgup":
		msg.Code = tea.KeyPgUp
	case "pgdown":
		msg.Code = tea.KeyPgDown
	case "home":
		msg.Code = tea.KeyHome
	case "end":
		msg.Code = tea.KeyEnd
	case "ctrl+c":
		msg.Code = 'c'
		msg.Mod = tea.ModCtrl
	case "enter":
		msg.Code = tea.KeyEnter
	case "space":
		msg.Code = ' '
		msg.Text = " "
	default:
		if s == "?" {
			msg.Code = '?'
			msg.Text = "?"
			return msg
		}
		if len(s) == 1 {
			msg.Code = rune(s[0])
			msg.Text = s
			return msg
		}
		// Fall back to setting Text for multi-rune; String() may not match.
		msg.Text = s
	}
	return msg
}

func apply(t *testing.T, m tui.Model, msgs ...tea.Msg) tui.Model {
	t.Helper()
	var model tea.Model = m
	for _, msg := range msgs {
		next, cmd := model.Update(msg)
		model = next
		if cmd == nil {
			continue
		}
		// Drain batch messages synchronously when they return Msg directly.
		drainCmds(t, &model, cmd)
	}
	return model.(tui.Model)
}

func drainCmds(t *testing.T, model *tea.Model, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	msg := cmd()
	switch msg := msg.(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			drainCmds(t, model, c)
		}
	case nil:
		return
	default:
		next, nextCmd := (*model).Update(msg)
		*model = next
		drainCmds(t, model, nextCmd)
	}
}

func TestSearchFilterTypesQ(t *testing.T) {
	details := &fakeDetails{}
	m := tui.NewModel(tui.Options{Index: fixtureIndex(), Details: details})
	m = apply(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})

	m = apply(t, m, key("c"))
	if m.Query() != "c" {
		t.Fatalf("query = %q", m.Query())
	}
	m = apply(t, m, key("q"))
	if m.Query() != "cq" {
		t.Fatalf("q must type into search, got %q", m.Query())
	}
	if m.Quitting() {
		t.Fatal("q must not quit")
	}

	m = apply(t, m, key("esc"))
	if m.Query() != "" {
		t.Fatalf("esc clear = %q", m.Query())
	}
}

func TestSelectionViewsDialogHelp(t *testing.T) {
	details := &fakeDetails{}
	pikachu := domain.SpeciesSummary{ID: 25, Slug: "pikachu", DisplayName: "Pikachu", Types: []string{"electric"}}
	m := tui.NewModel(tui.Options{Index: fixtureIndex(), Details: details, Selected: &pikachu})
	m = apply(t, m, tea.WindowSizeMsg{Width: 120, Height: 52})
	var model tea.Model = m
	drainCmds(t, &model, m.Init())
	m = model.(tui.Model)

	if id, ok := m.SelectedID(); !ok || id != 25 {
		t.Fatalf("selected = %d ok=%v", id, ok)
	}

	// Tab cycles Glance ↔ Art (digits stay free for dex-id search).
	m = apply(t, m, key("tab"))
	if m.ActiveBrowseView() != tui.BrowseArt {
		t.Fatalf("tab → Art, got %v", m.ActiveBrowseView())
	}
	if f := m.LayoutFlags(); !f.ShowSprite || f.SpriteSize != 40 {
		t.Fatalf("Art view show=%v size=%d want 40", f.ShowSprite, f.SpriteSize)
	}
	m = apply(t, m, key("tab"))
	if m.ActiveBrowseView() != tui.BrowseGlance {
		t.Fatalf("tab → Glance, got %v", m.ActiveBrowseView())
	}
	if details.coreCalls.Load() < 1 {
		t.Fatalf("browse should load dossier core for About, got %d", details.coreCalls.Load())
	}
	if details.extrasCalls.Load() != 0 {
		t.Fatalf("browse must not load extras, got %d", details.extrasCalls.Load())
	}

	selectedBefore, _ := m.SelectedID()
	m = apply(t, m, key("enter"))
	if !m.DialogVisible() {
		t.Fatal("enter should open dialog")
	}
	if m.ActiveDialogSection() != tui.DialogOverview {
		t.Fatalf("section = %v want Overview", m.ActiveDialogSection())
	}
	if details.coreCalls.Load() != 1 {
		t.Fatalf("core loads = %d want 1", details.coreCalls.Load())
	}
	m = apply(t, m, key("tab"))
	if m.ActiveDialogSection() != tui.DialogStats {
		t.Fatalf("tab section = %v want Stats", m.ActiveDialogSection())
	}
	m = apply(t, m, key("right"))
	if m.ActiveDialogSection() != tui.DialogEvolution {
		t.Fatalf("right section = %v want Evolution", m.ActiveDialogSection())
	}
	m = apply(t, m, key("left"))
	if m.ActiveDialogSection() != tui.DialogStats {
		t.Fatalf("left section = %v want Stats", m.ActiveDialogSection())
	}
	m = apply(t, m, key("shift+tab"), key("shift+tab"))
	if m.ActiveDialogSection() != tui.DialogBiology {
		t.Fatalf("shift-tab wrap section = %v want Biology", m.ActiveDialogSection())
	}
	m = apply(t, m, key("tab"))
	if m.ActiveDialogSection() != tui.DialogOverview {
		t.Fatalf("tab wrap section = %v want Overview", m.ActiveDialogSection())
	}

	// Wide rail: ↑/↓ move the section list (body scroll is PgUp/PgDn).
	m = apply(t, m, tea.WindowSizeMsg{Width: 120, Height: 40}, key("down"))
	if m.ActiveDialogSection() != tui.DialogStats {
		t.Fatalf("rail down section = %v want Stats", m.ActiveDialogSection())
	}
	m = apply(t, m, key("up"))
	if m.ActiveDialogSection() != tui.DialogOverview {
		t.Fatalf("rail up section = %v want Overview", m.ActiveDialogSection())
	}
	m = apply(t, m, key("esc"))
	if m.DialogVisible() {
		t.Fatal("esc should close dialog")
	}
	if id, ok := m.SelectedID(); !ok || id != selectedBefore {
		t.Fatalf("esc must preserve selection: got %d ok=%v want %d", id, ok, selectedBefore)
	}

	m = apply(t, m, key("?"))
	if !m.HelpVisible() {
		t.Fatal("help not shown")
	}
	view := m.View().Content
	if !strings.Contains(view, "Browse help") {
		t.Fatalf("help view missing: %q", view)
	}
	prevQuery := m.Query()
	m = apply(t, m, key("c")) // ignored while help open
	if m.Query() != prevQuery {
		t.Fatal("help should not mutate search")
	}
	m = apply(t, m, key("esc"))
	if m.HelpVisible() {
		t.Fatal("esc should close help")
	}
}

func TestAsyncRaceSuppression(t *testing.T) {
	details := &fakeDetails{}
	m := tui.NewModel(tui.Options{Index: fixtureIndex(), Details: details})
	m = apply(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	var model tea.Model = m
	drainCmds(t, &model, m.Init())
	m = model.(tui.Model)

	stale := tui.DossierMsgForTest(0, 4, domain.Dossier{
		Pokemon: domain.Pokemon{ID: 4, DisplayName: "Stale"},
		Species: domain.Species{ID: 4},
	}, nil)
	m = apply(t, m, key("down"))
	m = apply(t, m, stale)
	if res := m.ResourceName(4); res == "Stale" {
		t.Fatal("stale dossier applied")
	}
}

func TestProgressiveCoreBeforeExtras(t *testing.T) {
	details := &fakeDetails{coreOnly: true}
	m := tui.NewModel(tui.Options{Index: fixtureIndex(), Details: details})
	m = apply(t, m, tea.WindowSizeMsg{Width: 120, Height: 52})
	var model tea.Model = m
	drainCmds(t, &model, m.Init())
	m = model.(tui.Model)

	m = apply(t, m, key("enter"))
	if details.coreCalls.Load() != 1 {
		t.Fatalf("core loads = %d", details.coreCalls.Load())
	}
	if details.extrasCalls.Load() != 1 {
		t.Fatalf("extras loads = %d", details.extrasCalls.Load())
	}
	if !m.ExtrasReadyForTest(4) {
		t.Fatal("expected extras after enter")
	}
	view := m.View().Content
	if strings.Contains(view, "Loading…") {
		t.Fatalf("summary should paint before extras complete, got %q", view)
	}
}

func TestEnterDossierNoRefetch(t *testing.T) {
	details := &fakeDetails{}
	m := tui.NewModel(tui.Options{Index: fixtureIndex(), Details: details})
	m = apply(t, m, tea.WindowSizeMsg{Width: 120, Height: 52})
	var model tea.Model = m
	drainCmds(t, &model, m.Init())
	m = model.(tui.Model)

	m = apply(t, m, key("enter"))
	if details.loadCalls.Load() != 1 {
		t.Fatalf("dossier loads = %d", details.loadCalls.Load())
	}
	for i := 0; i < 12; i++ {
		m = apply(t, m, key("tab"))
	}
	if details.loadCalls.Load() != 1 {
		t.Fatalf("dossier refetched: %d", details.loadCalls.Load())
	}
	m = apply(t, m, key("esc"), key("enter"))
	if details.loadCalls.Load() != 1 {
		t.Fatalf("reopen should use cached dossier, got %d", details.loadCalls.Load())
	}
}

func TestSearchTypesLettersIncludingR(t *testing.T) {
	details := &fakeDetails{}
	m := tui.NewModel(tui.Options{Index: fixtureIndex(), Details: details})
	m = apply(t, m, tea.WindowSizeMsg{Width: 120, Height: 52}, key("r"), key("/"), key("a"))
	if m.Query() != "r/a" {
		t.Fatalf("letters must type into search, got %q", m.Query())
	}
}

func TestEscClearsThenQuits(t *testing.T) {
	details := &fakeDetails{}
	m := tui.NewModel(tui.Options{Index: fixtureIndex(), Details: details, Prefill: "char"})
	m = apply(t, m, tea.WindowSizeMsg{Width: 120, Height: 52}, key("esc"))
	if m.Query() != "" {
		t.Fatalf("first esc clears search, got %q", m.Query())
	}
	if m.Quitting() {
		t.Fatal("first esc must not quit while clearing")
	}
	m = apply(t, m, key("esc"))
	if !m.Quitting() {
		t.Fatal("second esc on empty search must quit")
	}
}

func TestPrefillAndSelected(t *testing.T) {
	details := &fakeDetails{}
	m := tui.NewModel(tui.Options{Index: fixtureIndex(), Details: details, Prefill: "char"})
	if m.Query() != "char" {
		t.Fatalf("prefill = %q", m.Query())
	}
	sel := domain.SpeciesSummary{ID: 133, Slug: "eevee", DisplayName: "Eevee"}
	m = tui.NewModel(tui.Options{Index: fixtureIndex(), Details: details, Selected: &sel})
	if id, ok := m.SelectedID(); !ok || id != 133 {
		t.Fatalf("selected = %v %v", id, ok)
	}
}

func TestCtrlCAlwaysQuits(t *testing.T) {
	details := &fakeDetails{}
	m := tui.NewModel(tui.Options{Index: fixtureIndex(), Details: details, Prefill: "abc"})
	m = apply(t, m, key("ctrl+c"))
	if !m.Quitting() {
		t.Fatal("ctrl+c should quit")
	}
}

func TestLoadErrorSurface(t *testing.T) {
	details := &fakeDetails{loadErr: errors.New("offline")}
	m := tui.NewModel(tui.Options{Index: fixtureIndex(), Details: details})
	m = apply(t, m, tea.WindowSizeMsg{Width: 120, Height: 52})
	var model tea.Model = m
	drainCmds(t, &model, m.Init())
	m = model.(tui.Model)
	m = apply(t, m, key("enter"))
	view := m.View().Content
	if !strings.Contains(view, "offline") && !strings.Contains(view, "[ERROR]") {
		t.Fatalf("expected error in dialog view, got %q", view)
	}
}

func TestEmptyResultsAndTooSmall(t *testing.T) {
	details := &fakeDetails{}
	m := tui.NewModel(tui.Options{Index: fixtureIndex(), Details: details, Prefill: "zzzz-no-match"})
	m = apply(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	view := m.View().Content
	if !strings.Contains(view, "No matches") {
		t.Fatalf("expected empty results cue:\n%s", view)
	}

	m = apply(t, m, tea.WindowSizeMsg{Width: 30, Height: 10})
	view = m.View().Content
	if !strings.Contains(view, "too small") && !strings.Contains(view, "Too small") && !strings.Contains(strings.ToLower(view), "too small") {
		t.Fatalf("expected too-small cue:\n%s", view)
	}
	m = apply(t, m, key("esc"))
	if !m.Quitting() {
		t.Fatal("esc should quit when terminal too small")
	}
}
