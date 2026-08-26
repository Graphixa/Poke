// Package dossierwarm pre-assembles species dossiers in the background so
// Enter/detail opens are usually disk hits. Selection (P0) and list neighbors
// (P1) preempt background warm-all (P2).
package dossierwarm

import (
	"container/heap"
	"context"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/domain"
)

const (
	PrioritySelected   = 0
	PriorityNeighbor   = 1
	PriorityBackground = 2
)

// Loader assembles or loads a dossier by species ID.
type Loader interface {
	LoadDossier(ctx context.Context, speciesID int) (domain.Dossier, error)
}

// ProgressFunc receives warm progress updates.
type ProgressFunc func(cache.Progress)

// Mode controls warm scope.
type Mode string

const (
	ModeAll    Mode = "all"
	ModeWindow Mode = "window"
	ModeOff    Mode = "off"
)

// Config configures a dossier warm Service.
type Config struct {
	Mode     Mode
	Workers  int
	Loader   Loader
	Index    []domain.SpeciesSummary
	Progress ProgressFunc
}

// Service owns the priority dossier warm queue.
type Service struct {
	cfg Config

	mu      sync.Mutex
	pq      priorityQueue
	seq     int64
	queued  map[int]bool
	done    map[int]bool
	wake    chan struct{}
	started atomic.Bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	doneN   atomic.Int64
	totalN  atomic.Int64
}

// New creates a dossier warm service. Call Start to begin workers.
func New(cfg Config) *Service {
	if cfg.Workers < 1 {
		cfg.Workers = runtime.GOMAXPROCS(0)
		if cfg.Workers < 1 {
			cfg.Workers = 2
		}
		if cfg.Workers > 4 {
			cfg.Workers = 4
		}
	}
	if cfg.Mode == "" {
		cfg.Mode = ModeAll
	}
	return &Service{
		cfg:    cfg,
		queued: make(map[int]bool),
		done:   make(map[int]bool),
		wake:   make(chan struct{}, 1),
	}
}

// Start launches worker goroutines.
func (s *Service) Start(ctx context.Context) {
	if s == nil || s.cfg.Mode == ModeOff || s.cfg.Loader == nil {
		return
	}
	if !s.started.CompareAndSwap(false, true) {
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	if s.cfg.Mode == ModeAll {
		s.enqueueAll(PriorityBackground)
	}

	for i := 0; i < s.cfg.Workers; i++ {
		s.wg.Add(1)
		go s.worker(runCtx)
	}
}

// Stop cancels workers and waits for them to exit.
func (s *Service) Stop() {
	if s == nil || s.cancel == nil {
		return
	}
	s.cancel()
	s.wg.Wait()
}

// Prioritize bumps a species to P0.
func (s *Service) Prioritize(speciesID int) {
	if s == nil || speciesID == 0 {
		return
	}
	s.enqueue(speciesID, PrioritySelected)
}

// SetNeighborsFromList enqueues neighbors around cursor at P1.
func (s *Service) SetNeighborsFromList(list []domain.SpeciesSummary, cursor, radius int) {
	if s == nil || len(list) == 0 {
		return
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(list) {
		cursor = len(list) - 1
	}
	if radius < 0 {
		radius = 0
	}
	lo := cursor - radius
	if lo < 0 {
		lo = 0
	}
	hi := cursor + radius
	if hi >= len(list) {
		hi = len(list) - 1
	}
	for i := lo; i <= hi; i++ {
		pri := PriorityNeighbor
		if i == cursor {
			pri = PrioritySelected
		}
		s.enqueue(list[i].ID, pri)
	}
}

func (s *Service) enqueueAll(priority int) {
	for _, sum := range s.cfg.Index {
		s.enqueue(sum.ID, priority)
	}
}

func (s *Service) enqueue(id, priority int) {
	if id == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done[id] {
		return
	}
	if s.queued[id] {
		return
	}
	s.queued[id] = true
	s.seq++
	heap.Push(&s.pq, &item{id: id, priority: priority, seq: s.seq})
	s.totalN.Add(1)
	s.signal()
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
		if ctx.Err() != nil {
			return
		}
		job := s.pop()
		if job == nil {
			select {
			case <-ctx.Done():
				return
			case <-s.wake:
			}
			continue
		}
		s.runJob(ctx, job.id)
	}
}

func (s *Service) pop() *item {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pq.Len() == 0 {
		return nil
	}
	it := heap.Pop(&s.pq).(*item)
	delete(s.queued, it.id)
	return it
}

func (s *Service) runJob(ctx context.Context, id int) {
	if ctx.Err() != nil {
		return
	}
	_, err := s.cfg.Loader.LoadDossier(ctx, id)
	if err != nil {
		debuglog.Cachef("dossier warm fail id=%d err=%v", id, err)
	} else {
		s.mu.Lock()
		s.done[id] = true
		s.mu.Unlock()
	}
	done := int(s.doneN.Add(1))
	total := int(s.totalN.Load())
	if s.cfg.Progress != nil {
		s.cfg.Progress(cache.Progress{
			Stage:   "dossier",
			Done:    done,
			Total:   total,
			Message: "dossier warm",
		})
	}
}

type item struct {
	id       int
	priority int
	seq      int64
}

type priorityQueue []*item

func (pq priorityQueue) Len() int { return len(pq) }
func (pq priorityQueue) Less(i, j int) bool {
	if pq[i].priority != pq[j].priority {
		return pq[i].priority < pq[j].priority
	}
	return pq[i].seq < pq[j].seq
}
func (pq priorityQueue) Swap(i, j int) { pq[i], pq[j] = pq[j], pq[i] }
func (pq *priorityQueue) Push(x any)   { *pq = append(*pq, x.(*item)) }
func (pq *priorityQueue) Pop() any {
	old := *pq
	n := len(old)
	it := old[n-1]
	old[n-1] = nil
	*pq = old[:n-1]
	return it
}
