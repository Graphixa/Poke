package dataset

import "github.com/Graphixa/Poke/internal/domain"

// PokemonFromDomain maps a domain Pokémon into the compact persisted form.
func PokemonFromDomain(p domain.Pokemon) DatasetPokemon {
	return DatasetPokemon{
		ID:              p.ID,
		Slug:            p.Slug,
		DisplayName:     p.DisplayName,
		SpeciesID:       p.SpeciesID,
		HeightDecimeter: p.HeightDecimeter,
		WeightHectogram: p.WeightHectogram,
		BaseExperience:  p.BaseExperience,
		Types:           p.TypeNames(),
		Abilities:       append([]domain.AbilitySlot(nil), p.Abilities...),
		Stats:           p.Stats,
		FormIDs:         append([]int(nil), p.FormIDs...),
		OfficialArtwork: p.Sprites.OfficialArtwork,
		PixelSprite:     p.Sprites.FrontDefault,
		HomeSprite:      p.Sprites.HomeFront,
	}
}

// PokemonToDomain reconstructs a domain Pokémon from the compact form.
// Move learnings are supplied separately from learnsets.json.
func PokemonToDomain(p DatasetPokemon, moves []domain.LearnedMove) domain.Pokemon {
	types := make([]domain.TypeSlot, 0, len(p.Types))
	for i, name := range p.Types {
		types = append(types, domain.TypeSlot{Slot: i + 1, Name: name})
	}
	return domain.Pokemon{
		ID:              p.ID,
		Slug:            p.Slug,
		DisplayName:     p.DisplayName,
		SpeciesID:       p.SpeciesID,
		HeightDecimeter: p.HeightDecimeter,
		WeightHectogram: p.WeightHectogram,
		BaseExperience:  p.BaseExperience,
		Types:           types,
		Abilities:       append([]domain.AbilitySlot(nil), p.Abilities...),
		Stats:           p.Stats,
		Moves:           moves,
		FormIDs:         append([]int(nil), p.FormIDs...),
		Sprites: domain.Sprites{
			FrontDefault:    p.PixelSprite,
			HomeFront:       p.HomeSprite,
			OfficialArtwork: p.OfficialArtwork,
		},
	}
}

// SpeciesFromDomain maps a domain species into the compact persisted form,
// dropping multilingual arrays and non-English flavour entries.
func SpeciesFromDomain(s domain.Species) DatasetSpecies {
	out := DatasetSpecies{
		ID:               s.ID,
		Slug:             s.Slug,
		DisplayName:      s.DisplayName,
		Genus:            s.Genus,
		Generation:       s.Generation,
		Shape:            s.Shape,
		Color:            s.Color,
		GrowthRate:       s.GrowthRate,
		CaptureRate:      s.CaptureRate,
		BaseHappiness:    s.BaseHappiness,
		GenderRate:       s.GenderRate,
		HatchCounter:     s.HatchCounter,
		IsBaby:           s.IsBaby,
		IsLegendary:      s.IsLegendary,
		IsMythical:       s.IsMythical,
		DefaultPokemonID: s.DefaultPokemonID,
		EvolutionChainID: s.EvolutionChainID,
		EggGroups:        append([]string(nil), s.EggGroups...),
	}
	if s.Habitat != "" {
		h := s.Habitat
		out.Habitat = &h
	}
	for _, f := range s.FlavorTextEntries {
		if f.Language != "" && f.Language != "en" {
			continue
		}
		out.FlavorTexts = append(out.FlavorTexts, DatasetFlavorText{
			Version:      f.Version,
			VersionID:    f.VersionID,
			VersionOrder: f.VersionOrder,
			Text:         f.Text,
		})
	}
	for _, v := range s.Varieties {
		out.Varieties = append(out.Varieties, DatasetVariety{
			PokemonID: v.PokemonID,
			Slug:      v.Slug,
			IsDefault: v.IsDefault,
		})
	}
	return out
}

