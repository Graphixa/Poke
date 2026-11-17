package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Poke/internal/domain"
	"github.com/Graphixa/Poke/internal/output"
	"github.com/Graphixa/Poke/internal/textwidth"
	"github.com/Graphixa/Poke/internal/tui/components"
	tuistyle "github.com/Graphixa/Poke/internal/tui/style"
	"github.com/Graphixa/Poke/internal/usererr"
)

func primaryText(s string) string {
	return lipgloss.NewStyle().Foreground(tuistyle.PrimaryColor()).Render(s)
}

func boldPrimary(s string) string {
	return lipgloss.NewStyle().Foreground(tuistyle.PrimaryColor()).Bold(true).Render(s)
}

func (m Model) browseViewLines() []string {
	// Browse is sprite-only; text detail lives in the Enter dialog.
	return nil
}

func formatStatsBars(p domain.Pokemon) []string {
	return components.StatsBars(p.Stats)
}

func formatHeight(dm int) string {
	return fmt.Sprintf("%.1f m", float64(dm)/10)
}

func formatWeight(hg int) string {
	return fmt.Sprintf("%.1f kg", float64(hg)/10)
}

func (m Model) dialogContentLines() []string {
	sel, ok := m.selected()
	if !ok {
		return []string{"No Pokémon selected."}
	}
	r := m.resources[sel.ID]
	if r == nil || (!r.HasCore && r.loadingCore) {
		return []string{"Loading…"}
	}
	if r.CoreErr != nil && !r.HasCore {
		return []string{usererr.Message(r.CoreErr)}
	}
	if !r.HasCore {
		return []string{"Loading…"}
	}

	bodyW := m.dialogGeom().BodyW
	poke := r.Pokemon
	if m.dialogPokemon.ID > 0 {
		poke = m.dialogPokemon
	}
	switch m.dialogSection {
	case DialogOverview:
		return m.formatOverviewLines(poke, r.Species, bodyW)
	case DialogStats:
		return formatStatsBars(poke)
	case DialogEvolution:
		if !r.HasEvolution && r.loadingExtras {
			return []string{"Loading…"}
		}
		if r.EvolutionErr != nil {
			return sectionErrorLines(r.EvolutionErr)
		}
		return formatEvolutionLines(r.Evolution, r.Species.Slug, r.Pokemon.DisplayName)
	case DialogMoves:
		if !r.HasMoves && r.loadingExtras {
			return []string{"Loading…"}
		}
		if r.MovesErr != nil {
			return sectionErrorLines(r.MovesErr)
		}
		moves := m.dialogMoveRows()
		if m.moveDetailOpen {
			return formatMoveDetailLines(moves, m.moveFocusIdx, bodyW)
		}
		// Scrollable rows only; sticky header/position come from dialogChrome.
		return formatMoveDataRowsFocused(moves, bodyW, true, m.moveFocusIdx)
	case DialogEncounters:
		if !r.HasEncounters && r.loadingExtras {
			return []string{"Loading…"}
		}
		if r.EncountersErr != nil {
			return sectionErrorLines(r.EncountersErr)
		}
		rows := m.dialogEncounterRows()
		return formatEncounterTableRowsFocused(rows, bodyW, m.encounterFocusIdx)
	case DialogDescriptions:
		return formatFlavourLinesForGame(r.Species, m.dialogGameID, m.dialogGameSlug)
	case DialogBiology:
		return formatBiologyLines(poke, r.Species)
	default:
		return nil
	}
}

func sectionErrorLines(err error) []string {
	return []string{
		usererr.Message(err),
		"",
		tuistyle.StatusMuted("Retry: leave and re-open this dossier, or rebuild the dataset."),
	}
}

