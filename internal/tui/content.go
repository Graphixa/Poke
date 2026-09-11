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

func (m Model) browseViewLines() []string {
	// Browse is sprite-only; text detail lives in the Enter dialog.
	return nil
}

func formatStatsBars(p domain.Pokemon) []string {
	return components.StatsBars(p.Stats)
}

func formatBrowseSummaryLines(p domain.Pokemon, s domain.Species) []string {
	abilities, hidden := splitAbilityNames(p.Abilities)
	ability := strings.Join(abilities, " / ")
	if ability == "" {
		ability = "—"
	}
	hiddenLabel := hidden
	if hiddenLabel == "" {
		hiddenLabel = "—"
	}
	lines := []string{
		fmt.Sprintf("Height  %-12s  Weight  %s", formatHeight(p.HeightDecimeter), formatWeight(p.WeightHectogram)),
		fmt.Sprintf("Ability %-12s  Hidden  %s", ability, hiddenLabel),
		"",
	}
	if summary := latestSummary(s); summary != "" {
		lines = append(lines, wrapWords(summary, 48)...)
		lines = append(lines, "")
	}
	gen := output.TitleFromSlug(s.Generation)
	if gen == "" {
		gen = "—"
	}
	lines = append(lines, gen)
	return lines
}

func splitAbilityNames(abilities []domain.AbilitySlot) (normal []string, hidden string) {
	for _, a := range abilities {
		name := output.TitleFromSlug(a.Name)
		if a.IsHidden {
			hidden = name
			continue
		}
		normal = append(normal, name)
	}
	return normal, hidden
}

func formatHeight(dm int) string {
	return fmt.Sprintf("%.1f m", float64(dm)/10)
}

func formatWeight(hg int) string {
	return fmt.Sprintf("%.1f kg", float64(hg)/10)
}

func formatBrowseEvolutionLines(tree domain.EvolutionTree) []string {
	lines := formatEvolutionLines(tree)
	// Drop the verbose "Trigger source" line for the glance view.
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(line, "Trigger source:") {
			continue
		}
		out = append(out, line)
	}
	return out
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

	switch m.dialogSection {
	case DialogSummary:
		return formatBrowseSummaryLines(r.Pokemon, r.Species)
	case DialogStats:
		return formatStatsBars(r.Pokemon)
	case DialogEvolution:
		if !r.HasEvolution && r.loadingExtras {
			return []string{"Loading…"}
		}
		if r.EvolutionErr != nil {
			return []string{usererr.Message(r.EvolutionErr)}
		}
		return formatBrowseEvolutionLines(r.Evolution)
	case DialogMoves:
		if !r.HasMoves && r.loadingExtras {
			return []string{"Loading…"}
		}
		if r.MovesErr != nil {
			return []string{usererr.Message(r.MovesErr)}
		}
		return formatMoveLines(r.Moves)
	case DialogLocations:
		if !r.HasEncounters && r.loadingExtras {
			return []string{"Loading…"}
		}
		if r.EncountersErr != nil {
			return []string{usererr.Message(r.EncountersErr)}
		}
		return formatEncounterLines(r.Encounters)
	case DialogAbilities:
		return formatAbilityDialogLines(r.Pokemon)
	case DialogFlavour:
		return formatFlavourLines(r.Species)
	case DialogBreeding:
		return formatBreedingLines(r.Species)
	case DialogTraining:
		return formatTrainingLines(r.Species)
	case DialogForms:
		return formatFormsLines(r.Species)
	case DialogMeta:
		return formatMetaLines(r.Pokemon, r.Species)
	default:
		return nil
	}
}

func formatAbilityDialogLines(p domain.Pokemon) []string {
	if len(p.Abilities) == 0 {
		return []string{"No abilities."}
	}
	var lines []string
	for _, a := range p.Abilities {
		name := output.TitleFromSlug(a.Name)
		slot := primaryText(fmt.Sprintf("Slot %d", a.Slot))
		if a.IsHidden {
			lines = append(lines, fmt.Sprintf("%s  %s  (hidden)", slot, name))
		} else {
			lines = append(lines, fmt.Sprintf("%s  %s", slot, name))
		}
	}
	return lines
}

func formatBreedingLines(s domain.Species) []string {
	return []string{
		fmt.Sprintf("Gender rate:  %s", formatGenderRate(s.GenderRate)),
		fmt.Sprintf("Legendary:    %s", yesNo(s.IsLegendary)),
		fmt.Sprintf("Mythical:     %s", yesNo(s.IsMythical)),
	}
}

func formatTrainingLines(s domain.Species) []string {
	return []string{
		fmt.Sprintf("Growth rate:    %s", dash(output.TitleFromSlug(s.GrowthRate))),
		fmt.Sprintf("Capture rate:   %d", s.CaptureRate),
		fmt.Sprintf("Base happiness: %d", s.BaseHappiness),
	}
}

func formatFormsLines(s domain.Species) []string {
	if len(s.Varieties) == 0 {
		return []string{"No alternate forms listed."}
	}
	var lines []string
	for _, v := range s.Varieties {
		name := output.TitleFromSlug(v.Slug)
		if v.IsDefault {
			lines = append(lines, name+"  (default)")
		} else {
			lines = append(lines, name)
		}
	}
	return lines
}

func formatMetaLines(p domain.Pokemon, s domain.Species) []string {
	lines := []string{
		fmt.Sprintf("Height:   %s", formatHeight(p.HeightDecimeter)),
		fmt.Sprintf("Weight:   %s", formatWeight(p.WeightHectogram)),
		fmt.Sprintf("Base Exp: %d", p.BaseExperience),
		"",
	}
	lines = append(lines, formatInfoLines(s)...)
	return lines
}

