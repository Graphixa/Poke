package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

var nativeDirectoryScanCount atomic.Int64

// ResetNativeDirectoryScanCount clears the test counter for native ReadDir calls.
func ResetNativeDirectoryScanCount() {
	nativeDirectoryScanCount.Store(0)
}

// NativeDirectoryScanCount returns how many times the native cache directory was read.
func NativeDirectoryScanCount() int64 {
	return nativeDirectoryScanCount.Load()
}

// NativeSizeStatus tracks warm completeness for one cell bucket.
type NativeSizeStatus struct {
	Done     int  `json:"done"`
	Total    int  `json:"total"`
	Complete bool `json:"complete,omitempty"`
}

// NativeManifest records per-size native frame cache completeness.
// Diagnostic/integrity support only — runtime scheduling uses ImageCacheManifest.
type NativeManifest struct {
	Version  string                   `json:"version"`
	Protocol string                   `json:"protocol"`
	Sizes    map[int]NativeSizeStatus `json:"sizes"`
}

func (s *Store) nativeManifestPath() string {
	return filepath.Join(s.root, "native", "manifest.json")
}

func (s *Store) readNativeManifestDisk() (NativeManifest, error) {
	path := s.nativeManifestPath()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return NativeManifest{Sizes: map[int]NativeSizeStatus{}}, nil
	}
	if err != nil {
		return NativeManifest{}, err
	}
	var m NativeManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return NativeManifest{}, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	if m.Sizes == nil {
		m.Sizes = map[int]NativeSizeStatus{}
	}
	return m, nil
}

// ReadNativeManifest loads the native cache manifest when present.
func (s *Store) ReadNativeManifest() (NativeManifest, error) {
	return s.nativeManifest().snapshot(), nil
}

// WriteNativeManifest persists the native cache manifest.
func (s *Store) WriteNativeManifest(m NativeManifest) error {
	if m.Sizes == nil {
		m.Sizes = map[int]NativeSizeStatus{}
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if err := writeAtomic(s.nativeManifestPath(), data, 0o600); err != nil {
		return err
	}
	st := s.nativeManifest()
	st.mu.Lock()
	st.m = m
	st.loaded = true
	st.dirty = false
	st.mu.Unlock()
	return nil
}

type nativeManifestState struct {
	store  *Store
	mu     sync.Mutex
	loaded bool
	m      NativeManifest
	dirty  bool
}

func (s *Store) nativeManifest() *nativeManifestState {
	if s.nativeMan == nil {
		s.nativeMan = &nativeManifestState{store: s}
	}
	return s.nativeMan
}

func (st *nativeManifestState) ensureLoaded() error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.loaded {
		return nil
	}
	m, err := st.store.readNativeManifestDisk()
	if err != nil {
		return err
	}
	st.m = m
	st.loaded = true
	return nil
}

