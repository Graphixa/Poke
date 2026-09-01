package sprite

import (
	"context"
	"sync"
	"time"

	"github.com/Graphixa/Pokedex/internal/debuglog"
)

// nativePersistQueueCap bounds async gzip writes: at most one active plus one
// pending foreground cold render may finish while another persists.
const nativePersistQueueCap = 2

type persistJob struct {
	key   string
	frame string
	done  chan error
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
				err := q.store(context.Background(), job.key, job.frame)
				if err != nil {
					debuglog.Renderf("persist.native err=%v key=%s", err, job.key)
				}
				if job.done != nil {
					job.done <- err
					close(job.done)
				}
			}
		}()
	})
}

func (q *persistQueue) waitCapacity(ctx context.Context) error {
	if q == nil {
		return nil
	}
	for {
		q.mu.Lock()
		closed := q.closed
		ch := q.ch
		full := ch != nil && len(ch) >= nativePersistQueueCap
		q.mu.Unlock()
		if closed {
			return context.Canceled
		}
		if !full {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func (q *persistQueue) pending() int {
	if q == nil {
		return 0
	}
	q.mu.Lock()
	ch := q.ch
	q.mu.Unlock()
	if ch == nil {
		return 0
	}
	return len(ch)
}

func (q *persistQueue) submit(ctx context.Context, key, frame string, wait bool) error {
	if q == nil || key == "" || frame == "" {
		return nil
	}
	q.mu.Lock()
	if q.closed || q.ch == nil {
		q.mu.Unlock()
		return context.Canceled
	}
	ch := q.ch
	q.mu.Unlock()

	if err := q.waitCapacity(ctx); err != nil {
		return err
	}

	job := persistJob{key: key, frame: frame}
	if wait {
		job.done = make(chan error, 1)
	}

	select {
	case ch <- job:
	case <-ctx.Done():
		return ctx.Err()
	}

	if !wait {
		return nil
	}
	select {
	case err := <-job.done:
		return err
	case <-ctx.Done():
		return ctx.Err()
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

// PersistFrame enqueues a native frame gzip write. Returns immediately; the
// worker blocks when the queue is full rather than dropping accepted jobs.
func (e *Engine) PersistFrame(_ context.Context, key, frame string) error {
	if e == nil || key == "" || frame == "" {
		return nil
	}
	return e.nativePersist().submit(context.Background(), key, frame, false)
}

// PersistFrameWait enqueues a native frame and blocks until it is stored or ctx
// is cancelled. Used by background warming so progress reflects persistence.
func (e *Engine) PersistFrameWait(ctx context.Context, key, frame string) error {
	if e == nil || key == "" || frame == "" {
		return nil
	}
	return e.nativePersist().submit(ctx, key, frame, true)
}

// WaitPersistCapacity blocks until the persistence queue has space. Background
// encoders call this before starting work to apply backpressure.
func (e *Engine) WaitPersistCapacity(ctx context.Context) error {
	if e == nil {
		return nil
	}
	return e.nativePersist().waitCapacity(ctx)
}

// PersistQueuePending returns the number of frames waiting to be written.
func (e *Engine) PersistQueuePending() int {
	if e == nil {
		return 0
	}
	return e.nativePersist().pending()
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
