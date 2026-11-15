package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Graphixa/Pokedex/internal/domain"
)

type stubCatalog struct {
	versions []domain.Version
	forms    []domain.SelectableForm
	pokemon  map[int]domain.Pokemon
	vg       map[int]int
	moves    map[int][]domain.LevelUpMove
}

func (s stubCatalog) CatalogVersions() ([]domain.Version, error) { return s.versions, nil }
func (s stubCatalog) VersionGroupIDForVersion(id int) (int, bool) {
	vg, ok := s.vg[id]
	return vg, ok
}
func (s stubCatalog) SelectableFormsForSpecies(int) ([]domain.SelectableForm, error) {
	return s.forms, nil
}
func (s stubCatalog) FormOptionsForGame(int, int) ([]domain.FormOption, error) {
	var out []domain.FormOption
	for _, f := range s.forms {
		out = append(out, domain.FormOption{Form: f, Compat: domain.FormCompatSupported})
	}
	return out, nil
}
func (s stubCatalog) ResolveDossierContext(speciesID, pokemonID, formID, gameVersionID int) (domain.ResolvedDossierContext, error) {
	return domain.ResolvedDossierContext{
		SpeciesID: speciesID, PokemonID: pokemonID, FormID: formID,
		GameVersionID: gameVersionID, Compat: domain.FormCompatSupported,
		FormLabel: "Default",
	}, nil
}
func (s stubCatalog) LevelUpMovesForGroup(pokemonID, vg int) ([]domain.LevelUpMove, error) {
	return s.moves[pokemonID*1000+vg], nil
}
func (s stubCatalog) MovesForGroup(pokemonID, vg int) ([]domain.LevelUpMove, error) {
	return s.moves[pokemonID*1000+vg], nil
}
func (s stubCatalog) PokemonByID(_ context.Context, id int) (domain.Pokemon, error) {
	return s.pokemon[id], nil
}
func (s stubCatalog) TypeChart() (map[string]domain.TypeInfo, error) {
	return map[string]domain.TypeInfo{}, nil
}
func (s stubCatalog) AbilityByID(int) (domain.Ability, bool) { return domain.Ability{}, false }
func (s stubCatalog) AbilityBySlug(string) (domain.Ability, bool) {
	return domain.Ability{}, false
}

func TestDialogContextLines(t *testing.T) {
	m := NewModel(Options{})
	m.dialogForms = []domain.SelectableForm{
		{PokemonID: 25, DisplayName: "Default", IsDefault: true},
		{PokemonID: 10080, DisplayName: "Partner"},
	}
	m.dialogFormIdx = 1
	m.dialogGameLabel = "Pearl"
	m.dialogGameID = 12
	m.dialogSection = DialogMoves
	m.dialogMoveMethod = "level-up"
	lines := m.dialogContextLines(80)
	joined := strings.Join(lines, "\n")
	// Form/Game/Method are top-border chips, not body context.
	for _, ban := range []string{"Form:", "Game:", "Method:"} {
		if strings.Contains(joined, ban) {
			t.Fatalf("body must hide %s: %v", ban, lines)
		}
	}
	m.dialogNotice = "Fell back to default form"
	lines = m.dialogContextLines(80)
	if len(lines) != 1 || lines[0] != m.dialogNotice {
		t.Fatalf("notice only: %v", lines)
	}
}

func TestEnsureMoveFocusVisible(t *testing.T) {
	m := NewModel(Options{})
	m.width = 100
	m.height = 24
	m.showDialog = true
	m.dialogSection = DialogMoves
	m.dialogContentFocus = true
	m.dialogGameID = 12
	m.dialogGameLabel = "Pearl"
	m.dialogMoveMethod = "level-up"
	pow := 40
	rows := make([]domain.LevelUpMove, 40)
	for i := range rows {
		rows[i] = domain.LevelUpMove{
			Level:  i + 1,
			Method: "level-up",
			Move:   domain.Move{DisplayName: fmt.Sprintf("Move%d", i+1), Type: "normal", DamageClass: "physical", Power: &pow},
		}
	}
	m.resources = map[int]*resourceCache{
		1: {
			HasCore:  true,
			HasMoves: true,
			Moves:    rows,
			Pokemon:  domain.Pokemon{ID: 1},
			Species:  domain.Species{ID: 1},
		},
	}
	m.filtered = []domain.SpeciesSummary{{ID: 1}}
	m.cursor = 0
	m.syncDialogViewport()
	vis := m.dialogPageSize()
	if vis < 2 {
		t.Fatalf("page size %d too small for test", vis)
	}
	m.moveFocusIdx = vis - 1
	m.dialogVP.SetYOffset(0)
	m.nudgeMoveFocus(1)
	m.syncDialogViewport()
	m.ensureMoveFocusVisible()
	off := m.dialogVP.YOffset()
	if m.moveFocusIdx < off || m.moveFocusIdx >= off+vis {
		t.Fatalf("focus %d not visible: off=%d vis=%d", m.moveFocusIdx, off, vis)
	}
}

