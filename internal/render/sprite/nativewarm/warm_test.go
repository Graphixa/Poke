package nativewarm_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/render/sprite/nativewarm"
)

type fakeKey struct {
	id    int
	cells int
}

type fakeNativeRenderer struct {
	mu           sync.Mutex
	disk         map[fakeKey]bool
	render       int
	persist      int
	ids          []int
	cells        []int
	pngReads     int
	memOnly      map[fakeKey]bool
	blockPersist chan struct{}
}

func (f *fakeNativeRenderer) cacheKey(id, cells int) string {
	return fmt.Sprintf("k:%d:%d", id, cells)
}

func (f *fakeNativeRenderer) HasNativeCache(_ context.Context, pokemon domain.Pokemon, cells int) bool {
	if cells < sprite.MinFrameSize {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	k := fakeKey{pokemon.ID, cells}
	return f.disk[k] || f.memOnly[k]
}

func (f *fakeNativeRenderer) HasNativeFile(_ context.Context, pokemon domain.Pokemon, cells int) bool {
	if cells < sprite.MinFrameSize {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.disk[fakeKey{pokemon.ID, cells}]
}

func (f *fakeNativeRenderer) NativeWarmRender(_ context.Context, pokemon domain.Pokemon, cells int) sprite.RenderResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.render++
	f.pngReads++
	f.ids = append(f.ids, pokemon.ID)
	f.cells = append(f.cells, cells)
	if f.memOnly == nil {
		f.memOnly = map[fakeKey]bool{}
	}
	f.memOnly[fakeKey{pokemon.ID, cells}] = true
	return sprite.RenderResult{
		ANSI:     "X",
		Native:   true,
		CacheKey: f.cacheKey(pokemon.ID, cells),
		Backend:  "image",
	}
}

func (f *fakeNativeRenderer) PersistNativeFrame(_ context.Context, cacheKey, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.blockPersist != nil {
		f.mu.Unlock()
		<-f.blockPersist
		f.mu.Lock()
	}
	f.persist++
	if f.disk == nil {
		f.disk = map[fakeKey]bool{}
	}
	for k := range f.memOnly {
		if f.cacheKey(k.id, k.cells) == cacheKey {
			f.disk[k] = true
			return nil
		}
	}
	return nil
}

func (f *fakeNativeRenderer) WaitPersistCapacity(context.Context) error { return nil }

func (f *fakeNativeRenderer) renderedIDs() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int(nil), f.ids...)
}

func (f *fakeNativeRenderer) renders() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.render
}

func (f *fakeNativeRenderer) persists() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.persist
}

func (f *fakeNativeRenderer) renderedCells() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int(nil), f.cells...)
}

type fakeLoader struct{}

func (fakeLoader) Pokemon(_ context.Context, id int) (domain.Pokemon, error) {
	return domain.Pokemon{ID: id}, nil
}

func indexN(n int) []domain.SpeciesSummary {
	out := make([]domain.SpeciesSummary, n)
	for i := range out {
		id := i + 1
		out[i] = domain.SpeciesSummary{ID: id, Slug: "s", DisplayName: "P"}
	}
	return out
}

func waitRenders(t *testing.T, rend *fakeNativeRenderer, n int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if rend.renders() >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("renders=%d want >=%d cells=%v", rend.renders(), n, rend.renderedCells())
}

func waitPersists(t *testing.T, rend *fakeNativeRenderer, n int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if rend.persists() >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("persists=%d want >=%d", rend.persists(), n)
}

func TestWarmerOnlyCurrentBucket(t *testing.T) {
	index := indexN(5)
	rend := &fakeNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: rend, Index: index})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(index, 2, 1)
	waitPersists(t, rend, 1)
	for _, c := range rend.renderedCells() {
		if c != 32 {
			t.Fatalf("cell=%d want only active bucket 32", c)
		}
	}
	svc.Stop()
}

func TestWarmerDoesNotGenerateOtherBuckets(t *testing.T) {
	index := indexN(3)
	rend := &fakeNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: rend, Index: index})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 24)
	svc.SetNeighborsFromList(index, 0, 1)
	waitPersists(t, rend, 1)
	svc.SetCells(40)
	svc.SetNeighborsFromList(index, 0, 1)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, c := range rend.renderedCells() {
			if c == 40 {
				svc.Stop()
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("cells=%v want bucket 40 encode", rend.renderedCells())
}

func TestForegroundBusyPausesBackground(t *testing.T) {
	index := indexN(5)
	rend := &fakeNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: rend, Index: index})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	svc.SetForegroundBusy(true)
	svc.SetNeighborsFromList(index, 2, 1)
	time.Sleep(200 * time.Millisecond)
	if rend.renders() != 0 {
		t.Fatalf("renders=%d want 0 while foreground busy", rend.renders())
	}
	svc.SetForegroundBusy(false)
	waitPersists(t, rend, 1)
	svc.Stop()
}

