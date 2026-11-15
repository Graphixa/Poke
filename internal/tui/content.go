package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/output"
	"github.com/Graphixa/Pokedex/internal/tui/components"
	tuistyle "github.com/Graphixa/Pokedex/internal/tui/style"
	"github.com/Graphixa/Pokedex/internal/usererr"
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
	lines := components.StatsBars(p.Stats)
	lines = append([]string{tuistyle.StatusMuted("Base stats (not battle-calculated)")}, lines...)
	return lines
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
		return prependGeneralRef(m.formatOverviewLines(poke, r.Species, bodyW))
	case DialogStats:
		return prependGeneralRef(formatStatsBars(poke))
	case DialogEvolution:
		if !r.HasEvolution && r.loadingExtras {
			return []string{"Loading…"}
		}
		if r.EvolutionErr != nil {
			return sectionErrorLines(r.EvolutionErr)
		}
		return prependGeneralRef(formatEvolutionLines(r.Evolution, r.Species.Slug, r.Pokemon.DisplayName))
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
		// Scrollable rows only; sticky header/position come from dialogMoveChrome.
		return formatMoveDataRowsFocused(moves, bodyW, m.dialogContentFocus, m.moveFocusIdx)
	case DialogEncounters:
		if !r.HasEncounters && r.loadingExtras {
			return []string{"Loading…"}
		}
		if r.EncountersErr != nil {
			return sectionErrorLines(r.EncountersErr)
		}
		return formatEncounterLinesForGame(r.Encounters, m.dialogGameID, m.dialogGameSlug)
	case DialogDescriptions:
		return formatFlavourLinesForGame(r.Species, m.dialogGameID, m.dialogGameSlug)
	case DialogBiology:
		return prependGeneralRef(formatBiologyLines(poke, r.Species))
	default:
		return nil
	}
}

func prependGeneralRef(lines []string) []string {
	out := []string{tuistyle.StatusMuted("General reference"), ""}
	return append(out, lines...)
}

func sectionErrorLines(err error) []string {
	return []string{
		usererr.Message(err),
		"",
		tuistyle.StatusMuted("Retry: leave and re-open this dossier, or rebuild the dataset."),
	}
}

// dialogMoveChrome returns sticky learnset banner + table header, and a position hint.
func (m Model) dialogMoveChrome(bodyW, scrollH, scrollOff int) (sticky []string, hint string) {
	if m.dialogSection != DialogMoves || m.moveDetailOpen || m.dialogOverlayOpen() {
		return nil, ""
	}
	rows := m.dialogMoveRows()
	if len(rows) == 0 {
		return nil, ""
	}
	gameLabel := m.dialogGameLabel
	if m.dialogGameID < 1 {
		gameLabel = "All"
	}
	sticky = moveStickyLines(rows, bodyW, gameLabel, m.dialogMoveMethod)
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
	// When content-focused, prefer selection window around focus.
	if m.dialogContentFocus && m.moveFocusIdx >= 0 {
		start = m.moveFocusIdx + 1
		end = start
		if end > len(rows) {
			end = len(rows)
		}
		hint = fmt.Sprintf("Row %d of %d", start, len(rows))
		return sticky, hint
	}
	if len(rows) > vis {
		hint = fmt.Sprintf("Rows %d–%d of %d", start, end, len(rows))
	} else if len(rows) > 0 {
		hint = fmt.Sprintf("Rows 1–%d of %d", len(rows), len(rows))
	}
	return sticky, hint
}

func (m Model) formatOverviewLines(p domain.Pokemon, s domain.Species, bodyW int) []string {
	return formatOverviewLines(p, s, bodyW, m.catalog)
}

