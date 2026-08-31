// Package nativewarm opportunistically pre-encodes native frames for the
// active terminal bucket (24/32/40) while the user browses.
package nativewarm

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

const maxWarmQueue = 16

// PokemonLoader loads Pokémon detail by species ID.
type PokemonLoader interface {
	Pokemon(ctx context.Context, id int) (domain.Pokemon, error)
}

// NativeRenderer encodes native graphics frames into the artefact cache.
type NativeRenderer interface {
	HasNativeCache(ctx context.Context, pokemon domain.Pokemon, cells int) bool
	NativeWarmRender(ctx context.Context, pokemon domain.Pokemon, cells int) sprite.RenderResult
}

// Config configures the opportunistic native warm service.
type Config struct {
	Loader   PokemonLoader
	Renderer NativeRenderer
	Index    []domain.SpeciesSummary
	// OnlyIDs when non-empty restricts warm targets (poke update tooling).
	OnlyIDs map[int]struct{}
}

// Service owns a single-lane background native encoder for the active bucket.
type Service struct {
	cfg Config

	mu          sync.Mutex
	cells       int
	list        []domain.SpeciesSummary
	cursor      int
	neighborRad int
	queue       []int
	queued      map[int]bool
	indexByID   map[int]domain.SpeciesSummary

	wake      chan struct{}
	started   atomic.Bool
	cancel    context.CancelFunc
	workerCtx context.Context
	wg        sync.WaitGroup
	activity  sprite.ActivityGate
}

// New creates a native warm service. Call EnsureStarted when browse layout is known.
func New(cfg Config) *Service {
	indexByID := make(map[int]domain.SpeciesSummary, len(cfg.Index))
	for _, sum := range cfg.Index {
		if sum.ID > 0 {
			indexByID[sum.ID] = sum
		}
	}
	return &Service{
		cfg:       cfg,
		queued:    make(map[int]bool),
		indexByID: indexByID,
		wake:      make(chan struct{}, 1),
	}
}

// Start launches the worker using cells as the initial bucket.
func (s *Service) Start(ctx context.Context) {
	if s == nil {
		return
	}
	s.EnsureStarted(ctx, sprite.MaxFrameSize)
}