// dialogChrome returns sticky table headers and the pinned status/position line.
func (m Model) dialogChrome(bodyW, scrollH, scrollOff int) (sticky []string, hint string) {
	if m.dialogOverlayOpen() {
		return nil, ""
	}
	switch m.dialogSection {
	case DialogMoves:
		if m.moveDetailOpen {
			return nil, ""
		}
		rows := m.dialogMoveRows()
		if len(rows) == 0 {
			return nil, ""
		}
		sticky = moveStickyLines(rows, bodyW, m.dialogMoveMethod)
		left := "Learnset: All"
		if m.dialogGameID >= 1 {
			left = "Learnset: " + m.dialogGameLabel
			if m.dialogGameLabel == "" {
				left = "Learnset: " + output.GameLabel(m.dialogGameSlug, "")
			}
		}
		vis := scrollH
		if vis < 1 {
			vis = 1
		}
		start := scrollOff + 1
		if start < 1 {
			start = 1
		}
		end := scrollOff + vis
		if end > len(rows) {
			end = len(rows)
		}
		if start > end {
			start = end
		}
		right := ""
		if m.moveFocusIdx >= 0 {
			start = m.moveFocusIdx + 1
			if start > len(rows) {
				start = len(rows)
			}
			right = fmt.Sprintf("Row %d of %d", start, len(rows))
		} else if len(rows) > vis {
			right = fmt.Sprintf("Rows %d–%d of %d", start, end, len(rows))
		} else if len(rows) > 0 {
			right = fmt.Sprintf("Rows 1–%d of %d", len(rows), len(rows))
		}
		hint = statusLine(left, right, bodyW)
		return sticky, hint
	case DialogEncounters:
		rows := m.dialogEncounterRows()
		if len(rows) == 0 {
			return nil, ""
		}
		sticky = []string{tableHeaderBand(encounterTableHeader(bodyW), bodyW)}
		left := "All"
		if m.dialogGameID >= 1 {
			left = m.dialogGameLabel
			if left == "" {
				left = output.GameLabel(m.dialogGameSlug, "")
			}
		}
		right := ""
		if m.encounterFocusIdx >= 0 && m.encounterFocusIdx < len(rows) {
			right = fmt.Sprintf("Row %d of %d", m.encounterFocusIdx+1, len(rows))
		} else if len(rows) > 0 {
			right = fmt.Sprintf("Row 1 of %d", len(rows))
		}
		hint = statusLine(left, right, bodyW)
		return sticky, hint
	case DialogDescriptions:
		if m.dialogGameID < 1 {
			return nil, ""
		}
		label := m.dialogGameLabel
		if label == "" {
			label = output.GameLabel(m.dialogGameSlug, "")
		}
		hint = statusLine(label, "", bodyW)
		return nil, hint
	default:
		return nil, ""
	}
}

func statusLine(left, right string, bodyW int) string {
	if bodyW < 1 {
		bodyW = 1
	}
	if right == "" {
		return left
	}
	gap := bodyW - textwidth.Width(left) - textwidth.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m Model) formatOverviewLines(p domain.Pokemon, s domain.Species, bodyW int) []string {
	return formatOverviewLines(p, s, bodyW, m.catalog, m.dialogGameID, m.dialogGameSlug)
}

func formatOverviewLines(p domain.Pokemon, s domain.Species, bodyW int, cat CatalogSource, gameID int, gameSlug string) []string {
	if bodyW < 24 {
		bodyW = 24
	}
	wrapW := bodyW
	if wrapW > 56 {
		wrapW = 56
	}

	var lines []string
	if text := overviewBlurb(s, gameID, gameSlug); text != "" {
		about := wrapWords(text, wrapW-factValueCol)
		if len(about) > 0 {
			lines = append(lines, boldPrimary(fmt.Sprintf("%-16s", "About"))+" "+about[0])
			indent := strings.Repeat(" ", factValueCol)
			for _, w := range about[1:] {
				lines = append(lines, indent+w)
			}
			lines = append(lines, "")
		}
	}

	types := tuistyle.TypeSummary(p.TypeNames())
	genus := strings.TrimSpace(s.Genus)
	if genus == "" {
		genus = "—"
	}
	lines = append(lines,
		factLine("Type", types),
		factLine("Category", genus),
		factLine("Height", formatHeight(p.HeightDecimeter)),
		factLine("Weight", formatWeight(p.WeightHectogram)),
	)
	gen := dash(output.GenerationNumeral(s.Generation))
	lines = append(lines, factLine("Generation", gen))

	if chart, err := catalogTypeChart(cat); err == nil && len(chart) > 0 {
		weak, resist, immune := typeMatchups(p.TypeNames(), chart)
		lines = append(lines, "")
		lines = append(lines, factLine("Weak to", joinOrDash(weak)))
		lines = append(lines, factLine("Resists", joinOrDash(resist)))
		if len(immune) > 0 {
			lines = append(lines, factLine("Immune", joinOrDash(immune)))
		}
	}

	if len(p.Abilities) > 0 {
		lines = append(lines, "")
		lines = append(lines, formatAbilityLines(p.Abilities, cat, wrapW)...)
	}
	return lines
}