func formatOverviewLines(p domain.Pokemon, s domain.Species, bodyW int, cat CatalogSource) []string {
	if bodyW < 24 {
		bodyW = 24
	}
	wrapW := bodyW
	if wrapW > 56 {
		wrapW = 56
	}

	var lines []string
	if text, ver := referenceSummary(s); text != "" {
		lines = append(lines, wrapWords(text, wrapW)...)
		if ver != "" {
			lines = append(lines, tuistyle.StatusMuted("Source: "+output.TitleFromSlug(ver)+" (reference)"))
		}
		lines = append(lines, "")
	}

	types := tuistyle.TypeSummary(p.TypeNames())
	genus := strings.TrimSpace(s.Genus)
	if genus == "" {
		genus = "—"
	}
	lines = append(lines,
		factLine("Type", types),
		factLine("Category", genus),
	)
	if bodyW >= 52 {
		lines = append(lines, factPair("Height", formatHeight(p.HeightDecimeter), "Weight", formatWeight(p.WeightHectogram)))
	} else {
		lines = append(lines,
			factLine("Height", formatHeight(p.HeightDecimeter)),
			factLine("Weight", formatWeight(p.WeightHectogram)),
		)
	}
	gen := dash(output.GenerationNumeral(s.Generation))
	lines = append(lines, factLine("Generation", gen))

	if chart, err := catalogTypeChart(cat); err == nil && len(chart) > 0 {
		weak, resist, immune := typeMatchups(p.TypeNames(), chart)
		lines = append(lines, "", tuistyle.StatusMuted("Type matchups (type-only)"))
		lines = append(lines, factLine("Weak to", joinOrDash(weak)))
		lines = append(lines, factLine("Resists", joinOrDash(resist)))
		if len(immune) > 0 {
			lines = append(lines, factLine("Immune", joinOrDash(immune)))
		}
	}

	if len(p.Abilities) > 0 {
		lines = append(lines, "", primaryText("Abilities"))
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

func formatAbilityLines(slots []domain.AbilitySlot, cat CatalogSource, wrapW int) []string {
	var lines []string
	for _, slot := range slots {
		name := output.TitleFromSlug(slot.Name)
		label := fmt.Sprintf("Slot %d", slot.Slot)
		if slot.IsHidden || slot.Slot >= 3 {
			label = "Hidden"
		}
		lines = append(lines, primaryText(label)+"  "+name)
		if effect := abilityEffect(slot, cat); effect != "" {
			for _, w := range wrapWords(effect, wrapW) {
				lines = append(lines, "  "+w)
			}
		}
	}
	return lines
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
	return primaryText(fmt.Sprintf("%-10s", label)) + " " + value
}

func factPair(l1, v1, l2, v2 string) string {
	left := primaryText(fmt.Sprintf("%-10s", l1)) + " " + fmt.Sprintf("%-14s", v1)
	right := primaryText(fmt.Sprintf("%-10s", l2)) + " " + v2
	return left + "  " + right
}

func formatBiologyLines(p domain.Pokemon, s domain.Species) []string {
	var lines []string
	lines = append(lines, primaryText("Physical"))
	lines = append(lines,
		factLine("Height", formatHeight(p.HeightDecimeter)),
		factLine("Weight", formatWeight(p.WeightHectogram)),
		factLine("Colour", dash(output.TitleFromSlug(s.Color))),
		factLine("Shape", dash(output.TitleFromSlug(s.Shape))),
	)
	lines = append(lines, "", primaryText("Habitat"))
	lines = append(lines, factLine("Habitat", dash(output.TitleFromSlug(s.Habitat))))

	lines = append(lines, "", primaryText("Breeding"))
	lines = append(lines, factLine("Gender", formatGenderRate(s.GenderRate)))
	if len(s.EggGroups) == 0 {
		lines = append(lines, factLine("Egg", "—"))
	} else {
		eggs := make([]string, len(s.EggGroups))
		for i, g := range s.EggGroups {
			eggs[i] = output.TitleFromSlug(g)
		}
		lines = append(lines, factLine("Egg", strings.Join(eggs, ", ")))
	}
	if s.HatchCounter > 0 {
		lines = append(lines, factLine("Hatch", fmt.Sprintf("%d cycles", s.HatchCounter)))
	} else {
		lines = append(lines, factLine("Hatch", "—"))
	}

	lines = append(lines, "", primaryText("Growth"))
	lines = append(lines,
		factLine("Growth", dash(output.TitleFromSlug(s.GrowthRate))),
		factLine("Happiness", fmt.Sprintf("%d", s.BaseHappiness)),
		factLine("Capture", fmt.Sprintf("%d", s.CaptureRate)),
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
		lines = append(lines, "", primaryText("Classification"))
		lines = append(lines, "  "+strings.Join(class, ", "))
	}

	if len(s.Varieties) > 0 {
		lines = append(lines, "", primaryText("Forms"))
		for _, v := range s.Varieties {
			name := output.TitleFromSlug(v.Slug)
			if v.IsDefault {
				lines = append(lines, "  "+name+"  (default)")
			} else {
				lines = append(lines, "  "+name)
			}
		}
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

func formatEncounterLines(rows []domain.Encounter) []string {
	if len(rows) == 0 {
		return []string{"No wild encounters recorded."}
	}
	grouped := map[string][]domain.Encounter{}
	var versions []string
	for _, e := range rows {
		v := e.Version
		if v == "" {
			v = "unknown"
		}
		if _, ok := grouped[v]; !ok {
			versions = append(versions, v)
		}
		grouped[v] = append(grouped[v], e)
	}
	sort.Strings(versions)
	var lines []string
	for _, v := range versions {
		lines = append(lines, primaryText(output.TitleFromSlug(v)))
		for _, e := range grouped[v] {
			level := fmt.Sprintf("Lv %d", e.MinimumLevel)
			if e.MaximumLevel != e.MinimumLevel {
				level = fmt.Sprintf("Lv %d–%d", e.MinimumLevel, e.MaximumLevel)
			}
			lines = append(lines, fmt.Sprintf("  %s · %s · %s · %d%%",
				output.TitleFromSlug(e.Area),
				output.TitleFromSlug(e.Method),
				level,
				e.Chance,
			))
			if len(e.Conditions) > 0 {
				conds := make([]string, 0, len(e.Conditions))
				for _, c := range e.Conditions {
					conds = append(conds, output.TitleFromSlug(c))
				}
				lines = append(lines, "    "+strings.Join(conds, ", "))
			}
		}
		lines = append(lines, "")
	}
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func formatEncounterLinesForGame(rows []domain.Encounter, gameID int, gameSlug string) []string {
	if gameID < 1 {
		return formatEncounterLines(rows)
	}
	var filtered []domain.Encounter
	for _, e := range rows {
		if e.VersionID == gameID || (gameSlug != "" && e.Version == gameSlug) {
			filtered = append(filtered, e)
		}
	}
	if len(filtered) == 0 {
		return []string{"No records available for this selection."}
	}
	return formatEncounterLines(filtered)
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
		lines = append(lines, primaryText(output.TitleFromSlug(v)))
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
	lines[focusIdx] = boldPrimary(stripANSIForFocus(lines[focusIdx]))
	return lines
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
		tuistyle.StatusMuted("Move data: current reference"),
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

func moveStickyLines(rows []domain.LevelUpMove, bodyW int, gameLabel, method string) []string {
	if method == "" {
		method = "level-up"
	}
	showAcc, showPP := moveColumnFlags(bodyW)
	banner := "Move attributes: current."
	if gameLabel != "" {
		banner = fmt.Sprintf("Learnset: %s · Move attributes: current.", gameLabel)
	}
	return []string{
		tuistyle.StatusMuted(banner),
		primaryText(moveTableHeader(method, showAcc, showPP)),
	}
}

// formatMoveLines builds a full moves view (sticky + rows) for tests.
func formatMoveLines(rows []domain.LevelUpMove, bodyW int, gameLabel string) []string {
	if len(rows) == 0 {
		return []string{"No records available for this selection."}
	}
	method := rows[0].Method
	if method == "" {
		method = "level-up"
	}
	out := moveStickyLines(rows, bodyW, gameLabel, method)
	out = append(out, formatMoveDataRows(rows, bodyW)...)
	return out
}

func formatMoveDataRows(rows []domain.LevelUpMove, bodyW int) []string {
	if len(rows) == 0 {
		return []string{"No records available for this selection."}
	}
	showAcc, showPP := moveColumnFlags(bodyW)
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
		lines = append(lines, formatMoveTableRow(row, rm, showAcc, showPP))
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

func moveTableHeader(method string, showAcc, showPP bool) string {
	var header string
	if method == "level-up" {
		header = fmt.Sprintf("%-5s %-16s %-10s %-8s %5s", "Lvl", "Move", "Type", "Cat", "Pow")
	} else {
		header = fmt.Sprintf("%-5s %-16s %-10s %-8s %5s", "", "Move", "Type", "Cat", "Pow")
	}
	if showAcc {
		header += fmt.Sprintf(" %5s", "Acc")
	}
	if showPP {
		header += fmt.Sprintf(" %4s", "PP")
	}
	return header
}

func formatMoveTableRow(row domain.LevelUpMove, method string, showAcc, showPP bool) string {
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
	line := fmt.Sprintf("%-5s %-16s %-10s %-8s %5s",
		lvl,
		truncate(name, 16),
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

// referenceSummary picks a stable English flavour entry for Overview (not the
// selected game). Prefer highest VersionOrder; return its version slug for labelling.
func referenceSummary(s domain.Species) (text, version string) {
	bestOrder := -1
	for _, f := range s.FlavorTextEntries {
		if f.Language != "" && f.Language != "en" {
			continue
		}
		if f.VersionOrder >= bestOrder {
			bestOrder = f.VersionOrder
			text = f.Text
			version = f.Version
		}
	}
	return output.CleanFlavor(text), version
}
