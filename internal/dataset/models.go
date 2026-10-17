package dataset

import "github.com/Graphixa/Pokedex/internal/domain"

// Builder accumulates in-memory dataset records during bootstrap (written once at end).
type Builder struct {
	Pokemon       map[int]DatasetPokemon
	Species       map[int]DatasetSpecies
	Learnsets     map[int][]DatasetLearn
	Moves         map[int]DatasetMove
	Evolutions    map[int]DatasetEvolutionTree
	Encounters    map[int][]DatasetEncounter
	Versions      map[int]DatasetVersion
	VersionGroups map[int]DatasetVersionGroup
	Types         map[int]DatasetType
	Abilities     map[int]DatasetAbility
	Forms         map[int]DatasetForm
	Browse        []domain.BrowseEntry
}

// NewBuilder returns an empty dataset builder with pre-sized maps.
func NewBuilder() *Builder {
	return &Builder{
		Pokemon:       make(map[int]DatasetPokemon),
		Species:       make(map[int]DatasetSpecies),
		Learnsets:     make(map[int][]DatasetLearn),
		Moves:         make(map[int]DatasetMove),
		Evolutions:    make(map[int]DatasetEvolutionTree),
		Encounters:    make(map[int][]DatasetEncounter),
		Versions:      make(map[int]DatasetVersion),
		VersionGroups: make(map[int]DatasetVersionGroup),
		Types:         make(map[int]DatasetType),
		Abilities:     make(map[int]DatasetAbility),
		Forms:         make(map[int]DatasetForm),
	}
}

// DatasetPokemon is the compact persisted Pokémon record (moves live in learnsets.json).
type DatasetPokemon struct {
	ID              int                  `json:"id"`
	Slug            string               `json:"slug"`
	DisplayName     string               `json:"display_name"`
	SpeciesID       int                  `json:"species_id"`
	HeightDecimeter int                  `json:"height_decimeter"`
	WeightHectogram int                  `json:"weight_hectogram"`
	BaseExperience  int                  `json:"base_experience"`
	Types           []string             `json:"types"`
	Abilities       []domain.AbilitySlot `json:"abilities"`
	Stats           domain.Stats         `json:"stats"`
	FormIDs         []int                `json:"form_ids,omitempty"`
	OfficialArtwork string               `json:"official_artwork,omitempty"`
	PixelSprite     string               `json:"pixel_sprite,omitempty"`
	HomeSprite      string               `json:"home_sprite,omitempty"`
}

// DatasetSpecies is the compact persisted species record (English-only flavour text).
type DatasetSpecies struct {
	ID               int                 `json:"id"`
	Slug             string              `json:"slug"`
	DisplayName      string              `json:"display_name"`
	Genus            string              `json:"genus,omitempty"`
	Generation       string              `json:"generation,omitempty"`
	Habitat          *string             `json:"habitat"` // null = unavailable from API
	Shape            string              `json:"shape,omitempty"`
	Color            string              `json:"color,omitempty"`
	GrowthRate       string              `json:"growth_rate,omitempty"`
	CaptureRate      int                 `json:"capture_rate,omitempty"`
	BaseHappiness    int                 `json:"base_happiness,omitempty"`
	GenderRate       int                 `json:"gender_rate,omitempty"`
	HatchCounter     int                 `json:"hatch_counter,omitempty"`
	IsBaby           bool                `json:"is_baby,omitempty"`
	IsLegendary      bool                `json:"is_legendary,omitempty"`
	IsMythical       bool                `json:"is_mythical,omitempty"`
	DefaultPokemonID int                 `json:"default_pokemon_id"`
	EvolutionChainID int                 `json:"evolution_chain_id,omitempty"`
	EggGroups        []string            `json:"egg_groups,omitempty"`
	FlavorTexts      []DatasetFlavorText `json:"flavor_texts,omitempty"`
	Varieties        []DatasetVariety    `json:"varieties,omitempty"`
}

// DatasetFlavorText retains English entries with exact version identity.
type DatasetFlavorText struct {
	Version      string `json:"version"`
	VersionID    int    `json:"version_id,omitempty"`
	VersionOrder int    `json:"version_order"`
	Text         string `json:"text"`
}

