package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Graphixa/Poke/internal/domain"
	"github.com/Graphixa/Poke/internal/textwidth"
	"github.com/Graphixa/Poke/internal/tui/components"
)

func TestEvolutionStylesNamesOnly(t *testing.T) {
	level := 16
	tree := domain.EvolutionTree{Root: domain.EvolutionNode{
		DisplayName: "Bulbasaur",
		SpeciesSlug: "bulbasaur",
		Children: []domain.EvolutionBranch{{
			Conditions: []domain.EvolutionCondition{{MinimumLevel: &level}},
			Target:     domain.EvolutionNode{DisplayName: "Ivysaur", SpeciesSlug: "ivysaur"},
		}},
	}}

	line := formatEvolutionLines(tree, "", "")[0]
	want := primaryText("Bulbasaur") + " ── Level 16 ──► " + primaryText("Ivysaur")
	if line != want {
		t.Fatalf("evolution line:\n got %q\nwant %q", line, want)
	}
	requireANSI(t, primaryText("Bulbasaur"))
	if strings.Contains(line, primaryText("Level 16")) {
		t.Fatalf("condition should remain plain: %q", line)
	}
}

func TestEvolutionBoldsCurrentSpecies(t *testing.T) {
	level := 16
	tree := domain.EvolutionTree{Root: domain.EvolutionNode{
		DisplayName: "Charmander",
		SpeciesSlug: "charmander",
		Children: []domain.EvolutionBranch{{
			Conditions: []domain.EvolutionCondition{{MinimumLevel: &level}},
			Target:     domain.EvolutionNode{DisplayName: "Charmeleon", SpeciesSlug: "charmeleon"},
		}},
	}}
	line := formatEvolutionLines(tree, "charmander", "Charmander")[0]
	if !strings.Contains(line, boldPrimary("Charmander")) {
		t.Fatalf("current species should be bold: %q", line)
	}
	if strings.Contains(line, boldPrimary("Charmeleon")) {
		t.Fatalf("other species should not be bold: %q", line)
	}
	if strings.Contains(formatEvolutionLines(tree, "charmander", "Charmander")[0], "Trigger source") {
		t.Fatal("evolution must omit Trigger source")
	}
}

func TestMoveHeaderIncludesPP(t *testing.T) {
	power, accuracy := 40, 100
	lines := formatMoveLines([]domain.LevelUpMove{{
		Level:  1,
		Method: "level-up",
		Move: domain.Move{
			DisplayName: "Ember",
			Type:        "fire",
			DamageClass: "special",
			Power:       &power,
			Accuracy:    &accuracy,
			PP:          25,
		},
	}}, 80)

	moveW, gameW := moveColWidths(80, true, true)
	wantHeader := tableHeaderBand(moveTableHeader("level-up", moveW, gameW, true, true), 80)
	var header, dataRow string
	for _, line := range lines {
		plain := stripANSIForFocus(line)
		if strings.HasPrefix(strings.TrimSpace(plain), "Lvl") {
			header = line
		}
		if strings.Contains(plain, "Ember") {
			dataRow = line
		}
	}
	if header != wantHeader {
		t.Fatalf("header not bold accent:\n got %q\nwant %q", header, wantHeader)
	}
	requireANSI(t, header)
	if strings.Contains(header, "\x1b[48") || strings.Contains(header, "48;") {
		t.Fatalf("header must not use background: %q", header)
	}
	requireANSI(t, dataRow) // type colour on unfocused rows
	if !strings.Contains(stripANSIForFocus(dataRow), "All") {
		t.Fatalf("Game column should show All: %q", dataRow)
	}
	if strings.Contains(stripANSIForFocus(dataRow), "All…") || strings.Contains(stripANSIForFocus(dataRow), "All Games") {
		t.Fatalf("Game must not truncate All / say All Games: %q", dataRow)
	}
	if !strings.Contains(dataRow, "25") {
		t.Fatalf("move row should include PP: %q", dataRow)
	}
	plainHeader := stripANSIForFocus(header)
	plainRow := stripANSIForFocus(dataRow)
	if !strings.Contains(plainHeader, "Category") {
		t.Fatalf("category header should not be clipped: %q", plainHeader)
	}
	if gapBetween(plainHeader, "Pow", "Acc") != gapBetween(plainHeader, "PP", "Game") {
		t.Fatalf("PP→Game header gap != Pow→Acc: %q", plainHeader)
	}
	if gapBetween(plainRow, "40", "100") != gapBetween(plainRow, "25", "All") {
		t.Fatalf("PP→Game row gap != Pow→Acc: %q", plainRow)
	}
}

