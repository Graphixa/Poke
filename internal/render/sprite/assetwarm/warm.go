// Package assetwarm downloads named PNG sprite assets (pixel or official artwork)
// for the species index without requiring chafa artefact rendering.
package assetwarm

import (
	"container/heap"
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

const (
	PrioritySelected   = 0
	PriorityNeighbor   = 1
	PriorityBackground = 2
)

// PokemonLoader loads Pokémon detail (including sprite URLs) by species ID.
type PokemonLoader interface {
	Pokemon(ctx context.Context, id int) (domain.Pokemon, error)
}

// Resolver fetches and caches PNG bytes for a tier.
type Resolver interface {
	Resolve(ctx context.Context, pokemon domain.Pokemon, tier ...sprite.AssetTier) sprite.SourceResult
}

// Progress reports artwork/pixel download work. App classifies this into TUI/CLI.
type Progress struct {
	Done      int
	Total     int
	Message   string
	SpeciesID int
}

// Mode selects which asset family to download.
type Mode string

const (
	ModePixel   Mode = "pixel"
	ModeArtwork Mode = "artwork"
	ModeOff     Mode = "off"
)

// Config configures an asset warm Service.
type Config struct {
	Mode     Mode
	Workers  int
	Loader   PokemonLoader
	Resolver Resolver
	Index    []domain.SpeciesSummary
	// Progress must be safe for concurrent calls.
	Progress func(Progress)
	Message  string
}

// Service owns a priority download queue.
type Service struct {
	cfg Config

	mu      sync.Mutex
	pq      priorityQueue
	seq     int64
	queued  map[int]bool
	done    map[int]bool
	byID    map[int]domain.SpeciesSummary
	wake    chan struct{}
	started atomic.Bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	doneN   atomic.Int64
	totalN  atomic.Int64
	ready   map[int]bool // successfully stored artwork/pixel for species

	lastReport atomic.Int64 // unix nano of last Progress emit
}

// New creates an asset warm service. Call Start to begin workers.
func New(cfg Config) *Service {
	if cfg.Mode == "" {
		cfg.Mode = ModePixel
	}
	if cfg.Workers < 1 {
		cfg.Workers = runtime.GOMAXPROCS(0)
		if cfg.Workers < 1 {
			cfg.Workers = 2
		}
	}
	switch cfg.Mode {
	case ModeArtwork:
		cfg.Workers = ArtworkWorkers(cfg.Workers)
	case ModePixel:
		cfg.Workers = PixelWorkers(cfg.Workers)
	default:
		if cfg.Workers > 8 {
			cfg.Workers = 8
		}
	}
	if cfg.Message == "" {
		if cfg.Mode == ModeArtwork {
			cfg.Message = "Downloading official artwork"
		} else {
			cfg.Message = "Downloading sprites"
		}
	}
	byID := make(map[int]domain.SpeciesSummary, len(cfg.Index))
	for _, sum := range cfg.Index {
		byID[sum.ID] = sum
	}
	return &Service{
		cfg:    cfg,
		byID:   byID,
		queued: make(map[int]bool),
		done:   make(map[int]bool),
		ready:  make(map[int]bool),
		wake:   make(chan struct{}, 1),
	}
}

// Start launches workers and enqueues the full index.
func (s *Service) Start(ctx context.Context) {
	if s == nil || s.cfg.Mode == ModeOff || s.cfg.Resolver == nil {
		return
	}
	if len(s.cfg.Index) == 0 && s.cfg.Loader == nil {
		return
	}
	if !s.started.CompareAndSwap(false, true) {
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.enqueueAll(PriorityBackground)
	for i := 0; i < s.cfg.Workers; i++ {
		s.wg.Add(1)
		go s.worker(runCtx)
	}
}

// Stop cancels workers and waits for them to exit.
func (s *Service) Stop() {
	if s == nil || !s.started.Load() {
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

// Prioritize bumps the selected species to P0.
// Retries species that previously missed (done but not ready).
func (s *Service) Prioritize(speciesID int) {
	if s == nil || speciesID < 1 || s.cfg.Mode == ModeOff {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ready[speciesID] {
		return
	}
	if s.done[speciesID] {
		// Allow another attempt after a miss; progress count stays as-is.
		delete(s.done, speciesID)
		delete(s.queued, speciesID)
	}
	s.pushLocked(job{speciesID: speciesID, priority: PrioritySelected})
	s.signal()
}

// Ready reports whether a successful PNG was stored for speciesID.
func (s *Service) Ready(speciesID int) bool {
	if s == nil || speciesID < 1 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ready[speciesID]
}

// SetNeighbors enqueues ±radius neighbors at P1 (national dex index order).
func (s *Service) SetNeighbors(selectedID int, radius int) {
	if s == nil || s.cfg.Mode == ModeOff || radius < 1 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, sum := range s.cfg.Index {
		if sum.ID == selectedID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	lo := idx - radius
	if lo < 0 {
		lo = 0
	}
	hi := idx + radius
	if hi >= len(s.cfg.Index) {
		hi = len(s.cfg.Index) - 1
	}
	for i := lo; i <= hi; i++ {
		id := s.cfg.Index[i].ID
		if id == selectedID || s.done[id] {
			continue
		}
		s.pushLocked(job{speciesID: id, priority: PriorityNeighbor})
	}
	s.signal()
}

// SetNeighborsFromList enqueues ±radius neighbors from a filtered browse list.
func (s *Service) SetNeighborsFromList(list []domain.SpeciesSummary, cursor, radius int) {
	if s == nil || s.cfg.Mode == ModeOff || radius < 1 || len(list) == 0 {
		return
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(list) {
		cursor = len(list) - 1
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	lo := cursor - radius
	if lo < 0 {
		lo = 0
	}
	hi := cursor + radius
	if hi >= len(list) {
		hi = len(list) - 1
	}
	selectedID := list[cursor].ID
	for i := lo; i <= hi; i++ {
		id := list[i].ID
		if id == selectedID || s.done[id] {
			continue
		}
		s.pushLocked(job{speciesID: id, priority: PriorityNeighbor})
	}
	s.signal()
}

// Wait blocks until all queued work is done or ctx is cancelled.
func (s *Service) Wait(ctx context.Context) error {
	if s == nil {
		return nil
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		done, total := s.Snapshot()
		if total > 0 && done >= total {
			return nil
		}
		if total == 0 && len(s.cfg.Index) == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// Snapshot returns current done/total counts.
func (s *Service) Snapshot() (done, total int) {
	if s == nil {
		return 0, 0
	}
	return int(s.doneN.Load()), int(s.totalN.Load())
}

func (s *Service) enqueueAll(priority int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sum := range s.cfg.Index {
		if s.done[sum.ID] || s.queued[sum.ID] {
			continue
		}
		s.pushLocked(job{speciesID: sum.ID, priority: priority})
	}
	s.totalN.Store(int64(len(s.cfg.Index)))
	s.signal()
}

func (s *Service) pushLocked(j job) {
	if j.speciesID < 1 {
		return
	}
	s.seq++
	j.seq = s.seq
	heap.Push(&s.pq, j)
	s.queued[j.speciesID] = true
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
		j, ok := s.pop()
		if !ok {
			select {
			case <-ctx.Done():
				return
			case <-s.wake:
			}
			continue
		}
		s.runJob(ctx, j)
	}
}

func (s *Service) pop() (job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for s.pq.Len() > 0 {
		j := heap.Pop(&s.pq).(job)
		if s.done[j.speciesID] {
			delete(s.queued, j.speciesID)
			continue
		}
		delete(s.queued, j.speciesID)
		return j, true
	}
	return job{}, false
}

func (s *Service) runJob(ctx context.Context, j job) {
	poke, ok := s.pokemonForJob(ctx, j.speciesID)
	if !ok {
		debuglog.Renderf("assetwarm skip species=%d (no pokemon)", j.speciesID)
		s.markDone(j.speciesID)
		s.report(true, j.speciesID)
		return
	}
	tier := sprite.AssetTierPixel
	if s.cfg.Mode == ModeArtwork {
		tier = sprite.AssetTierHD
	}
	if err := ctx.Err(); err != nil {
		return
	}
	res := s.cfg.Resolver.Resolve(ctx, poke, tier)
	if res.UsePlaceholder || len(res.PNG) == 0 {
		debuglog.Renderf("assetwarm miss species=%d mode=%s", j.speciesID, s.cfg.Mode)
		// Do not mark ready — Prioritize can retry. Still count as done for progress.
		s.markDone(j.speciesID)
		s.report(false, j.speciesID)
		return
	}
	s.markReady(j.speciesID)
	s.markDone(j.speciesID)
	s.report(false, j.speciesID)
}

func (s *Service) markReady(speciesID int) {
	s.mu.Lock()
	s.ready[speciesID] = true
	s.mu.Unlock()
}

// pokemonForJob builds a CDN-URL pokemon from the index (no JSON) when possible.
func (s *Service) pokemonForJob(ctx context.Context, speciesID int) (domain.Pokemon, bool) {
	if sum, ok := s.byID[speciesID]; ok {
		return PokemonForCDN(sum, s.cfg.Mode), true
	}
	if s.cfg.Loader == nil {
		return domain.Pokemon{}, false
	}
	poke, err := s.cfg.Loader.Pokemon(ctx, speciesID)
	if err != nil {
		return domain.Pokemon{}, false
	}
	return poke, true
}

// PokemonForCDN synthesizes sprite URLs from the PokeAPI CDN using national dex ID.
func PokemonForCDN(sum domain.SpeciesSummary, mode Mode) domain.Pokemon {
	p := domain.Pokemon{
		ID:          sum.ID,
		Slug:        sum.Slug,
		DisplayName: sum.DisplayName,
	}
	if mode == ModeArtwork {
		p.Sprites.OfficialArtwork = sprite.CDNArtworkURL(sum.ID)
	} else {
		p.Sprites.FrontDefault = sprite.CDNPixelURL(sum.ID)
	}
	return p
}

func (s *Service) markDone(speciesID int) {
	s.mu.Lock()
	if !s.done[speciesID] {
		s.done[speciesID] = true
		s.doneN.Add(1)
	}
	s.mu.Unlock()
}

const progressMinInterval = 100 * time.Millisecond

func (s *Service) report(force bool, speciesID int) {
	if s.cfg.Progress == nil {
		return
	}
	done := int(s.doneN.Load())
	total := int(s.totalN.Load())
	// Always emit when a specific species finished so browse can pop in art.
	// Aggregate-only ticks stay throttled.
	if !force && speciesID == 0 && done < total {
		now := time.Now().UnixNano()
		prev := s.lastReport.Load()
		if prev != 0 && time.Duration(now-prev) < progressMinInterval && done%25 != 0 {
			return
		}
		if !s.lastReport.CompareAndSwap(prev, now) {
			if done < total {
				return
			}
		}
	} else {
		s.lastReport.Store(time.Now().UnixNano())
	}
	s.cfg.Progress(Progress{
		Done:      done,
		Total:     total,
		Message:   s.cfg.Message,
		SpeciesID: speciesID,
	})
}

type job struct {
	speciesID int
	priority  int
	seq       int64
}

type priorityQueue []job

func (pq priorityQueue) Len() int { return len(pq) }
func (pq priorityQueue) Less(i, j int) bool {
	if pq[i].priority != pq[j].priority {
		return pq[i].priority < pq[j].priority
	}
	return pq[i].seq < pq[j].seq
}
func (pq priorityQueue) Swap(i, j int) { pq[i], pq[j] = pq[j], pq[i] }
func (pq *priorityQueue) Push(x any)   { *pq = append(*pq, x.(job)) }
func (pq *priorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[:n-1]
	return item
}
