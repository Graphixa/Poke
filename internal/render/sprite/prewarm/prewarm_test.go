package prewarm_test

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/render/sprite/prewarm"
)

type fakeKey struct{ id, cells int }

// fakeNative is an in-memory NativeRenderer: render -> mem, persist -> disk.
type fakeNative struct {
	mu       sync.Mutex
	disk     map[fakeKey]bool
	mem      map[fakeKey]bool
	ids      []int
	cells    []int
	persists int
	failID   map[int]bool // render as placeholder once
	block    chan struct{}
}

func (f *fakeNative) HasNativeFile(_ context.Context, p domain.Pokemon, cells int) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.disk[fakeKey{p.ID, cells}]
}

func (f *fakeNative) NativeWarmRender(_ context.Context, p domain.Pokemon, cells int) sprite.RenderResult {
	if f.block != nil {
		<-f.block
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ids = append(f.ids, p.ID)
	f.cells = append(f.cells, cells)
	if f.failID[p.ID] {
		delete(f.failID, p.ID)
		return sprite.RenderResult{UsePlaceholder: true}
	}
	if f.mem == nil {
		f.mem = map[fakeKey]bool{}
	}
	f.mem[fakeKey{p.ID, cells}] = true
	return sprite.RenderResult{ANSI: "X", Native: true, CacheKey: fmt.Sprintf("%d:%d", p.ID, cells), Backend: "image"}
}

func (f *fakeNative) PersistNativeFrame(_ context.Context, key, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.disk == nil {
		f.disk = map[fakeKey]bool{}
	}
	for k := range f.mem {
		if fmt.Sprintf("%d:%d", k.id, k.cells) == key {
			f.disk[k] = true
		}
	}
	f.persists++
	return nil
}

func (f *fakeNative) WaitPersistCapacity(context.Context) error { return nil }

func (f *fakeNative) rendered() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.ids)
}

func (f *fakeNative) persisted() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.persists
}

type fakeLoader struct{}

func (fakeLoader) Pokemon(_ context.Context, id int) (domain.Pokemon, error) {
	p := domain.Pokemon{ID: id}
	p.Sprites.FrontDefault = "http://x"
	return p, nil
}

func indexN(n int) []domain.SpeciesSummary {
	out := make([]domain.SpeciesSummary, n)
	for i := range out {
		out[i] = domain.SpeciesSummary{ID: i + 1, Slug: "s", DisplayName: "P"}
	}
	return out
}

func waitFor(t *testing.T, what string, pred func() bool) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if pred() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout: %s", what)
}

type recorder struct {
	mu  sync.Mutex
	all []prewarm.Progress
}

func (r *recorder) record(p prewarm.Progress) {
	r.mu.Lock()
	r.all = append(r.all, p)
	r.mu.Unlock()
}

func (r *recorder) snapshot() []prewarm.Progress {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.all)
}

func TestNativeHintsThenScanFromAnchor(t *testing.T) {
	index := indexN(5)
	rend := &fakeNative{}
	svc := prewarm.NewNative(index, fakeLoader{}, rend, nil)
	svc.EnsureStarted(context.Background(), 32)
	svc.SetNeighborsFromList(index, 2, 1)
	defer svc.Stop()
	waitFor(t, "5 persists", func() bool { return rend.persisted() >= 5 })

	got := rend.rendered()
	// Neighbour hints (cursor±1 → ids 4 and 2) must lead; order between them races.
	if len(got) < 2 || !((got[0] == 4 && got[1] == 2) || (got[0] == 2 && got[1] == 4)) {
		t.Fatalf("order=%v want neighbour hints {4,2} first", got)
	}
	slices.Sort(got)
	if !slices.Equal(got, []int{1, 2, 3, 4, 5}) {
		t.Fatalf("rendered=%v want each species once", got)
	}
}

func TestNativeCacheHitSkipsRender(t *testing.T) {
	index := indexN(3)
	rend := &fakeNative{disk: map[fakeKey]bool{{3, 32}: true}}
	svc := prewarm.NewNative(index, fakeLoader{}, rend, nil)
	svc.EnsureStarted(context.Background(), 32)
	svc.SetNeighborsFromList(index, 1, 1)
	defer svc.Stop()
	waitFor(t, "done", func() bool { d, tot := svc.Snapshot(); return d == tot && tot == 3 })
	if n := len(rend.rendered()); n != 2 {
		t.Fatalf("renders=%d want 2 (species 3 already on disk)", n)
	}
}

func TestNativeForegroundBusyPauses(t *testing.T) {
	index := indexN(4)
	rend := &fakeNative{}
	svc := prewarm.NewNative(index, fakeLoader{}, rend, nil)
	svc.SetForegroundBusy(true)
	svc.EnsureStarted(context.Background(), 32)
	svc.SetNeighborsFromList(index, 1, 1)
	defer svc.Stop()
	time.Sleep(150 * time.Millisecond)
	if n := len(rend.rendered()); n != 0 {
		t.Fatalf("renders=%d want 0 while foreground busy", n)
	}
	svc.SetForegroundBusy(false)
	waitFor(t, "resume", func() bool { return rend.persisted() >= 1 })
}

func TestNativeFailureRetriesWithBackoff(t *testing.T) {
	index := indexN(2)
	rend := &fakeNative{failID: map[int]bool{1: true}}
	svc := prewarm.NewNative(index, fakeLoader{}, rend, nil)
	svc.EnsureStarted(context.Background(), 32)
	defer svc.Stop()
	waitFor(t, "species 2 done", func() bool { d, _ := svc.Snapshot(); return d == 1 })
	if d, _ := svc.Snapshot(); d != 1 {
		t.Fatalf("done=%d want 1 while species 1 backs off", d)
	}
	waitFor(t, "retry completes", func() bool { d, tot := svc.Snapshot(); return d == tot })
}

