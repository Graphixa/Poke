package cache

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/output"
)

// Dossier returns a cache-first species dossier (disk, migrate, or assemble from API).
func (s *Service) Dossier(ctx context.Context, speciesID int) (domain.Dossier, error) {
	v, err := s.group.Do(fmt.Sprintf("dossier:%d", speciesID), func() (any, error) {
		return s.getDossier(ctx, speciesID, "")
	})
	if err != nil {
		return domain.Dossier{}, err
	}
	return v.(domain.Dossier), nil
}

// DossierWithSlug is like Dossier but prefers a known slug for the on-disk path.
func (s *Service) DossierWithSlug(ctx context.Context, speciesID int, slug string) (domain.Dossier, error) {
	v, err := s.group.Do(fmt.Sprintf("dossier:%d", speciesID), func() (any, error) {
		return s.getDossier(ctx, speciesID, slug)
	})
	if err != nil {
		return domain.Dossier{}, err
	}
	return v.(domain.Dossier), nil
}

func (s *Service) getDossier(ctx context.Context, speciesID int, slug string) (domain.Dossier, error) {
	cached, cacheErr := s.store.Dossier(ctx, speciesID, slug)
	haveCache := cacheErr == nil
	if haveCache && !s.dossierStale(speciesID, slugOr(cached, slug)) {
		debuglog.Cachef("hit dossier id=%d", speciesID)
		return cached, nil
	}
	if cacheErr != nil && !errors.Is(cacheErr, ErrNotFound) && !errors.Is(cacheErr, ErrCorrupt) {
		return domain.Dossier{}, cacheErr
	}

	if !haveCache {
		if migrated, ok := s.store.TryMigrateDossier(ctx, speciesID); ok {
			migrated = s.ensureLevelUpMoves(ctx, migrated)
			_ = s.store.StoreDossier(ctx, migrated, s.clock.Now())
			debuglog.Cachef("migrate dossier id=%d", speciesID)
			if !s.dossierStale(speciesID, slugOr(migrated, slug)) {
				return migrated, nil
			}
			cached = migrated
			haveCache = true
		}
	}

	fresh, fetchErr := s.assembleDossier(ctx, speciesID)
	if fetchErr != nil {
		if haveCache {
			debuglog.Cachef("stale-serve dossier id=%d err=%v", speciesID, fetchErr)
			return cached, nil
		}
		return domain.Dossier{}, fetchErr
	}
	_ = s.store.StoreDossier(ctx, fresh, s.clock.Now())
	// Keep legacy split files warm for callers still using Pokemon/Species getters.
	_ = s.store.StoreSpecies(ctx, fresh.Species, s.clock.Now())
	_ = s.store.StorePokemon(ctx, fresh.Pokemon, s.clock.Now())
	if fresh.Evolution != nil && fresh.Species.EvolutionChainID != 0 {
		_ = s.store.StoreEvolutionChain(ctx, fresh.Species.EvolutionChainID, *fresh.Evolution, s.clock.Now())
	}
	_ = s.store.StoreEncounters(ctx, fresh.Pokemon.ID, fresh.Encounters, s.clock.Now())
	debuglog.Cachef("store dossier id=%d", speciesID)
	return fresh, nil
}

// WarmCore fetches and caches pokemon+species only (no evolution/encounters/moves).
// Used by window-mode dossier warm so browse headers stay snappy without API storms.
func (s *Service) WarmCore(ctx context.Context, speciesID int) error {
	if speciesID < 1 {
		return fmt.Errorf("cache: invalid species id")
	}
	species, err := s.Species(ctx, speciesID)
	if err != nil {
		return err
	}
	pokemonID := species.DefaultPokemonID
	if pokemonID == 0 {
		pokemonID = speciesID
	}
	_, err = s.Pokemon(ctx, pokemonID)
	return err
}

func slugOr(d domain.Dossier, slug string) string {
	if d.Species.Slug != "" {
		return d.Species.Slug
	}
	if d.Pokemon.Slug != "" {
		return d.Pokemon.Slug
	}
	return slug
}

