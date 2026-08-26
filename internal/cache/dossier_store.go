package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Graphixa/Pokedex/internal/domain"
)

// Dossier returns a cached species dossier when present.
func (s *Store) Dossier(_ context.Context, speciesID int, slug string) (domain.Dossier, error) {
	path, err := s.resolveDossierPath(speciesID, slug)
	if err != nil {
		return domain.Dossier{}, err
	}
	var value domain.Dossier
	if _, err := s.readJSON(path, &value); err != nil {
		return domain.Dossier{}, err
	}
	return value, nil
}

// StoreDossier writes pokemon/<id>-<slug>/dossier.json.
func (s *Store) StoreDossier(_ context.Context, dossier domain.Dossier, fetchedAt time.Time) error {
	slug := dossier.Species.Slug
	if slug == "" {
		slug = dossier.Pokemon.Slug
	}
	id := dossier.Species.ID
	if id == 0 {
		id = dossier.Pokemon.SpeciesID
	}
	if id == 0 {
		id = dossier.Pokemon.ID
	}
	if id == 0 {
		return errors.New("cache: dossier missing species id")
	}
	return s.writeJSON(s.dossierPath(id, slug), fetchedAt, dossier)
}

// DossierFetchedAt returns the envelope timestamp for a dossier.
func (s *Store) DossierFetchedAt(speciesID int, slug string) (time.Time, error) {
	path, err := s.resolveDossierPath(speciesID, slug)
	if err != nil {
		return time.Time{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return time.Time{}, ErrNotFound
	}
	if err != nil {
		return time.Time{}, err
	}
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		_ = s.quarantine(path, data)
		return time.Time{}, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	return env.FetchedAt, nil
}

// NamedSprite reads sprites/ or official-artwork/ PNG bytes.
func (s *Store) NamedSprite(_ context.Context, id int, slug, kind string) ([]byte, error) {
	path, err := s.namedSpritePath(id, slug, kind)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

// StoreNamedSprite writes a named PNG under sprites/ or official-artwork/.
func (s *Store) StoreNamedSprite(_ context.Context, id int, slug, kind string, data []byte) error {
	if len(data) == 0 {
		return errors.New("cache: refuse to store empty named sprite")
	}
	path, err := s.namedSpritePath(id, slug, kind)
	if err != nil {
		return err
	}
	return writeAtomic(path, data, 0o600)
}

func (s *Store) namedSpritePath(id int, slug, kind string) (string, error) {
	key := PokemonFolderKey(id, slug)
	switch kind {
	case SpriteKindPixel, "":
		return filepath.Join(s.root, "sprites", key, SpriteFileFrontDefault), nil
	case SpriteKindArtwork:
		return filepath.Join(s.root, "official-artwork", key, SpriteFileFrontDefault), nil
	default:
		return "", fmt.Errorf("unknown sprite kind %q", kind)
	}
}

func (s *Store) dossierPath(id int, slug string) string {
	return filepath.Join(s.root, "pokemon", PokemonFolderKey(id, slug), "dossier.json")
}

func (s *Store) resolveDossierPath(speciesID int, slug string) (string, error) {
	if slug != "" {
		path := s.dossierPath(speciesID, slug)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	pattern := filepath.Join(s.root, "pokemon", fmt.Sprintf("%03d-*", speciesID), "dossier.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		// Also try unpadded id prefix for robustness.
		pattern = filepath.Join(s.root, "pokemon", strconv.Itoa(speciesID)+"-*", "dossier.json")
		matches, err = filepath.Glob(pattern)
		if err != nil {
			return "", err
		}
	}
	if len(matches) == 0 {
		return "", ErrNotFound
	}
	return matches[0], nil
}

// legacyPokemonPath is the pre-dossier flat file.
func (s *Store) legacyPokemonPath(id int) string {
	return filepath.Join(s.root, "pokemon", strconv.Itoa(id)+".json")
}

func (s *Store) legacySpeciesPath(id int) string {
	return filepath.Join(s.root, "species", strconv.Itoa(id)+".json")
}

func (s *Store) legacyEvolutionPath(id int) string {
	return filepath.Join(s.root, "evolution", strconv.Itoa(id)+".json")
}

func (s *Store) legacyEncountersPath(id int) string {
	return filepath.Join(s.root, "encounters", strconv.Itoa(id)+".json")
}

// TryMigrateDossier assembles a dossier from legacy split files when present.
func (s *Store) TryMigrateDossier(ctx context.Context, speciesID int) (domain.Dossier, bool) {
	var species domain.Species
	if _, err := s.readJSON(s.legacySpeciesPath(speciesID), &species); err != nil {
		return domain.Dossier{}, false
	}
	pokemonID := species.DefaultPokemonID
	if pokemonID == 0 {
		pokemonID = speciesID
	}
	var pokemon domain.Pokemon
	if _, err := s.readJSON(s.legacyPokemonPath(pokemonID), &pokemon); err != nil {
		return domain.Dossier{}, false
	}
	d := domain.Dossier{
		Pokemon:    pokemon,
		Species:    species,
		Encounters: nil,
	}
	if species.EvolutionChainID != 0 {
		var tree domain.EvolutionTree
		if _, err := s.readJSON(s.legacyEvolutionPath(species.EvolutionChainID), &tree); err == nil {
			d.Evolution = &tree
		}
	}
	var enc []domain.Encounter
	if _, err := s.readJSON(s.legacyEncountersPath(pokemonID), &enc); err == nil {
		d.Encounters = enc
	}
	_ = ctx
	return d, true
}

// fillIndexSpritePaths sets SpritePath on summaries when empty.
func fillIndexSpritePaths(index []domain.SpeciesSummary) {
	for i := range index {
		if index[i].SpritePath == "" && index[i].ID != 0 {
			index[i].SpritePath = SpriteRelPath(index[i].ID, index[i].Slug)
		}
	}
}

// stripUnsafeRel rejects paths that escape the cache root via ..
func stripUnsafeRel(rel string) bool {
	clean := filepath.Clean(rel)
	return strings.Contains(clean, "..") || filepath.IsAbs(clean)
}
