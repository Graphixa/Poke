package cache

import (
	"context"
	"sync"

	"github.com/Graphixa/Pokedex/internal/debuglog"
	"github.com/Graphixa/Pokedex/internal/domain"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

// ImageCacheProgress is the user-facing aggregate image-cache status.
type ImageCacheProgress struct {
	Done   int
	Total  int
	Active bool
}

// PokemonCacheWork tracks one outstanding Pokémon's remaining image-cache work.
type PokemonCacheWork struct {
	SpeciesID   int
	Slug        string
	SourceReady bool
	SourceHash  string
	NativeReady map[int]bool
	Complete    bool
}

// ImageCacheCoordinator owns the session outstanding set and Pokémon-level progress.
type ImageCacheCoordinator struct {
	mu                sync.Mutex
	store             *Store
	spec              ImageCacheSpec
	specID            string
	datasetID         string
	pending           map[int]*PokemonCacheWork
	total             int
	done              int
	writer            *imageManifestWriter
	progress          ProgressFunc
	deferAudit        bool
	onReconcileActive func()
}

// StartImageCacheSession audits the current index against the aggregate
// manifest and required artefacts. Total is fixed for the session.
func StartImageCacheSession(store *Store, index []domain.SpeciesSummary, spec ImageCacheSpec) (*ImageCacheCoordinator, error) {
	return startImageCacheSession(store, index, spec, false)
}

// StartImageCacheSessionDeferred is the fast startup path: trusts manifest
// completeness without per-Pokémon native file stats. Call ReconcileImageCacheSession
// in the background to verify outstanding work.
func StartImageCacheSessionDeferred(store *Store, index []domain.SpeciesSummary, spec ImageCacheSpec) (*ImageCacheCoordinator, error) {
	return startImageCacheSession(store, index, spec, true)
}

func startImageCacheSession(store *Store, index []domain.SpeciesSummary, spec ImageCacheSpec, deferAudit bool) (*ImageCacheCoordinator, error) {
	if store == nil {
		return &ImageCacheCoordinator{spec: spec, specID: spec.ID(), pending: map[int]*PokemonCacheWork{}}, nil
	}
	specID := spec.ID()
	datasetID, _ := store.ActiveDatasetID()
	man, err := store.ReadImageCacheManifest()
	if err != nil {
		return nil, err
	}
	versionInvalid := man.Version != "" && man.Version != ImageCacheManifestVersion
	specChanged := man.SpecID != "" && man.SpecID != specID
	if versionInvalid || specChanged {
		man = ImageCacheManifest{
			Version: ImageCacheManifestVersion,
			SpecID:  specID,
			Pokemon: map[int]PokemonImageCacheState{},
		}
	} else {
		if man.Version == "" {
			man.Version = ImageCacheManifestVersion
		}
		if man.SpecID == "" {
			man.SpecID = specID
		}
		pruneManifestToIndex(&man, index)
	}
	if datasetID != "" {
		man.DatasetID = datasetID
	}
	man.PokemonCount = len(index)

	trusted := deferAudit && imageManifestTrusted(man, index, specID)

	pending := make(map[int]*PokemonCacheWork)

	if deferAudit && trusted {
		writer := newImageManifestWriter(store, man)
		return &ImageCacheCoordinator{
			store: store, spec: spec, specID: specID, datasetID: datasetID,
			pending: pending, writer: writer, deferAudit: true,
		}, nil
	}

	if deferAudit && !trusted {
		for _, sum := range index {
			if sum.ID < 1 {
				continue
			}
			if st, ok := man.Pokemon[sum.ID]; ok && st.Complete && st.SpecID == specID {
				continue
			}
			pending[sum.ID] = &PokemonCacheWork{
				SpeciesID: sum.ID, Slug: sum.Slug, NativeReady: map[int]bool{},
			}
		}
		writer := newImageManifestWriter(store, man)
		return &ImageCacheCoordinator{
			store: store, spec: spec, specID: specID, datasetID: datasetID,
			pending: pending, total: len(pending), writer: writer, deferAudit: true,
		}, nil
	}

	for _, sum := range index {
		if sum.ID < 1 {
			continue
		}
		work := auditPokemonImageWork(store, spec, specID, sum, man.Pokemon[sum.ID], specChanged, false)
		if work.Complete {
			man.Pokemon[sum.ID] = PokemonImageCacheState{
				SpeciesID: sum.ID, SourceHash: work.SourceHash, SpecID: specID, Complete: true,
			}
			continue
		}
		pending[sum.ID] = work
		delete(man.Pokemon, sum.ID)
	}

	writer := newImageManifestWriter(store, man)
	if err := writer.flush(); err != nil {
		return nil, err
	}

	return &ImageCacheCoordinator{
		store: store, spec: spec, specID: specID, datasetID: datasetID,
		pending: pending, total: len(pending), writer: writer,
	}, nil
}

func pruneManifestToIndex(man *ImageCacheManifest, index []domain.SpeciesSummary) {
	if man == nil || man.Pokemon == nil {
		return
	}
	current := make(map[int]struct{}, len(index))
	for _, sum := range index {
		if sum.ID > 0 {
			current[sum.ID] = struct{}{}
		}
	}
	for id := range man.Pokemon {
		if _, ok := current[id]; !ok {
			delete(man.Pokemon, id)
		}
	}
}

func imageManifestTrusted(man ImageCacheManifest, index []domain.SpeciesSummary, specID string) bool {
	if man.SpecID != specID || man.Version != ImageCacheManifestVersion {
		return false
	}
	if man.PokemonCount != len(index) {
		return false
	}
	for _, sum := range index {
		st, ok := man.Pokemon[sum.ID]
		if !ok || !st.Complete || st.SpecID != specID {
			return false
		}
	}
	return true
}

func auditPokemonImageWork(store *Store, spec ImageCacheSpec, specID string, sum domain.SpeciesSummary, st PokemonImageCacheState, specChanged, trusted bool) *PokemonCacheWork {
	work := &PokemonCacheWork{
		SpeciesID:   sum.ID,
		Slug:        sum.Slug,
		NativeReady: make(map[int]bool, len(spec.RequiredSizes)),
	}
	hash, _, ready, _ := store.NamedAssetMeta(context.Background(), sum.ID, sum.Slug, sourceKind(spec.SourceMode))
	if ready && isHexSHA256(hash) {
		work.SourceReady = true
		work.SourceHash = hash
	}

	if trusted && !specChanged && st.Complete && st.SpecID == specID {
		work.Complete = true
		for _, cells := range spec.RequiredSizes {
			work.NativeReady[cells] = true
		}
		return work
	}

	if !specChanged && work.SourceReady && st.Complete && st.SpecID == specID && st.SourceHash == work.SourceHash {
		if allRequiredNativePresent(store, spec, work.SourceHash) {
			work.Complete = true
			for _, cells := range spec.RequiredSizes {
				work.NativeReady[cells] = true
			}
			return work
		}
	}

	if st.SourceHash != "" && work.SourceHash != "" && st.SourceHash != work.SourceHash {
		_ = store.DeleteNativeForSpecies(sum.ID)
	}

	if work.SourceReady && !specChanged {
		for _, cells := range spec.RequiredSizes {
			if store.HasNativeFile(spec.nativeKey(work.SourceHash, cells)) {
				work.NativeReady[cells] = true
			}
		}
	}
	if !specChanged && work.SourceReady && allNativeReady(work, spec.RequiredSizes) {
		work.Complete = true
	}
	return work
}

func allRequiredNativePresent(store *Store, spec ImageCacheSpec, sourceHash string) bool {
	if store == nil || !isHexSHA256(sourceHash) {
		return false
	}
	for _, cells := range spec.RequiredSizes {
		if !store.HasNativeFile(spec.nativeKey(sourceHash, cells)) {
			return false
		}
	}
	return true
}

func allNativeReady(work *PokemonCacheWork, sizes []int) bool {
	if work == nil || len(sizes) == 0 {
		return false
	}
	for _, cells := range sizes {
		if !work.NativeReady[cells] {
			return false
		}
	}
	return true
}

func sourceKind(mode string) string {
	if mode == string(sprite.SourcePixel) {
		return SpriteKindPixel
	}
	return SpriteKindArtwork
}

// SetOnReconcileActive runs when deferred reconciliation discovers session work.
func (c *ImageCacheCoordinator) SetOnReconcileActive(fn func()) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.onReconcileActive = fn
	c.mu.Unlock()
}

