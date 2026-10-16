package dataset

import "github.com/Graphixa/Pokedex/internal/domain"

// Builder accumulates in-memory dataset records during bootstrap (written once at end).
type Builder struct {
	Pokemon    map[int]DatasetPokemon
	Species    map[int]DatasetSpecies
	Learnsets  map[int][]DatasetLearn
	Moves      map[int]DatasetMove
	Evolutions map[int]DatasetEvolutionTree
	Encounters map[int][]DatasetEncounter
	Browse     []domain.BrowseEntry
}

// NewBuilder returns an empty dataset builder with pre-sized maps.
func NewBuilder() *Builder {
	return &Builder{
		Pokemon:    make(map[int]DatasetPokemon),
		Species:    make(map[int]DatasetSpecies),
		Learnsets:  make(map[int][]DatasetLearn),
		Moves:      make(map[int]DatasetMove),
		Evolutions: make(map[int]DatasetEvolutionTree),
		Encounters: make(map[int][]DatasetEncounter),
	}
}

// DatasetPokemon is the compact persisted Pokémon record (moves live in learnsets.json).
type DatasetPokemon struct {
	ID              int              `json:"id"`
	Slug            string           `json:"slug"`
	DisplayName     string           `json:"display_name"`
	SpeciesID       int              `json:"species_id"`
	HeightDecimeter int              `json:"height_decimeter"`
	WeightHectogram int              `json:"weight_hectogram"`
	BaseExperience  int              `json:"base_experience"`
	Types           []string              `json:"types"`
	Abilities       []domain.AbilitySlot  `json:"abilities"`
	Stats           domain.Stats          `json:"stats"`
	OfficialArtwork string           `json:"official_artwork,omitempty"`
	PixelSprite     string           `json:"pixel_sprite,omitempty"`
	HomeSprite      string           `json:"home_sprite,omitempty"`
}

// DatasetSpecies is the compact persisted species record (English-only flavour text).
type DatasetSpecies struct {
	ID               int                 `json:"id"`
	Slug             string              `json:"slug"`
	DisplayName      string              `json:"display_name"`
	Genus            string              `json:"genus,omitempty"`
	Generation       string              `json:"generation,omitempty"`
	Habitat          string              `json:"habitat,omitempty"`
	Shape            string              `json:"shape,omitempty"`
	Color            string              `json:"color,omitempty"`
	GrowthRate       string              `json:"growth_rate,omitempty"`
	CaptureRate      int                 `json:"capture_rate,omitempty"`
	BaseHappiness    int                 `json:"base_happiness,omitempty"`
	GenderRate       int                 `json:"gender_rate,omitempty"`
	IsLegendary      bool                `json:"is_legendary,omitempty"`
	IsMythical       bool                `json:"is_mythical,omitempty"`
	DefaultPokemonID int                 `json:"default_pokemon_id"`
	EvolutionChainID int                 `json:"evolution_chain_id,omitempty"`
	FlavorTexts      []DatasetFlavorText `json:"flavor_texts,omitempty"`
	Varieties        []DatasetVariety    `json:"varieties,omitempty"`
}

// DatasetFlavorText retains only English entries and fields used by the TUI/CLI.
type DatasetFlavorText struct {
	Version      string `json:"version"`
	VersionOrder int    `json:"version_order"`
	Text         string `json:"text"`
}

type DatasetVariety struct {
	PokemonID int    `json:"pokemon_id,omitempty"`
	Slug      string `json:"slug"`
	IsDefault bool   `json:"is_default,omitempty"`
}

// DatasetLearn is one level-up learnset row (keyed by Pokémon ID in learnsets.json).
type DatasetLearn struct {
	Level  int `json:"level"`
	MoveID int `json:"move_id"`
}

// DatasetMove is compact move metadata (no multilingual names).
type DatasetMove struct {
	ID          int    `json:"id"`
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	Type        string `json:"type"`
	DamageClass string `json:"damage_class"`
	Power       *int   `json:"power,omitempty"`
	Accuracy    *int   `json:"accuracy,omitempty"`
	PP          int    `json:"pp"`
}

// DatasetEvolutionTree mirrors the domain evolution tree (already display-focused).
type DatasetEvolutionTree struct {
	Root DatasetEvolutionNode `json:"root"`
}

type DatasetEvolutionNode struct {
	SpeciesID   int                      `json:"species_id"`
	SpeciesSlug string                   `json:"species_slug"`
	DisplayName string                   `json:"display_name"`
	Children    []DatasetEvolutionBranch `json:"children,omitempty"`
}

type DatasetEvolutionBranch struct {
	Conditions []DatasetEvolutionCondition `json:"conditions,omitempty"`
	Target     DatasetEvolutionNode        `json:"target"`
}

type DatasetEvolutionCondition struct {
	Trigger               string `json:"trigger,omitempty"`
	MinimumLevel          *int   `json:"minimum_level,omitempty"`
	Item                  string `json:"item,omitempty"`
	HeldItem              string `json:"held_item,omitempty"`
	KnownMove             string `json:"known_move,omitempty"`
	KnownMoveType         string `json:"known_move_type,omitempty"`
	Location              string `json:"location,omitempty"`
	TimeOfDay             string `json:"time_of_day,omitempty"`
	MinimumHappiness      *int   `json:"minimum_happiness,omitempty"`
	MinimumBeauty         *int   `json:"minimum_beauty,omitempty"`
	MinimumAffection      *int   `json:"minimum_affection,omitempty"`
	NeedsOverworldRain    bool   `json:"needs_overworld_rain,omitempty"`
	TurnUpsideDown        bool   `json:"turn_upside_down,omitempty"`
	RelativePhysicalStats *int   `json:"relative_physical_stats,omitempty"`
	Gender                *int   `json:"gender,omitempty"`
	TradeSpecies          string `json:"trade_species,omitempty"`
}

// DatasetEncounter is one wild encounter row.
type DatasetEncounter struct {
	Version           string   `json:"version,omitempty"`
	VersionGroup      string   `json:"version_group,omitempty"`
	VersionGroupOrder int      `json:"version_group_order,omitempty"`
	Area              string   `json:"area,omitempty"`
	Method            string   `json:"method,omitempty"`
	MinimumLevel      int      `json:"minimum_level,omitempty"`
	MaximumLevel      int      `json:"maximum_level,omitempty"`
	Chance            int      `json:"chance,omitempty"`
	Conditions        []string `json:"conditions,omitempty"`
}
