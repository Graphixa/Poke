package cache_test

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/dataset"
	"github.com/Graphixa/Pokedex/internal/domain"
)

func TestActivateDatasetRefusesIncomplete(t *testing.T) {
	root := t.TempDir()
	store := cache.NewStore(root, fixedClock{t: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)})
	id := cache.DatasetIDForTime(time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC))
	if err := store.EnsureDatasetDir(id); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteDatasetManifest(cache.DatasetManifest{
		SchemaVersion:  cache.DatasetSchemaVersion,
		DatasetVersion: id,
		Complete:       false,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.ActivateDataset(id); err == nil {
		t.Fatal("expected refuse incomplete snapshot")
	}
	if _, err := store.ActiveDatasetID(); err == nil {
		t.Fatal("active pointer must stay absent")
	}
}

func TestFailedSnapshotDoesNotReplaceActive(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	good := "2026-08-29T000000"
	if err := store.EnsureDatasetDir(good); err != nil {
		t.Fatal(err)
	}
	entries := []domain.BrowseEntry{{ID: 1, Slug: "bulbasaur", Name: "Bulbasaur", Types: []string{"grass"}}}
	if err := store.StoreBrowse(good, entries); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteDatasetManifest(cache.DatasetManifest{
		SchemaVersion:  cache.DatasetSchemaVersion,
		DatasetVersion: good,
		PokemonCount:   1,
		Complete:       true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.ActivateDataset(good); err != nil {
		t.Fatal(err)
	}

	bad := "2026-08-29T010000"
	if err := store.EnsureDatasetDir(bad); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteDatasetManifest(cache.DatasetManifest{
		SchemaVersion:  cache.DatasetSchemaVersion,
		DatasetVersion: bad,
		Complete:       false,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.ActivateDataset(bad); err == nil {
		t.Fatal("must not activate incomplete")
	}
	got, err := store.ActiveDatasetID()
	if err != nil || got != good {
		t.Fatalf("active=%q err=%v want %s", got, err, good)
	}
}

func TestCompactCacheJSON(t *testing.T) {
	root := t.TempDir()
	store := cache.NewStore(root, nil)
	move := domain.Move{ID: 1, Slug: "pound", DisplayName: "Pound", PP: 35}
	if err := store.StoreMove(context.Background(), move, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "moves", "1.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "\n  ") {
		t.Fatalf("expected compact JSON, got indented:\n%s", raw)
	}
	var env map[string]any
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
}

func TestNamedAssetMetaDoesNotReadPNG(t *testing.T) {
	root := t.TempDir()
	store := cache.NewStore(root, nil)
	png := []byte{0x89, 'P', 'N', 'G'}
	if err := store.StoreNamedSpriteWithMeta(context.Background(), 25, "pikachu", cache.SpriteKindArtwork, "https://example/art.png", "abc123", png); err != nil {
		t.Fatal(err)
	}
	hash, path, ready, err := store.NamedAssetMeta(context.Background(), 25, "pikachu", cache.SpriteKindArtwork)
	if err != nil || !ready || hash == "" || path == "" {
		t.Fatalf("meta hash=%q path=%q ready=%v err=%v", hash, path, ready, err)
	}
	got, err := store.ReadAssetBytes(path)
	if err != nil || len(got) != len(png) {
		t.Fatalf("ReadAssetBytes abs path err=%v len=%d", err, len(got))
	}
}

func TestBootstrapDatasetActivatesSnapshot(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	clock := fixedClock{t: now}
	store := cache.NewStore(root, clock)
	api := &fakeAPI{
		list: []domain.SpeciesSummary{{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}},
		pokemon: map[int]domain.Pokemon{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", Stats: domain.Stats{Speed: 90}, Types: []domain.TypeSlot{{Name: "electric"}}},
		},
		species: map[int]domain.Species{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", DefaultPokemonID: 25},
		},
	}
	svc := cache.NewService(store, api, testConfig(), clock)
	if err := svc.BootstrapDataset(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if !svc.HasActiveDataset() {
		t.Fatal("expected active dataset")
	}
	if !svc.LocalOnly() {
		t.Fatal("expected local-only after bootstrap")
	}
	entries, err := store.LoadBrowse()
	if err != nil || len(entries) != 1 || entries[0].Speed != 90 {
		t.Fatalf("browse=%#v err=%v", entries, err)
	}
}

func TestOfflineReadsAfterBootstrap(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	clock := fixedClock{t: now}
	store := cache.NewStore(root, clock)
	api := &fakeAPI{
		list: []domain.SpeciesSummary{{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}},
		pokemon: map[int]domain.Pokemon{
			25: {
				ID: 25, Slug: "pikachu", DisplayName: "Pikachu",
				Stats: domain.Stats{Speed: 90},
				Types: []domain.TypeSlot{{Name: "electric"}},
			},
		},
		species: map[int]domain.Species{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", DefaultPokemonID: 25},
		},
	}
	svc := cache.NewService(store, api, testConfig(), clock)
	if err := svc.BootstrapDataset(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	before := api.calls.Load()
	p, err := svc.Pokemon(context.Background(), 25)
	if err != nil || p.ID != 25 {
		t.Fatalf("pokemon=%#v err=%v", p, err)
	}
	sp, err := svc.Species(context.Background(), 25)
	if err != nil || sp.ID != 25 {
		t.Fatalf("species=%#v err=%v", sp, err)
	}
	d, err := svc.Dossier(context.Background(), 25)
	if err != nil || d.Pokemon.ID != 25 {
		t.Fatalf("dossier=%#v err=%v", d, err)
	}
	if api.calls.Load() != before {
		t.Fatalf("offline path hit network: calls %d -> %d", before, api.calls.Load())
	}
}

func TestOfflineReadsFromSnapshotOnly(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	clock := fixedClock{t: now}
	store := cache.NewStore(root, clock)
	api := &fakeAPI{
		list: []domain.SpeciesSummary{{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}},
		pokemon: map[int]domain.Pokemon{
			25: {
				ID: 25, Slug: "pikachu", DisplayName: "Pikachu", SpeciesID: 25,
				Stats: domain.Stats{Speed: 90},
				Types: []domain.TypeSlot{{Name: "electric"}},
				Moves: []domain.LearnedMove{{MoveID: 1, Learnings: []domain.MoveLearning{{Method: "level-up", Level: 1}}}},
			},
		},
		species: map[int]domain.Species{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", DefaultPokemonID: 25, EvolutionChainID: 10},
		},
		evolution: map[int]domain.EvolutionTree{
			10: {Root: domain.EvolutionNode{SpeciesID: 25, SpeciesSlug: "pikachu", DisplayName: "Pikachu"}},
		},
		encounters: map[int][]domain.Encounter{
			25: {{Area: "forest", MinimumLevel: 5}},
		},
		moves: map[int]domain.Move{
			1: {ID: 1, Slug: "pound", DisplayName: "Pound", PP: 35},
		},
	}
	svc := cache.NewService(store, api, testConfig(), clock)
	if err := svc.BootstrapDataset(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"pokemon", "species", "evolution", "encounters", "moves"} {
		if err := os.RemoveAll(filepath.Join(root, dir)); err != nil {
			t.Fatal(err)
		}
	}
	before := api.calls.Load()
	p, err := svc.Pokemon(context.Background(), 25)
	if err != nil || p.ID != 25 {
		t.Fatalf("pokemon=%#v err=%v", p, err)
	}
	sp, err := svc.Species(context.Background(), 25)
	if err != nil || sp.ID != 25 {
		t.Fatalf("species=%#v err=%v", sp, err)
	}
	tree, err := svc.EvolutionChain(context.Background(), 10)
	if err != nil || tree.Root.SpeciesID != 25 {
		t.Fatalf("evolution=%#v err=%v", tree, err)
	}
	enc, err := svc.Encounters(context.Background(), 25)
	if err != nil || len(enc) != 1 {
		t.Fatalf("encounters=%#v err=%v", enc, err)
	}
	mv, err := svc.Move(context.Background(), 1)
	if err != nil || mv.ID != 1 {
		t.Fatalf("move=%#v err=%v", mv, err)
	}
	d, err := svc.Dossier(context.Background(), 25)
	if err != nil || d.Pokemon.ID != 25 {
		t.Fatalf("dossier=%#v err=%v", d, err)
	}
	if api.calls.Load() != before {
		t.Fatalf("snapshot-only offline path hit network: calls %d -> %d", before, api.calls.Load())
	}
}

func TestBootstrapPartialFailureDoesNotActivate(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	clock := fixedClock{t: now}
	store := cache.NewStore(root, clock)
	api := &fakeAPI{
		list: []domain.SpeciesSummary{
			{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"},
			{ID: 26, Slug: "raichu", DisplayName: "Raichu"},
		},
		pokemon: map[int]domain.Pokemon{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", Stats: domain.Stats{Speed: 90}},
		},
		species: map[int]domain.Species{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", DefaultPokemonID: 25},
			26: {ID: 26, Slug: "raichu", DisplayName: "Raichu", DefaultPokemonID: 26},
		},
	}
	svc := cache.NewService(store, api, testConfig(), clock)
	if err := svc.BootstrapDataset(context.Background(), nil); err == nil {
		t.Fatal("expected incomplete bootstrap to fail")
	}
	if svc.HasActiveDataset() {
		t.Fatal("incomplete snapshot must not become active")
	}
	id := cache.DatasetIDForTime(now)
	man, err := store.ReadDatasetManifest(id)
	if err != nil {
		t.Fatal(err)
	}
	if man.Complete {
		t.Fatal("manifest must stay incomplete")
	}
	rep, err := store.ReadBootstrapReport(id)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Ready() {
		t.Fatalf("report ready with failures: %+v", rep)
	}
	if len(rep.Failures) == 0 {
		t.Fatal("expected persisted failure diagnostics")
	}
}

func TestBootstrapFailurePreservesPreviousActive(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	clock := fixedClock{t: now}
	store := cache.NewStore(root, clock)
	goodAPI := &fakeAPI{
		list: []domain.SpeciesSummary{{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}},
		pokemon: map[int]domain.Pokemon{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", Stats: domain.Stats{Speed: 90}},
		},
		species: map[int]domain.Species{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", DefaultPokemonID: 25},
		},
	}
	svc := cache.NewService(store, goodAPI, testConfig(), clock)
	if err := svc.BootstrapDataset(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	goodID, err := store.ActiveDatasetID()
	if err != nil || goodID == "" {
		t.Fatalf("active=%q err=%v", goodID, err)
	}

	later := fixedClock{t: now.Add(time.Hour)}
	twoSpecies := []domain.SpeciesSummary{
		{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"},
		{ID: 26, Slug: "raichu", DisplayName: "Raichu"},
	}
	if err := store.StoreSpeciesIndex(context.Background(), twoSpecies, later.t); err != nil {
		t.Fatal(err)
	}
	failAPI := &fakeAPI{
		list: []domain.SpeciesSummary{
			{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"},
			{ID: 26, Slug: "raichu", DisplayName: "Raichu"},
		},
		pokemonFn: func(id int) (domain.Pokemon, error) {
			if id == 26 {
				return domain.Pokemon{}, errors.New("forced pokemon miss")
			}
			return domain.Pokemon{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}, nil
		},
		species: map[int]domain.Species{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", DefaultPokemonID: 25},
		},
	}
	svc2 := cache.NewService(store, failAPI, testConfig(), later)
	if err := svc2.BootstrapDataset(context.Background(), nil); err == nil {
		t.Fatal("expected second bootstrap to fail")
	}
	got, err := store.ActiveDatasetID()
	if err != nil || got != goodID {
		t.Fatalf("active=%q err=%v want %s", got, err, goodID)
	}
}

func TestDatasetSnapshotHasBoundedFileCount(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	clock := fixedClock{t: now}
	store := cache.NewStore(root, clock)
	api := &fakeAPI{
		list: []domain.SpeciesSummary{{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}},
		pokemon: map[int]domain.Pokemon{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", SpeciesID: 25, Stats: domain.Stats{Speed: 90}},
		},
		species: map[int]domain.Species{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", DefaultPokemonID: 25},
		},
	}
	svc := cache.NewService(store, api, testConfig(), clock)
	if err := svc.BootstrapDataset(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	id, _ := store.ActiveDatasetID()
	dir := filepath.Join(root, "datasets", id)
	count := 0
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		count++
		return nil
	})
	if count > 12 {
		t.Fatalf("dataset snapshot files=%d want <=12", count)
	}
}

func TestOfflineSpeciesSummaryRead(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	clock := fixedClock{t: now}
	store := cache.NewStore(root, clock)
	api := &fakeAPI{
		list: []domain.SpeciesSummary{{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}},
		pokemon: map[int]domain.Pokemon{25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", SpeciesID: 25}},
		species: map[int]domain.Species{25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", DefaultPokemonID: 25, Genus: "Mouse"}},
	}
	svc := cache.NewService(store, api, testConfig(), clock)
	if err := svc.BootstrapDataset(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	before := api.calls.Load()
	sp, err := svc.Species(context.Background(), 25)
	if err != nil || sp.Genus == "" {
		t.Fatalf("species=%#v err=%v", sp, err)
	}
	if api.calls.Load() != before {
		t.Fatal("offline species read hit network")
	}
}

func TestOfflineEvolutionRead(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	clock := fixedClock{t: now}
	store := cache.NewStore(root, clock)
	api := &fakeAPI{
		list: []domain.SpeciesSummary{{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}},
		pokemon: map[int]domain.Pokemon{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", SpeciesID: 25, Stats: domain.Stats{Speed: 90}},
		},
		species: map[int]domain.Species{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", DefaultPokemonID: 25, EvolutionChainID: 10},
		},
		evolution: map[int]domain.EvolutionTree{
			10: {Root: domain.EvolutionNode{SpeciesID: 25, SpeciesSlug: "pikachu", DisplayName: "Pikachu"}},
		},
	}
	svc := cache.NewService(store, api, testConfig(), clock)
	if err := svc.BootstrapDataset(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	before := api.calls.Load()
	tree, err := svc.EvolutionChain(context.Background(), 10)
	if err != nil || tree.Root.SpeciesID != 25 {
		t.Fatalf("evolution=%#v err=%v", tree, err)
	}
	if api.calls.Load() != before {
		t.Fatal("offline evolution read hit network")
	}
}

func TestOfflineEncountersRead(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	clock := fixedClock{t: now}
	store := cache.NewStore(root, clock)
	api := &fakeAPI{
		list: []domain.SpeciesSummary{{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"}},
		pokemon: map[int]domain.Pokemon{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", SpeciesID: 25, Stats: domain.Stats{Speed: 90}},
		},
		species: map[int]domain.Species{
			25: {ID: 25, Slug: "pikachu", DisplayName: "Pikachu", DefaultPokemonID: 25},
		},
		encounters: map[int][]domain.Encounter{
			25: {{Area: "forest", MinimumLevel: 5}},
		},
	}
	svc := cache.NewService(store, api, testConfig(), clock)
	if err := svc.BootstrapDataset(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	before := api.calls.Load()
	enc, err := svc.Encounters(context.Background(), 25)
	if err != nil || len(enc) != 1 {
		t.Fatalf("encounters=%#v err=%v", enc, err)
	}
	if api.calls.Load() != before {
		t.Fatal("offline encounters read hit network")
	}
}

func TestAggregateJSONUsesNoEnvelope(t *testing.T) {
	root := t.TempDir()
	store := cache.NewStore(root, nil)
	id := "test"
	b := dataset.NewBuilder()
	b.Pokemon[1] = dataset.PokemonFromDomain(domain.Pokemon{ID: 1, Slug: "a", DisplayName: "A"})
	if err := store.StoreAggregatePokemon(id, b.Pokemon); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(store.AggregatePokemonPath(id))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "schema_version") {
		t.Fatalf("aggregate must not use envelope: %s", raw)
	}
}

func TestAggregatePathsAndRoundTrip(t *testing.T) {
	root := t.TempDir()
	store := cache.NewStore(root, nil)
	id := "2026-08-29T120000"
	if err := store.EnsureDatasetDir(id); err != nil {
		t.Fatal(err)
	}

	b := dataset.NewBuilder()
	b.Pokemon[25] = dataset.PokemonFromDomain(domain.Pokemon{
		ID: 25, Slug: "pikachu", DisplayName: "Pikachu", SpeciesID: 25,
		Stats: domain.Stats{Speed: 90},
		Types: []domain.TypeSlot{{Name: "electric"}},
	})
	b.Species[25] = dataset.SpeciesFromDomain(domain.Species{
		ID: 25, Slug: "pikachu", DisplayName: "Pikachu", DefaultPokemonID: 25,
	})
	b.Learnsets[25] = []dataset.DatasetLearn{{Level: 1, MoveID: 84}}
	b.Moves[84] = dataset.DatasetMove{ID: 84, Slug: "thunder-shock", DisplayName: "Thunder Shock", PP: 30}
	b.Evolutions[10] = dataset.EvolutionFromDomain(domain.EvolutionTree{
		Root: domain.EvolutionNode{SpeciesID: 25, SpeciesSlug: "pikachu", DisplayName: "Pikachu"},
	})
	b.Encounters[25] = dataset.EncountersFromDomain([]domain.Encounter{{Area: "forest", MinimumLevel: 5}})

	if err := store.StoreAggregateBuilder(id, b); err != nil {
		t.Fatal(err)
	}

	for _, file := range []string{
		dataset.FilePokemon,
		dataset.FileSpecies,
		dataset.FileLearnsets,
		dataset.FileMoves,
		dataset.FileEvolutions,
		dataset.FileEncounters,
	} {
		path := filepath.Join(root, "datasets", id, file)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if strings.Contains(string(raw), "\n  ") {
			t.Fatalf("expected compact JSON for %s:\n%s", file, raw)
		}
	}

	pokemon, err := store.LoadAggregatePokemon(id)
	if err != nil || pokemon[25].Slug != "pikachu" {
		t.Fatalf("pokemon=%#v err=%v", pokemon, err)
	}
	species, err := store.LoadAggregateSpecies(id)
	if err != nil || species[25].DisplayName != "Pikachu" {
		t.Fatalf("species=%#v err=%v", species, err)
	}
	learnsets, err := store.LoadAggregateLearnsets(id)
	if err != nil || learnsets[25][0].MoveID != 84 {
		t.Fatalf("learnsets=%#v err=%v", learnsets, err)
	}
	moves, err := store.LoadAggregateMoves(id)
	if err != nil || moves[84].Slug != "thunder-shock" {
		t.Fatalf("moves=%#v err=%v", moves, err)
	}
	evolutions, err := store.LoadAggregateEvolutions(id)
	if err != nil || evolutions[10].Root.SpeciesID != 25 {
		t.Fatalf("evolutions=%#v err=%v", evolutions, err)
	}
	encounters, err := store.LoadAggregateEncounters(id)
	if err != nil || encounters[25][0].Area != "forest" {
		t.Fatalf("encounters=%#v err=%v", encounters, err)
	}
}

func TestAggregatePathHelpers(t *testing.T) {
	store := cache.NewStore(t.TempDir(), nil)
	id := "test-id"
	if got := store.AggregatePokemonPath(id); !strings.HasSuffix(got, filepath.Join("datasets", id, dataset.FilePokemon)) {
		t.Fatalf("pokemon path=%q", got)
	}
	if got := store.AggregateSpeciesPath(id); !strings.HasSuffix(got, dataset.FileSpecies) {
		t.Fatalf("species path=%q", got)
	}
}