func catalogTypeChart(cat CatalogSource) (map[string]domain.TypeInfo, error) {
	if cat == nil {
		return nil, fmt.Errorf("no catalog")
	}
	return cat.TypeChart()
}

// factValueCol is where factLine values begin (16-char label + space).
const factValueCol = 17

func formatAbilityLines(slots []domain.AbilitySlot, cat CatalogSource, wrapW int) []string {
	indent := strings.Repeat(" ", factValueCol)
	effectW := wrapW - factValueCol
	if effectW < 24 {
		effectW = 24
	}
	var lines []string
	for i, slot := range slots {
		if i > 0 {
			lines = append(lines, "")
		}
		title := abilityTitle(slot)
		if i == 0 {
			lines = append(lines, boldPrimary(fmt.Sprintf("%-16s", "Abilities"))+" "+title)
		} else {
			lines = append(lines, indent+title)
		}
		if effect := abilityEffect(slot, cat); effect != "" {
			for _, w := range wrapWords(effect, effectW) {
				lines = append(lines, indent+w)
			}
		}
	}
	return lines
}

func abilityTitle(slot domain.AbilitySlot) string {
	name := output.TitleFromSlug(slot.Name)
	if slot.IsHidden || slot.Slot >= 3 {
		return primaryText(name) + tuistyle.StatusMuted(" (Hidden)")
	}
	return primaryText(name)
}

func abilityEffect(slot domain.AbilitySlot, cat CatalogSource) string {
	if cat == nil {
		return ""
	}
	if slot.ID > 0 {
		if a, ok := cat.AbilityByID(slot.ID); ok {
			return strings.TrimSpace(a.Effect)
		}
	}
	if a, ok := cat.AbilityBySlug(slot.Name); ok {
		return strings.TrimSpace(a.Effect)
	}
	return ""
}

// typeMatchups returns attacking types that deal >1×, <1× (but >0), and 0× to defenders.
func typeMatchups(defenders []string, chart map[string]domain.TypeInfo) (weak, resist, immune []string) {
	if len(defenders) == 0 || len(chart) == 0 {
		return nil, nil, nil
	}
	attackers := make([]string, 0, len(chart))
	for slug := range chart {
		attackers = append(attackers, slug)
	}
	sort.Strings(attackers)
	for _, atk := range attackers {
		m := damageTaken(atk, defenders, chart)
		title := output.TitleFromSlug(atk)
		switch {
		case m == 0:
			immune = append(immune, title)
		case m > 1:
			weak = append(weak, title)
		case m < 1:
			resist = append(resist, title)
		}
	}
	return weak, resist, immune
}

func damageTaken(attacker string, defenders []string, chart map[string]domain.TypeInfo) float64 {
	m := 1.0
	for _, d := range defenders {
		ti, ok := chart[strings.ToLower(d)]
		if !ok {
			continue
		}
		for _, t := range ti.NoDamage {
			if t == attacker {
				m = 0
			}
		}
		if m == 0 {
			continue
		}
		for _, t := range ti.Half {
			if t == attacker {
				m *= 0.5
			}
		}
		for _, t := range ti.Double {
			if t == attacker {
				m *= 2
			}
		}
	}
	return m
}

func joinOrDash(parts []string) string {
	if len(parts) == 0 {
		return "—"
	}
	return strings.Join(parts, ", ")
}

func factLine(label, value string) string {
	return boldPrimary(fmt.Sprintf("%-16s", label)) + " " + value
}

