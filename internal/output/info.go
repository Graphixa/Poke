package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/Graphixa/Poke/internal/domain"
)

// Stats writes the six base stats and total.
func Stats(w io.Writer, p domain.Pokemon, displayName string) error {
	name := displayName
	if name == "" {
		name = p.DisplayName
	}
	if _, err := fmt.Fprintf(w, "%s %s\n\n", name, FormatDex(p.ID)); err != nil {
		return err
	}
	rows := []struct {
		label string
		value int
	}{
		{"HP", p.Stats.HP},
		{"Attack", p.Stats.Attack},
		{"Defense", p.Stats.Defense},
		{"Special Attack", p.Stats.SpecialAttack},
		{"Special Defense", p.Stats.SpecialDefense},
		{"Speed", p.Stats.Speed},
		{"Total", p.Stats.Total()},
	}
	labelWidth := 0
	for _, row := range rows {
		if len(row.label) > labelWidth {
			labelWidth = len(row.label)
		}
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(w, "%-*s  %3d\n", labelWidth, row.label, row.value); err != nil {
			return err
		}
	}
	return nil
}

// Info writes compact Pokémon information using English text and metric units.
func Info(w io.Writer, p domain.Pokemon, s domain.Species) error {
	name := s.DisplayName
	if name == "" {
		name = p.DisplayName
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", name, FormatDex(speciesOrPokemonID(s, p)))
	fmt.Fprintf(&b, "Type: %s\n", JoinTypes(typeNames(p)))
	if s.Genus != "" {
		fmt.Fprintf(&b, "Species: %s\n", s.Genus)
	}
	fmt.Fprintf(&b, "Height: %s\n", formatHeight(p.HeightDecimeter))
	fmt.Fprintf(&b, "Weight: %s\n", formatWeight(p.WeightHectogram))

	abilities, hidden := splitAbilities(p.Abilities)
	if len(abilities) > 0 {
		fmt.Fprintf(&b, "Ability: %s\n", strings.Join(abilities, " / "))
	}
	if hidden != "" {
		fmt.Fprintf(&b, "Hidden: %s\n", hidden)
	}
	fmt.Fprintf(&b, "Base Exp: %d\n", p.BaseExperience)
	if summary := latestEnglishFlavor(s); summary != "" {
		fmt.Fprintf(&b, "Summary: %s\n", summary)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func speciesOrPokemonID(s domain.Species, p domain.Pokemon) int {
	if s.ID != 0 {
		return s.ID
	}
	return p.ID
}

func typeNames(p domain.Pokemon) []string {
	out := make([]string, 0, len(p.Types))
	for _, t := range p.Types {
		out = append(out, t.Name)
	}
	return out
}

func formatHeight(dm int) string {
	return fmt.Sprintf("%.1f m", float64(dm)/10)
}

func formatWeight(hg int) string {
	return fmt.Sprintf("%.1f kg", float64(hg)/10)
}

func splitAbilities(abilities []domain.AbilitySlot) (normal []string, hidden string) {
	for _, a := range abilities {
		name := TitleFromSlug(a.Name)
		if a.IsHidden {
			hidden = name
			continue
		}
		normal = append(normal, name)
	}
	return normal, hidden
}

func latestEnglishFlavor(s domain.Species) string {
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
	return CleanFlavor(text)
}
