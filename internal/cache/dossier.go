package cache

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/output"
)

// DossierCoreResult is the fast Enter path: species+pokemon, optionally complete.
type DossierCoreResult struct {
	Dossier  domain.Dossier
	Complete bool // true when evolution, encounters, and moves are included
}

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

// DossierCore loads species+pokemon quickly for the dialog Summary tab.
// When Complete is true the dossier already includes extras (disk hit or migrate).
func (s *Service) DossierCore(ctx context.Context, speciesID int, slug string) (DossierCoreResult, error) {
	v, err := s.group.Do(fmt.Sprintf("dossier-core:%d", speciesID), func() (any, error) {
		return s.getDossierCore(ctx, speciesID, slug)
	})
	if err != nil {
		return DossierCoreResult{}, err
	}
	return v.(DossierCoreResult), nil
}

// DossierExtras fills evolution, encounters, and level-up moves for a core dossier.
func (s *Service) DossierExtras(ctx context.Context, speciesID int, slug string, core domain.Dossier) (domain.Dossier, error) {
	v, err := s.group.Do(fmt.Sprintf("dossier-extras:%d", speciesID), func() (any, error) {
		return s.getDossierExtras(ctx, speciesID, slug, core)
	})
	if err != nil {
		return domain.Dossier{}, err
	}
	return v.(domain.Dossier), nil
}

func (s *Service) getDossier(ctx context.Context, speciesID int, slug string) (domain.Dossier, error) {
	core, err := s.getDossierCore(ctx, speciesID, slug)
	if err != nil {
		return domain.Dossier{}, err
	}
	if core.Complete {
		return core.Dossier, nil
	}
	return s.getDossierExtras(ctx, speciesID, slug, core.Dossier)
}

func (s *Service) getDossierCore(ctx context.Context, speciesID int, slug string) (DossierCoreResult, error) {
	cached, _, cacheErr := s.store.Dossier(ctx, speciesID, slug)
	haveCache := cacheErr == nil
	if haveCache {
		debuglog.Cachef("detail.cache.hit dossier id=%d", speciesID)
		return DossierCoreResult{Dossier: cached, Complete: dossierComplete(cached)}, nil
	}
	if cacheErr != nil && !errors.Is(cacheErr, ErrNotFound) && !errors.Is(cacheErr, ErrCorrupt) {
		return DossierCoreResult{}, cacheErr
	}

	if !haveCache {
		if migrated, ok := s.store.TryMigrateDossier(ctx, speciesID); ok {
			debuglog.Cachef("migrate dossier id=%d", speciesID)
			if slugOr(migrated, slug) != "" {
				_ = s.store.StoreDossier(ctx, migrated, s.clock.Now())
			}
			return DossierCoreResult{
				Dossier:  migrated,
				Complete: dossierComplete(migrated),
			}, nil
		}
	}

	if haveCache {
		// Stale-serve core immediately; extras refresh separately.
		return DossierCoreResult{Dossier: cached, Complete: dossierComplete(cached)}, nil
	}
	if s.localOnly.Load() {
		d, err := s.assembleCore(ctx, speciesID)
		if err != nil {
			return DossierCoreResult{}, err
		}
		complete := dossierComplete(d)
		if !complete {
			extras, err := s.assembleExtras(ctx, d)
			if err == nil {
				d = extras
				complete = dossierComplete(d)
			}
		}
		return DossierCoreResult{Dossier: d, Complete: complete}, nil
	}

	d, err := s.assembleCore(ctx, speciesID)
	if err != nil {
		return DossierCoreResult{}, err
	}
	return DossierCoreResult{Dossier: d, Complete: false}, nil
}

func (s *Service) getDossierExtras(ctx context.Context, speciesID int, slug string, core domain.Dossier) (domain.Dossier, error) {
	if dossierComplete(core) {
		return core, nil
	}

	cached, _, cacheErr := s.store.Dossier(ctx, speciesID, slug)
	haveCache := cacheErr == nil
	if haveCache && dossierComplete(cached) {
		debuglog.Cachef("hit dossier extras id=%d", speciesID)
		return cached, nil
	}
	if cacheErr != nil && !errors.Is(cacheErr, ErrNotFound) && !errors.Is(cacheErr, ErrCorrupt) {
		return domain.Dossier{}, cacheErr
	}

	d := core
	if haveCache && dossierComplete(cached) {
		return cached, nil
	}
	if s.localOnly.Load() {
		fresh, fetchErr := s.assembleExtras(ctx, d)
		if fetchErr != nil {
			if haveCache {
				return cached, nil
			}
			return d, fetchErr
		}
		return fresh, nil
	}

	fresh, fetchErr := s.assembleExtras(ctx, d)
	if fetchErr != nil {
		if haveCache {
			debuglog.Cachef("stale-serve dossier extras id=%d err=%v", speciesID, fetchErr)
			return cached, nil
		}
		return domain.Dossier{}, fetchErr
	}

	now := s.clock.Now()
	_ = s.store.StoreDossier(ctx, fresh, now)
	s.storeLegacySplits(fresh, now)
	debuglog.Cachef("store dossier id=%d", speciesID)
	return fresh, nil
}

