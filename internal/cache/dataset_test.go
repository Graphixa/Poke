package cache_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/cache"
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