func TestToggleDialogFocus(t *testing.T) {
	m := NewModel(Options{})
	if m.dialogContentFocus {
		t.Fatal("start sidebar")
	}
	m.toggleDialogFocus()
	if !m.dialogContentFocus {
		t.Fatal("tab to content")
	}
	m.toggleDialogFocus()
	if m.dialogContentFocus {
		t.Fatal("tab back to sidebar")
	}
}

func TestSectionUsesGame(t *testing.T) {
	if !DialogMoves.UsesGame() || DialogOverview.UsesGame() {
		t.Fatal("UsesGame mapping wrong")
	}
}

func TestSelectFormUsesCatalogPokemon(t *testing.T) {
	cat := stubCatalog{
		forms: []domain.SelectableForm{
			{Key: "pokemon:25", PokemonID: 25, DisplayName: "Default", IsDefault: true},
			{Key: "pokemon:26", PokemonID: 26, DisplayName: "Partner"},
		},
		pokemon: map[int]domain.Pokemon{
			25: {ID: 25}, 26: {ID: 26},
		},
	}
	m := NewModel(Options{Catalog: cat, Context: context.Background()})
	m.catalog = cat
	m.dialogSpeciesID = 25
	m.dialogForms = cat.forms
	m.dialogFormIdx = 0
	m.selectDialogForm(1)
	if m.dialogPokemon.ID != 26 {
		t.Fatalf("poke=%d", m.dialogPokemon.ID)
	}
}

func TestDossierPickersOpenAsPopup(t *testing.T) {
	cat := stubCatalog{
		forms: []domain.SelectableForm{
			{Key: "pokemon:25", PokemonID: 25, DisplayName: "Default", IsDefault: true},
			{Key: "pokemon:10080", PokemonID: 10080, DisplayName: "Partner"},
		},
		pokemon: map[int]domain.Pokemon{25: {ID: 25}, 10080: {ID: 10080}},
		versions: []domain.Version{
			{ID: 1, Slug: "red", DisplayName: "Red", VersionGroupID: 1, Order: 1},
			{ID: 12, Slug: "pearl", DisplayName: "Pearl", VersionGroupID: 8, Order: 12},
		},
		vg: map[int]int{1: 1, 12: 8},
		moves: map[int][]domain.LevelUpMove{
			25*1000 + 8: {
				{Level: 1, Method: "level-up", Move: domain.Move{DisplayName: "Tackle"}},
				{Level: 0, Method: "machine", Move: domain.Move{DisplayName: "Rest"}},
			},
		},
	}
	m := NewModel(Options{Catalog: cat, Context: context.Background()})
	m.catalog = cat
	m.width, m.height = 100, 40
	m.showDialog = true
	m.dialogSpeciesID = 25
	m.dialogForms = cat.forms
	m.dialogFormIdx = 0
	m.dialogSection = DialogOverview
	m.resources[25] = &resourceCache{
		HasCore: true,
		Pokemon: domain.Pokemon{ID: 25, DisplayName: "Pikachu"},
		Species: domain.Species{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"},
	}
	m.filtered = []domain.SpeciesSummary{{ID: 25, DisplayName: "Pikachu"}}
	m.cursor = 0

	m.openGamePicker()
	view := m.View().Content
	if !strings.Contains(view, "Choose game") || !strings.Contains(view, "Red") {
		t.Fatalf("game popup missing:\n%s", view)
	}
	m.closeGamePicker(false)

	m.openFormPicker()
	view = m.View().Content
	if !strings.Contains(view, "Choose form") || !strings.Contains(view, "Partner") {
		t.Fatalf("form popup missing:\n%s", view)
	}
	m.closeFormPicker(false)

	m.dialogSection = DialogMoves
	m.dialogGameID = 12
	m.dialogGameSlug = "pearl"
	m.dialogGameLabel = "Pearl"
	m.ensureMoveMethod()
	m.openMethodPicker()
	view = m.View().Content
	if !strings.Contains(view, "Choose method") || !strings.Contains(view, "Machine") {
		t.Fatalf("method popup missing:\n%s", view)
	}
	m.handleMethodPickerKey("down")
	m.handleMethodPickerKey("enter")
	if m.dialogMoveMethod != "machine" {
		t.Fatalf("method=%q", m.dialogMoveMethod)
	}
}
