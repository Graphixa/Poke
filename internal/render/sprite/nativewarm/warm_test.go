package nativewarm_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
	"github.com/Graphixa/Pokedex/internal/render/sprite/nativewarm"
)

type manifestStub struct {
	mu       sync.Mutex
	complete map[int]bool
	init     []int
	marked   []int
}

func (m *manifestStub) EnsureManifest(_ string) error { return nil }
func (m *manifestStub) NativeSizeComplete(cells, _ int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.complete[cells]
}
func (m *manifestStub) InitNativeSizePhase(cells, _ int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.init = append(m.init, cells)
	return nil
}
func (m *manifestStub) MarkNativeSizeComplete(cells, _ int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.complete[cells] = true
	m.marked = append(m.marked, cells)
	return nil
}
func (m *manifestStub) MarkAllNativeSizesComplete(sizes []int, _ int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, cells := range sizes {
		m.complete[cells] = true
		m.marked = append(m.marked, cells)
	}
	return nil
}
func (m *manifestStub) RecountNativeSize(cells int) (int, error) { return 0, nil }

type fakeKey struct {
	id    int
	cells int
}

type fakeNativeRenderer struct {
	mu       sync.Mutex
	cached   map[fakeKey]bool
	allSizes map[int]bool
	render   int
	ids      []int
	cells    []int
}

func (f *fakeNativeRenderer) HasNativeCache(_ context.Context, pokemon domain.Pokemon, cells int) bool {
	if cells < sprite.MinFrameSize {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.allSizes[pokemon.ID] {
		return true
	}
	return f.cached[fakeKey{pokemon.ID, cells}]
}

func (f *fakeNativeRenderer) NativeWarmRender(_ context.Context, pokemon domain.Pokemon, cells int) sprite.RenderResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.render++
	f.ids = append(f.ids, pokemon.ID)
	f.cells = append(f.cells, cells)
	if f.cached == nil {
		f.cached = map[fakeKey]bool{}
	}
	f.cached[fakeKey{pokemon.ID, cells}] = true
	return sprite.RenderResult{ANSI: "X", Native: true, Backend: "image"}
}

func (f *fakeNativeRenderer) NativeWarmBatch(_ context.Context, pokemon domain.Pokemon, sizes []int) sprite.NativeWarmBatchResult {
	var ready, encoded []int
	var last sprite.RenderResult
	for _, cells := range sizes {
		if f.HasNativeCache(context.Background(), pokemon, cells) {
			ready = append(ready, cells)
			continue
		}
		last = f.NativeWarmRender(context.Background(), pokemon, cells)
		if last.Native {
			encoded = append(encoded, cells)
		}
	}
	return sprite.NativeWarmBatchResult{ReadySizes: ready, EncodedSizes: encoded, Encoded: len(encoded), Last: last}
}

func (f *fakeNativeRenderer) renders() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.render
}

type fakeLoader struct{}

func (fakeLoader) Pokemon(_ context.Context, id int) (domain.Pokemon, error) {
	return domain.Pokemon{ID: id}, nil
}