func formatBiologyLines(p domain.Pokemon, s domain.Species) []string {
	var lines []string
	lines = append(lines,
		factLine("Height", formatHeight(p.HeightDecimeter)),
		factLine("Weight", formatWeight(p.WeightHectogram)),
		factLine("Colour", dash(output.TitleFromSlug(s.Color))),
		factLine("Shape", dash(output.TitleFromSlug(s.Shape))),
		factLine("Habitat", dash(output.TitleFromSlug(s.Habitat))),
		factLine("Gender", formatGenderRate(s.GenderRate)),
	)
	if len(s.EggGroups) == 0 {
		lines = append(lines, factLine("Egg groups", "—"))
	} else {
		eggs := make([]string, len(s.EggGroups))
		for i, g := range s.EggGroups {
			eggs[i] = output.TitleFromSlug(g)
		}
		lines = append(lines, factLine("Egg groups", strings.Join(eggs, ", ")))
	}
	if s.HatchCounter > 0 {
		lines = append(lines, factLine("Egg cycles", fmt.Sprintf("%d", s.HatchCounter)))
	} else {
		lines = append(lines, factLine("Egg cycles", "—"))
	}
	lines = append(lines,
		factLine("Growth", dash(output.TitleFromSlug(s.GrowthRate))),
		factLine("Base friendship", fmt.Sprintf("%d", s.BaseHappiness)),
		factLine("Catch rate", fmt.Sprintf("%d", s.CaptureRate)),
	)

	var class []string
	if s.IsBaby {
		class = append(class, "Baby")
	}
	if s.IsLegendary {
		class = append(class, "Legendary")
	}
	if s.IsMythical {
		class = append(class, "Mythical")
	}
	if len(class) > 0 {
		lines = append(lines, "", strings.Join(class, ", "))
	}
	return lines
}

func formatEvolutionLines(tree domain.EvolutionTree, currentSlug, currentName string) []string {
	var lines []string
	writeEvoNode(&lines, tree.Root, true, currentSlug, currentName)
	if len(lines) == 0 {
		return []string{"No evolution data."}
	}
	return lines
}

func evoSpeciesLabel(node domain.EvolutionNode, currentSlug, currentName string) string {
	name := node.DisplayName
	if name == "" {
		name = output.TitleFromSlug(node.SpeciesSlug)
	}
	if isCurrentSpecies(node, currentSlug, currentName) {
		return boldPrimary(name)
	}
	return primaryText(name)
}

func isCurrentSpecies(node domain.EvolutionNode, currentSlug, currentName string) bool {
	if currentSlug != "" && strings.EqualFold(node.SpeciesSlug, currentSlug) {
		return true
	}
	if currentName != "" && strings.EqualFold(node.DisplayName, currentName) {
		return true
	}
	return false
}

func writeEvoNode(lines *[]string, node domain.EvolutionNode, isRoot bool, currentSlug, currentName string) {
	name := evoSpeciesLabel(node, currentSlug, currentName)
	if isRoot && len(node.Children) == 0 {
		*lines = append(*lines, name+" (no evolutions)")
		return
	}
	for _, branch := range node.Children {
		label := formatEvoConditions(branch.Conditions)
		if label == "" {
			label = "Evolution"
		}
		target := evoSpeciesLabel(branch.Target, currentSlug, currentName)
		*lines = append(*lines, fmt.Sprintf("%s ── %s ──► %s", name, label, target))
		writeEvoNode(lines, branch.Target, false, currentSlug, currentName)
	}
}

func formatEvoConditions(conds []domain.EvolutionCondition) string {
	if len(conds) == 0 {
		return ""
	}
	parts := make([]string, 0, len(conds))
	for _, c := range conds {
		if s := formatOneCondition(c); s != "" {
			parts = append(parts, s)
		}
	}
	// Multiple conditions on one branch are AND; sibling branches are alternative routes.
	return strings.Join(parts, " + ")
}

