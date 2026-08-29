package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

// ReadNativeManifest loads the native cache manifest when present.
func (s *Store) ReadNativeManifest() (NativeManifest, error) {
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

// WriteNativeManifest persists the native cache manifest.
func (s *Store) WriteNativeManifest(m NativeManifest) error {
	if m.Sizes == nil {
		m.Sizes = map[int]NativeSizeStatus{}
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return writeAtomic(s.nativeManifestPath(), data, 0o600)
}

// EnsureNativeManifest resets the manifest when version/protocol changes and
// heals per-size counts when version/protocol already match.
func (s *Store) EnsureNativeManifest(version, protocol string) error {
	m, err := s.ReadNativeManifest()
	if err != nil {
		return err
	}
	if m.Version != version || m.Protocol != protocol {
		m = NativeManifest{
			Version:  version,
			Protocol: protocol,
			Sizes:    map[int]NativeSizeStatus{},
		}
		return s.WriteNativeManifest(m)
	}
	return nil
}

// InitNativeSizePhase prepares manifest tracking for a warm phase.
func (s *Store) InitNativeSizePhase(cells, total int) error {
	m, err := s.ReadNativeManifest()
	if err != nil {
		return err
	}
	if m.Sizes == nil {
		m.Sizes = map[int]NativeSizeStatus{}
	}
	entry := m.Sizes[cells]
	entry.Total = total
	entry.Complete = false
	if counts, err := s.auditNativeFrameCounts(m.Protocol); err == nil {
		done := counts[cells]
		entry.Done = done
		entry.Complete = total > 0 && done >= total
	}
	m.Sizes[cells] = entry
	return s.WriteNativeManifest(m)
}

// MarkNativeSizeComplete marks a cell bucket complete and recounts disk.
func (s *Store) MarkNativeSizeComplete(cells, total int) error {
	m, err := s.ReadNativeManifest()
	if err != nil {
		return err
	}
	if m.Sizes == nil {
		m.Sizes = map[int]NativeSizeStatus{}
	}
	counts, err := s.auditNativeFrameCounts(m.Protocol)
	if err != nil {
		return err
	}
	done := counts[cells]
	entry := m.Sizes[cells]
	entry.Done = done
	if total > 0 {
		entry.Total = total
	}
	entry.Complete = total > 0 && done >= total
	m.Sizes[cells] = entry
	return s.WriteNativeManifest(m)
}

// RecountNativeSize updates done count from disk for one bucket.
func (s *Store) RecountNativeSize(cells int) (int, error) {
	m, err := s.ReadNativeManifest()
	if err != nil {
		return 0, err
	}
	counts, err := s.auditNativeFrameCounts(m.Protocol)
	if err != nil {
		return 0, err
	}
	done := counts[cells]
	if m.Sizes == nil {
		m.Sizes = map[int]NativeSizeStatus{}
	}
	entry := m.Sizes[cells]
	entry.Done = done
	entry.Complete = entry.Total > 0 && done >= entry.Total
	m.Sizes[cells] = entry
	if err := s.WriteNativeManifest(m); err != nil {
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
	m, err := s.ReadNativeManifest()
	if err != nil || m.Version != sprite.AlgorithmVersion {
		return false
	}
	entry, ok := m.Sizes[cells]
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
	m, err := s.ReadNativeManifest()
	if err != nil {
		return err
	}
	if len(m.Sizes) == 0 {
		return nil
	}
	counts, err := s.auditNativeFrameCounts(m.Protocol)
	if err != nil {
		return err
	}
	for cells := range m.Sizes {
		entry := m.Sizes[cells]
		entry.Done = counts[cells]
		entry.Complete = entry.Total > 0 && entry.Done >= entry.Total
		m.Sizes[cells] = entry
	}
	return s.WriteNativeManifest(m)
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

func (s *Service) RecountNativeSize(cells int) (int, error) {
	return s.store.RecountNativeSize(cells)
}

func (s *Service) NativeSizeComplete(cells, indexTotal int) bool {
	return s.store.NativeSizeComplete(cells, indexTotal)
}
