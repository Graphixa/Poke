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
	switch m.dialogSection {
	case DialogOverview:
		return formatOverviewLines(r.Pokemon, r.Species, bodyW)
	case DialogStats:
		return formatStatsBars(r.Pokemon)
	case DialogEvolution:
		if !r.HasEvolution && r.loadingExtras {
			return []string{"Loading…"}
		}
		if r.EvolutionErr != nil {
			return []string{usererr.Message(r.EvolutionErr)}
		}
		return formatEvolutionLines(r.Evolution, r.Species.Slug, r.Pokemon.DisplayName)
	case DialogMoves:
		if !r.HasMoves && r.loadingExtras {
			return []string{"Loading…"}
		}
		if r.MovesErr != nil {
			return []string{usererr.Message(r.MovesErr)}
		}
		return formatMoveLines(r.Moves, bodyW)
	case DialogLocations:
		if !r.HasEncounters && r.loadingExtras {
			return []string{"Loading…"}
		}
		if r.EncountersErr != nil {
			return []string{usererr.Message(r.EncountersErr)}
		}
		return formatEncounterLines(r.Encounters)
	case DialogPokedex:
		return formatFlavourLines(r.Species)
	case DialogBiology:
		return formatBiologyLines(r.Species)
	default:
		return nil
	}
}

func formatOverviewLines(p domain.Pokemon, s domain.Species, bodyW int) []string {
	if bodyW < 24 {
		bodyW = 24
	}
	wrapW := bodyW
	if wrapW > 56 {
		wrapW = 56
	}

	var lines []string
	if summary := latestSummary(s); summary != "" {
		lines = append(lines, wrapWords(summary, wrapW)...)
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
		lines = append(lines,
			factPair("Height", formatHeight(p.HeightDecimeter), "Weight", formatWeight(p.WeightHectogram)),
			factPair("Base Exp", fmt.Sprintf("%d", p.BaseExperience), "Catch", fmt.Sprintf("%d", s.CaptureRate)),
		)
	} else {
		lines = append(lines,
			factLine("Height", formatHeight(p.HeightDecimeter)),
			factLine("Weight", formatWeight(p.WeightHectogram)),
			factLine("Base Exp", fmt.Sprintf("%d", p.BaseExperience)),
			factLine("Catch", fmt.Sprintf("%d", s.CaptureRate)),
		)
	}

	gen := dash(output.GenerationNumeral(s.Generation))
	lines = append(lines, "", factLine("Generation", gen))
	return lines
}

func factLine(label, value string) string {
	return primaryText(fmt.Sprintf("%-10s", label)) + " " + value
}

func factPair(l1, v1, l2, v2 string) string {
	left := primaryText(fmt.Sprintf("%-10s", l1)) + " " + fmt.Sprintf("%-14s", v1)
	right := primaryText(fmt.Sprintf("%-10s", l2)) + " " + v2
	return left + "  " + right
}

func formatBiologyLines(s domain.Species) []string {
	lines := []string{
		factLine("Gender", formatGenderRate(s.GenderRate)),
		factLine("Legendary", yesNo(s.IsLegendary)),
		factLine("Mythical", yesNo(s.IsMythical)),
		"",
		factLine("Growth", dash(output.TitleFromSlug(s.GrowthRate))),
		factLine("Capture", fmt.Sprintf("%d", s.CaptureRate)),
		factLine("Happiness", fmt.Sprintf("%d", s.BaseHappiness)),
		"",
		factLine("Habitat", dash(output.TitleFromSlug(s.Habitat))),
		factLine("Shape", dash(output.TitleFromSlug(s.Shape))),
		factLine("Colour", dash(output.TitleFromSlug(s.Color))),
		"",
		primaryText("Forms"),
	}
	if len(s.Varieties) == 0 {
		lines = append(lines, "  —")
		return lines
	}
	for _, v := range s.Varieties {
		name := output.TitleFromSlug(v.Slug)
		if v.IsDefault {
			lines = append(lines, "  "+name+"  (default)")
		} else {
			lines = append(lines, "  "+name)
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
	finalName := evolutionFinalName(tree.Root)
	lines = append(lines, "")
	if len(tree.Root.Children) == 0 {
		lines = append(lines, "Stage: Final")
	} else {
		lines = append(lines, "Stage: Basic")
	}
	lines = append(lines, "Final evolution: "+primaryText(finalName))
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
	last := node.Children[len(node.Children)-1].Target
	return evolutionFinalName(last)
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

func formatMoveLines(rows []domain.LevelUpMove, bodyW int) []string {
	if len(rows) == 0 {
		return []string{"No level-up moves available."}
	}
	// Lvl + Move + Type + Cat + Pow = 48; Acc +6; PP +5.
	const baseW, accCol, ppCol = 48, 6, 5
	showAcc, showPP := true, true
	need := baseW + accCol + ppCol
	if bodyW > 0 && bodyW < need {
		showAcc = false
		need -= accCol
	}
	if bodyW > 0 && bodyW < need {
		showPP = false
	}

	header := fmt.Sprintf("%-5s %-16s %-10s %-8s %5s", "Lvl", "Move", "Type", "Cat", "Pow")
	if showAcc {
		header += fmt.Sprintf(" %5s", "Acc")
	}
	if showPP {
		header += fmt.Sprintf(" %4s", "PP")
	}
	lines := []string{primaryText(header)}
	for _, row := range rows {
		pow := "—"
		if row.Move.Power != nil {
			pow = fmt.Sprintf("%d", *row.Move.Power)
		}
		name := row.Move.DisplayName
		if name == "" {
			name = output.TitleFromSlug(row.Move.Slug)
		}
		line := fmt.Sprintf("%-5d %-16s %-10s %-8s %5s",
			row.Level,
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
		lines = append(lines, line)
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

func formatGenderRate(rate int) string {
	if rate < 0 {
		return "Genderless"
	}
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
