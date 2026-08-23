package pokeapi

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/Graphixa/Pokedex/internal/domain"
)

func idFromURL(raw string) (int, error) {
	if raw == "" {
		return 0, fmt.Errorf("empty resource url")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return 0, err
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) == 0 {
		return 0, fmt.Errorf("invalid resource url %q", raw)
	}
	return strconv.Atoi(parts[len(parts)-1])
}

func resourceName(r *namedAPIResource) string {
	if r == nil {
		return ""
	}
	return r.Name
}

func DisplayNameFromSlug(slug string) string {
	if slug == "" {
		return ""
	}
	parts := strings.Split(slug, "-")
	for i, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		parts[i] = string(runes)
	}
	return strings.Join(parts, " ")
}

func englishName(entries []struct {
	Name     string           `json:"name"`
	Language namedAPIResource `json:"language"`
}, fallback string) string {
	for _, entry := range entries {
		if entry.Language.Name == "en" && entry.Name != "" {
			return entry.Name
		}
	}
	return DisplayNameFromSlug(fallback)
}

func mapPokemon(dto pokemonDTO) (domain.Pokemon, error) {
	speciesID, err := idFromURL(dto.Species.URL)
	if err != nil {
		speciesID = 0
	}
	p := domain.Pokemon{
		ID:              dto.ID,
		Slug:            dto.Name,
		DisplayName:     DisplayNameFromSlug(dto.Name),
		SpeciesID:       speciesID,
		HeightDecimeter: dto.Height,
		WeightHectogram: dto.Weight,
		BaseExperience:  dto.BaseExperience,
		Sprites:         domain.Sprites{},
	}
	if dto.Sprites.FrontDefault != nil {
		p.Sprites.FrontDefault = *dto.Sprites.FrontDefault
	}
	for _, t := range dto.Types {
		p.Types = append(p.Types, domain.TypeSlot{Slot: t.Slot, Name: t.Type.Name})
	}
	for _, a := range dto.Abilities {
		p.Abilities = append(p.Abilities, domain.AbilitySlot{
			Slot: a.Slot, Name: a.Ability.Name, IsHidden: a.IsHidden,
		})
	}
	for _, s := range dto.Stats {
		switch s.Stat.Name {
		case "hp":
			p.Stats.HP = s.BaseStat
		case "attack":
			p.Stats.Attack = s.BaseStat
		case "defense":
			p.Stats.Defense = s.BaseStat
		case "special-attack":
			p.Stats.SpecialAttack = s.BaseStat
		case "special-defense":
			p.Stats.SpecialDefense = s.BaseStat
		case "speed":
			p.Stats.Speed = s.BaseStat
		}
	}
	for _, m := range dto.Moves {
		moveID, _ := idFromURL(m.Move.URL)
		learned := domain.LearnedMove{MoveID: moveID, Slug: m.Move.Name}
		for _, detail := range m.VersionGroupDetails {
			order, _ := idFromURL(detail.VersionGroup.URL)
			learned.Learnings = append(learned.Learnings, domain.MoveLearning{
				VersionGroup:      detail.VersionGroup.Name,
				VersionGroupOrder: order,
				Method:            detail.MoveLearnMethod.Name,
				Level:             detail.LevelLearnedAt,
			})
		}
		p.Moves = append(p.Moves, learned)
	}
	return p, nil
}

func mapSpecies(dto speciesDTO) (domain.Species, error) {
	s := domain.Species{
		ID:            dto.ID,
		Slug:          dto.Name,
		DisplayName:   englishName(dto.Names, dto.Name),
		CaptureRate:   dto.CaptureRate,
		BaseHappiness: dto.BaseHappiness,
		GenderRate:    dto.GenderRate,
		IsLegendary:   dto.IsLegendary,
		IsMythical:    dto.IsMythical,
		GrowthRate:    resourceName(dto.GrowthRate),
		Color:         resourceName(dto.Color),
		Shape:         resourceName(dto.Shape),
		Habitat:       resourceName(dto.Habitat),
		Generation:    resourceName(dto.Generation),
	}
	if id, err := idFromURL(dto.EvolutionChain.URL); err == nil {
		s.EvolutionChainID = id
	}
	for _, n := range dto.Names {
		s.Names = append(s.Names, domain.LocalizedName{Language: n.Language.Name, Value: n.Name})
	}
	for _, g := range dto.Genera {
		s.Genera = append(s.Genera, domain.LocalizedText{Language: g.Language.Name, Value: g.Genus})
		if g.Language.Name == "en" && s.Genus == "" {
			s.Genus = g.Genus
		}
	}
	for _, f := range dto.FlavorTextEntries {
		order, _ := idFromURL(f.Version.URL)
		s.FlavorTextEntries = append(s.FlavorTextEntries, domain.FlavorText{
			Language:     f.Language.Name,
			Version:      f.Version.Name,
			VersionOrder: order,
			Text:         f.FlavorText,
		})
	}
	for _, v := range dto.Varieties {
		pokemonID, _ := idFromURL(v.Pokemon.URL)
		s.Varieties = append(s.Varieties, domain.Variety{
			PokemonID: pokemonID, Slug: v.Pokemon.Name, IsDefault: v.IsDefault,
		})
		if v.IsDefault {
			s.DefaultPokemonID = pokemonID
		}
	}
	if s.DefaultPokemonID == 0 && len(s.Varieties) > 0 {
		s.DefaultPokemonID = s.Varieties[0].PokemonID
	}
	return s, nil
}

