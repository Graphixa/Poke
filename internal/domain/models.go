// Package domain contains transport- and presentation-independent application
// data. PokeAPI JSON DTOs must be mapped into these types at the API boundary.
package domain

type Pokemon struct {
	ID              int
	Slug            string
	DisplayName     string
	SpeciesID       int
	HeightDecimeter int
	WeightHectogram int
	BaseExperience  int
	Types           []TypeSlot
	Abilities       []AbilitySlot
	Stats           Stats
	Moves           []LearnedMove
	Sprites         Sprites
}

type TypeSlot struct {
	Slot int
	Name string
}

type AbilitySlot struct {
	Slot     int
	Name     string
	IsHidden bool
}

type Stats struct {
	HP             int
	Attack         int
	Defense        int
	SpecialAttack  int
	SpecialDefense int
	Speed          int
}

func (s Stats) Total() int {
	return s.HP + s.Attack + s.Defense + s.SpecialAttack + s.SpecialDefense + s.Speed
}

// Sprites holds original source-image URLs (not rendered terminal output).
// OfficialArtwork is sprites.other.official-artwork.front_default.
// FrontDefault is the pixel sprite (sprites.front_default).
type Sprites struct {
	FrontDefault    string
	HomeFront       string
	OfficialArtwork string
}

// Images is the preferred name for source-asset URLs.
type Images struct {
	PixelFrontDefault string
	OfficialArtwork   string
}

func (s Sprites) Images() Images {
	return Images{
		PixelFrontDefault: s.FrontDefault,
		OfficialArtwork:   s.OfficialArtwork,
	}
}

// BrowseEntry is the compact, in-memory row used for list selection and glance
// stats/About (so the right pane does not wait on dossier core).
type BrowseEntry struct {
	ID              int      `json:"id"`
	Slug            string   `json:"slug"`
	Name            string   `json:"name"`
	Types           []string `json:"types"`
	HP              int      `json:"hp"`
	Attack          int      `json:"attack"`
	Defence         int      `json:"defence"`
	SpAttack        int      `json:"sp_attack"`
	SpDefence       int      `json:"sp_defence"`
	Speed           int      `json:"speed"`
	HeightDecimeter int      `json:"height"`
	WeightHectogram int      `json:"weight"`
	BaseExperience  int      `json:"base_exp"`
	Genus           string   `json:"genus,omitempty"`
}

func (b BrowseEntry) Summary() SpeciesSummary {
	return SpeciesSummary{
		ID:          b.ID,
		Slug:        b.Slug,
		DisplayName: b.Name,
		Types:       b.Types,
	}
}

func (b BrowseEntry) Stats() Stats {
	return Stats{
		HP: b.HP, Attack: b.Attack, Defense: b.Defence,
		SpecialAttack: b.SpAttack, SpecialDefense: b.SpDefence, Speed: b.Speed,
	}
}

// HasAbout reports whether glance About can render from this row (not dossier).
// BaseExperience alone is not enough — old browse.json rows had exp without size.
func (b BrowseEntry) HasAbout() bool {
	return b.HeightDecimeter > 0 || b.WeightHectogram > 0
}

func BrowseEntryFrom(summary SpeciesSummary, stats Stats) BrowseEntry {
	return BrowseEntry{
		ID:        summary.ID,
		Slug:      summary.Slug,
		Name:      summary.DisplayName,
		Types:     summary.Types,
		HP:        stats.HP,
		Attack:    stats.Attack,
		Defence:   stats.Defense,
		SpAttack:  stats.SpecialAttack,
		SpDefence: stats.SpecialDefense,
		Speed:     stats.Speed,
	}
}

type Species struct {
	ID                int
	Slug              string
	DisplayName       string
	Genus             string
	Generation        string
	Habitat           string
	Shape             string
	Color             string
	GrowthRate        string
	CaptureRate       int
	BaseHappiness     int
	GenderRate        int
	IsLegendary       bool
	IsMythical        bool
	DefaultPokemonID  int
	EvolutionChainID  int
	Names             []LocalizedName
	Genera            []LocalizedText
	FlavorTextEntries []FlavorText
	Varieties         []Variety
}

type LocalizedName struct {
	Language string
	Value    string
}

type LocalizedText struct {
	Language string
	Value    string
}

type FlavorText struct {
	Language          string
	Version           string
	VersionOrder      int
	VersionGroup      string
	VersionGroupOrder int
	Text              string
}

type Variety struct {
	PokemonID int
	Slug      string
	IsDefault bool
}

type EvolutionTree struct {
	Root EvolutionNode
}

type EvolutionNode struct {
	SpeciesID   int
	SpeciesSlug string
	DisplayName string
	Children    []EvolutionBranch
}

type EvolutionBranch struct {
	Conditions []EvolutionCondition
	Target     EvolutionNode
}

type EvolutionCondition struct {
	Trigger               string
	MinimumLevel          *int
	Item                  string
	HeldItem              string
	KnownMove             string
	KnownMoveType         string
	Location              string
	TimeOfDay             string
	MinimumHappiness      *int
	MinimumBeauty         *int
	MinimumAffection      *int
	NeedsOverworldRain    bool
	TurnUpsideDown        bool
	RelativePhysicalStats *int
	Gender                *int
	TradeSpecies          string
}

type LearnedMove struct {
	MoveID    int
	Slug      string
	Learnings []MoveLearning
}

type MoveLearning struct {
	VersionGroup      string
	VersionGroupOrder int
	Method            string
	Level             int
}

type Move struct {
	ID          int
	Slug        string
	DisplayName string
	Type        string
	DamageClass string
	Power       *int
	Accuracy    *int
	PP          int
	Names       []LocalizedName
}

type Encounter struct {
	Version           string
	VersionGroup      string
	VersionGroupOrder int
	Area              string
	Method            string
	MinimumLevel      int
	MaximumLevel      int
	Chance            int
	Conditions        []string
}

type SpeciesSummary struct {
	ID          int
	Slug        string
	DisplayName string
	Types       []string
	// SpritePath is a cache-relative path hint, e.g. sprites/025-pikachu/front_default.png.
	SpritePath string
}

// LevelUpMove is a resolved level-up learnset row for dossier / moves UI.
type LevelUpMove struct {
	Level int
	Move  Move
}

// Dossier is the per-species detail bundle persisted as dossier.json.
type Dossier struct {
	Pokemon      Pokemon
	Species      Species
	Evolution    *EvolutionTree // nil when unavailable
	Encounters   []Encounter
	LevelUpMoves []LevelUpMove
}
