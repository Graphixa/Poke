package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/Graphixa/Pokedex/internal/domain"
)

// Evolution writes a branched evolution tree with trigger details.
func Evolution(w io.Writer, tree domain.EvolutionTree) error {
	var b strings.Builder
	name := nodeName(tree.Root)
	fmt.Fprintf(&b, "%s\n", name)
	writeBranches(&b, tree.Root.Children, "")
	_, err := io.WriteString(w, b.String())
	return err
}

func writeBranches(b *strings.Builder, children []domain.EvolutionBranch, prefix string) {
	for i, child := range children {
		last := i == len(children)-1
		branch := "├─"
		nextPrefix := prefix + "│  "
		if last {
			branch = "└─"
			nextPrefix = prefix + "   "
		}
		label := formatConditions(child.Conditions)
		if label == "" {
			label = "Evolution"
		}
		fmt.Fprintf(b, "%s%s %s → %s\n", prefix, branch, label, nodeName(child.Target))
		if len(child.Target.Children) > 0 {
			writeBranches(b, child.Target.Children, nextPrefix)
		}
	}
}

func nodeName(node domain.EvolutionNode) string {
	if node.DisplayName != "" {
		return node.DisplayName
	}
	return TitleFromSlug(node.SpeciesSlug)
}

func formatConditions(conds []domain.EvolutionCondition) string {
	if len(conds) == 0 {
		return ""
	}
	parts := make([]string, 0, len(conds))
	for _, c := range conds {
		if s := formatCondition(c); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "; ")
}

func formatCondition(c domain.EvolutionCondition) string {
	var parts []string
	if c.MinimumLevel != nil {
		parts = append(parts, fmt.Sprintf("Level %d", *c.MinimumLevel))
	}
	if c.Item != "" {
		parts = append(parts, TitleFromSlug(c.Item))
	}
	if c.HeldItem != "" {
		parts = append(parts, "holding "+TitleFromSlug(c.HeldItem))
	}
	if c.KnownMove != "" {
		parts = append(parts, "knowing "+TitleFromSlug(c.KnownMove))
	}
	if c.KnownMoveType != "" {
		parts = append(parts, "knowing "+TitleFromSlug(c.KnownMoveType)+" move")
	}
	if c.Location != "" {
		parts = append(parts, "at "+TitleFromSlug(c.Location))
	}
	if c.TimeOfDay != "" {
		parts = append(parts, c.TimeOfDay)
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
	if c.NeedsOverworldRain {
		parts = append(parts, "in rain")
	}
	if c.TurnUpsideDown {
		parts = append(parts, "turn upside-down")
	}
	if c.TradeSpecies != "" {
		parts = append(parts, "trade for "+TitleFromSlug(c.TradeSpecies))
	}
	if c.Gender != nil {
		switch *c.Gender {
		case 1:
			parts = append(parts, "female")
		case 2:
			parts = append(parts, "male")
		}
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
	case "shed":
		return "Shed"
	case "":
		return ""
	default:
		return TitleFromSlug(c.Trigger)
	}
}