func formatEvolutionLines(tree domain.EvolutionTree) []string {
	var lines []string
	writeEvoNode(&lines, tree.Root, true)
	if len(lines) == 0 {
		return []string{"No evolution data."}
	}
	rootName := tree.Root.DisplayName
	if rootName == "" {
		rootName = output.TitleFromSlug(tree.Root.SpeciesSlug)
	}
	finalName := evolutionFinalName(tree.Root)
	lines = append(lines, "")
	if len(tree.Root.Children) == 0 {
		lines = append(lines, "Stage: Final")
	} else {
		lines = append(lines, "Stage: Basic")
	}
	lines = append(lines, "Final evolution: "+primaryText(finalName))
	lines = append(lines, "Trigger source: evolution-chain")
	_ = rootName
	return lines
}

func evolutionFinalName(node domain.EvolutionNode) string {
	if len(node.Children) == 0 {
		name := node.DisplayName
		if name == "" {
			return output.TitleFromSlug(node.SpeciesSlug)
		}
		return name
	}
	// Prefer the last branch's deepest leaf (linear chains).
	last := node.Children[len(node.Children)-1].Target
	return evolutionFinalName(last)
}

func writeEvoNode(lines *[]string, node domain.EvolutionNode, isRoot bool) {
	name := node.DisplayName
	if name == "" {
		name = output.TitleFromSlug(node.SpeciesSlug)
	}
	if isRoot && len(node.Children) == 0 {
		*lines = append(*lines, primaryText(name)+" (no evolutions)")
		return
	}
	for _, branch := range node.Children {
		label := formatEvoConditions(branch.Conditions)
		if label == "" {
			label = "Evolution"
		}
		target := branch.Target.DisplayName
		if target == "" {
			target = output.TitleFromSlug(branch.Target.SpeciesSlug)
		}
		*lines = append(*lines, fmt.Sprintf("%s ── %s ──► %s", primaryText(name), label, primaryText(target)))
		writeEvoNode(lines, branch.Target, false)
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
	return strings.Join(parts, "; ")
}

func formatOneCondition(c domain.EvolutionCondition) string {
	var parts []string
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
	if c.MinimumHappiness != nil {
		parts = append(parts, fmt.Sprintf("Friendship %d+", *c.MinimumHappiness))
	}
	if c.TimeOfDay != "" {
		parts = append(parts, c.TimeOfDay)
	}
	if c.Location != "" {
		parts = append(parts, "at "+output.TitleFromSlug(c.Location))
	}
	if len(parts) > 0 {
		return strings.Join(parts, ", ")
	}
	switch c.Trigger {
	case "level-up":
		return "Level up"
	case "trade":
		return "Trade"
	case "use-item":
		return "Use item"
	case "":
		return ""
	default:
		return output.TitleFromSlug(c.Trigger)
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
		}
		lines = append(lines, "")
	}
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func formatMoveLines(rows []LevelUpMove) []string {
	if len(rows) == 0 {
		return []string{"No level-up moves available."}
	}
	header := fmt.Sprintf("%-5s %-16s %-10s %-8s %5s %5s", "Lvl", "Move", "Type", "Cat", "Pow", "Acc")
	lines := []string{primaryText(header)}
	for _, row := range rows {
		pow := "—"
		if row.Move.Power != nil {
			pow = fmt.Sprintf("%d", *row.Move.Power)
		}
		acc := "—"
		if row.Move.Accuracy != nil {
			acc = fmt.Sprintf("%d", *row.Move.Accuracy)
		}
		name := row.Move.DisplayName
		if name == "" {
			name = output.TitleFromSlug(row.Move.Slug)
		}
		lines = append(lines, fmt.Sprintf("%-5d %-16s %-10s %-8s %5s %5s",
			row.Level,
			truncate(name, 16),
			truncate(output.TitleFromSlug(row.Move.Type), 10),
			truncate(output.TitleFromSlug(row.Move.DamageClass), 8),
			pow,
			acc,
		))
	}
	return lines
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

func formatInfoLines(s domain.Species) []string {
	return []string{
		fmt.Sprintf("Generation:    %s", output.TitleFromSlug(s.Generation)),
		fmt.Sprintf("Habitat:       %s", dash(output.TitleFromSlug(s.Habitat))),
		fmt.Sprintf("Shape:         %s", dash(output.TitleFromSlug(s.Shape))),
		fmt.Sprintf("Colour:        %s", dash(output.TitleFromSlug(s.Color))),
		fmt.Sprintf("Growth rate:   %s", dash(output.TitleFromSlug(s.GrowthRate))),
		fmt.Sprintf("Capture rate:  %d", s.CaptureRate),
		fmt.Sprintf("Base happiness: %d", s.BaseHappiness),
		fmt.Sprintf("Gender rate:   %s", formatGenderRate(s.GenderRate)),
		fmt.Sprintf("Legendary:     %s", yesNo(s.IsLegendary)),
		fmt.Sprintf("Mythical:      %s", yesNo(s.IsMythical)),
	}
}

func formatGenderRate(rate int) string {
	if rate < 0 {
		return "Genderless"
	}
	// PokeAPI: eighths female; -1 genderless.
	female := float64(rate) / 8 * 100
	return fmt.Sprintf("%.1f%% female", female)
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
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

func latestSummary(s domain.Species) string {
	bestOrder := -1
	text := ""
	for _, f := range s.FlavorTextEntries {
		if f.Language != "en" {
			continue
		}
		if f.VersionOrder >= bestOrder {
			bestOrder = f.VersionOrder
			text = f.Text
		}
	}
	return output.CleanFlavor(text)
}
