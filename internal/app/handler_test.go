package app_test

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/app"
	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/cli"
	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/domain"
)

type stubCacheCommands struct {
	clear  func() error
	update func(context.Context) error
}

func (s stubCacheCommands) Clear() error {
	if s.clear != nil {
		return s.clear()
	}
	return nil
}

func (s stubCacheCommands) Refresh(ctx context.Context) error {
	if s.update != nil {
		return s.update(ctx)
	}
	return nil
}

type stubAPI struct {
	list      []domain.SpeciesSummary
	pokemon   map[int]domain.Pokemon
	species   map[int]domain.Species
	evolution map[int]domain.EvolutionTree
}

func (s *stubAPI) ListSpecies(context.Context) ([]domain.SpeciesSummary, error) {
	return append([]domain.SpeciesSummary(nil), s.list...), nil
}
func (s *stubAPI) GetPokemon(_ context.Context, id int) (domain.Pokemon, error) {
	p, ok := s.pokemon[id]
	if !ok {
		return domain.Pokemon{}, errors.New("pokemon missing")
	}
	return p, nil
}
func (s *stubAPI) GetSpecies(_ context.Context, id int) (domain.Species, error) {
	sp, ok := s.species[id]
	if !ok {
		return domain.Species{}, errors.New("species missing")
	}
	return sp, nil
}
func (s *stubAPI) GetEvolutionChain(_ context.Context, id int) (domain.EvolutionTree, error) {
	t, ok := s.evolution[id]
	if !ok {
		return domain.EvolutionTree{}, errors.New("evolution missing")
	}
	return t, nil
}
func (s *stubAPI) GetEncounters(context.Context, int) ([]domain.Encounter, error) {
	return nil, nil
}
func (s *stubAPI) GetMove(context.Context, int) (domain.Move, error) {
	return domain.Move{}, errors.New("unused")
}

func testCfg() config.Config {
	cfg := config.Default()
	cfg.Cache.AutoRefresh = false
	cfg.Network.MaxConcurrency = 2
	return cfg
}

func level(n int) *int { return &n }