func (st *nativeManifestState) snapshot() NativeManifest {
	if err := st.ensureLoaded(); err != nil {
		return NativeManifest{Sizes: map[int]NativeSizeStatus{}}
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	out := NativeManifest{Version: st.m.Version, Protocol: st.m.Protocol, Sizes: make(map[int]NativeSizeStatus, len(st.m.Sizes))}
	for k, v := range st.m.Sizes {
		out.Sizes[k] = v
	}
	return out
}

func (st *nativeManifestState) persistLocked() error {
	data, err := json.Marshal(st.m)
	if err != nil {
		return err
	}
	if err := writeAtomic(st.store.nativeManifestPath(), data, 0o600); err != nil {
		return err
	}
	st.dirty = false
	return nil
}

func (st *nativeManifestState) incrementFrame(cells int) {
	if err := st.ensureLoaded(); err != nil {
		return
	}
	st.mu.Lock()
	if st.m.Sizes == nil {
		st.m.Sizes = map[int]NativeSizeStatus{}
	}
	entry := st.m.Sizes[cells]
	entry.Done++
	entry.Complete = entry.Total > 0 && entry.Done >= entry.Total
	st.m.Sizes[cells] = entry
	st.dirty = true
	st.mu.Unlock()
}

// EnsureNativeManifest resets the manifest when version/protocol changes and
// heals per-size counts when version/protocol already match.
func (s *Store) EnsureNativeManifest(version, protocol string) error {
	st := s.nativeManifest()
	if err := st.ensureLoaded(); err != nil {
		return err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.m.Version != version || st.m.Protocol != protocol {
		st.m = NativeManifest{
			Version:  version,
			Protocol: protocol,
			Sizes:    map[int]NativeSizeStatus{},
		}
		return st.persistLocked()
	}
	return nil
}

// InitNativeSizePhase prepares manifest tracking for a warm phase.
func (s *Store) InitNativeSizePhase(cells, total int) error {
	st := s.nativeManifest()
	if err := st.ensureLoaded(); err != nil {
		return err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.m.Sizes == nil {
		st.m.Sizes = map[int]NativeSizeStatus{}
	}
	entry := st.m.Sizes[cells]
	entry.Total = total
	entry.Complete = false
	if counts, err := s.auditNativeFrameCounts(st.m.Protocol); err == nil {
		done := counts[cells]
		entry.Done = done
		entry.Complete = total > 0 && done >= total
	}
	st.m.Sizes[cells] = entry
	return st.persistLocked()
}

// MarkNativeSizeComplete marks a cell bucket complete and recounts disk.
func (s *Store) MarkNativeSizeComplete(cells, total int) error {
	return s.markNativeSizesComplete([]int{cells}, total)
}

// MarkAllNativeSizesComplete recounts disk once and updates every listed bucket.
func (s *Store) MarkAllNativeSizesComplete(sizes []int, total int) error {
	return s.markNativeSizesComplete(sizes, total)
}

func (s *Store) markNativeSizesComplete(sizes []int, total int) error {
	if len(sizes) == 0 {
		return nil
	}
	st := s.nativeManifest()
	if err := st.ensureLoaded(); err != nil {
		return err
	}
	protocol := st.snapshot().Protocol
	counts, err := s.auditNativeFrameCounts(protocol)
	if err != nil {
		return err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.m.Sizes == nil {
		st.m.Sizes = map[int]NativeSizeStatus{}
	}
	for _, cells := range sizes {
		done := counts[cells]
		entry := st.m.Sizes[cells]
		entry.Done = done
		if total > 0 {
			entry.Total = total
		}
		entry.Complete = total > 0 && done >= total
		st.m.Sizes[cells] = entry
	}
	return st.persistLocked()
}

// RecountNativeSize updates done count from disk for one bucket.
func (s *Store) RecountNativeSize(cells int) (int, error) {
	st := s.nativeManifest()
	if err := st.ensureLoaded(); err != nil {
		return 0, err
	}
	protocol := st.snapshot().Protocol
	counts, err := s.auditNativeFrameCounts(protocol)
	if err != nil {
		return 0, err
	}
	done := counts[cells]
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.m.Sizes == nil {
		st.m.Sizes = map[int]NativeSizeStatus{}
	}
	entry := st.m.Sizes[cells]
	entry.Done = done
	entry.Complete = entry.Total > 0 && done >= entry.Total
	st.m.Sizes[cells] = entry
	if err := st.persistLocked(); err != nil {
		return done, err
	}
	return done, nil
}

// NativeSizeComplete reports whether manifest marks a cell bucket complete.
// Hot path: manifest JSON only (no disk scan). Recount on store/delete/mark/heal.
func (s *Store) NativeSizeComplete(cells, indexTotal int) bool {
	if cells < sprite.MinFrameSize || indexTotal <= 0 {
		return false
	}
	st := s.nativeManifest()
	if err := st.ensureLoaded(); err != nil {
		return false
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.m.Version != sprite.AlgorithmVersion {
		return false
	}
	entry, ok := st.m.Sizes[cells]
	if !ok {
		return false
	}
	return entry.Total == indexTotal && entry.Done >= indexTotal
}

// CountNativeSpeciesForCells counts unique species with native frames at cells.
func (s *Store) CountNativeSpeciesForCells(cells int, protocol string) (int, error) {
	w, h := sprite.FrameSizeFor(cells)
	pxW, pxH := sprite.PixelSize(w, h)
	dimTag := fmt.Sprintf("-%dx%d-", pxW, pxH)
	protoNeedle := "image"
	if protocol != "" {
		protoNeedle = "image-" + protocol
	}
	entries, err := s.readNativeDirEntries()
	if err != nil {
		return 0, err
	}
	seen := make(map[string]bool)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".frame") {
			continue
		}
		if !strings.Contains(name, dimTag) || !strings.Contains(name, protoNeedle) {
			continue
		}
		key := strings.TrimSuffix(name, ".frame")
		hash, proto, fw, fh, _, _, parseErr := sprite.ParseNativeCacheKey(key)
		if parseErr != nil || hash == "" || fw != pxW || fh != pxH {
			continue
		}
		if protocol != "" && proto != protocol {
			continue
		}
		seen[hash] = true
	}
	return len(seen), nil
}

func speciesIDFromNativeFilename(name string) int {
	key := strings.TrimSuffix(name, ".frame")
	if !strings.HasPrefix(key, "named-") {
		return 0
	}
	rest := key[len("named-"):]
	i := strings.IndexByte(rest, '-')
	if i <= 0 {
		return 0
	}
	id, err := strconv.Atoi(rest[:i])
	if err != nil || id < 1 {
		return 0
	}
	return id
}

// InvalidateNativeManifestSizes recounts all tracked sizes after native deletes.
func (s *Store) InvalidateNativeManifestSizes() error {
	st := s.nativeManifest()
	if err := st.ensureLoaded(); err != nil {
		return err
	}
	st.mu.Lock()
	if len(st.m.Sizes) == 0 {
		st.mu.Unlock()
		return nil
	}
	protocol := st.m.Protocol
	st.mu.Unlock()
	counts, err := s.auditNativeFrameCounts(protocol)
	if err != nil {
		return err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	for cells := range st.m.Sizes {
		entry := st.m.Sizes[cells]
		entry.Done = counts[cells]
		entry.Complete = entry.Total > 0 && entry.Done >= entry.Total
		st.m.Sizes[cells] = entry
	}
	return st.persistLocked()
}

func (s *Store) readNativeDirEntries() ([]os.DirEntry, error) {
	dir := filepath.Join(s.root, "native")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	nativeDirectoryScanCount.Add(1)
	return entries, nil
}

func (s *Store) auditNativeFrameCounts(protocol string) (map[int]int, error) {
	entries, err := s.readNativeDirEntries()
	if err != nil {
		return nil, err
	}
	seen := map[int]map[string]bool{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".frame") {
			continue
		}
		key := strings.TrimSuffix(name, ".frame")
		hash, proto, fw, fh, _, ver, parseErr := sprite.ParseNativeCacheKey(key)
		if parseErr != nil || hash == "" {
			continue
		}
		if protocol != "" && proto != protocol {
			continue
		}
		if ver != "" && ver != sprite.AlgorithmVersion {
			continue
		}
		cells := cellsForNativeFrameDims(fw, fh)
		if cells < sprite.MinFrameSize {
			continue
		}
		if seen[cells] == nil {
			seen[cells] = map[string]bool{}
		}
		seen[cells][hash] = true
	}
	counts := map[int]int{}
	for cells, hashes := range seen {
		counts[cells] = len(hashes)
	}
	return counts, nil
}

func cellsForNativeFrameDims(fw, fh int) int {
	for _, cells := range sprite.FrameSteps() {
		w, h := sprite.FrameSizeFor(cells)
		pxW, pxH := sprite.PixelSize(w, h)
		if fw == pxW && fh == pxH {
			return cells
		}
	}
	return 0
}

// Service wrappers for native manifest operations.

func (s *Service) EnsureNativeManifest(protocol string) error {
	return s.store.EnsureNativeManifest(sprite.AlgorithmVersion, protocol)
}

// EnsureManifest aligns with nativewarm.ManifestStore.
func (s *Service) EnsureManifest(protocol string) error {
	return s.EnsureNativeManifest(protocol)
}

func (s *Service) InitNativeSizePhase(cells, total int) error {
	return s.store.InitNativeSizePhase(cells, total)
}

func (s *Service) MarkNativeSizeComplete(cells, total int) error {
	return s.store.MarkNativeSizeComplete(cells, total)
}

func (s *Service) MarkAllNativeSizesComplete(sizes []int, total int) error {
	return s.store.MarkAllNativeSizesComplete(sizes, total)
}

func (s *Service) RecountNativeSize(cells int) (int, error) {
	return s.store.RecountNativeSize(cells)
}

func (s *Service) NativeSizeComplete(cells, indexTotal int) bool {
	return s.store.NativeSizeComplete(cells, indexTotal)
}

func (s *Store) recordNativeFrameManifest(cells int) {
	s.nativeManifest().incrementFrame(cells)
}