func dossierComplete(d domain.Dossier) bool {
	movesDone := len(d.LevelUpMoves) > 0 || !pokemonHasLevelUpMoves(d.Pokemon)
	evoDone := d.Evolution != nil || d.Species.EvolutionChainID == 0
	encDone := d.Encounters != nil
	return movesDone && evoDone && encDone
}

func pokemonHasLevelUpMoves(p domain.Pokemon) bool {
	for _, lm := range p.Moves {
		for _, learn := range lm.Learnings {
			if learn.Method == "level-up" {
				return true
			}
		}
	}
	return false
}

func (s *Service) storeLegacySplits(fresh domain.Dossier, now time.Time) {
	ctx := context.Background()
	_ = s.store.StoreSpecies(ctx, fresh.Species, now)
	_ = s.store.StorePokemon(ctx, fresh.Pokemon, now)
	if fresh.Evolution != nil && fresh.Species.EvolutionChainID != 0 {
		_ = s.store.StoreEvolutionChain(ctx, fresh.Species.EvolutionChainID, *fresh.Evolution, now)
	}
	_ = s.store.StoreEncounters(ctx, fresh.Pokemon.ID, fresh.Encounters, now)
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

func (s *Service) assembleCore(ctx context.Context, speciesID int) (domain.Dossier, error) {
	var (
		species    domain.Species
		pokemon    domain.Pokemon
		speciesErr error
		pokemonErr error
		wg         sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		species, speciesErr = s.Species(ctx, speciesID)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		pokemon, pokemonErr = s.Pokemon(ctx, speciesID)
	}()
	wg.Wait()
	if speciesErr != nil {
		return domain.Dossier{}, speciesErr
	}
	pokemonID := species.DefaultPokemonID
	if pokemonID == 0 {
		pokemonID = speciesID
	}
	if pokemonErr != nil || pokemon.ID != pokemonID {
		var err error
		pokemon, err = s.Pokemon(ctx, pokemonID)
		if err != nil {
			return domain.Dossier{}, err
		}
	}
	if species.DisplayName != "" {
		pokemon.DisplayName = species.DisplayName
	}
	return domain.Dossier{Pokemon: pokemon, Species: species}, nil
}

func (s *Service) assembleExtras(ctx context.Context, d domain.Dossier) (domain.Dossier, error) {
	species := d.Species
	pokemon := d.Pokemon
	pokemonID := pokemon.ID
	if pokemonID == 0 {
		pokemonID = species.DefaultPokemonID
	}
	if pokemonID == 0 {
		pokemonID = species.ID
	}

	var (
		wg         sync.WaitGroup
		evoTree    domain.EvolutionTree
		evoOK      bool
		encounters []domain.Encounter
		encOK      bool
		moves      []domain.LevelUpMove
		movesOK    bool
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
	wg.Add(1)
	go func() {
		defer wg.Done()
		rows, err := resolveLevelUpMoves(ctx, pokemon, func(id int) (domain.Move, error) {
			return s.Move(ctx, id)
		}, s.moveWorkers())
		if err == nil {
			moves = rows
			movesOK = true
		}
	}()
	wg.Wait()

	if evoOK {
		d.Evolution = &evoTree
	}
	if encOK {
		d.Encounters = encounters
	}
	if movesOK {
		d.LevelUpMoves = moves
	}
	return d, nil
}

func (s *Service) assembleDossier(ctx context.Context, speciesID int) (domain.Dossier, error) {
	d, err := s.assembleCore(ctx, speciesID)
	if err != nil {
		return domain.Dossier{}, err
	}
	return s.assembleExtras(ctx, d)
}

func (s *Service) moveWorkers() int {
	n := s.maxConcurrency - interactiveReserve
	if n < 2 {
		n = 2
	}
	if n > 8 {
		n = 8
	}
	return n
}

func (s *Service) ensureLevelUpMoves(ctx context.Context, d domain.Dossier) domain.Dossier {
	if len(d.LevelUpMoves) > 0 {
		return d
	}
	rows, err := resolveLevelUpMoves(ctx, d.Pokemon, func(id int) (domain.Move, error) {
		return s.Move(ctx, id)
	}, s.moveWorkers())
	if err != nil {
		return d
	}
	d.LevelUpMoves = rows
	return d
}

func resolveLevelUpMoves(ctx context.Context, pokemon domain.Pokemon, getMove func(int) (domain.Move, error), workers int) ([]domain.LevelUpMove, error) {
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

	if workers < 1 {
		workers = 1
	}
	if workers > len(rows) && len(rows) > 0 {
		workers = len(rows)
	}

	out := make([]domain.LevelUpMove, len(rows))
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex

	for i, row := range rows {
		if err := ctx.Err(); err != nil {
			return out[:0], err
		}
		wg.Add(1)
		go func(i int, row pending) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := ctx.Err(); err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
				return
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
			out[i] = domain.LevelUpMove{Level: row.level, Move: move}
		}(i, row)
	}
	wg.Wait()
	if firstErr != nil {
		return out, firstErr
	}
	// Trim trailing zero entries if any goroutine was cancelled early.
	trim := len(out)
	for trim > 0 && out[trim-1].Move.Slug == "" && out[trim-1].Move.ID == 0 {
		trim--
	}
	return out[:trim], nil
}