func phase3Fixture(t *testing.T) (*app.Handler, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	root := t.TempDir()
	clock := app.FixedClock{Instant: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	api := &stubAPI{
		list: []domain.SpeciesSummary{
			{ID: 25, Slug: "pikachu", DisplayName: "Pikachu", Types: []string{"electric"}},
			{ID: 122, Slug: "mr-mime", DisplayName: "Mr. Mime", Types: []string{"psychic", "fairy"}},
			{ID: 133, Slug: "eevee", DisplayName: "Eevee", Types: []string{"normal"}},
		},
		pokemon: map[int]domain.Pokemon{
			25: {
				ID: 25, Slug: "pikachu", DisplayName: "Pikachu", SpeciesID: 25,
				HeightDecimeter: 4, WeightHectogram: 60, BaseExperience: 112,
				Types: []domain.TypeSlot{{Slot: 1, Name: "electric"}},
				Abilities: []domain.AbilitySlot{
					{Slot: 1, Name: "static"},
					{Slot: 3, Name: "lightning-rod", IsHidden: true},
				},
				Stats: domain.Stats{HP: 35, Attack: 55, Defense: 40, SpecialAttack: 50, SpecialDefense: 50, Speed: 90},
			},
			122: {
				ID: 122, Slug: "mr-mime", DisplayName: "Mr Mime", SpeciesID: 122,
				HeightDecimeter: 13, WeightHectogram: 545, BaseExperience: 161,
				Types: []domain.TypeSlot{{Slot: 1, Name: "psychic"}, {Slot: 2, Name: "fairy"}},
				Stats: domain.Stats{HP: 40, Attack: 45, Defense: 65, SpecialAttack: 100, SpecialDefense: 120, Speed: 90},
			},
			133: {
				ID: 133, Slug: "eevee", DisplayName: "Eevee", SpeciesID: 133,
				Stats: domain.Stats{HP: 55, Attack: 55, Defense: 50, SpecialAttack: 45, SpecialDefense: 65, Speed: 55},
			},
		},
		species: map[int]domain.Species{
			25: {
				ID: 25, Slug: "pikachu", DisplayName: "Pikachu", Genus: "Mouse Pokémon",
				DefaultPokemonID: 25, EvolutionChainID: 10,
				FlavorTextEntries: []domain.FlavorText{
					{Language: "en", VersionOrder: 1, Text: "Old text"},
					{Language: "en", VersionOrder: 40, Text: "Cheek sacs that are extra soft."},
				},
			},
			122: {
				ID: 122, Slug: "mr-mime", DisplayName: "Mr. Mime", Genus: "Barrier Pokémon",
				DefaultPokemonID: 122, EvolutionChainID: 60,
			},
			133: {
				ID: 133, Slug: "eevee", DisplayName: "Eevee", DefaultPokemonID: 133, EvolutionChainID: 67,
			},
		},
		evolution: map[int]domain.EvolutionTree{
			10: {Root: domain.EvolutionNode{
				DisplayName: "Pichu", SpeciesSlug: "pichu",
				Children: []domain.EvolutionBranch{{
					Conditions: []domain.EvolutionCondition{{Trigger: "level-up", MinimumHappiness: level(220)}},
					Target: domain.EvolutionNode{
						DisplayName: "Pikachu", SpeciesSlug: "pikachu",
						Children: []domain.EvolutionBranch{{
							Conditions: []domain.EvolutionCondition{{Trigger: "use-item", Item: "thunder-stone"}},
							Target:     domain.EvolutionNode{DisplayName: "Raichu", SpeciesSlug: "raichu"},
						}},
					},
				}},
			}},
			67: {Root: domain.EvolutionNode{
				DisplayName: "Eevee", SpeciesSlug: "eevee",
				Children: []domain.EvolutionBranch{
					{Conditions: []domain.EvolutionCondition{{Trigger: "use-item", Item: "water-stone"}}, Target: domain.EvolutionNode{DisplayName: "Vaporeon"}},
					{Conditions: []domain.EvolutionCondition{{Trigger: "use-item", Item: "thunder-stone"}}, Target: domain.EvolutionNode{DisplayName: "Jolteon"}},
					{Conditions: []domain.EvolutionCondition{{Trigger: "use-item", Item: "fire-stone"}}, Target: domain.EvolutionNode{DisplayName: "Flareon"}},
				},
			}},
		},
	}
	svc := cache.NewService(store, api, testCfg(), clock)
	if err := svc.Refresh(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	services := &app.Services{Config: testCfg(), Paths: config.Paths{CacheDir: root}, Cache: svc}
	handler := &app.Handler{
		Services: func() (*app.Services, error) { return services, nil },
		Stdout:   stdout,
		Stderr:   stderr,
		Rand:     app.FixedRand{N: 0},
	}
	return handler, stdout, stderr
}

func TestSearchAndInlineCommands(t *testing.T) {
	handler, stdout, _ := phase3Fixture(t)

	if err := handler.Search(context.Background(), "mr"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Mr. Mime") {
		t.Fatalf("search output = %q", stdout.String())
	}
	stdout.Reset()

	if err := handler.RunRoot(context.Background(), cli.RootRequest{Target: "25", InlineMode: cli.InlineStats}); err != nil {
		t.Fatal(err)
	}
	statsByNumber := stdout.String()
	stdout.Reset()
	if err := handler.RunRoot(context.Background(), cli.RootRequest{Target: "pikachu", InlineMode: cli.InlineStats}); err != nil {
		t.Fatal(err)
	}
	if statsByNumber != stdout.String() {
		t.Fatalf("number/name stats mismatch:\n%q\n%q", statsByNumber, stdout.String())
	}
	stdout.Reset()

	if err := handler.RunRoot(context.Background(), cli.RootRequest{Target: "mr. mime", InlineMode: cli.InlineInfo}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Mr. Mime") || !strings.Contains(stdout.String(), "Psychic/Fairy") {
		t.Fatalf("info = %q", stdout.String())
	}
	stdout.Reset()

	if err := handler.RunRoot(context.Background(), cli.RootRequest{Target: "eevee", InlineMode: cli.InlineEvolution}); err != nil {
		t.Fatal(err)
	}
	evo := stdout.String()
	if !strings.Contains(evo, "Eevee") || !strings.Contains(evo, "Vaporeon") || !strings.Contains(evo, "Water Stone") {
		t.Fatalf("evolution = %q", evo)
	}
}

func TestInlineNotFound(t *testing.T) {
	handler, _, _ := phase3Fixture(t)
	err := handler.RunRoot(context.Background(), cli.RootRequest{Target: "missingno", InlineMode: cli.InlineInfo})
	var nf *app.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("err = %v", err)
	}
	if app.ExitCode(err) != 1 {
		t.Fatalf("exit = %d", app.ExitCode(err))
	}
}

func TestRandomInjection(t *testing.T) {
	handler, stdout, _ := phase3Fixture(t)
	handler.Rand = app.FixedRand{N: 1} // mr-mime in fixture list order after refresh
	if err := handler.Random(context.Background(), cli.InlineInfo); err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	// Index order follows API list: pikachu=0, mr-mime=1, eevee=2
	if !strings.Contains(out, "Mr. Mime") {
		t.Fatalf("random info = %q", out)
	}
}

func TestDetailAggregation(t *testing.T) {
	handler, _, _ := phase3Fixture(t)
	svc, err := handler.Services()
	if err != nil {
		t.Fatal(err)
	}
	details := app.NewDetailService(svc.Cache)
	detail, err := details.Load(context.Background(), 25)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Pokemon.Stats.Speed != 90 || detail.Species.Genus != "Mouse Pokémon" {
		t.Fatalf("detail = %#v", detail)
	}
	tree, err := details.Evolution(context.Background(), detail.Species)
	if err != nil || tree.Root.DisplayName != "Pichu" {
		t.Fatalf("evolution = %#v err=%v", tree, err)
	}
}

func TestMainInlineNotFoundWritesStderr(t *testing.T) {
	handler, _, _ := phase3Fixture(t)
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	runtime := app.Runtime{
		InvocationPath: "poke",
		Args:           []string{"missingno", "--info"},
		Getenv:         func(string) string { return "" },
		Directories: config.DirectoryResolver{
			UserConfigDir: func() (string, error) { return filepath.Join(t.TempDir(), "config"), nil },
			UserCacheDir:  func() (string, error) { return filepath.Join(t.TempDir(), "cache"), nil },
		},
		App: handler,
		Cache: stubCacheCommands{
			clear: func() error { return nil },
		},
		Stdout: stdout,
		Stderr: stderr,
	}
	code := app.Main(context.Background(), runtime)
	if code != 1 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