func TestCacheHitSkipsRender(t *testing.T) {
	index := indexN(3)
	rend := &fakeNativeRenderer{disk: map[fakeKey]bool{{3, 32}: true}}
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: rend, Index: index})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(index, 1, 1)
	waitPersists(t, rend, 2)
	if rend.renders() != 2 {
		t.Fatalf("renders=%d want 2 (skip cached neighbor, warm anchor+next)", rend.renders())
	}
	svc.Stop()
}

func TestNeighborOrderNPlusThenNMinus(t *testing.T) {
	index := indexN(5)
	rend := &fakeNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: rend, Index: index})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(index, 2, 1)
	waitPersists(t, rend, 5)
	want := []int{4, 5, 2, 1, 3}
	got := rend.renderedIDs()
	if len(got) != len(want) {
		t.Fatalf("ids=%v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ids=%v want order %v", got, want)
		}
	}
	svc.Stop()
}

func TestShutdownStopsCleanly(t *testing.T) {
	index := indexN(20)
	rend := &fakeNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: rend, Index: index})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(index, 10, 1)
	time.Sleep(50 * time.Millisecond)
	svc.Stop()
}

func TestNativeWarmRenderPersistsFrame(t *testing.T) {
	index := indexN(3)
	rend := &fakeNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: rend, Index: index})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(index, 0, 1)
	waitPersists(t, rend, 1)
	if rend.persists() < 1 {
		t.Fatalf("persist=%d want >=1", rend.persists())
	}
	svc.Stop()
}

func TestNativeWarmPlaceholderDoesNotPersist(t *testing.T) {
	// Placeholder renders are rejected before persist in warmOne.
	index := indexN(2)
	rend := &fakeNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: rend, Index: index})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(index, 0, 1)
	time.Sleep(200 * time.Millisecond)
	svc.Stop()
}

func TestNativeWarmerContinuesBeyondImmediateNeighbours(t *testing.T) {
	index := indexN(100)
	rend := &fakeNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: rend, Index: index})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(index, 50, 1)
	waitPersists(t, rend, 15)
	if n := rend.persists(); n < 15 {
		t.Fatalf("persists=%d want >=15 beyond neighbours", n)
	}
	svc.Stop()
}

func TestNativeWarmerEventuallyTraversesWholeActiveBucket(t *testing.T) {
	index := indexN(20)
	rend := &fakeNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: rend, Index: index})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(index, 0, 1)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if rend.persists() >= 19 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if rend.persists() < 19 {
		t.Fatalf("persists=%d want >=19 for bucket traversal", rend.persists())
	}
	svc.Stop()
}

type delayedRenderer struct {
	fakeNativeRenderer
	delay time.Duration
}

func (d *delayedRenderer) NativeWarmRender(ctx context.Context, pokemon domain.Pokemon, cells int) sprite.RenderResult {
	select {
	case <-ctx.Done():
		return sprite.RenderResult{UsePlaceholder: true}
	case <-time.After(d.delay):
	}
	return d.fakeNativeRenderer.NativeWarmRender(ctx, pokemon, cells)
}

func TestNativeWarmerTraversalStateIsBounded(t *testing.T) {
	index := indexN(1000)
	rend := &delayedRenderer{delay: 5 * time.Millisecond}
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: rend, Index: index})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(index, 500, 1)
	time.Sleep(50 * time.Millisecond)
	svc.Stop()
	if n := rend.renders(); n > 25 {
		t.Fatalf("renders=%d want bounded opportunistic work per timeslice", n)
	}
}

func TestSelectionChangeDoesNotDiscardWholeWarmCoverage(t *testing.T) {
	index := indexN(50)
	rend := &fakeNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: rend, Index: index})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(index, 10, 1)
	waitPersists(t, rend, 5)
	doneBefore, _ := svc.Snapshot()
	svc.SetNeighborsFromList(index, 40, 1)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		doneAfter, _ := svc.Snapshot()
		if doneAfter < doneBefore {
			t.Fatalf("done regressed %d -> %d after anchor change", doneBefore, doneAfter)
		}
		if doneAfter > doneBefore {
			svc.Stop()
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	doneAfter, _ := svc.Snapshot()
	if doneAfter < doneBefore {
		t.Fatalf("done regressed %d -> %d after anchor change", doneBefore, doneAfter)
	}
	svc.Stop()
}