func TestEnsureStartedSkipsWhenManifestComplete(t *testing.T) {
	index := []domain.SpeciesSummary{
		{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"},
	}
	complete := map[int]bool{}
	for _, cells := range sprite.FrameSteps() {
		complete[cells] = true
	}
	manifest := &manifestStub{complete: complete}
	svc := nativewarm.New(nativewarm.Config{
		Cells:    sprite.MaxFrameSize,
		Workers:  1,
		Loader:   fakeLoader{},
		Renderer: &fakeNativeRenderer{},
		Index:    index,
		Manifest: manifest,
		Protocol: "sixel",
	})
	svc.EnsureStarted(context.Background(), sprite.MaxFrameSize)
	done, total := svc.Snapshot()
	if total != 0 || done != 0 {
		t.Fatalf("snapshot done=%d total=%d want skip", done, total)
	}
}

func TestEnsureStartedOnlyEncodesMissing(t *testing.T) {
	index := []domain.SpeciesSummary{
		{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"},
		{ID: 26, Slug: "raichu", DisplayName: "Raichu"},
	}
	rend := &fakeNativeRenderer{allSizes: map[int]bool{25: true}}
	svc := nativewarm.New(nativewarm.Config{
		Cells:    sprite.MaxFrameSize,
		Workers:  1,
		Loader:   fakeLoader{},
		Renderer: rend,
		Index:    index,
	})
	ctx := context.Background()
	svc.EnsureStarted(ctx, sprite.MaxFrameSize)
	if err := svc.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	want := len(sprite.FrameSteps())
	if rend.renders() != want {
		t.Fatalf("render calls=%d want %d (one Pokémon, all sizes)", rend.renders(), want)
	}
}

func TestEnsureStartedMarksManifestComplete(t *testing.T) {
	index := []domain.SpeciesSummary{
		{ID: 25, Slug: "pikachu", DisplayName: "Pikachu"},
		{ID: 26, Slug: "raichu", DisplayName: "Raichu"},
	}
	complete := map[int]bool{}
	for _, cells := range sprite.FrameSteps() {
		if cells != sprite.MaxFrameSize {
			complete[cells] = true
		}
	}
	manifest := &manifestStub{complete: complete}
	rend := &fakeNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{
		Cells:    sprite.MaxFrameSize,
		Workers:  1,
		Loader:   fakeLoader{},
		Renderer: rend,
		Index:    index,
		Manifest: manifest,
		Protocol: "sixel",
	})
	ctx := context.Background()
	svc.EnsureStarted(ctx, sprite.MaxFrameSize)
	if err := svc.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, cells := range manifest.marked {
		if cells == sprite.MaxFrameSize {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("marked=%v want to include %d", manifest.marked, sprite.MaxFrameSize)
	}
}

func TestP2CompletesAllSizesForOnePokemonBeforeNext(t *testing.T) {
	index := []domain.SpeciesSummary{
		{ID: 25, Slug: "pikachu"},
		{ID: 26, Slug: "raichu"},
	}
	rend := &fakeNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{
		Workers:  1,
		Loader:   fakeLoader{},
		Renderer: rend,
		Index:    index,
	})
	ctx := context.Background()
	svc.EnsureStarted(ctx, sprite.MaxFrameSize)
	if err := svc.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	n := len(sprite.FrameSteps())
	ids := rend.renderedIDs()
	if len(ids) != 2*n {
		t.Fatalf("ids=%v want %d renders", ids, 2*n)
	}
	for i := 0; i < n; i++ {
		if ids[i] != 25 {
			t.Fatalf("first Pokémon incomplete before next; ids=%v", ids)
		}
	}
	for i := n; i < 2*n; i++ {
		if ids[i] != 26 {
			t.Fatalf("second Pokémon mixed with first; ids=%v", ids)
		}
	}
}

func TestWarmAllSizesMaxFirst(t *testing.T) {
	index := []domain.SpeciesSummary{{ID: 25, Slug: "pikachu"}}
	manifest := &manifestStub{complete: map[int]bool{}}
	rend := &fakeNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{
		Workers:  1,
		Loader:   fakeLoader{},
		Renderer: rend,
		Index:    index,
		Manifest: manifest,
		Protocol: "sixel",
	})
	sizes := []int{40, 32}
	ctx := context.Background()
	if err := svc.WarmAllSizes(ctx, sizes); err != nil {
		t.Fatal(err)
	}
	if rend.renders() != 2 {
		t.Fatalf("render=%d want 2 sizes", rend.renders())
	}
	if len(rend.renderedCells()) != 2 {
		t.Fatalf("cells=%v want two sizes", rend.renderedCells())
	}
}

func (f *fakeNativeRenderer) renderedIDs() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int(nil), f.ids...)
}

func (f *fakeNativeRenderer) renderedCells() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int(nil), f.cells...)
}

func waitNativeRenders(t *testing.T, rend *fakeNativeRenderer, n int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if rend.renders() >= n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("renders=%d want >=%d ids=%v", rend.renders(), n, rend.renderedIDs())
}

