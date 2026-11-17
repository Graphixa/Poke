package sprite

import (
	"context"
	"sync"
	"time"

	"github.com/Graphixa/Poke/internal/debuglog"
)

// nativePersistQueueCap bounds async gzip writes: at most one active plus one
// pending foreground cold render may finish while another persists.
const nativePersistQueueCap = 2

// ponytail: persist queue stays native-only (gzip writes) while ANSI artefacts
// are stored inline in renderChain; unify by routing StoreArtefact through the
// queue once ANSI writes show up in profiles.
type persistJob struct {
	key   string
	frame string
	done  chan error
}

type persistQueue struct {
	mu       sync.Mutex
	ch       chan persistJob
	once     sync.Once
	closed   bool
	submitWG sync.WaitGroup
	wg       sync.WaitGroup
	store    func(ctx context.Context, key, frame string) error
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
		full := len(ch) >= nativePersistQueueCap
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
	q.submitWG.Add(1)
	q.mu.Unlock()
	defer q.submitWG.Done()

	if err := q.waitCapacity(ctx); err != nil {
		return err
	}

	job := persistJob{key: key, frame: frame}
	if wait {
		job.done = make(chan error, 1)
	}

	q.mu.Lock()
	if q.closed || q.ch == nil {
		q.mu.Unlock()
		return context.Canceled
	}
	ch := q.ch
	q.mu.Unlock()

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
	q.mu.Unlock()

	q.submitWG.Wait()

	q.mu.Lock()
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
	if err := e.persistCanceled(); err != nil {
		return nil
	}
	e.persistMu.Lock()
	defer e.persistMu.Unlock()
	if e.persistClosed {
		return e.persistQ
	}
	if e.persistQ == nil {
		q := &persistQueue{}
		if e.Artefacts != nil {
			art := e.Artefacts
			q.start(func(ctx context.Context, key, frame string) error {
				return art.StoreNative(ctx, key, frame)
			})
		}
		e.persistQ = q
	}
	return e.persistQ
}

// PersistFrame enqueues a native frame gzip write. Returns immediately; the
// worker blocks when the queue is full rather than dropping accepted jobs.
func (e *Engine) PersistFrame(_ context.Context, key, frame string) error {
	if e == nil || key == "" || frame == "" {
		return nil
	}
	if err := e.persistCanceled(); err != nil {
		return err
	}
	q := e.nativePersist()
	if q == nil {
		return context.Canceled
	}
	return q.submit(context.Background(), key, frame, false)
}

// PersistFrameWait enqueues a native frame and blocks until it is stored or ctx
// is cancelled. Used by background warming so progress reflects persistence.
func (e *Engine) PersistFrameWait(ctx context.Context, key, frame string) error {
	if e == nil || key == "" || frame == "" {
		return nil
	}
	if err := e.persistCanceled(); err != nil {
		return err
	}
	q := e.nativePersist()
	if q == nil {
		return context.Canceled
	}
	return q.submit(ctx, key, frame, true)
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
	e.persistMu.Lock()
	if e.persistClosed {
		e.persistMu.Unlock()
		return
	}
	e.persistClosed = true
	q := e.persistQ
	e.persistMu.Unlock()
	if q != nil {
		q.close()
	}
}

func (e *Engine) persistCanceled() error {
	e.persistMu.Lock()
	closed := e.persistClosed
	e.persistMu.Unlock()
	if closed {
		return context.Canceled
	}
	return nil
}