type DatasetVariety struct {
	PokemonID int    `json:"pokemon_id,omitempty"`
	Slug      string `json:"slug"`
	IsDefault bool   `json:"is_default,omitempty"`
}

// DatasetLearn is one learnset row (all methods / version groups).
// Exact duplicate rows are deduped; shared move IDs across methods/groups are kept.
type DatasetLearn struct {
	PokemonID      int    `json:"pokemon_id"`
	MoveID         int    `json:"move_id"`
	VersionGroupID int    `json:"version_group_id"`
	LearnMethod    string `json:"learn_method"`
	LevelLearnedAt int    `json:"level_learned_at"`
}

// DatasetMove is compact move metadata (English effect; power/accuracy stay nullable).
type DatasetMove struct {
	ID          int    `json:"id"`
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	Type        string `json:"type"`
	DamageClass string `json:"damage_class"`
	Power       *int   `json:"power"`    // null preserved
	Accuracy    *int   `json:"accuracy"` // null preserved
	PP          int    `json:"pp"`
	Effect      string `json:"effect,omitempty"`
}

// DatasetVersion is one game version in the catalog.
type DatasetVersion struct {
	ID             int    `json:"id"`
	Slug           string `json:"slug"`
	DisplayName    string `json:"display_name"`
	VersionGroupID int    `json:"version_group_id"`
	Generation     string `json:"generation,omitempty"`
	Order          int    `json:"order"`
}

// DatasetVersionGroup is one version group in the catalog.
type DatasetVersionGroup struct {
	ID          int    `json:"id"`
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	Generation  string `json:"generation,omitempty"`
	Order       int    `json:"order"`
	VersionIDs  []int  `json:"version_ids,omitempty"`
}

// DatasetType is a type with defensive damage relations.
type DatasetType struct {
	ID       int      `json:"id"`
	Slug     string   `json:"slug"`
	Name     string   `json:"name"`
	NoDamage []string `json:"no_damage_from,omitempty"`
	Half     []string `json:"half_damage_from,omitempty"`
	Double   []string `json:"double_damage_from,omitempty"`
}

// DatasetAbility is ability metadata.
type DatasetAbility struct {
	ID          int    `json:"id"`
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	Effect      string `json:"effect,omitempty"`
}

// DatasetForm is a pokemon-form row.
type DatasetForm struct {
	ID                  int      `json:"id"`
	Slug                string   `json:"slug"`
	DisplayName         string   `json:"display_name"`
	FormName            string   `json:"form_name,omitempty"`
	FormOrder           int      `json:"form_order,omitempty"`
	Order               int      `json:"order,omitempty"`
	IsDefault           bool     `json:"is_default,omitempty"`
	IsBattleOnly        bool     `json:"is_battle_only,omitempty"`
	IsMega              bool     `json:"is_mega,omitempty"`
	PokemonID           int      `json:"pokemon_id"`
	IntroVersionGroupID int      `json:"intro_version_group_id,omitempty"`
	IntroVersionGroup   string   `json:"intro_version_group,omitempty"`
	Types               []string `json:"types,omitempty"`
	OfficialArtwork     string   `json:"official_artwork,omitempty"`
	PixelSprite         string   `json:"pixel_sprite,omitempty"`
	HomeSprite          string   `json:"home_sprite,omitempty"`
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

// DatasetEncounter is one wild encounter row (exact version identity).
type DatasetEncounter struct {
	Version           string   `json:"version,omitempty"`
	VersionID         int      `json:"version_id,omitempty"`
	VersionGroup      string   `json:"version_group,omitempty"`
	VersionGroupID    int      `json:"version_group_id,omitempty"`
	VersionGroupOrder int      `json:"version_group_order,omitempty"`
	Area              string   `json:"area,omitempty"`
	Method            string   `json:"method,omitempty"`
	MinimumLevel      int      `json:"minimum_level,omitempty"`
	MaximumLevel      int      `json:"maximum_level,omitempty"`
	Chance            int      `json:"chance,omitempty"`
	Conditions        []string `json:"conditions,omitempty"`
}