func gapBetween(s, left, right string) int {
	i := strings.Index(s, left)
	if i < 0 {
		return -1
	}
	rest := s[i+len(left):]
	j := strings.Index(rest, right)
	if j < 0 {
		return -1
	}
	return j
}

func TestMoveColumnsDropAccThenPPWhenNarrow(t *testing.T) {
	power, accuracy := 40, 100
	row := []domain.LevelUpMove{{
		Level:  1,
		Method: "level-up",
		Move: domain.Move{
			DisplayName: "Ember",
			Type:        "fire",
			DamageClass: "special",
			Power:       &power,
			Accuracy:    &accuracy,
			PP:          25,
		},
	}}
	findHeader := func(lines []string) string {
		for _, line := range lines {
			plain := strings.TrimSpace(stripANSIForFocus(line))
			if strings.HasPrefix(plain, "Lvl") {
				return plain
			}
		}
		return ""
	}
	mid := findHeader(formatMoveLines(row, 55))
	if strings.Contains(mid, "Acc") || !strings.Contains(mid, "PP") {
		t.Fatalf("mid width should drop Acc keep PP: %q", mid)
	}
	narrow := findHeader(formatMoveLines(row, 40))
	if strings.Contains(narrow, "Acc") || strings.Contains(narrow, "PP") {
		t.Fatalf("narrow width should drop Acc and PP: %q", narrow)
	}
}

func TestOverviewAndBiologyLabels(t *testing.T) {
	p := domain.Pokemon{
		DisplayName:     "Charmander",
		HeightDecimeter: 6,
		WeightHectogram: 85,
		BaseExperience:  62,
		Types:           []domain.TypeSlot{{Name: "fire"}},
		Abilities: []domain.AbilitySlot{
			{Slot: 1, Name: "blaze"},
			{Slot: 3, Name: "solar-power", IsHidden: true},
		},
	}
	s := domain.Species{
		Genus:         "Lizard Pokémon",
		Generation:    "generation-iv",
		CaptureRate:   45,
		GenderRate:    1,
		GrowthRate:    "medium-slow",
		BaseHappiness: 50,
		Habitat:       "mountain",
		Shape:         "upright",
		Color:         "red",
		EggGroups:     []string{"monster", "dragon"},
		HatchCounter:  20,
		Varieties:     []domain.Variety{{Slug: "charmander", IsDefault: true}},
		FlavorTextEntries: []domain.FlavorText{{
			Language: "en", Version: "red", VersionOrder: 1,
			Text: "Prefers hot places.",
		}},
	}
	overview := formatOverviewLines(p, s, 60, nil, 0, "")
	joined := strings.Join(overview, "\n")
	if strings.Contains(joined, "General reference") || strings.Contains(joined, "Source:") {
		t.Fatalf("overview must not show disclaimer labels:\n%s", joined)
	}
	if !strings.Contains(joined, boldPrimary("About           ")) || !strings.Contains(joined, "Prefers hot places.") {
		t.Fatalf("About blurb missing:\n%s", joined)
	}
	if !strings.Contains(joined, boldPrimary("Height          ")) || !strings.Contains(joined, boldPrimary("Weight          ")) {
		t.Fatalf("Height and Weight should be separate lines:\n%s", joined)
	}
	if !strings.Contains(joined, boldPrimary("Type            ")) || !strings.Contains(joined, "Lizard Pokémon") {
		t.Fatalf("overview missing Type/Category:\n%s", joined)
	}
	if !strings.Contains(joined, "Blaze") || !strings.Contains(joined, "(Hidden)") {
		t.Fatalf("overview should list abilities:\n%s", joined)
	}
	if strings.Contains(joined, "Catch rate") || strings.Contains(joined, "Capture") {
		t.Fatalf("overview must not list capture:\n%s", joined)
	}
	if !strings.Contains(joined, boldPrimary("Generation      ")) || !strings.Contains(joined, " IV") {
		t.Fatalf("overview Generation should be labelled Roman IV:\n%s", joined)
	}
	if strings.Contains(joined, "Iv") && !strings.Contains(joined, "IV") {
		t.Fatalf("generation must not title-case to Iv:\n%s", joined)
	}

	bio := formatBiologyLines(p, s)
	bioJoined := strings.Join(bio, "\n")
	for _, want := range []string{"Habitat", "Egg groups", "Egg cycles", "Monster", "Base friendship", "Catch rate"} {
		if !strings.Contains(bioJoined, want) {
			t.Fatalf("biology missing %q:\n%s", want, bioJoined)
		}
	}
	for _, ban := range []string{"Physical", "Breeding", "Forms", "(default)", "Classification"} {
		if strings.Contains(bioJoined, ban) {
			t.Fatalf("biology must not show %q:\n%s", ban, bioJoined)
		}
	}
}