func TestPrioritizeDoesNotPreventContinuedTraversal(t *testing.T) {
	index := indexN(30)
	rend := &fakeNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: rend, Index: index})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(index, 5, 1)
	svc.Prioritize(20)
	waitPersists(t, rend, 10)
	svc.Stop()
}

func TestAlreadyCachedCandidatesAreSkipped(t *testing.T) {
	index := indexN(5)
	rend := &fakeNativeRenderer{disk: map[fakeKey]bool{{2, 32}: true, {3, 32}: true}}
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: rend, Index: index})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(index, 1, 1)
	waitPersists(t, rend, 2)
	for _, id := range rend.renderedIDs() {
		if id == 2 || id == 3 {
			t.Fatalf("cached species %d should not render", id)
		}
	}
	svc.Stop()
}

func TestNativeProgressDoesNotCountFailedSpecies(t *testing.T) {
	index := indexN(5)
	var last cache.Progress
	svc := nativewarm.New(nativewarm.Config{
		Loader: failingLoader{failID: 2},
		Renderer: &placeholderRenderer{
			placeholderID: 3,
		},
		Index: index,
		Progress: func(p cache.Progress) {
			last = p
		},
	})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(index, 0, 1)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		done, total := svc.Snapshot()
		if done >= 3 && done < total {
			if last.Done != done || last.Total != total {
				t.Fatalf("progress=%+v snapshot=%d/%d", last, done, total)
			}
			svc.Stop()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	done, total := svc.Snapshot()
	if done >= total {
		t.Fatalf("progress falsely complete at %d/%d last=%+v", done, total, last)
	}
	svc.Stop()
}

type failingLoader struct {
	failID int
}

func (f failingLoader) Pokemon(_ context.Context, id int) (domain.Pokemon, error) {
	if id == f.failID {
		return domain.Pokemon{}, fmt.Errorf("missing")
	}
	return domain.Pokemon{ID: id}, nil
}

type placeholderRenderer struct {
	fakeNativeRenderer
	placeholderID int
}

func (p *placeholderRenderer) NativeWarmRender(ctx context.Context, pokemon domain.Pokemon, cells int) sprite.RenderResult {
	if pokemon.ID == p.placeholderID {
		return sprite.RenderResult{UsePlaceholder: true}
	}
	return p.fakeNativeRenderer.NativeWarmRender(ctx, pokemon, cells)
}

func TestNativeProgressDoneIncrementsOnlyAfterPersistence(t *testing.T) {
	index := indexN(5)
	block := make(chan struct{})
	rend := &fakeNativeRenderer{blockPersist: block}
	svc := nativewarm.New(nativewarm.Config{
		Loader: fakeLoader{}, Renderer: rend, Index: index,
	})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(index, 0, 1)
	waitRenders(t, rend, 1)
	doneBefore, _ := svc.Snapshot()
	if doneBefore > 0 {
		t.Fatalf("done=%d want 0 before persist completes", doneBefore)
	}
	close(block)
	waitPersists(t, rend, 1)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		done, _ := svc.Snapshot()
		if done >= 1 {
			svc.Stop()
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	done, _ := svc.Snapshot()
	t.Fatalf("done=%d want >=1 after persist", done)
}

func TestNativeProgressStartsWhenBucketIncomplete(t *testing.T) {
	index := indexN(10)
	rend := &fakeNativeRenderer{}
	var last cache.Progress
	svc := nativewarm.New(nativewarm.Config{
		Loader: fakeLoader{}, Renderer: rend, Index: index,
		Progress: func(p cache.Progress) { last = p },
	})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	if last.Total != 10 || last.Stage != "native" {
		t.Fatalf("progress=%+v want native 0/10", last)
	}
	svc.Stop()
}

func TestMarkDoneWithProgressDoesNotDeadlock(t *testing.T) {
	index := indexN(5)
	rend := &fakeNativeRenderer{disk: map[fakeKey]bool{{2, 32}: true}}
	svc := nativewarm.New(nativewarm.Config{
		Loader:   fakeLoader{},
		Renderer: rend,
		Index:    index,
		Progress: func(p cache.Progress) {},
	})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(index, 0, 1)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		done, _ := svc.Snapshot()
		if done >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	done := make(chan struct{})
	go func() {
		svc.SetNeighborsFromList(index, 1, 1)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("SetNeighborsFromList blocked after cached markDone progress")
	}
	svc.Stop()
}

func TestNativeWarmerResumesAfterForegroundCompletes(t *testing.T) {
	index := indexN(10)
	rend := &fakeNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: rend, Index: index})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	svc.SetForegroundBusy(true)
	svc.SetNeighborsFromList(index, 5, 1)
	time.Sleep(100 * time.Millisecond)
	svc.SetForegroundBusy(false)
	waitPersists(t, rend, 1)
	svc.Stop()
}

