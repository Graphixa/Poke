package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/output"
	"github.com/Graphixa/Pokedex/internal/usererr"
)

func (m Model) detailContentLines() []string {
	sel, ok := m.selected()
	if !ok {
		return []string{"No Pokémon selected."}
	}
	r := m.resources[sel.ID]
	if r == nil || (!r.HasCore && r.loadingCore) {
		return []string{"Loading…"}
	}
	if r != nil && r.CoreErr != nil && !r.HasCore {
		return []string{usererr.Message(r.CoreErr)}
	}
	if r == nil || !r.HasCore {
		return []string{"Loading…"}
	}

	switch m.activeTab {
	case TabStats:
		return formatStatsLines(r.Pokemon)
	case TabEvolution:
		if r.loadingEvolution {
			return []string{"Loading evolution…"}
		}
		if r.EvolutionErr != nil {
			return []string{usererr.Message(r.EvolutionErr)}
		}
		if !r.HasEvolution {
			return []string{"Loading evolution…"}
		}
		return formatEvolutionLines(r.Evolution)
	case TabLocations:
		if r.loadingEncounters {
			return []string{"Loading locations…"}
		}
		if r.EncountersErr != nil {
			return []string{usererr.Message(r.EncountersErr)}
		}
		if !r.HasEncounters {
			return []string{"Loading locations…"}
		}
		return formatEncounterLines(r.Encounters)
	case TabMoves:
		if r.loadingMoves {
			return []string{"Loading moves…"}
		}
		if r.MovesErr != nil {
			return []string{usererr.Message(r.MovesErr)}
		}
		if !r.HasMoves {
			return []string{"Loading moves…"}
		}
		return formatMoveLines(r.Moves)
	case TabFlavour:
		return formatFlavourLines(r.Species)
	case TabInfo:
		return formatInfoLines(r.Species)
	default:
		return nil
	}
}

func formatStatsLines(p domain.Pokemon) []string {
	return []string{
		fmt.Sprintf("HP               %3d", p.Stats.HP),
		fmt.Sprintf("Attack           %3d", p.Stats.Attack),
		fmt.Sprintf("Defense          %3d", p.Stats.Defense),
		fmt.Sprintf("Special Attack   %3d", p.Stats.SpecialAttack),
		fmt.Sprintf("Special Defense  %3d", p.Stats.SpecialDefense),
		fmt.Sprintf("Speed            %3d", p.Stats.Speed),
		"",
		fmt.Sprintf("Total            %3d", p.Stats.Total()),
	}
}

func formatEvolutionLines(tree domain.EvolutionTree) []string {
	var lines []string
	writeEvoNode(&lines, tree.Root, true)
	if len(lines) == 0 {
		return []string{"No evolution data."}
	}
	return lines
}

func writeEvoNode(lines *[]string, node domain.EvolutionNode, isRoot bool) {
	name := node.DisplayName
	if name == "" {
		name = output.TitleFromSlug(node.SpeciesSlug)
	}
	if isRoot && len(node.Children) == 0 {
		*lines = append(*lines, name+" (no evolutions)")
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
		*lines = append(*lines, fmt.Sprintf("%s ── %s ──► %s", name, label, target))
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
		lines = append(lines, output.TitleFromSlug(v))
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
	lines := []string{fmt.Sprintf("%-5s %-16s %-10s %-8s %5s %5s", "Lvl", "Move", "Type", "Cat", "Pow", "Acc")}
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
		lines = append(lines, output.TitleFromSlug(ver))
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

func abbreviateTypes(types []string, max int) string {
	joined := output.JoinTypes(types)
	if max <= 0 || len(joined) <= max {
		return joined
	}
	if len(types) < 2 {
		return truncate(joined, max)
	}
	first := output.TitleFromSlug(types[0])
	second := output.TitleFromSlug(types[1])
	if len(second) > 4 {
		second = second[:4]
	}
	out := first + "/" + second
	if len(out) > max {
		return truncate(out, max)
	}
	return out
}