// SpeciesToDomain reconstructs a domain species from the compact form.
func SpeciesToDomain(s DatasetSpecies) domain.Species {
	out := domain.Species{
		ID:               s.ID,
		Slug:             s.Slug,
		DisplayName:      s.DisplayName,
		Genus:            s.Genus,
		Generation:       s.Generation,
		Shape:            s.Shape,
		Color:            s.Color,
		GrowthRate:       s.GrowthRate,
		CaptureRate:      s.CaptureRate,
		BaseHappiness:    s.BaseHappiness,
		GenderRate:       s.GenderRate,
		HatchCounter:     s.HatchCounter,
		IsBaby:           s.IsBaby,
		IsLegendary:      s.IsLegendary,
		IsMythical:       s.IsMythical,
		DefaultPokemonID: s.DefaultPokemonID,
		EvolutionChainID: s.EvolutionChainID,
		EggGroups:        append([]string(nil), s.EggGroups...),
	}
	if s.Habitat != nil {
		out.Habitat = *s.Habitat
	}
	for _, f := range s.FlavorTexts {
		out.FlavorTextEntries = append(out.FlavorTextEntries, domain.FlavorText{
			Language:     "en",
			Version:      f.Version,
			VersionID:    f.VersionID,
			VersionOrder: f.VersionOrder,
			Text:         f.Text,
		})
	}
	for _, v := range s.Varieties {
		out.Varieties = append(out.Varieties, domain.Variety{
			PokemonID: v.PokemonID,
			Slug:      v.Slug,
			IsDefault: v.IsDefault,
		})
	}
	return out
}

// LearnedMovesFromLearnset rebuilds domain move-learning stubs for dossier resolution.
func LearnedMovesFromLearnset(rows []DatasetLearn, slugs map[int]string) []domain.LearnedMove {
	if len(rows) == 0 {
		return nil
	}
	byMove := make(map[int][]domain.MoveLearning)
	for _, row := range rows {
		byMove[row.MoveID] = append(byMove[row.MoveID], domain.MoveLearning{
			VersionGroupID:    row.VersionGroupID,
			VersionGroupOrder: row.VersionGroupID,
			Method:            row.LearnMethod,
			Level:             row.LevelLearnedAt,
		})
	}
	out := make([]domain.LearnedMove, 0, len(byMove))
	for moveID, learnings := range byMove {
		out = append(out, domain.LearnedMove{
			MoveID:    moveID,
			Slug:      slugs[moveID],
			Learnings: learnings,
		})
	}
	return out
}

// MoveFromDomain maps a domain move into the compact persisted form.
func MoveFromDomain(m domain.Move) DatasetMove {
	return DatasetMove{
		ID:          m.ID,
		Slug:        m.Slug,
		DisplayName: m.DisplayName,
		Type:        m.Type,
		DamageClass: m.DamageClass,
		Power:       m.Power,
		Accuracy:    m.Accuracy,
		PP:          m.PP,
		Effect:      m.Effect,
	}
}

// MoveToDomain reconstructs a domain move from the compact form.
func MoveToDomain(m DatasetMove) domain.Move {
	return domain.Move{
		ID:          m.ID,
		Slug:        m.Slug,
		DisplayName: m.DisplayName,
		Type:        m.Type,
		DamageClass: m.DamageClass,
		Power:       m.Power,
		Accuracy:    m.Accuracy,
		PP:          m.PP,
		Effect:      m.Effect,
	}
}

// VersionFromDomain / VersionToDomain catalog mappers.
func VersionFromDomain(v domain.Version) DatasetVersion {
	return DatasetVersion{
		ID: v.ID, Slug: v.Slug, DisplayName: v.DisplayName,
		VersionGroupID: v.VersionGroupID, Generation: v.Generation, Order: v.Order,
	}
}

func VersionToDomain(v DatasetVersion) domain.Version {
	return domain.Version{
		ID: v.ID, Slug: v.Slug, DisplayName: v.DisplayName,
		VersionGroupID: v.VersionGroupID, Generation: v.Generation, Order: v.Order,
	}
}

func VersionGroupFromDomain(g domain.VersionGroup) DatasetVersionGroup {
	return DatasetVersionGroup{
		ID: g.ID, Slug: g.Slug, DisplayName: g.DisplayName,
		Generation: g.Generation, Order: g.Order, VersionIDs: append([]int(nil), g.VersionIDs...),
	}
}