func TestNativePhasesProgressAndSilentWarmBucket(t *testing.T) {
	buckets := sprite.BrowseWarmBuckets()
	if len(buckets) < 2 {
		t.Skip("need two warm buckets")
	}
	first, second := buckets[0], buckets[1]
	index := indexN(3)
	rend := &fakeNative{disk: map[fakeKey]bool{}}
	for _, sum := range index { // second bucket already fully on disk
		rend.disk[fakeKey{sum.ID, second}] = true
	}
	rec := &recorder{}
	svc := prewarm.NewNative(index, fakeLoader{}, rend, rec.record)
	svc.SetBuckets(buckets)
	svc.Start(context.Background())
	defer svc.Stop()

	waitFor(t, "first bucket complete", func() bool {
		for _, p := range rec.snapshot() {
			if p.Cells == first && p.Total > 0 && p.Done >= p.Total {
				return true
			}
		}
		return false
	})
	time.Sleep(100 * time.Millisecond)
	for _, p := range rec.snapshot() {
		if p.Cells == second {
			t.Fatalf("already-warm bucket must stay silent, got %+v", p)
		}
	}
	if slices.Contains(func() []int { rend.mu.Lock(); defer rend.mu.Unlock(); return rend.cells }(), second) {
		t.Fatal("second bucket was re-encoded")
	}
}

func TestChafaWindowOnlyHintedAllScansIndex(t *testing.T) {
	index := indexN(4)
	var mu sync.Mutex
	var warmed []int
	rend := chafaFunc(func(p domain.Pokemon) {
		mu.Lock()
		warmed = append(warmed, p.ID)
		mu.Unlock()
	})
	count := func() int { mu.Lock(); defer mu.Unlock(); return len(warmed) }

	win := prewarm.NewChafa(index, fakeLoader{}, rend, false, nil)
	win.SetBuckets(sprite.BrowseWarmBuckets()[:1])
	win.Start(context.Background())
	time.Sleep(100 * time.Millisecond)
	if count() != 0 {
		t.Fatalf("window mode warmed %d without hints", count())
	}
	win.Prioritize(3)
	waitFor(t, "hinted species", func() bool { return count() == 1 })
	win.Stop()

	all := prewarm.NewChafa(index, fakeLoader{}, rend, true, nil)
	all.SetBuckets(sprite.BrowseWarmBuckets()[:1])
	all.Start(context.Background())
	defer all.Stop()
	waitFor(t, "full scan", func() bool { d, tot := all.Snapshot(); return tot == 4 && d == 4 })
}

type chafaFunc func(domain.Pokemon)

func (f chafaFunc) WarmRender(_ context.Context, p domain.Pokemon, _ int) sprite.RenderResult {
	f(p)
	return sprite.RenderResult{Backend: "chafa"}
}

type fakeResolver struct {
	mu   sync.Mutex
	miss map[int]bool
}

func (r *fakeResolver) Resolve(_ context.Context, p domain.Pokemon, _ ...sprite.AssetTier) sprite.SourceResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.miss[p.ID] {
		return sprite.SourceResult{UsePlaceholder: true}
	}
	return sprite.SourceResult{PNG: []byte("png")}
}

func TestArtworkMissCountsDoneThenPrioritizeRetries(t *testing.T) {
	index := indexN(6)
	res := &fakeResolver{miss: map[int]bool{2: true}}
	svc := prewarm.NewArtwork(index, nil, res, 8, nil)
	svc.Start(context.Background())
	defer svc.Stop()
	if err := svc.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if done, total := svc.Snapshot(); done != 6 || total != 6 {
		t.Fatalf("snapshot=%d/%d want 6/6", done, total)
	}
	if !svc.Ready(1) || svc.Ready(2) {
		t.Fatalf("ready(1)=%v ready(2)=%v want true,false", svc.Ready(1), svc.Ready(2))
	}
	res.mu.Lock()
	res.miss = nil
	res.mu.Unlock()
	svc.Prioritize(2)
	waitFor(t, "retry ready", func() bool { return svc.Ready(2) })
}

type fakeDossier struct {
	mu   sync.Mutex
	full []int
	core []int
}

func (f *fakeDossier) LoadDossier(_ context.Context, id int) (domain.Dossier, error) {
	f.mu.Lock()
	f.full = append(f.full, id)
	f.mu.Unlock()
	return domain.Dossier{}, nil
}

func (f *fakeDossier) WarmCore(_ context.Context, id int) error {
	f.mu.Lock()
	f.core = append(f.core, id)
	f.mu.Unlock()
	return nil
}

func (f *fakeDossier) counts() (full, core int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.full), len(f.core)
}

func TestDossierWindowWarmsCoreAllWarmsFull(t *testing.T) {
	index := indexN(5)
	win := &fakeDossier{}
	w := prewarm.NewDossier(index, win, false, nil)
	w.Start(context.Background())
	w.Prioritize(2)
	w.SetNeighborsFromList(index, 1, 1)
	waitFor(t, "core x3", func() bool { _, c := win.counts(); return c == 3 })
	w.Stop()
	if f, _ := win.counts(); f != 0 {
		t.Fatalf("window mode loaded %d full dossiers", f)
	}

	all := &fakeDossier{}
	a := prewarm.NewDossier(index, all, true, nil)
	a.Start(context.Background())
	defer a.Stop()
	waitFor(t, "full x5", func() bool { f, _ := all.counts(); return f == 5 })
}

func TestStopBeforeStartIsSafe(t *testing.T) {
	var nilSvc *prewarm.Service
	nilSvc.Stop()
	nilSvc.Prioritize(1)
	prewarm.NewDossier(nil, &fakeDossier{}, true, nil).Stop()
}