// EnsureStarted starts the single background worker when needed.
func (s *Service) EnsureStarted(ctx context.Context, cells int) {
	if s == nil || s.cfg.Renderer == nil || s.cfg.Loader == nil {
		return
	}
	s.SetCells(cells)
	if s.started.Load() {
		return
	}
	if !s.started.CompareAndSwap(false, true) {
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.workerCtx = runCtx
	s.wg.Add(1)
	go s.worker(runCtx)
}

// Stop cancels the worker and waits for exit.
func (s *Service) Stop() {
	if s == nil || !s.started.Load() {
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

// SetCells switches the active bucket; future work targets the new size only.
func (s *Service) SetCells(cells int) {
	if s == nil {
		return
	}
	cells = sprite.ClampFrameSide(cells)
	s.mu.Lock()
	if s.cells != cells {
		s.cells = cells
		s.queue = nil
		s.queued = make(map[int]bool)
	}
	s.mu.Unlock()
	s.signal()
}

// ActiveCells returns the current warm bucket (for tests).
func (s *Service) ActiveCells() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cells
}

// Prioritize bumps speciesID to the front of the warm queue.
func (s *Service) Prioritize(speciesID int) {
	if s == nil || speciesID < 1 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.shouldWarm(speciesID) {
		return
	}
	s.enqueueFrontLocked(speciesID)
	s.signal()
}

// NoteActivity is retained for interface compatibility; opportunistic warming
// does not throttle on scroll activity.
func (s *Service) NoteActivity() {}

// SetForegroundBusy pauses background encode while foreground cold work runs.
func (s *Service) SetForegroundBusy(busy bool) {
	if s == nil {
		return
	}
	s.activity.SetForegroundBusy(busy)
	s.signal()
}

// SetNeighborsFromList rebuilds the warm queue from list neighbors at the active bucket.
// Order: N+1..max, then N-1..1.
func (s *Service) SetNeighborsFromList(list []domain.SpeciesSummary, cursor, radius int) {
	if s == nil || radius < 1 || len(list) == 0 {
		return
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(list) {
		cursor = len(list) - 1
	}
	s.mu.Lock()
	s.list = list
	s.cursor = cursor
	s.neighborRad = radius
	s.rebuildQueueLocked()
	s.mu.Unlock()
	s.signal()
}

// Snapshot returns 0,0 — full-dex completion progress was removed.
func (s *Service) Snapshot() (done, total int) {
	return 0, 0
}

func (s *Service) shouldWarm(speciesID int) bool {
	if speciesID < 1 {
		return false
	}
	if len(s.cfg.OnlyIDs) == 0 {
		return true
	}
	_, ok := s.cfg.OnlyIDs[speciesID]
	return ok
}

func (s *Service) rebuildQueueLocked() {
	if len(s.list) == 0 {
		return
	}
	order := s.neighborOrderLocked()
	s.queue = nil
	s.queued = make(map[int]bool)
	for _, id := range order {
		s.enqueueBackLocked(id)
		if len(s.queue) >= maxWarmQueue {
			break
		}
	}
}

func (s *Service) neighborOrderLocked() []int {
	list := s.list
	cursor := s.cursor
	radius := s.neighborRad
	if radius < 1 {
		radius = 1
	}
	var ids []int
	for d := 1; d <= radius; d++ {
		if i := cursor + d; i < len(list) {
			ids = append(ids, list[i].ID)
		}
	}
	for d := 1; d <= radius; d++ {
		if i := cursor - d; i >= 0 {
			ids = append(ids, list[i].ID)
		}
	}
	return ids
}

func (s *Service) enqueueFrontLocked(speciesID int) {
	if !s.shouldWarm(speciesID) {
		return
	}
	if s.queued[speciesID] {
		for i, id := range s.queue {
			if id == speciesID {
				copy(s.queue[1:i+1], s.queue[0:i])
				s.queue[0] = speciesID
				return
			}
		}
	}
	if len(s.queue) >= maxWarmQueue {
		oldest := s.queue[len(s.queue)-1]
		delete(s.queued, oldest)
		s.queue = s.queue[:len(s.queue)-1]
	}
	s.queue = append([]int{speciesID}, s.queue...)
	s.queued[speciesID] = true
}

func (s *Service) enqueueBackLocked(speciesID int) {
	if !s.shouldWarm(speciesID) || s.queued[speciesID] {
		return
	}
	if len(s.queue) >= maxWarmQueue {
		return
	}
	s.queue = append(s.queue, speciesID)
	s.queued[speciesID] = true
}

func (s *Service) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Service) worker(ctx context.Context) {
	defer s.wg.Done()
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		if s.activity.ForegroundBusy() {
			if !s.wait(ctx) {
				return
			}
			continue
		}
		id, ok := s.popNext()
		if !ok {
			if !s.wait(ctx) {
				return
			}
			continue
		}
		s.warmOne(ctx, id)
	}
}

func (s *Service) wait(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-s.wake:
		return true
	case <-time.After(250 * time.Millisecond):
		return true
	}
}

func (s *Service) popNext() (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for len(s.queue) > 0 {
		id := s.queue[0]
		s.queue = s.queue[1:]
		delete(s.queued, id)
		return id, true
	}
	return 0, false
}

func (s *Service) warmOne(ctx context.Context, speciesID int) {
	cells := s.ActiveCells()
	if cells < sprite.MinFrameSize {
		return
	}
	poke, err := s.cfg.Loader.Pokemon(ctx, speciesID)
	if err != nil {
		debuglog.Renderf("nativewarm skip species=%d err=%v", speciesID, err)
		return
	}
	if sum, ok := s.indexByID[speciesID]; ok && sum.Slug != "" {
		poke.Slug = sum.Slug
	}
	if poke.ID == 0 {
		poke.ID = speciesID
	}
	if err := ctx.Err(); err != nil {
		return
	}
	if s.cfg.Renderer.HasNativeCache(ctx, poke, cells) {
		return
	}
	res := s.cfg.Renderer.NativeWarmRender(ctx, poke, cells)
	if res.UsePlaceholder || !res.Native {
		debuglog.Renderf("nativewarm miss species=%d backend=%s", speciesID, res.Backend)
	}
}