func mapEvolutionChain(dto evolutionChainDTO) domain.EvolutionTree {
	return domain.EvolutionTree{Root: mapChainLink(dto.Chain)}
}

func mapChainLink(link chainLinkDTO) domain.EvolutionNode {
	speciesID, _ := idFromURL(link.Species.URL)
	node := domain.EvolutionNode{
		SpeciesID:   speciesID,
		SpeciesSlug: link.Species.Name,
		DisplayName: DisplayNameFromSlug(link.Species.Name),
	}
	for _, child := range link.EvolvesTo {
		branch := domain.EvolutionBranch{Target: mapChainLink(child)}
		details := child.EvolutionDetails
		if len(details) == 0 {
			node.Children = append(node.Children, branch)
			continue
		}
		for _, detail := range details {
			b := domain.EvolutionBranch{
				Conditions: []domain.EvolutionCondition{mapEvolutionDetail(detail)},
				Target:     mapChainLink(child),
			}
			node.Children = append(node.Children, b)
		}
	}
	return node
}

func mapEvolutionDetail(d evolutionDetailDTO) domain.EvolutionCondition {
	return domain.EvolutionCondition{
		Trigger:               resourceName(d.Trigger),
		MinimumLevel:          d.MinLevel,
		Item:                  resourceName(d.Item),
		HeldItem:              resourceName(d.HeldItem),
		KnownMove:             resourceName(d.KnownMove),
		KnownMoveType:         resourceName(d.KnownMoveType),
		Location:              resourceName(d.Location),
		TimeOfDay:             d.TimeOfDay,
		MinimumHappiness:      d.MinHappiness,
		MinimumBeauty:         d.MinBeauty,
		MinimumAffection:      d.MinAffection,
		NeedsOverworldRain:    d.NeedsOverworldRain,
		TurnUpsideDown:        d.TurnUpsideDown,
		RelativePhysicalStats: d.RelativePhysicalStats,
		Gender:                d.Gender,
		TradeSpecies:          resourceName(d.TradeSpecies),
	}
}

func mapEncounters(dtos []encounterDTO) []domain.Encounter {
	var out []domain.Encounter
	for _, area := range dtos {
		for _, version := range area.VersionDetails {
			order, _ := idFromURL(version.Version.URL)
			for _, detail := range version.EncounterDetails {
				conds := make([]string, 0, len(detail.ConditionValues))
				for _, c := range detail.ConditionValues {
					conds = append(conds, c.Name)
				}
				out = append(out, domain.Encounter{
					Version:           version.Version.Name,
					VersionGroup:      "",
					VersionGroupOrder: order,
					Area:              area.LocationArea.Name,
					Method:            detail.Method.Name,
					MinimumLevel:      detail.MinLevel,
					MaximumLevel:      detail.MaxLevel,
					Chance:            detail.Chance,
					Conditions:        conds,
				})
			}
		}
	}
	return out
}

func mapMove(dto moveDTO) domain.Move {
	m := domain.Move{
		ID:          dto.ID,
		Slug:        dto.Name,
		DisplayName: englishName(dto.Names, dto.Name),
		Type:        dto.Type.Name,
		DamageClass: dto.DamageClass.Name,
		Power:       dto.Power,
		Accuracy:    dto.Accuracy,
		PP:          dto.PP,
	}
	for _, n := range dto.Names {
		m.Names = append(m.Names, domain.LocalizedName{Language: n.Language.Name, Value: n.Name})
	}
	return m
}
