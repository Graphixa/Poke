package sprite

import (
	"context"
	"sync"

	"github.com/Graphixa/Pokedex/internal/debuglog"
)

// nativePersistQueueCap bounds async gzip writes: at most one active plus one
// pending foreground cold render may finish while another persists.
const nativePersistQueueCap = 2

type persistJob struct {
	key   string
	frame string
}

type persistQueue struct {
	mu     sync.Mutex
	ch     chan persistJob
	once   sync.Once
	closed bool
	wg     sync.WaitGroup
	store  func(ctx context.Context, key, frame string) error
}

func (q *persistQueue) start(store func(ctx context.Context, key, frame string) error) {
	if q == nil || store == nil {
		return
	}
	q.store = store
	q.once.Do(func() {
		q.ch = make(chan persistJob, nativePersistQueueCap)
		q.wg.Add(1)
		go func() {
			defer q.wg.Done()
			for job := range q.ch {
				if err := q.store(context.Background(), job.key, job.frame); err != nil {
					debuglog.Renderf("persist.native err=%v key=%s", err, job.key)
				}
			}
		}()
	})
}

func (q *persistQueue) enqueue(key, frame string) {
	if q == nil || key == "" || frame == "" {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed || q.ch == nil {
		return
	}
	job := persistJob{key: key, frame: frame}
	select {
	case q.ch <- job:
		return
	default:
	}
	// Queue full: drop oldest pending write, keep newest (latest selection wins).
	select {
	case <-q.ch:
	default:
	}
	select {
	case q.ch <- job:
	default:
	}
}

func (q *persistQueue) close() {
	if q == nil {
		return
	}
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	q.closed = true
	ch := q.ch
	q.mu.Unlock()
	if ch != nil {
		close(ch)
	}
	q.wg.Wait()
}

func (e *Engine) nativePersist() *persistQueue {
	if e == nil {
		return nil
	}
	e.persistOnce.Do(func() {
		e.persistQ = &persistQueue{}
		if e.Artefacts != nil {
			art := e.Artefacts
			e.persistQ.start(func(ctx context.Context, key, frame string) error {
				return art.StoreNative(ctx, key, frame)
			})
		}
	})
	return e.persistQ
}

// PersistFrame enqueues a native frame gzip write. Returns immediately.
func (e *Engine) PersistFrame(_ context.Context, key, frame string) error {
	if e == nil || key == "" || frame == "" {
		return nil
	}
	e.nativePersist().enqueue(key, frame)
	return nil
}

// ClosePersist drains the persistence worker (best-effort on shutdown).
func (e *Engine) ClosePersist() {
	if e == nil {
		return
	}
	if q := e.persistQ; q != nil {
		q.close()
	}
}
