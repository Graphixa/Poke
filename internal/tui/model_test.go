package tui_test

import (
	"context"
	"errors"
	"fmt"
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
	loadCalls    atomic.Int32
	evoCalls     atomic.Int32
	encCalls     atomic.Int32
	moveCalls    atomic.Int32
	blockLoad    chan struct{}
	loadErr      error
	evolutionErr error
}

func (f *fakeDetails) Load(_ context.Context, speciesID int) (domain.Pokemon, domain.Species, error) {
	f.loadCalls.Add(1)
	if f.blockLoad != nil {
		<-f.blockLoad
	}
	if f.loadErr != nil {
		return domain.Pokemon{}, domain.Species{}, f.loadErr
	}
	return domain.Pokemon{
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
		}, domain.Species{
			ID: speciesID, Slug: "x", DisplayName: "X", Genus: "Lizard Pokémon",
			DefaultPokemonID: speciesID, EvolutionChainID: 2,
			Generation: "generation-i", Habitat: "mountain", Shape: "upright", Color: "red",
			GrowthRate: "medium-slow", CaptureRate: 45, BaseHappiness: 70, GenderRate: 1,
			FlavorTextEntries: []domain.FlavorText{
				{Language: "en", Version: "red", VersionOrder: 1, Text: "Prefers hot places."},
				{Language: "en", Version: "sword", VersionOrder: 40, Text: "Obviously prefers hot places."},
			},
		}, nil
}

func (f *fakeDetails) Evolution(context.Context, domain.Species) (domain.EvolutionTree, error) {
	f.evoCalls.Add(1)
	if f.evolutionErr != nil {
		return domain.EvolutionTree{}, f.evolutionErr
	}
	lv := 16
	return domain.EvolutionTree{Root: domain.EvolutionNode{
		DisplayName: "Charmander",
		Children: []domain.EvolutionBranch{{
			Conditions: []domain.EvolutionCondition{{Trigger: "level-up", MinimumLevel: &lv}},
			Target:     domain.EvolutionNode{DisplayName: "Charmeleon"},
		}},
	}}, nil
}

func (f *fakeDetails) Encounters(context.Context, int) ([]domain.Encounter, error) {
	f.encCalls.Add(1)
	return []domain.Encounter{{
		Version: "red", Area: "route-3", Method: "walk", MinimumLevel: 3, MaximumLevel: 5, Chance: 20,
	}}, nil
}

func (f *fakeDetails) LevelUpMoves(context.Context, domain.Pokemon) ([]tui.LevelUpMove, error) {
	f.moveCalls.Add(1)
	pow := 40
	return []tui.LevelUpMove{{
		Level: 1,
		Move:  domain.Move{Slug: "scratch", DisplayName: "Scratch", Type: "normal", DamageClass: "physical", Power: &pow},
	}}, nil
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
		// Skip delayed tick follow-ups so tests do not hang on summary auto-scroll.
		typeName := fmt.Sprintf("%T", msg)
		if strings.Contains(typeName, "summaryTickMsg") {
			next, _ := (*model).Update(msg)
			*model = next
			return
		}
		next, nextCmd := (*model).Update(msg)
		*model = next
		drainCmds(t, model, nextCmd)
	}
}

func TestSearchFilterAndQuitEligibility(t *testing.T) {
	details := &fakeDetails{}
	m := tui.NewModel(tui.Options{Index: fixtureIndex(), Details: details})
	m = apply(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})

	m = apply(t, m, key("c"))
	if m.Query() != "c" {
		t.Fatalf("query = %q", m.Query())
	}
	if m.CanQuitWithQ() {
		t.Fatal("q should not quit while query non-empty")
	}
	m = apply(t, m, key("q"))
	if m.Query() != "cq" {
		t.Fatalf("typed q should filter, got %q", m.Query())
	}

	m = apply(t, m, key("esc"))
	if m.Query() != "" {
		t.Fatalf("esc clear = %q", m.Query())
	}
	if !m.CanQuitWithQ() {
		t.Fatal("q should quit when empty")
	}
}