func formatOneCondition(c domain.EvolutionCondition) string {
	var parts []string
	if trig := triggerLabel(c.Trigger); trig != "" {
		parts = append(parts, trig)
	}
	if c.MinimumLevel != nil {
		parts = append(parts, fmt.Sprintf("Level %d", *c.MinimumLevel))
	}
	if c.Item != "" {
		parts = append(parts, output.TitleFromSlug(c.Item))
	}
	if c.HeldItem != "" {
		parts = append(parts, "holding "+output.TitleFromSlug(c.HeldItem))
	}
	if c.KnownMove != "" {
		parts = append(parts, "knowing "+output.TitleFromSlug(c.KnownMove))
	}
	if c.KnownMoveType != "" {
		parts = append(parts, "knowing "+output.TitleFromSlug(c.KnownMoveType)+" move")
	}
	if c.MinimumHappiness != nil {
		parts = append(parts, fmt.Sprintf("Friendship %d+", *c.MinimumHappiness))
	}
	if c.MinimumBeauty != nil {
		parts = append(parts, fmt.Sprintf("Beauty %d+", *c.MinimumBeauty))
	}
	if c.MinimumAffection != nil {
		parts = append(parts, fmt.Sprintf("Affection %d+", *c.MinimumAffection))
	}
	if c.TimeOfDay != "" {
		parts = append(parts, c.TimeOfDay)
	}
	if c.Location != "" {
		parts = append(parts, "at "+output.TitleFromSlug(c.Location))
	}
	if c.NeedsOverworldRain {
		parts = append(parts, "in rain")
	}
	if c.TurnUpsideDown {
		parts = append(parts, "upside-down")
	}
	if c.TradeSpecies != "" {
		parts = append(parts, "for "+output.TitleFromSlug(c.TradeSpecies))
	}
	if c.Gender != nil {
		switch *c.Gender {
		case 1:
			parts = append(parts, "female")
		case 2:
			parts = append(parts, "male")
		}
	}
	return strings.Join(parts, ", ")
}

func triggerLabel(trigger string) string {
	switch trigger {
	case "", "level-up":
		return "" // implied by Level N / other level-up details
	case "trade":
		return "Trade"
	case "use-item":
		return "Use item"
	case "shed":
		return "Shed"
	case "other":
		return ""
	default:
		return output.TitleFromSlug(trigger)
	}
}

func formatEncounterLinesForGame(rows []domain.Encounter, gameID int, gameSlug string, bodyW int) []string {
	var filtered []domain.Encounter
	if gameID < 1 {
		filtered = rows
	} else {
		for _, e := range rows {
			if e.VersionID == gameID || (gameSlug != "" && e.Version == gameSlug) {
				filtered = append(filtered, e)
			}
		}
	}
	if len(filtered) == 0 {
		if gameID < 1 {
			return []string{"No wild encounters recorded."}
		}
		return []string{"No records available for this selection."}
	}
	return formatEncounterTableRows(filtered, bodyW)
}

func encounterColumnFlags(bodyW int) (showChance bool) {
	// Area(flex) + Method + Level + Game + spaces; Chance optional (" %5d%%" = 7).
	const fixedNoChance, chanceCol = 39, 7
	showChance = true
	if bodyW > 0 && bodyW < fixedNoChance+chanceCol {
		showChance = false
	}
	return showChance
}

func encounterAreaWidth(bodyW int, showChance bool) int {
	// "%-*s %-12s %-10s" + optional " %5d%%" + " %-14s"
	fixed := 1 + 12 + 1 + 10 + 1 + 14
	if showChance {
		fixed += 7 // " %5d%%"
	}
	areaW := bodyW - fixed
	if areaW < 12 {
		areaW = 12
	}
	return areaW
}

func encounterTableHeader(bodyW int) string {
	showChance := encounterColumnFlags(bodyW)
	areaW := encounterAreaWidth(bodyW, showChance)
	header := fmt.Sprintf("%-*s %-12s %-10s", areaW, "Area", "Method", "Level")
	if showChance {
		header += fmt.Sprintf(" %6s", "Chance")
	}
	header += fmt.Sprintf(" %-14s", "Game")
	return header
}

func formatEncounterTableRows(rows []domain.Encounter, bodyW int) []string {
	lines, _ := formatEncounterTableRowsMapped(rows, bodyW)
	return lines
}