func VersionGroupToDomain(g DatasetVersionGroup) domain.VersionGroup {
	return domain.VersionGroup{
		ID: g.ID, Slug: g.Slug, DisplayName: g.DisplayName,
		Generation: g.Generation, Order: g.Order, VersionIDs: append([]int(nil), g.VersionIDs...),
	}
}

func TypeFromDomain(t domain.TypeInfo) DatasetType {
	return DatasetType{
		ID: t.ID, Slug: t.Slug, Name: t.Name,
		NoDamage: append([]string(nil), t.NoDamage...),
		Half:     append([]string(nil), t.Half...),
		Double:   append([]string(nil), t.Double...),
	}
}

func TypeToDomain(t DatasetType) domain.TypeInfo {
	return domain.TypeInfo{
		ID: t.ID, Slug: t.Slug, Name: t.Name,
		NoDamage: append([]string(nil), t.NoDamage...),
		Half:     append([]string(nil), t.Half...),
		Double:   append([]string(nil), t.Double...),
	}
}

func AbilityFromDomain(a domain.Ability) DatasetAbility {
	return DatasetAbility{ID: a.ID, Slug: a.Slug, DisplayName: a.DisplayName, Effect: a.Effect}
}

func AbilityToDomain(a DatasetAbility) domain.Ability {
	return domain.Ability{ID: a.ID, Slug: a.Slug, DisplayName: a.DisplayName, Effect: a.Effect}
}

func FormFromDomain(f domain.PokemonForm) DatasetForm {
	return DatasetForm{
		ID: f.ID, Slug: f.Slug, DisplayName: f.DisplayName, FormName: f.FormName,
		FormOrder: f.FormOrder, Order: f.Order, IsDefault: f.IsDefault,
		IsBattleOnly: f.IsBattleOnly, IsMega: f.IsMega, PokemonID: f.PokemonID,
		IntroVersionGroupID: f.IntroVersionGroupID, IntroVersionGroup: f.IntroVersionGroup,
		Types:           append([]string(nil), f.Types...),
		OfficialArtwork: f.Sprites.OfficialArtwork,
		PixelSprite:     f.Sprites.FrontDefault,
		HomeSprite:      f.Sprites.HomeFront,
	}
}

func FormToDomain(f DatasetForm) domain.PokemonForm {
	return domain.PokemonForm{
		ID: f.ID, Slug: f.Slug, DisplayName: f.DisplayName, FormName: f.FormName,
		FormOrder: f.FormOrder, Order: f.Order, IsDefault: f.IsDefault,
		IsBattleOnly: f.IsBattleOnly, IsMega: f.IsMega, PokemonID: f.PokemonID,
		IntroVersionGroupID: f.IntroVersionGroupID, IntroVersionGroup: f.IntroVersionGroup,
		Types: append([]string(nil), f.Types...),
		Sprites: domain.Sprites{
			FrontDefault: f.PixelSprite, HomeFront: f.HomeSprite, OfficialArtwork: f.OfficialArtwork,
		},
	}
}

// EvolutionFromDomain maps a domain evolution tree into the compact persisted form.
func EvolutionFromDomain(t domain.EvolutionTree) DatasetEvolutionTree {
	return DatasetEvolutionTree{Root: evolutionNodeFromDomain(t.Root)}
}

// EvolutionToDomain reconstructs a domain evolution tree from the compact form.
func EvolutionToDomain(t DatasetEvolutionTree) domain.EvolutionTree {
	return domain.EvolutionTree{Root: evolutionNodeToDomain(t.Root)}
}

