package pokeapi

// Transport DTOs stay private to this package and are mapped to domain types.

type namedAPIResource struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type apiResource struct {
	URL string `json:"url"`
}

type namedResourceList struct {
	Count    int                `json:"count"`
	Next     string             `json:"next"`
	Previous string             `json:"previous"`
	Results  []namedAPIResource `json:"results"`
}

type pokemonDTO struct {
	ID             int              `json:"id"`
	Name           string           `json:"name"`
	BaseExperience int              `json:"base_experience"`
	Height         int              `json:"height"`
	Weight         int              `json:"weight"`
	Species        namedAPIResource `json:"species"`
	Types          []struct {
		Slot int              `json:"slot"`
		Type namedAPIResource `json:"type"`
	} `json:"types"`
	Abilities []struct {
		IsHidden bool             `json:"is_hidden"`
		Slot     int              `json:"slot"`
		Ability  namedAPIResource `json:"ability"`
	} `json:"abilities"`
	Stats []struct {
		BaseStat int              `json:"base_stat"`
		Stat     namedAPIResource `json:"stat"`
	} `json:"stats"`
	Moves []struct {
		Move                namedAPIResource `json:"move"`
		VersionGroupDetails []struct {
			LevelLearnedAt  int              `json:"level_learned_at"`
			MoveLearnMethod namedAPIResource `json:"move_learn_method"`
			VersionGroup    namedAPIResource `json:"version_group"`
		} `json:"version_group_details"`
	} `json:"moves"`
	Sprites struct {
		FrontDefault *string `json:"front_default"`
	} `json:"sprites"`
}

type speciesDTO struct {
	ID             int               `json:"id"`
	Name           string            `json:"name"`
	CaptureRate    int               `json:"capture_rate"`
	BaseHappiness  int               `json:"base_happiness"`
	GenderRate     int               `json:"gender_rate"`
	IsLegendary    bool              `json:"is_legendary"`
	IsMythical     bool              `json:"is_mythical"`
	GrowthRate     *namedAPIResource `json:"growth_rate"`
	Color          *namedAPIResource `json:"color"`
	Shape          *namedAPIResource `json:"shape"`
	Habitat        *namedAPIResource `json:"habitat"`
	Generation     *namedAPIResource `json:"generation"`
	EvolutionChain apiResource       `json:"evolution_chain"`
	Names          []struct {
		Name     string           `json:"name"`
		Language namedAPIResource `json:"language"`
	} `json:"names"`
	Genera []struct {
		Genus    string           `json:"genus"`
		Language namedAPIResource `json:"language"`
	} `json:"genera"`
	FlavorTextEntries []struct {
		FlavorText string           `json:"flavor_text"`
		Language   namedAPIResource `json:"language"`
		Version    namedAPIResource `json:"version"`
	} `json:"flavor_text_entries"`
	Varieties []struct {
		IsDefault bool             `json:"is_default"`
		Pokemon   namedAPIResource `json:"pokemon"`
	} `json:"varieties"`
}

type evolutionChainDTO struct {
	ID    int          `json:"id"`
	Chain chainLinkDTO `json:"chain"`
}

type chainLinkDTO struct {
	Species          namedAPIResource     `json:"species"`
	EvolutionDetails []evolutionDetailDTO `json:"evolution_details"`
	EvolvesTo        []chainLinkDTO       `json:"evolves_to"`
}

type evolutionDetailDTO struct {
	Trigger               *namedAPIResource `json:"trigger"`
	Item                  *namedAPIResource `json:"item"`
	HeldItem              *namedAPIResource `json:"held_item"`
	KnownMove             *namedAPIResource `json:"known_move"`
	KnownMoveType         *namedAPIResource `json:"known_move_type"`
	Location              *namedAPIResource `json:"location"`
	TimeOfDay             string            `json:"time_of_day"`
	MinLevel              *int              `json:"min_level"`
	MinHappiness          *int              `json:"min_happiness"`
	MinBeauty             *int              `json:"min_beauty"`
	MinAffection          *int              `json:"min_affection"`
	NeedsOverworldRain    bool              `json:"needs_overworld_rain"`
	TurnUpsideDown        bool              `json:"turn_upside_down"`
	RelativePhysicalStats *int              `json:"relative_physical_stats"`
	Gender                *int              `json:"gender"`
	TradeSpecies          *namedAPIResource `json:"trade_species"`
}

type encounterDTO struct {
	LocationArea   namedAPIResource `json:"location_area"`
	VersionDetails []struct {
		Version          namedAPIResource `json:"version"`
		MaxChance        int              `json:"max_chance"`
		EncounterDetails []struct {
			MinLevel        int                `json:"min_level"`
			MaxLevel        int                `json:"max_level"`
			Chance          int                `json:"chance"`
			Method          namedAPIResource   `json:"method"`
			ConditionValues []namedAPIResource `json:"condition_values"`
		} `json:"encounter_details"`
	} `json:"version_details"`
}

type moveDTO struct {
	ID          int              `json:"id"`
	Name        string           `json:"name"`
	Power       *int             `json:"power"`
	Accuracy    *int             `json:"accuracy"`
	PP          int              `json:"pp"`
	Type        namedAPIResource `json:"type"`
	DamageClass namedAPIResource `json:"damage_class"`
	Names       []struct {
		Name     string           `json:"name"`
		Language namedAPIResource `json:"language"`
	} `json:"names"`
}