// formatEncounterTableRowsMapped returns display lines and mainLine[i] = line index for encounter i.
func formatEncounterTableRowsMapped(rows []domain.Encounter, bodyW int) (lines []string, mainLine []int) {
	showChance := encounterColumnFlags(bodyW)
	areaW := encounterAreaWidth(bodyW, showChance)
	mainLine = make([]int, len(rows))
	for i, e := range rows {
		mainLine[i] = len(lines)
		level := fmt.Sprintf("Lv %d", e.MinimumLevel)
		if e.MaximumLevel != e.MinimumLevel {
			level = fmt.Sprintf("Lv %d–%d", e.MinimumLevel, e.MaximumLevel)
		}
		game := output.GameLabel(e.Version, "")
		if game == "" {
			game = "—"
		}
		line := fmt.Sprintf("%-*s %-12s %-10s",
			areaW, truncate(output.TitleFromSlug(e.Area), areaW),
			truncate(output.TitleFromSlug(e.Method), 12),
			truncate(level, 10),
		)
		if showChance {
			line += fmt.Sprintf(" %5d%%", e.Chance)
		}
		line += fmt.Sprintf(" %-14s", truncate(game, 14))
		lines = append(lines, line)
		if len(e.Conditions) > 0 {
			conds := make([]string, 0, len(e.Conditions))
			for _, c := range e.Conditions {
				conds = append(conds, output.TitleFromSlug(c))
			}
			lines = append(lines, "  ↳ "+strings.Join(conds, ", "))
		}
	}
	return lines, mainLine
}

func formatEncounterTableRowsFocused(rows []domain.Encounter, bodyW int, focusIdx int) []string {
	lines, mainLine := formatEncounterTableRowsMapped(rows, bodyW)
	if focusIdx < 0 || focusIdx >= len(mainLine) {
		return lines
	}
	li := mainLine[focusIdx]
	if li >= 0 && li < len(lines) {
		lines[li] = tableFocusBand(stripANSIForFocus(lines[li]), bodyW)
	}
	return lines
}