// SetProgressFunc receives aggregate progress (session start/finish and Pokémon completion).
func (c *ImageCacheCoordinator) SetProgressFunc(fn ProgressFunc) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.progress = fn
	c.mu.Unlock()
}

// PublishStart emits 0/Total once so the footer appears before the first size finishes.
func (c *ImageCacheCoordinator) PublishStart() {
	if c == nil {
		return
	}
	c.mu.Lock()
	fn := c.progress
	p := c.snapshotLocked()
	c.mu.Unlock()
	if fn == nil || p.Total <= 0 {
		return
	}
	fn(Progress{
		Stage:   "native",
		Done:    p.Done,
		Total:   p.Total,
		Message: "Building image cache",
	})
}

// SpecID returns the session cache specification identity.
func (c *ImageCacheCoordinator) SpecID() string {
	if c == nil {
		return ""
	}
	return c.specID
}

// OutstandingIDs returns species that still need image-cache work.
func (c *ImageCacheCoordinator) OutstandingIDs() []int {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	ids := make([]int, 0, len(c.pending))
	for id, w := range c.pending {
		if w != nil && !w.Complete {
			ids = append(ids, id)
		}
	}
	return ids
}

// IsComplete reports in-memory Pokémon completeness (no directory scan).
func (c *ImageCacheCoordinator) IsComplete(speciesID int) bool {
	if c == nil || speciesID < 1 {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	w := c.pending[speciesID]
	if w == nil {
		return true
	}
	return w.Complete
}

// NoteSource records that official artwork (or configured source) is ready.
func (c *ImageCacheCoordinator) NoteSource(speciesID int, sourceHash string) {
	if c == nil || speciesID < 1 || !isHexSHA256(sourceHash) {
		return
	}
	c.mu.Lock()
	w := c.pending[speciesID]
	if w == nil || w.Complete {
		c.mu.Unlock()
		return
	}
	if w.SourceHash != "" && w.SourceHash != sourceHash {
		for k := range w.NativeReady {
			delete(w.NativeReady, k)
		}
		if c.store != nil {
			_ = c.store.DeleteNativeForSpecies(speciesID)
		}
	}
	w.SourceHash = sourceHash
	w.SourceReady = true
	st, persist := c.tryCompleteLocked(w)
	fn := c.progress
	p := c.snapshotLocked()
	c.mu.Unlock()
	c.persistAndEmit(persist, st, fn, p, speciesID)
}

// NoteVariant records that one required native size is present.
func (c *ImageCacheCoordinator) NoteVariant(speciesID, cells int) {
	if c == nil || speciesID < 1 {
		return
	}
	c.mu.Lock()
	w := c.pending[speciesID]
	if w == nil || w.Complete {
		c.mu.Unlock()
		return
	}
	if w.NativeReady == nil {
		w.NativeReady = map[int]bool{}
	}
	w.NativeReady[cells] = true
	debuglog.Cachef("cache species=%d size=%d", speciesID, cells)
	c.refreshSourceLocked(w)
	st, persist := c.tryCompleteLocked(w)
	fn := c.progress
	p := c.snapshotLocked()
	c.mu.Unlock()
	c.persistAndEmit(persist, st, fn, p, speciesID)
}

func (c *ImageCacheCoordinator) refreshSourceLocked(w *PokemonCacheWork) {
	if w.SourceReady && isHexSHA256(w.SourceHash) {
		return
	}
	if c.store == nil {
		return
	}
	hash, _, ready, _ := c.store.NamedAssetMeta(context.Background(), w.SpeciesID, w.Slug, sourceKind(c.spec.SourceMode))
	if ready && isHexSHA256(hash) {
		w.SourceHash = hash
		w.SourceReady = true
	}
}

func (c *ImageCacheCoordinator) tryCompleteLocked(w *PokemonCacheWork) (PokemonImageCacheState, bool) {
	if w.Complete || !w.SourceReady || w.SourceHash == "" {
		return PokemonImageCacheState{}, false
	}
	if !allNativeReady(w, c.spec.RequiredSizes) {
		return PokemonImageCacheState{}, false
	}
	if c.store != nil && !allRequiredNativePresent(c.store, c.spec, w.SourceHash) {
		return PokemonImageCacheState{}, false
	}
	w.Complete = true
	c.done++
	debuglog.Cachef("cache species=%d complete", w.SpeciesID)
	return PokemonImageCacheState{
		SpeciesID:  w.SpeciesID,
		SourceHash: w.SourceHash,
		SpecID:     c.specID,
		Complete:   true,
	}, true
}

func (c *ImageCacheCoordinator) persistAndEmit(persist bool, st PokemonImageCacheState, fn ProgressFunc, p ImageCacheProgress, speciesID int) {
	if persist && c.writer != nil {
		c.writer.set(st)
		_ = c.writer.maybeFlush(false)
	}
	if !persist || fn == nil {
		return
	}
	fn(Progress{
		Stage:     "native",
		Done:      p.Done,
		Total:     p.Total,
		Message:   "Building image cache",
		SpeciesID: speciesID,
	})
}

// Progress returns the current aggregate snapshot. Total is session-fixed.
func (c *ImageCacheCoordinator) Progress() ImageCacheProgress {
	if c == nil {
		return ImageCacheProgress{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.snapshotLocked()
}

// AsCacheProgress converts the snapshot into the shared Progress event.
func (c *ImageCacheCoordinator) AsCacheProgress(speciesID int) Progress {
	p := c.Progress()
	return Progress{
		Stage:     "native",
		Done:      p.Done,
		Total:     p.Total,
		Message:   "Building image cache",
		SpeciesID: speciesID,
	}
}

func (c *ImageCacheCoordinator) snapshotLocked() ImageCacheProgress {
	active := c.total > 0 && c.done < c.total
	return ImageCacheProgress{Done: c.done, Total: c.total, Active: active}
}

// Flush writes pending manifest changes (graceful shutdown).
func (c *ImageCacheCoordinator) Flush() error {
	if c == nil || c.writer == nil {
		return nil
	}
	return c.writer.flush()
}

// CompletePokemonIDs returns species marked complete in the persisted manifest.
func (c *ImageCacheCoordinator) CompletePokemonIDs() []int {
	if c == nil || c.writer == nil {
		return nil
	}
	man := c.writer.snapshot()
	ids := make([]int, 0, len(man.Pokemon))
	for id, st := range man.Pokemon {
		if st.Complete {
			ids = append(ids, id)
		}
	}
	return ids
}

// ReconcileImageCacheSession verifies outstanding image-cache work after deferred startup.
func (c *ImageCacheCoordinator) ReconcileImageCacheSession(index []domain.SpeciesSummary) {
	if c == nil || c.store == nil {
		return
	}
	c.mu.Lock()
	if !c.deferAudit {
		c.mu.Unlock()
		return
	}
	c.deferAudit = false
	sessionTotal := c.total
	sessionDone := c.done
	pendingIDs := make([]int, 0, len(c.pending))
	for id := range c.pending {
		pendingIDs = append(pendingIDs, id)
	}
	indexByID := make(map[int]domain.SpeciesSummary, len(index))
	for _, sum := range index {
		if sum.ID > 0 {
			indexByID[sum.ID] = sum
		}
	}
	c.mu.Unlock()

	manifestSnap := c.writer.snapshot()

	// Trusted fast path started with zero session work; discover any newly outstanding Pokémon.
	if sessionTotal == 0 && len(pendingIDs) == 0 {
		for _, sum := range index {
			if sum.ID < 1 {
				continue
			}
			st := manifestSnap.Pokemon[sum.ID]
			work := auditPokemonImageWork(c.store, c.spec, c.specID, sum, st, false, false)
			if !work.Complete {
				pendingIDs = append(pendingIDs, sum.ID)
			}
		}
		sessionTotal = len(pendingIDs)
		sessionDone = 0
	}

	newPending := make(map[int]*PokemonCacheWork)
	done := sessionDone
	for _, id := range pendingIDs {
		sum, ok := indexByID[id]
		if !ok {
			continue
		}
		st := manifestSnap.Pokemon[id]
		work := auditPokemonImageWork(c.store, c.spec, c.specID, sum, st, false, false)
		if work.Complete {
			done++
			if c.writer != nil {
				c.writer.set(PokemonImageCacheState{
					SpeciesID: id, SourceHash: work.SourceHash,
					SpecID: c.specID, Complete: true,
				})
			}
			continue
		}
		newPending[id] = work
	}

	c.mu.Lock()
	c.pending = newPending
	if done > sessionTotal {
		done = sessionTotal
	}
	c.total = sessionTotal
	c.done = done
	fn := c.progress
	onActive := c.onReconcileActive
	p := c.snapshotLocked()
	c.mu.Unlock()

	if c.writer != nil {
		_ = c.writer.flush()
	}
	if fn != nil && p.Total > 0 {
		fn(Progress{Stage: "native", Done: p.Done, Total: p.Total, Message: "Building image cache"})
	}
	if onActive != nil && p.Total > 0 && p.Done < p.Total {
		onActive()
	}
}

func (s *Service) StartImageCacheSessionDeferred(index []domain.SpeciesSummary, spec ImageCacheSpec) (*ImageCacheCoordinator, error) {
	if s == nil {
		return StartImageCacheSessionDeferred(nil, index, spec)
	}
	return StartImageCacheSessionDeferred(s.store, index, spec)
}