func TestEnsureStartedHotPathNoDirectoryScan(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)
	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		t.Fatal(err)
	}
	if err := store.InitNativeSizePhase(sprite.MaxFrameSize, 2); err != nil {
		t.Fatal(err)
	}
	index := []domain.SpeciesSummary{
		{ID: 25, Slug: "pikachu"},
		{ID: 26, Slug: "raichu"},
	}
	rend := &fakeNativeRenderer{}
	svcWarm := nativewarm.New(nativewarm.Config{
		Cells:    sprite.MaxFrameSize,
		Workers:  1,
		Loader:   fakeLoader{},
		Renderer: rend,
		Index:    index,
		Manifest: svc,
		Protocol: "sixel",
	})
	ctx := context.Background()
	svcWarm.EnsureStarted(ctx, sprite.MaxFrameSize)
	waitNativeRenders(t, rend, 1)
	cache.ResetNativeDirectoryScanCount()
	for i := 0; i < 1000; i++ {
		svcWarm.EnsureStarted(ctx, sprite.MaxFrameSize)
	}
	if n := cache.NativeDirectoryScanCount(); n != 0 {
		t.Fatalf("EnsureStarted scans=%d want 0", n)
	}
	svcWarm.Stop()
}

func TestForegroundBusySkipsBackgroundKeepsSelected(t *testing.T) {
	index := []domain.SpeciesSummary{
		{ID: 25, Slug: "pikachu"},
		{ID: 26, Slug: "raichu"},
		{ID: 27, Slug: "sandshrew"},
	}
	rend := &fakeNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{
		Cells:    sprite.MaxFrameSize,
		Workers:  1,
		Loader:   fakeLoader{},
		Renderer: rend,
		Index:    index,
	})
	svc.SetForegroundBusy(true)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	svc.EnsureStarted(ctx, sprite.MaxFrameSize)
	time.Sleep(80 * time.Millisecond)
	if n := rend.renders(); n != 0 {
		t.Fatalf("P2 ran while foreground busy; renders=%d ids=%v", n, rend.renderedIDs())
	}
	svc.Prioritize(25)
	svc.SetNeighborsFromList(index, 0, 1)
	waitNativeRenders(t, rend, 2)
	for _, id := range rend.renderedIDs() {
		if id == 27 {
			t.Fatalf("P2 species 27 encoded while busy; ids=%v", rend.renderedIDs())
		}
	}
	svc.SetForegroundBusy(false)
	if err := svc.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	nSizes := len(sprite.FrameSteps())
	if rend.renders() != 3*nSizes {
		t.Fatalf("after idle render=%d want %d ids=%v", rend.renders(), 3*nSizes, rend.renderedIDs())
	}
}

func TestActivityPausesBackgroundUntilIdle(t *testing.T) {
	index := []domain.SpeciesSummary{
		{ID: 25, Slug: "pikachu"},
		{ID: 26, Slug: "raichu"},
	}
	rend := &fakeNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{
		Cells:    sprite.MaxFrameSize,
		Workers:  1,
		Loader:   fakeLoader{},
		Renderer: rend,
		Index:    index,
	})
	svc.NoteActivity()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	svc.EnsureStarted(ctx, sprite.MaxFrameSize)
	time.Sleep(80 * time.Millisecond)
	if n := rend.renders(); n != 0 {
		t.Fatalf("P2 ran during activity window; renders=%d", n)
	}
	if err := svc.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	nSizes := len(sprite.FrameSteps())
	if rend.renders() != 2*nSizes {
		t.Fatalf("after idle render=%d want %d", rend.renders(), 2*nSizes)
	}
}

type memCompleter struct {
	mu     sync.Mutex
	sizes  []int
	ready  map[int]map[int]bool
	doneAt map[int]bool
	total  int
	done   int
}

func newMemCompleter(ids, sizes []int) *memCompleter {
	c := &memCompleter{
		sizes:  sizes,
		ready:  make(map[int]map[int]bool, len(ids)),
		doneAt: make(map[int]bool, len(ids)),
		total:  len(ids),
	}
	for _, id := range ids {
		c.ready[id] = map[int]bool{}
	}
	return c
}

