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
	FormIDs         []int
	Sprites         Sprites
}

// TypeNames returns type slugs in slot order (empty slots omitted).
func (p Pokemon) TypeNames() []string {
	if len(p.Types) == 0 {
		return nil
	}
	out := make([]string, len(p.Types))
	for i, t := range p.Types {
		out[i] = t.Name
	}
	return out
}

type TypeSlot struct {
	Slot int
	Name string
}

type AbilitySlot struct {
	Slot     int    `json:"slot,omitempty"`
	ID       int    `json:"id,omitempty"`
	Name     string `json:"name"`
	IsHidden bool   `json:"is_hidden,omitempty"`
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

// BrowseEntry is the compact, in-memory row used for list selection and glance
// stats/About (so the right pane does not wait on dossier core).
//
// ponytail: flat stat ints match browse.json keys (defence/sp_*); nested domain.Stats
// would need a migration — use BrowseEntry.Stats() / BrowseEntryFrom at boundaries.
type BrowseEntry struct {
	ID              int           `json:"id"`
	Slug            string        `json:"slug"`
	Name            string        `json:"name"`
	Types           []string      `json:"types"`
	HP              int           `json:"hp"`
	Attack          int           `json:"attack"`
	Defence         int           `json:"defence"`
	SpAttack        int           `json:"sp_attack"`
	SpDefence       int           `json:"sp_defence"`
	Speed           int           `json:"speed"`
	HeightDecimeter int           `json:"height"`
	WeightHectogram int           `json:"weight"`
	BaseExperience  int           `json:"base_exp"`
	Genus           string        `json:"genus,omitempty"`
	Abilities       []AbilitySlot `json:"abilities,omitempty"`
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

// HasAbilities reports whether glance Abilities can render from this row.
func (b BrowseEntry) HasAbilities() bool {
	return len(b.Abilities) > 0
}

func BrowseEntryFrom(summary SpeciesSummary, stats Stats) BrowseEntry {
	b := BrowseEntry{
		ID: summary.ID, Slug: summary.Slug, Name: summary.DisplayName, Types: summary.Types,
	}
	applyStatsToBrowse(&b, stats)
	return b
}

// BrowseEntryFromRecord builds a browse.json row from species index + default Pokémon + species.
func BrowseEntryFromRecord(sum SpeciesSummary, poke Pokemon, sp Species) BrowseEntry {
	e := BrowseEntryFrom(sum, poke.Stats)
	if len(e.Types) == 0 {
		e.Types = poke.TypeNames()
	}
	if e.Name == "" {
		e.Name = poke.DisplayName
	}
	e.HeightDecimeter = poke.HeightDecimeter
	e.WeightHectogram = poke.WeightHectogram
	e.BaseExperience = poke.BaseExperience
	e.Genus = sp.Genus
	e.Abilities = poke.Abilities
	return e
}

func applyStatsToBrowse(b *BrowseEntry, s Stats) {
	b.HP, b.Attack, b.Defence = s.HP, s.Attack, s.Defense
	b.SpAttack, b.SpDefence, b.Speed = s.SpecialAttack, s.SpecialDefense, s.Speed
}

type Species struct {
	ID                int
	Slug              string
	DisplayName       string
	Genus             string
	Generation        string
	Habitat           string // empty means unavailable / not set by API
	Shape             string
	Color             string
	GrowthRate        string
	CaptureRate       int
	BaseHappiness     int
	GenderRate        int
	HatchCounter      int
	IsBaby            bool
	IsLegendary       bool
	IsMythical        bool
	DefaultPokemonID  int
	EvolutionChainID  int
	EggGroups         []string
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
	VersionID         int
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
	VersionGroupID    int
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
	Effect      string // English short effect; current reference unless historically resolved
	Names       []LocalizedName
}

type Encounter struct {
	Version           string
	VersionID         int
	VersionGroup      string
	VersionGroupID    int
	VersionGroupOrder int
	Area              string
	Method            string
	MinimumLevel      int
	MaximumLevel      int
	Chance            int
	Conditions        []string
}

// Version is one game version (exact identity for encounters / descriptions).
type Version struct {
	ID             int
	Slug           string
	DisplayName    string
	VersionGroupID int
	Generation     string
	Order          int // explicit chronological order within catalog
}

// VersionGroup groups highly similar versions for learnset resolution.
type VersionGroup struct {
	ID          int
	Slug        string
	DisplayName string
	Generation  string
	Order       int // explicit chronological order from API
	VersionIDs  []int
}

// Ability is ability metadata with English effect text.
type Ability struct {
	ID          int
	Slug        string
	DisplayName string
	Effect      string // English short effect
}

// TypeInfo holds defensive damage relations for a type.
type TypeInfo struct {
	ID       int
	Slug     string
	Name     string
	NoDamage []string // damage_relations.no_damage_from
	Half     []string // half_damage_from
	Double   []string // double_damage_from
}

// PokemonForm is a pokemon-form resource (often cosmetic; parent is a Pokémon).
type PokemonForm struct {
	ID                  int
	Slug                string
	DisplayName         string
	FormName            string
	FormOrder           int
	Order               int
	IsDefault           bool
	IsBattleOnly        bool
	IsMega              bool
	PokemonID           int
	IntroVersionGroupID int    // version group where the form was introduced (0 unknown)
	IntroVersionGroup   string // slug; empty unknown
	Types               []string
	Sprites             Sprites
}

// FormGameCompat is v1 form×game compatibility.
type FormGameCompat int

const (
	FormCompatUnknown FormGameCompat = iota
	FormCompatSupported
	FormCompatUnsupported
)

// FormOption is a form picker row with compatibility for the selected game.
type FormOption struct {
	Form   SelectableForm
	Compat FormGameCompat
}

// SelectableForm is a dossier form choice (variety and/or named form).
type SelectableForm struct {
	Key         string // stable UI key: "pokemon:<id>" or "form:<id>"
	PokemonID   int
	FormID      int // 0 when choice is the whole Pokémon variety
	Slug        string
	DisplayName string // "Default" for the species default; short label otherwise
	IsDefault   bool
}

// ResolvedDossierContext is the shared identity after form/game resolution.
type ResolvedDossierContext struct {
	SpeciesID         int
	PokemonID         int
	FormID            int
	GameVersionID     int
	VersionGroupID    int
	Compat            FormGameCompat
	FellBackToDefault bool
	Notice            string
	FormLabel         string // resolved choice label ("Default", "Alola", …)
}

type SpeciesSummary struct {
	ID          int
	Slug        string
	DisplayName string
	Types       []string
	// SpritePath is a cache-relative path hint, e.g. sprites/025-pikachu/front_default.png.
	SpritePath string
}

// LevelUpMove is a resolved learnset row for dossier / moves UI.
// Method is the learn method slug (level-up, machine, egg, tutor, …).
// Game is an optional display label when the All-games learnset expands per version.
type LevelUpMove struct {
	Level  int
	Method string
	Game   string
	Move   Move
}

// Dossier is the per-species detail bundle persisted as dossier.json.
type Dossier struct {
	Pokemon      Pokemon
	Species      Species
	Evolution    *EvolutionTree // nil when unavailable
	Encounters   []Encounter
	LevelUpMoves []LevelUpMove
}