func formatFlavourLinesForGame(s domain.Species, gameID int, gameSlug string) []string {
	if gameID < 1 {
		return formatFlavourLinesAll(s)
	}
	var lines []string
	seen := map[string]struct{}{}
	for _, f := range s.FlavorTextEntries {
		if f.Language != "" && f.Language != "en" {
			continue
		}
		if f.VersionID != gameID && f.Version != gameSlug {
			continue
		}
		text := output.CleanFlavor(f.Text)
		if _, ok := seen[text]; ok {
			continue
		}
		seen[text] = struct{}{}
		lines = append(lines, wrapWords(text, 56)...)
		lines = append(lines, "")
	}
	if len(lines) == 0 {
		return []string{"No records available for this selection."}
	}
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// formatFlavourLinesAll groups English flavour text by game version.
func formatFlavourLinesAll(s domain.Species) []string {
	grouped := map[string][]string{}
	var versions []string
	seen := map[string]map[string]struct{}{}
	for _, f := range s.FlavorTextEntries {
		if f.Language != "" && f.Language != "en" {
			continue
		}
		v := f.Version
		if v == "" {
			v = "unknown"
		}
		text := output.CleanFlavor(f.Text)
		if seen[v] == nil {
			seen[v] = map[string]struct{}{}
			versions = append(versions, v)
		}
		if _, ok := seen[v][text]; ok {
			continue
		}
		seen[v][text] = struct{}{}
		grouped[v] = append(grouped[v], text)
	}
	if len(versions) == 0 {
		return []string{"No records available for this selection."}
	}
	var lines []string
	for _, v := range versions {
		lines = append(lines, primaryText(output.GameLabel(v, "")))
		for _, text := range grouped[v] {
			lines = append(lines, wrapWords(text, 56)...)
			lines = append(lines, "")
		}
	}
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func formatMoveDataRowsFocused(rows []domain.LevelUpMove, bodyW int, focus bool, focusIdx int) []string {
	lines := formatMoveDataRows(rows, bodyW)
	if !focus || len(rows) == 0 || focusIdx < 0 || focusIdx >= len(lines) {
		return lines
	}
	lines[focusIdx] = tableFocusBand(stripANSIForFocus(lines[focusIdx]), bodyW)
	return lines
}

func tableHeaderBand(plain string, bodyW int) string {
	_ = bodyW
	return boldPrimary(stripANSIForFocus(plain))
}

func tableFocusBand(plain string, bodyW int) string {
	padded := textwidth.PadRight(stripANSIForFocus(plain), max(1, bodyW))
	// White selection band — keeps primary accent for labels/headers only.
	return lipgloss.NewStyle().
		Foreground(tuistyle.BaseColor()).
		Background(lipgloss.Color("#ffffff")).
		Bold(true).
		Render(padded)
}

func formatMoveDetailLines(rows []domain.LevelUpMove, idx, bodyW int) []string {
	if len(rows) == 0 {
		return []string{"No moves available."}
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(rows) {
		idx = len(rows) - 1
	}
	mv := rows[idx].Move
	name := mv.DisplayName
	if name == "" {
		name = output.TitleFromSlug(mv.Slug)
	}
	pow, acc := "—", "—"
	if mv.Power != nil {
		pow = fmt.Sprintf("%d", *mv.Power)
	}
	if mv.Accuracy != nil {
		acc = fmt.Sprintf("%d", *mv.Accuracy)
	}
	lines := []string{
		boldPrimary(name),
		fmt.Sprintf("%s · %s · Pow %s · Acc %s · PP %d",
			output.TitleFromSlug(mv.Type), output.TitleFromSlug(mv.DamageClass), pow, acc, mv.PP),
		"",
	}
	if mv.Effect != "" {
		lines = append(lines, wrapWords(mv.Effect, max(24, bodyW))...)
	} else {
		lines = append(lines, "No effect text available.")
	}
	return lines
}

func stripANSIForFocus(s string) string {
	// Focus restyles the whole line; drop prior colour codes if present.
	var b strings.Builder
	inESC := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\x1b' {
			inESC = true
			continue
		}
		if inESC {
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
				inESC = false
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func moveColumnFlags(bodyW int) (showAcc, showPP bool) {
	const baseW, accCol, ppCol = 48, 6, 5
	showAcc, showPP = true, true
	need := baseW + accCol + ppCol
	if bodyW > 0 && bodyW < need {
		showAcc = false
		need -= accCol
	}
	if bodyW > 0 && bodyW < need {
		showPP = false
	}
	return showAcc, showPP
}

// moveNameWidth gives leftover body width to the Move column.
func moveNameWidth(bodyW int, showAcc, showPP bool) int {
	// Lvl(5)+sp + Type(10)+sp + Cat(8)+sp + Pow(5) = 32
	fixed := 5 + 1 + 10 + 1 + 8 + 1 + 5
	if showAcc {
		fixed += 1 + 5
	}
	if showPP {
		fixed += 1 + 4
	}
	w := bodyW - fixed
	if w < 12 {
		w = 12
	}
	return w
}

func moveStickyLines(rows []domain.LevelUpMove, bodyW int, method string) []string {
	if method == "" {
		method = "level-up"
	}
	showAcc, showPP := moveColumnFlags(bodyW)
	moveW := moveNameWidth(bodyW, showAcc, showPP)
	return []string{tableHeaderBand(moveTableHeader(method, moveW, showAcc, showPP), bodyW)}
}

// formatMoveLines builds a full moves view (sticky + rows) for tests.
func formatMoveLines(rows []domain.LevelUpMove, bodyW int) []string {
	if len(rows) == 0 {
		return []string{"No records available for this selection."}
	}
	method := rows[0].Method
	if method == "" {
		method = "level-up"
	}
	out := moveStickyLines(rows, bodyW, method)
	out = append(out, formatMoveDataRows(rows, bodyW)...)
	return out
}

func formatMoveDataRows(rows []domain.LevelUpMove, bodyW int) []string {
	if len(rows) == 0 {
		return []string{"No records available for this selection."}
	}
	showAcc, showPP := moveColumnFlags(bodyW)
	moveW := moveNameWidth(bodyW, showAcc, showPP)
	method := rows[0].Method
	if method == "" {
		method = "level-up"
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		rm := row.Method
		if rm == "" {
			rm = method
		}
		lines = append(lines, formatMoveTableRow(row, rm, moveW, showAcc, showPP))
	}
	return lines
}

func methodHeading(method string) string {
	switch method {
	case "level-up":
		return "Level-up"
	case "machine":
		return "Machine"
	case "egg":
		return "Egg"
	case "tutor":
		return "Tutor"
	default:
		return output.TitleFromSlug(method)
	}
}

func moveTableHeader(method string, moveW int, showAcc, showPP bool) string {
	if moveW < 4 {
		moveW = 4
	}
	var header string
	if method == "level-up" {
		header = fmt.Sprintf("%-5s %-*s %-10s %-8s %5s", "Lvl", moveW, "Move", "Type", "Cat", "Pow")
	} else {
		header = fmt.Sprintf("%-5s %-*s %-10s %-8s %5s", "", moveW, "Move", "Type", "Cat", "Pow")
	}
	if showAcc {
		header += fmt.Sprintf(" %5s", "Acc")
	}
	if showPP {
		header += fmt.Sprintf(" %4s", "PP")
	}
	return header
}

func formatMoveTableRow(row domain.LevelUpMove, method string, moveW int, showAcc, showPP bool) string {
	if moveW < 4 {
		moveW = 4
	}
	pow := "—"
	if row.Move.Power != nil {
		pow = fmt.Sprintf("%d", *row.Move.Power)
	}
	name := row.Move.DisplayName
	if name == "" {
		name = output.TitleFromSlug(row.Move.Slug)
	}
	lvl := ""
	if method == "level-up" {
		lvl = fmt.Sprintf("%d", row.Level)
	}
	line := fmt.Sprintf("%-5s %-*s %-10s %-8s %5s",
		lvl,
		moveW, truncate(name, moveW),
		truncate(output.TitleFromSlug(row.Move.Type), 10),
		truncate(output.TitleFromSlug(row.Move.DamageClass), 8),
		pow,
	)
	if showAcc {
		acc := "—"
		if row.Move.Accuracy != nil {
			acc = fmt.Sprintf("%d", *row.Move.Accuracy)
		}
		line += fmt.Sprintf(" %5s", acc)
	}
	if showPP {
		line += fmt.Sprintf(" %4d", row.Move.PP)
	}
	return line
}

func formatFlavourLines(s domain.Species) []string {
	type entry struct {
		order int
		ver   string
		text  string
	}
	var entries []entry
	for _, f := range s.FlavorTextEntries {
		if f.Language != "en" {
			continue
		}
		entries = append(entries, entry{
			order: f.VersionOrder,
			ver:   f.Version,
			text:  output.CleanFlavor(f.Text),
		})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].order > entries[j].order
	})
	if len(entries) == 0 {
		return []string{"No flavour text available."}
	}
	var lines []string
	for _, e := range entries {
		ver := e.ver
		if ver == "" {
			ver = "unknown"
		}
		lines = append(lines, primaryText(output.TitleFromSlug(ver)))
		lines = append(lines, wrapWords(e.text, 56)...)
		lines = append(lines, "")
	}
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func formatGenderRate(rate int) string {
	if rate < 0 {
		return "Genderless"
	}
	female := float64(rate) / 8 * 100
	return fmt.Sprintf("%.1f%% female", female)
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n <= 1 {
		return string(runes[:n])
	}
	return string(runes[:n-1]) + "…"
}

func wrapWords(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	var cur strings.Builder
	for _, w := range words {
		if cur.Len() == 0 {
			cur.WriteString(w)
			continue
		}
		if cur.Len()+1+len(w) > width {
			lines = append(lines, cur.String())
			cur.Reset()
			cur.WriteString(w)
			continue
		}
		cur.WriteByte(' ')
		cur.WriteString(w)
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}

// overviewBlurb picks English flavour for Overview. With a game selected, use
// that game's entry; otherwise the chronologically newest English entry.
func overviewBlurb(s domain.Species, gameID int, gameSlug string) string {
	if gameID > 0 || gameSlug != "" {
		for _, f := range s.FlavorTextEntries {
			if f.Language != "" && f.Language != "en" {
				continue
			}
			if (gameID > 0 && f.VersionID == gameID) || (gameSlug != "" && f.Version == gameSlug) {
				return output.CleanFlavor(f.Text)
			}
		}
	}
	bestOrder := -1
	text := ""
	for _, f := range s.FlavorTextEntries {
		if f.Language != "" && f.Language != "en" {
			continue
		}
		if f.VersionOrder >= bestOrder {
			bestOrder = f.VersionOrder
			text = f.Text
		}
	}
	return output.CleanFlavor(text)
}