func (s *Service) dossierStale(speciesID int, slug string) bool {
	fetchedAt, err := s.store.DossierFetchedAt(speciesID, slug)
	if err != nil {
		return true
	}
	return s.isStale(fetchedAt, s.cfg.Cache.DetailTTL.Duration)
}

func (s *Service) assembleDossier(ctx context.Context, speciesID int) (domain.Dossier, error) {
	species, err := s.Species(ctx, speciesID)
	if err != nil {
		return domain.Dossier{}, err
	}
	pokemonID := species.DefaultPokemonID
	if pokemonID == 0 {
		pokemonID = speciesID
	}
	pokemon, err := s.Pokemon(ctx, pokemonID)
	if err != nil {
		return domain.Dossier{}, err
	}
	if species.DisplayName != "" {
		pokemon.DisplayName = species.DisplayName
	}
	d := domain.Dossier{
		Pokemon: pokemon,
		Species: species,
	}

	var (
		wg       sync.WaitGroup
		evoTree  domain.EvolutionTree
		evoOK    bool
		encounters []domain.Encounter
		encOK    bool
	)
	if species.EvolutionChainID != 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tree, evoErr := s.EvolutionChain(ctx, species.EvolutionChainID)
			if evoErr == nil {
				evoTree = tree
				evoOK = true
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		enc, encErr := s.Encounters(ctx, pokemonID)
		if encErr == nil {
			encounters = enc
			encOK = true
		}
	}()
	wg.Wait()
	if evoOK {
		d.Evolution = &evoTree
	}
	if encOK {
		d.Encounters = encounters
	}
	d = s.ensureLevelUpMoves(ctx, d)
	return d, nil
}

func (s *Service) ensureLevelUpMoves(ctx context.Context, d domain.Dossier) domain.Dossier {
	if len(d.LevelUpMoves) > 0 {
		return d
	}
	rows, err := resolveLevelUpMoves(ctx, d.Pokemon, func(id int) (domain.Move, error) {
		return s.Move(ctx, id)
	})
	if err != nil {
		return d
	}
	d.LevelUpMoves = rows
	return d
}

func resolveLevelUpMoves(ctx context.Context, pokemon domain.Pokemon, getMove func(int) (domain.Move, error)) ([]domain.LevelUpMove, error) {
	bestOrder := -1
	bestGroup := ""
	for _, lm := range pokemon.Moves {
		for _, learn := range lm.Learnings {
			if learn.Method != "level-up" {
				continue
			}
			if learn.VersionGroupOrder >= bestOrder {
				bestOrder = learn.VersionGroupOrder
				bestGroup = learn.VersionGroup
			}
		}
	}
	if bestOrder < 0 {
		return nil, nil
	}

	type pending struct {
		level  int
		moveID int
		slug   string
	}
	var rows []pending
	for _, lm := range pokemon.Moves {
		for _, learn := range lm.Learnings {
			if learn.Method != "level-up" {
				continue
			}
			if learn.VersionGroup != bestGroup && learn.VersionGroupOrder != bestOrder {
				continue
			}
			if learn.VersionGroupOrder != bestOrder {
				continue
			}
			rows = append(rows, pending{level: learn.Level, moveID: lm.MoveID, slug: lm.Slug})
			break
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].level != rows[j].level {
			return rows[i].level < rows[j].level
		}
		return rows[i].slug < rows[j].slug
	})

	out := make([]domain.LevelUpMove, 0, len(rows))
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		move, err := getMove(row.moveID)
		if err != nil {
			move = domain.Move{
				ID:          row.moveID,
				Slug:        row.slug,
				DisplayName: output.TitleFromSlug(row.slug),
			}
		}
		if move.DisplayName == "" {
			move.DisplayName = output.TitleFromSlug(move.Slug)
		}
		out = append(out, domain.LevelUpMove{Level: row.level, Move: move})
	}
	return out, nil
}