func TestSelectionTabsScrollHelp(t *testing.T) {
	details := &fakeDetails{}
	pikachu := domain.SpeciesSummary{ID: 25, Slug: "pikachu", DisplayName: "Pikachu", Types: []string{"electric"}}
	m := tui.NewModel(tui.Options{Index: fixtureIndex(), Details: details, Selected: &pikachu})
	m = apply(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	var model tea.Model = m
	drainCmds(t, &model, m.Init())
	m = model.(tui.Model)

	if id, ok := m.SelectedID(); !ok || id != 25 {
		t.Fatalf("selected = %d ok=%v", id, ok)
	}

	m = apply(t, m, key("tab"))
	if m.ActiveTab() != tui.TabEvolution {
		t.Fatalf("tab = %v", m.ActiveTab())
	}
	if details.evoCalls.Load() != 1 {
		t.Fatalf("evolution loads = %d", details.evoCalls.Load())
	}
	m = apply(t, m, key("tab"))       // locations
	m = apply(t, m, key("shift+tab")) // back to evolution
	if details.evoCalls.Load() != 1 {
		t.Fatalf("evolution refetched: %d", details.evoCalls.Load())
	}

	m = apply(t, m, key("pgdown"), key("home"))
	if m.DetailOffset() != 0 {
		t.Fatalf("home offset = %d", m.DetailOffset())
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

	// Stale core message for a previous generation must be ignored.
	stale := tui.CoreMsgForTest(0, 4, domain.Pokemon{ID: 4, DisplayName: "Stale"}, domain.Species{ID: 4}, nil)
	m = apply(t, m, key("down")) // selection change bumps gen
	before := details.loadCalls.Load()
	m = apply(t, m, stale)
	if res := m.ResourceName(4); res == "Stale" {
		t.Fatal("stale core applied")
	}
	_ = before
}

func TestLazyLoadNoRefetch(t *testing.T) {
	details := &fakeDetails{}
	m := tui.NewModel(tui.Options{Index: fixtureIndex(), Details: details})
	m = apply(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	var model tea.Model = m
	drainCmds(t, &model, m.Init())
	m = model.(tui.Model)

	m = apply(t, m, key("tab"), key("tab"), key("tab")) // Stats->Evo->Loc->Moves
	if details.moveCalls.Load() != 1 || details.encCalls.Load() != 1 || details.evoCalls.Load() != 1 {
		t.Fatalf("loads evo=%d enc=%d move=%d", details.evoCalls.Load(), details.encCalls.Load(), details.moveCalls.Load())
	}
	// Cycle back through the same tabs — cached, no refetch.
	for i := 0; i < 6; i++ {
		m = apply(t, m, key("tab"))
	}
	if details.moveCalls.Load() != 1 || details.encCalls.Load() != 1 || details.evoCalls.Load() != 1 {
		t.Fatalf("refetch evo=%d enc=%d move=%d", details.evoCalls.Load(), details.encCalls.Load(), details.moveCalls.Load())
	}
}

func TestUnboundKeys(t *testing.T) {
	details := &fakeDetails{}
	m := tui.NewModel(tui.Options{Index: fixtureIndex(), Details: details})
	m = apply(t, m, tea.WindowSizeMsg{Width: 120, Height: 40}, key("enter"), key("r"), key("/"))
	if m.Query() != "" {
		t.Fatalf("unbound keys mutated query: %q", m.Query())
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
	m = apply(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	var model tea.Model = m
	drainCmds(t, &model, m.Init())
	m = model.(tui.Model)
	view := m.View().Content
	if !strings.Contains(view, "offline") && !strings.Contains(view, "[ERROR]") {
		t.Fatalf("expected error in view, got %q", view)
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
	m = apply(t, m, key("q"))
	if !m.Quitting() {
		t.Fatal("q should quit when terminal too small")
	}
}
