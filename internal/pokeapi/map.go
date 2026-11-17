package pokeapi

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/Graphixa/Poke/internal/domain"
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
	if dto.Sprites.Other != nil {
		if dto.Sprites.Other.Home != nil && dto.Sprites.Other.Home.FrontDefault != nil {
			p.Sprites.HomeFront = *dto.Sprites.Other.Home.FrontDefault
		}
		if dto.Sprites.Other.OfficialArtwork != nil && dto.Sprites.Other.OfficialArtwork.FrontDefault != nil {
			p.Sprites.OfficialArtwork = *dto.Sprites.Other.OfficialArtwork.FrontDefault
		}
	}
	for _, t := range dto.Types {
		p.Types = append(p.Types, domain.TypeSlot{Slot: t.Slot, Name: t.Type.Name})
	}
	for _, a := range dto.Abilities {
		abilityID, _ := idFromURL(a.Ability.URL)
		p.Abilities = append(p.Abilities, domain.AbilitySlot{
			Slot: a.Slot, ID: abilityID, Name: a.Ability.Name, IsHidden: a.IsHidden,
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
			vgID, _ := idFromURL(detail.VersionGroup.URL)
			learned.Learnings = append(learned.Learnings, domain.MoveLearning{
				VersionGroupID:    vgID,
				VersionGroup:      detail.VersionGroup.Name,
				VersionGroupOrder: vgID, // catalog Order replaces this at lookup time when available
				Method:            detail.MoveLearnMethod.Name,
				Level:             detail.LevelLearnedAt,
			})
		}
		p.Moves = append(p.Moves, learned)
	}
	for _, f := range dto.Forms {
		if id, err := idFromURL(f.URL); err == nil && id > 0 {
			p.FormIDs = append(p.FormIDs, id)
		}
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
		HatchCounter:  dto.HatchCounter,
		IsBaby:        dto.IsBaby,
		IsLegendary:   dto.IsLegendary,
		IsMythical:    dto.IsMythical,
		GrowthRate:    resourceName(dto.GrowthRate),
		Color:         resourceName(dto.Color),
		Shape:         resourceName(dto.Shape),
		Habitat:       resourceName(dto.Habitat),
		Generation:    resourceName(dto.Generation),
	}
	for _, eg := range dto.EggGroups {
		if eg.Name != "" {
			s.EggGroups = append(s.EggGroups, eg.Name)
		}
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
		vid, _ := idFromURL(f.Version.URL)
		s.FlavorTextEntries = append(s.FlavorTextEntries, domain.FlavorText{
			Language:     f.Language.Name,
			Version:      f.Version.Name,
			VersionID:    vid,
			VersionOrder: vid,
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
			vid, _ := idFromURL(version.Version.URL)
			for _, detail := range version.EncounterDetails {
				conds := make([]string, 0, len(detail.ConditionValues))
				for _, c := range detail.ConditionValues {
					conds = append(conds, c.Name)
				}
				out = append(out, domain.Encounter{
					Version:           version.Version.Name,
					VersionID:         vid,
					VersionGroup:      "",
					VersionGroupOrder: vid,
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
		Effect:      englishEffect(dto.EffectEntries),
	}
	for _, n := range dto.Names {
		m.Names = append(m.Names, domain.LocalizedName{Language: n.Language.Name, Value: n.Name})
	}
	return m
}

func englishEffect(entries []struct {
	ShortEffect string           `json:"short_effect"`
	Effect      string           `json:"effect"`
	Language    namedAPIResource `json:"language"`
}) string {
	for _, e := range entries {
		if e.Language.Name == "en" {
			if e.ShortEffect != "" {
				return e.ShortEffect
			}
			return e.Effect
		}
	}
	return ""
}

func mapVersion(dto versionDTO) domain.Version {
	vgID, _ := idFromURL(dto.VersionGroup.URL)
	return domain.Version{
		ID:             dto.ID,
		Slug:           dto.Name,
		DisplayName:    englishName(dto.Names, dto.Name),
		VersionGroupID: vgID,
		Order:          dto.ID, // refined when joined to version-group.order in bootstrap
	}
}

func mapVersionGroup(dto versionGroupDTO) domain.VersionGroup {
	g := domain.VersionGroup{
		ID:          dto.ID,
		Slug:        dto.Name,
		DisplayName: englishName(dto.Names, dto.Name),
		Generation:  resourceName(dto.Generation),
		Order:       dto.Order,
	}
	for _, v := range dto.Versions {
		if id, err := idFromURL(v.URL); err == nil {
			g.VersionIDs = append(g.VersionIDs, id)
		}
	}
	return g
}

func mapType(dto typeDTO) domain.TypeInfo {
	t := domain.TypeInfo{
		ID:   dto.ID,
		Slug: dto.Name,
		Name: englishName(dto.Names, dto.Name),
	}
	for _, r := range dto.DamageRelations.NoDamageFrom {
		t.NoDamage = append(t.NoDamage, r.Name)
	}
	for _, r := range dto.DamageRelations.HalfDamageFrom {
		t.Half = append(t.Half, r.Name)
	}
	for _, r := range dto.DamageRelations.DoubleDamageFrom {
		t.Double = append(t.Double, r.Name)
	}
	return t
}

func mapAbility(dto abilityDTO) domain.Ability {
	return domain.Ability{
		ID:          dto.ID,
		Slug:        dto.Name,
		DisplayName: englishName(dto.Names, dto.Name),
		Effect:      englishEffect(dto.EffectEntries),
	}
}

func mapPokemonForm(dto pokemonFormDTO) domain.PokemonForm {
	pokemonID, _ := idFromURL(dto.Pokemon.URL)
	vgID, _ := idFromURL(dto.VersionGroup.URL)
	f := domain.PokemonForm{
		ID:                  dto.ID,
		Slug:                dto.Name,
		DisplayName:         englishName(dto.Names, dto.Name),
		FormName:            englishName(dto.FormNames, dto.FormName),
		FormOrder:           dto.FormOrder,
		Order:               dto.Order,
		IsDefault:           dto.IsDefault,
		IsBattleOnly:        dto.IsBattleOnly,
		IsMega:              dto.IsMega,
		PokemonID:           pokemonID,
		IntroVersionGroupID: vgID,
		IntroVersionGroup:   dto.VersionGroup.Name,
	}
	if f.FormName == "" {
		f.FormName = dto.FormName
	}
	for _, t := range dto.Types {
		f.Types = append(f.Types, t.Type.Name)
	}
	if dto.Sprites.FrontDefault != nil {
		f.Sprites.FrontDefault = *dto.Sprites.FrontDefault
	}
	if dto.Sprites.Other != nil {
		if dto.Sprites.Other.Home != nil && dto.Sprites.Other.Home.FrontDefault != nil {
			f.Sprites.HomeFront = *dto.Sprites.Other.Home.FrontDefault
		}
		if dto.Sprites.Other.OfficialArtwork != nil && dto.Sprites.Other.OfficialArtwork.FrontDefault != nil {
			f.Sprites.OfficialArtwork = *dto.Sprites.Other.OfficialArtwork.FrontDefault
		}
	}
	return f
}