func (c *memCompleter) OutstandingIDs() []int {
	c.mu.Lock()
	defer c.mu.Unlock()
	ids := make([]int, 0, len(c.ready))
	for id := range c.ready {
		if !c.doneAt[id] {
			ids = append(ids, id)
		}
	}
	return ids
}

func (c *memCompleter) NoteVariant(speciesID, cells int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ready[speciesID] == nil {
		c.ready[speciesID] = map[int]bool{}
	}
	c.ready[speciesID][cells] = true
	if c.doneAt[speciesID] {
		return
	}
	for _, sz := range c.sizes {
		if !c.ready[speciesID][sz] {
			return
		}
	}
	c.doneAt[speciesID] = true
	c.done++
}

func (c *memCompleter) IsComplete(speciesID int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.doneAt[speciesID]
}

func (c *memCompleter) Progress() cache.ImageCacheProgress {
	c.mu.Lock()
	defer c.mu.Unlock()
	return cache.ImageCacheProgress{Done: c.done, Total: c.total, Active: c.total > 0 && c.done < c.total}
}

type gatingNativeRenderer struct {
	mu     sync.Mutex
	allow  bool
	render int
	cached map[fakeKey]bool
}

func (f *gatingNativeRenderer) setAllow(v bool) {
	f.mu.Lock()
	f.allow = v
	f.mu.Unlock()
}

func (f *gatingNativeRenderer) renders() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.render
}

func (f *gatingNativeRenderer) HasNativeCache(_ context.Context, pokemon domain.Pokemon, cells int) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cached[fakeKey{pokemon.ID, cells}]
}

func (f *gatingNativeRenderer) NativeWarmRender(_ context.Context, pokemon domain.Pokemon, cells int) sprite.RenderResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.render++
	if !f.allow {
		return sprite.RenderResult{UsePlaceholder: true, Backend: "image"}
	}
	if f.cached == nil {
		f.cached = map[fakeKey]bool{}
	}
	f.cached[fakeKey{pokemon.ID, cells}] = true
	return sprite.RenderResult{ANSI: "X", Native: true, Backend: "image"}
}

func (f *gatingNativeRenderer) NativeWarmBatch(_ context.Context, pokemon domain.Pokemon, sizes []int) sprite.NativeWarmBatchResult {
	var ready, encoded []int
	var last sprite.RenderResult
	for _, cells := range sizes {
		if f.HasNativeCache(context.Background(), pokemon, cells) {
			ready = append(ready, cells)
			continue
		}
		last = f.NativeWarmRender(context.Background(), pokemon, cells)
		if last.Native {
			encoded = append(encoded, cells)
		}
	}
	return sprite.NativeWarmBatchResult{ReadySizes: ready, EncodedSizes: encoded, Encoded: len(encoded), Last: last}
}

func TestP2DoesNotAbandonIncompletePokemon(t *testing.T) {
	index := []domain.SpeciesSummary{{ID: 25, Slug: "pikachu"}}
	sizes := sprite.FrameSteps()
	comp := newMemCompleter([]int{25}, sizes)
	rend := &gatingNativeRenderer{}
	svc := nativewarm.New(nativewarm.Config{
		Workers:       1,
		Loader:        fakeLoader{},
		Renderer:      rend,
		Index:         index,
		Completer:     comp,
		RequiredSizes: sizes,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	svc.EnsureStarted(ctx, sprite.MaxFrameSize)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if rend.renders() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if rend.renders() < 1 {
		t.Fatal("expected placeholder P2 attempts")
	}
	if p := comp.Progress(); p.Done != 0 || !p.Active {
		t.Fatalf("placeholder pass must not complete Pokémon: %+v", p)
	}
	rend.setAllow(true)
	if err := svc.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	if p := comp.Progress(); p.Done != 1 || p.Active {
		t.Fatalf("after successful retry progress=%+v want done=1", p)
	}
	svc.Stop()
}

// compile-time check manifestStub implements interface
var _ nativewarm.ManifestStore = (*manifestStub)(nil)
var _ nativewarm.ImageCompleter = (*cache.ImageCacheCoordinator)(nil)
var _ nativewarm.ImageCompleter = (*memCompleter)(nil)

// unused import guard
var _ = cache.Progress{}