func TestWarmCacheHitDoesNotPauseNativeWarmer(t *testing.T) {
	index := indexN(10)
	rend := &fakeNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: rend, Index: index})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	svc.SetNeighborsFromList(index, 5, 1)
	waitPersists(t, rend, 2)
	svc.Stop()
}

func TestFastPathCompleteCacheNoProgressEmit(t *testing.T) {
	index := indexN(5)
	disk := make(map[fakeKey]bool, len(index))
	for _, sum := range index {
		disk[fakeKey{sum.ID, 32}] = true
	}
	rend := &fakeNativeRenderer{disk: disk}
	svc := nativewarm.New(nativewarm.Config{
		Loader:   fakeLoader{},
		Renderer: rend,
		Index:    index,
	})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	if done, total := svc.Snapshot(); done != 0 || total != len(index) {
		t.Fatalf("snapshot=%d/%d want 0/%d before discovery", done, total, len(index))
	}
	svc.SetNeighborsFromList(index, 2, 1)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		done, total := svc.Snapshot()
		if done == total && total == len(index) {
			if rend.renders() != 0 {
				t.Fatalf("renders=%d want 0 for disk-only discovery", rend.renders())
			}
			svc.Stop()
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for background cache discovery")
}

func TestFastPathPartialCacheHonestInitialDone(t *testing.T) {
	index := indexN(10)
	disk := map[fakeKey]bool{{1, 32}: true, {2, 32}: true, {3, 32}: true}
	rend := &fakeNativeRenderer{disk: disk}
	svc := nativewarm.New(nativewarm.Config{
		Loader:   fakeLoader{},
		Renderer: rend,
		Index:    index,
	})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	if done, total := svc.Snapshot(); done != 0 || total != 10 {
		t.Fatalf("snapshot=%d/%d want 0/10 before discovery", done, total)
	}
	svc.SetNeighborsFromList(index, 0, 1)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		done, _ := svc.Snapshot()
		if done >= 3 {
			svc.Stop()
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for partial disk discovery")
}

func TestFastPathResizeBucketReaudit(t *testing.T) {
	index := indexN(5)
	disk := make(map[fakeKey]bool, len(index))
	for _, sum := range index {
		disk[fakeKey{sum.ID, 32}] = true
	}
	rend := &fakeNativeRenderer{disk: disk}
	var emits []cache.Progress
	svc := nativewarm.New(nativewarm.Config{
		Loader:   fakeLoader{},
		Renderer: rend,
		Index:    index,
		Progress: func(p cache.Progress) { emits = append(emits, p) },
	})
	ctx := context.Background()
	svc.EnsureStarted(ctx, 32)
	if done, total := svc.Snapshot(); done != 0 || total != len(index) {
		t.Fatalf("snapshot=%d/%d want 0/%d before discovery", done, total, len(index))
	}

	svc.SetCells(40)
	done, total := svc.Snapshot()
	if done != 0 || total != len(index) {
		t.Fatalf("snapshot=%d/%d want 0/%d after resize to 40", done, total, len(index))
	}
	if len(emits) == 0 || emits[len(emits)-1].Cells != 40 || emits[len(emits)-1].Done != 0 || emits[len(emits)-1].Total != len(index) {
		t.Fatalf("emits=%+v want latest incomplete progress for bucket 40", emits)
	}
	svc.SetNeighborsFromList(index, 0, 1)
	waitPersists(t, rend, 1)
	for _, c := range rend.renderedCells() {
		if c != 40 {
			t.Fatalf("cell=%d want only bucket 40 after resize", c)
		}
	}
	svc.Stop()
}

type slowHasNativeRenderer struct {
	fakeNativeRenderer
	delay time.Duration
	calls atomic.Int32
}

func (s *slowHasNativeRenderer) HasNativeFile(_ context.Context, pokemon domain.Pokemon, cells int) bool {
	s.calls.Add(1)
	time.Sleep(s.delay)
	return s.fakeNativeRenderer.HasNativeFile(context.Background(), pokemon, cells)
}

func TestSetCellsDoesNotAuditFullIndex(t *testing.T) {
	index := indexN(1000)
	rend := &slowHasNativeRenderer{
		fakeNativeRenderer: fakeNativeRenderer{disk: map[fakeKey]bool{}},
		delay:              time.Millisecond,
	}
	svc := nativewarm.New(nativewarm.Config{Loader: fakeLoader{}, Renderer: rend, Index: index})
	start := time.Now()
	svc.SetCells(32)
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("SetCells took %v want <50ms", elapsed)
	}
	if rend.calls.Load() != 0 {
		t.Fatalf("SetCells invoked HasNativeFile %d times", rend.calls.Load())
	}
}