func TestGlanceAbilityGridStableSlots(t *testing.T) {
	// Wartortle-like: slot 1 + hidden only — Slot 2 shows placeholder; Hidden is full width.
	lines := components.GlanceAbilityGrid([]domain.AbilitySlot{
		{Slot: 1, Name: "torrent"},
		{Slot: 3, Name: "rain-dish", IsHidden: true},
	}, 48)
	if len(lines) != 2 {
		t.Fatalf("want 2 rows, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "Torrent") || !strings.Contains(lines[0], "Slot 2") {
		t.Fatalf("row1 should keep Slot 1 + Slot 2 cells: %q", lines[0])
	}
	if strings.Contains(lines[0], "Rain Dish") {
		t.Fatalf("hidden must not sit on row1: %q", lines[0])
	}
	if !strings.Contains(lines[1], "Hidden") || !strings.Contains(lines[1], "Rain Dish") {
		t.Fatalf("row2 should be full-width Hidden: %q", lines[1])
	}
	if strings.Contains(lines[1], "(hidden)") {
		t.Fatalf("must not suffix (hidden) when label is Hidden: %q", lines[1])
	}
	full := components.GlanceAbilityGrid([]domain.AbilitySlot{
		{Slot: 1, Name: "poison-point"},
		{Slot: 2, Name: "rivalry"},
		{Slot: 3, Name: "sheer-force", IsHidden: true},
	}, 48)
	s1a := strings.Index(lines[0], "Slot 1")
	s1b := strings.Index(full[0], "Slot 1")
	s2a := strings.Index(lines[0], "Slot 2")
	s2b := strings.Index(full[0], "Slot 2")
	if s1a < 0 || s1a != s1b || s2a < 0 || s2a != s2b {
		t.Fatalf("slot columns shifted: empty s1=%d s2=%d full s1=%d s2=%d", s1a, s2a, s1b, s2b)
	}
	if !strings.Contains(full[1], "Sheer Force") || strings.Contains(full[1], "Slot 3") {
		t.Fatalf("row2 should be Hidden name only: %q", full[1])
	}
}

func TestFlavourStylesVersionOnly(t *testing.T) {
	lines := formatFlavourLinesAll(domain.Species{FlavorTextEntries: []domain.FlavorText{{
		Language:     "en",
		Version:      "lets-go-eevee",
		VersionOrder: 1,
		Text:         "Tough fights could excite this Pokémon.",
	}}})

	plain := stripANSIForFocus(lines[0])
	if !strings.HasPrefix(plain, "Lets Go Eevee") || !strings.Contains(plain, "Tough fights") {
		t.Fatalf("want labelled flavour fact line: %q", lines[0])
	}
	wantLabel := boldPrimary(fmt.Sprintf("%-16s", "Lets Go Eevee"))
	if !strings.HasPrefix(lines[0], wantLabel) {
		t.Fatalf("game label should match About-style fact line:\n got %q\nwant prefix %q", lines[0], wantLabel)
	}
	requireANSI(t, lines[0])
}

func TestLocationsTableAllGames(t *testing.T) {
	lines := formatEncounterLinesForGame([]domain.Encounter{{
		Version:      "omega-ruby",
		Area:         "route-101",
		Method:       "walk",
		MinimumLevel: 2,
		MaximumLevel: 3,
		Chance:       20,
		Conditions:   []string{"time-morning"},
	}}, 0, "", 80)

	if !strings.Contains(lines[0], "Omega Ruby") || !strings.Contains(lines[0], "Route 101") {
		t.Fatalf("All-games table should include game + area: %q", lines[0])
	}
	if strings.Contains(lines[0], "\x1b[") {
		t.Fatalf("location data should remain plain: %q", lines[0])
	}
	if len(lines) < 2 || !strings.Contains(lines[1], "↳") || !strings.Contains(lines[1], "Morning") {
		t.Fatalf("conditions should use ↳: %v", lines)
	}
	requireANSI(t, lines[1]) // condition sub-rows are muted
	if textwidth.Width(lines[0]) > 80 {
		t.Fatalf("row wider than body (%d): %q", textwidth.Width(lines[0]), lines[0])
	}
	header := encounterTableHeader(80)
	if !strings.Contains(header, "Game") || !strings.Contains(header, "Area") {
		t.Fatalf("header missing Game/Area: %q", header)
	}
	filtered := formatEncounterLinesForGame([]domain.Encounter{{
		Version: "omega-ruby", Area: "route-101", Method: "walk",
		MinimumLevel: 2, MaximumLevel: 3, Chance: 20,
	}}, 1, "omega-ruby", 80)
	if !strings.Contains(filtered[0], "Omega Ruby") {
		t.Fatalf("filtered table still shows Game column: %q", filtered[0])
	}
}

func TestStatsStyleLabelsOnly(t *testing.T) {
	lines := formatStatsBars(domain.Pokemon{Stats: domain.Stats{HP: 58, Attack: 64, Defense: 58, SpecialAttack: 80, SpecialDefense: 65, Speed: 90}})
	if len(lines) < 1 || strings.Contains(stripANSIForFocus(lines[0]), "Base stats") {
		t.Fatalf("stats must not show base-stats disclaimer: %q", lines[0])
	}
	hp := lines[0]
	if !strings.HasPrefix(hp, primaryText("HP      ")) {
		t.Fatalf("stat label not styled: %q", hp)
	}
	if strings.Contains(strings.TrimPrefix(hp, primaryText("HP      ")), "\x1b[") {
		t.Fatalf("stat bar and value should remain plain: %q", hp)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "Total") || !strings.Contains(joined, "415") {
		t.Fatalf("stats should include Total sum:\n%s", joined)
	}
}

func TestAbilityLayout(t *testing.T) {
	lines := formatAbilityLines([]domain.AbilitySlot{
		{Slot: 1, Name: "static"},
		{Slot: 3, Name: "lightning-rod", IsHidden: true},
	}, nil, 60)
	joined := strings.Join(lines, "\n")
	if !strings.HasPrefix(lines[0], boldPrimary("Abilities       ")) {
		t.Fatalf("first ability should share Abilities label line: %q", lines[0])
	}
	if !strings.Contains(lines[0], "Static") {
		t.Fatalf("Static should sit on Abilities line: %q", lines[0])
	}
	if !strings.Contains(joined, "Lightning Rod") || !strings.Contains(joined, "(Hidden)") {
		t.Fatalf("hidden ability should be Name (Hidden):\n%s", joined)
	}
	if strings.Contains(joined, "draws in Electric") {
		t.Fatalf("matchup notes removed:\n%s", joined)
	}
	if !strings.Contains(joined, "\n\n") {
		t.Fatalf("abilities should be separated by a blank line:\n%s", joined)
	}
}

func TestChanceColumnPadsBeforeGame(t *testing.T) {
	row := formatEncounterTableRows([]domain.Encounter{{
		Version: "yellow", Area: "cerulean-city-area", Method: "gift",
		MinimumLevel: 10, MaximumLevel: 10, Chance: 100,
	}}, 80)[0]
	// "100%" left-aligned in %-7s — must not butt directly against Game.
	if strings.Contains(row, "100%Yellow") || !strings.Contains(row, "100% ") {
		t.Fatalf("Chance should leave gap before Game: %q", row)
	}
	idx := strings.Index(row, "100%")
	rest := row[idx+len("100%"):]
	if !strings.HasPrefix(rest, " ") || !strings.Contains(rest, "Yellow") {
		t.Fatalf("Chance/Game crowding: %q", row)
	}
}

func TestMoveDetailFactLayoutAndSideList(t *testing.T) {
	pow, acc := 40, 100
	rows := []domain.LevelUpMove{
		{Level: 1, Method: "level-up", Move: domain.Move{DisplayName: "Tackle", Type: "normal", DamageClass: "physical", Power: &pow, Accuracy: &acc, PP: 35, Effect: "Inflicts regular damage."}},
		{Level: 3, Method: "level-up", Move: domain.Move{DisplayName: "Growl", Type: "normal", DamageClass: "status", PP: 40}},
	}
	lines := formatMoveDetailLines(rows, 0, 72)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(lines[0], boldPrimary("Tackle")) {
		t.Fatalf("name first: %q", lines[0])
	}
	if !strings.Contains(joined, "Inflicts regular damage.") {
		t.Fatalf("effect under name:\n%s", joined)
	}
	for _, label := range []string{"Type", "Pow", "Acc", "PP"} {
		if !strings.Contains(joined, boldPrimary(fmt.Sprintf("%-16s", label))) {
			t.Fatalf("missing fact %s:\n%s", label, joined)
		}
	}
	if !strings.Contains(joined, "Growl") {
		t.Fatalf("side list should show sibling moves:\n%s", joined)
	}
	if !strings.Contains(joined, "Move List") {
		t.Fatalf("side list needs Move List header:\n%s", joined)
	}
	plain0 := stripANSIForFocus(lines[0])
	sep := strings.Index(plain0, "│")
	if sep < 0 {
		t.Fatalf("detail/list need vertical divider:\n%s", joined)
	}
	after := sep + len("│")
	if sep < 1 || plain0[sep-1] != ' ' || after >= len(plain0) || plain0[after] != ' ' {
		t.Fatalf("divider needs space on both sides: %q", plain0)
	}
	if !strings.Contains(plain0[:sep], "Move List") {
		t.Fatalf("Move List should be left of divider: %q", plain0)
	}
	if !strings.Contains(plain0[sep:], "Tackle") {
		t.Fatalf("detail name should be right of divider: %q", plain0)
	}
	plain1 := stripANSIForFocus(lines[1])
	left1 := plain1[:strings.Index(plain1, "│")]
	if !strings.Contains(left1, "  Tackle") {
		t.Fatalf("move names should be indented under Move List: %q", left1)
	}
	requireANSI(t, joined) // type colour on Type value
}

func TestMoveDetailGroupsByGame(t *testing.T) {
	rows := []domain.LevelUpMove{
		{Game: "Red (Japan)", Move: domain.Move{DisplayName: "Growl"}},
		{Game: "Red (Japan)", Move: domain.Move{DisplayName: "Leech Seed"}},
		{Game: "Red (Japan)", Move: domain.Move{DisplayName: "Tackle"}},
		{Game: "Emerald", Move: domain.Move{DisplayName: "Growl"}},
		{Game: "Emerald", Move: domain.Move{DisplayName: "Tackle"}},
	}
	lines := moveDetailSideList(rows, 1, 24)
	var plain []string
	for _, l := range lines {
		plain = append(plain, stripANSIForFocus(l))
	}
	joined := strings.Join(plain, "\n")
	if !strings.Contains(joined, "Red (Japan)") || !strings.Contains(joined, "Emerald") {
		t.Fatalf("game dividers missing:\n%s", joined)
	}
	var sawCursor bool
	for i, l := range plain {
		trim := strings.TrimSpace(l)
		if trim == "Red (Japan)" || trim == "Emerald" {
			if strings.Contains(l, ">") {
				t.Fatalf("cursor landed on game header %q", l)
			}
			if lines[i] == boldPrimary(trim) {
				t.Fatalf("game header should stay placeholder ink: %q", lines[i])
			}
			continue
		}
		if strings.Contains(l, "Leech Seed") {
			if !strings.Contains(l, "> Leech Seed") {
				t.Fatalf("focused move wants > cursor: %q", l)
			}
			sawCursor = true
		}
	}
	if !sawCursor {
		t.Fatalf("missing Leech Seed cursor:\n%s", joined)
	}
	// Up/down still walks moves, so a header between games is not a row.
	if strings.Contains(plain[0], ">") {
		t.Fatalf("Move List title is not a cursor row: %q", plain[0])
	}
}

func TestEncounterSortFollowsGameOrder(t *testing.T) {
	rows := []domain.Encounter{
		{VersionID: 3, Version: "yellow", Area: "a", Method: "walk"},
		{VersionID: 1, Version: "red", Area: "b", Method: "walk"},
		{VersionID: 2, Version: "blue", Area: "c", Method: "walk"},
	}
	order := map[string]int{"id:1": 1, "id:2": 2, "id:3": 3}
	sorted := sortEncountersByGameOrder(rows, order)
	if sorted[0].Version != "red" || sorted[1].Version != "blue" || sorted[2].Version != "yellow" {
		t.Fatalf("order=%v %v %v", sorted[0].Version, sorted[1].Version, sorted[2].Version)
	}
}

func TestDialogChromeKeepsHeaderDuringGamePicker(t *testing.T) {
	m := NewModel(Options{})
	m.width, m.height = 100, 40
	m.showDialog = true
	m.dialogSection = DialogEncounters
	m.filtered = []domain.SpeciesSummary{{ID: 1, DisplayName: "Bulbasaur"}}
	m.cursor = 0
	m.resources[1] = &resourceCache{
		HasEncounters: true,
		Encounters: []domain.Encounter{{
			Version: "red", Area: "pallet-town", Method: "gift",
			MinimumLevel: 5, MaximumLevel: 5, Chance: 100,
		}},
	}
	m.gamePickerOpen = true
	sticky, hint := m.dialogChrome(m.dialogGeom().BodyW, 10, 0)
	if len(sticky) == 0 || !strings.Contains(stripANSIForFocus(sticky[0]), "Area") {
		t.Fatalf("sticky header should survive G picker: %v", sticky)
	}
	if !strings.Contains(hint, "Game:") {
		t.Fatalf("status should keep Game label during picker: %q", hint)
	}
}

func TestDialogOverviewPaintsFromBrowseWithoutCore(t *testing.T) {
	m := NewModel(Options{
		BrowseEntries: map[int]domain.BrowseEntry{
			4: {
				ID: 4, Slug: "charmander", Name: "Charmander", Types: []string{"fire"},
				HP: 39, Attack: 52, Defence: 43, SpAttack: 60, SpDefence: 50, Speed: 65,
				HeightDecimeter: 6, WeightHectogram: 85, Genus: "Lizard Pokémon",
				Abilities: []domain.AbilitySlot{{Name: "blaze"}},
			},
		},
		DisableSprite: true,
	})
	m.width, m.height = 100, 40
	m.showDialog = true
	m.dialogSection = DialogOverview
	m.filtered = []domain.SpeciesSummary{{ID: 4, Slug: "charmander", DisplayName: "Charmander", Types: []string{"fire"}}}
	m.cursor = 0
	m.dialogPokemon = domain.PokemonFromBrowse(m.browseEntries[4])

	joined := strings.Join(m.dialogContentLines(), "\n")
	if strings.Contains(joined, "Loading…") {
		t.Fatalf("overview should not wait on core: %q", joined)
	}
	if !strings.Contains(stripANSIForFocus(joined), "Lizard Pokémon") {
		t.Fatalf("want browse genus, got %q", joined)
	}

	m.dialogSection = DialogMoves
	if got := strings.Join(m.dialogContentLines(), "\n"); got != "Loading…" {
		t.Fatalf("moves without core should wait, got %q", got)
	}
}

func requireANSI(t *testing.T, s string) {
	t.Helper()
	if !strings.Contains(s, "\x1b[") {
		t.Fatalf("expected ANSI styling in %q", s)
	}
}