func evolutionNodeFromDomain(n domain.EvolutionNode) DatasetEvolutionNode {
	out := DatasetEvolutionNode{
		SpeciesID:   n.SpeciesID,
		SpeciesSlug: n.SpeciesSlug,
		DisplayName: n.DisplayName,
	}
	for _, b := range n.Children {
		conds := make([]DatasetEvolutionCondition, 0, len(b.Conditions))
		for _, c := range b.Conditions {
			conds = append(conds, DatasetEvolutionCondition{
				Trigger:               c.Trigger,
				MinimumLevel:          c.MinimumLevel,
				Item:                  c.Item,
				HeldItem:              c.HeldItem,
				KnownMove:             c.KnownMove,
				KnownMoveType:         c.KnownMoveType,
				Location:              c.Location,
				TimeOfDay:             c.TimeOfDay,
				MinimumHappiness:      c.MinimumHappiness,
				MinimumBeauty:         c.MinimumBeauty,
				MinimumAffection:      c.MinimumAffection,
				NeedsOverworldRain:    c.NeedsOverworldRain,
				TurnUpsideDown:        c.TurnUpsideDown,
				RelativePhysicalStats: c.RelativePhysicalStats,
				Gender:                c.Gender,
				TradeSpecies:          c.TradeSpecies,
			})
		}
		out.Children = append(out.Children, DatasetEvolutionBranch{
			Conditions: conds,
			Target:     evolutionNodeFromDomain(b.Target),
		})
	}
	return out
}

func evolutionNodeToDomain(n DatasetEvolutionNode) domain.EvolutionNode {
	out := domain.EvolutionNode{
		SpeciesID:   n.SpeciesID,
		SpeciesSlug: n.SpeciesSlug,
		DisplayName: n.DisplayName,
	}
	for _, b := range n.Children {
		conds := make([]domain.EvolutionCondition, 0, len(b.Conditions))
		for _, c := range b.Conditions {
			conds = append(conds, domain.EvolutionCondition{
				Trigger:               c.Trigger,
				MinimumLevel:          c.MinimumLevel,
				Item:                  c.Item,
				HeldItem:              c.HeldItem,
				KnownMove:             c.KnownMove,
				KnownMoveType:         c.KnownMoveType,
				Location:              c.Location,
				TimeOfDay:             c.TimeOfDay,
				MinimumHappiness:      c.MinimumHappiness,
				MinimumBeauty:         c.MinimumBeauty,
				MinimumAffection:      c.MinimumAffection,
				NeedsOverworldRain:    c.NeedsOverworldRain,
				TurnUpsideDown:        c.TurnUpsideDown,
				RelativePhysicalStats: c.RelativePhysicalStats,
				Gender:                c.Gender,
				TradeSpecies:          c.TradeSpecies,
			})
		}
		out.Children = append(out.Children, domain.EvolutionBranch{
			Conditions: conds,
			Target:     evolutionNodeToDomain(b.Target),
		})
	}
	return out
}

// EncountersFromDomain maps domain encounter rows into the compact persisted form.
func EncountersFromDomain(rows []domain.Encounter) []DatasetEncounter {
	out := make([]DatasetEncounter, 0, len(rows))
	for _, e := range rows {
		out = append(out, DatasetEncounter{
			Version:           e.Version,
			VersionID:         e.VersionID,
			VersionGroup:      e.VersionGroup,
			VersionGroupID:    e.VersionGroupID,
			VersionGroupOrder: e.VersionGroupOrder,
			Area:              e.Area,
			Method:            e.Method,
			MinimumLevel:      e.MinimumLevel,
			MaximumLevel:      e.MaximumLevel,
			Chance:            e.Chance,
			Conditions:        append([]string(nil), e.Conditions...),
		})
	}
	return out
}

// EncountersToDomain reconstructs domain encounter rows from the compact form.
func EncountersToDomain(rows []DatasetEncounter) []domain.Encounter {
	out := make([]domain.Encounter, 0, len(rows))
	for _, e := range rows {
		out = append(out, domain.Encounter{
			Version:           e.Version,
			VersionID:         e.VersionID,
			VersionGroup:      e.VersionGroup,
			VersionGroupID:    e.VersionGroupID,
			VersionGroupOrder: e.VersionGroupOrder,
			Area:              e.Area,
			Method:            e.Method,
			MinimumLevel:      e.MinimumLevel,
			MaximumLevel:      e.MaximumLevel,
			Chance:            e.Chance,
			Conditions:        append([]string(nil), e.Conditions...),
		})
	}
	return out
}
