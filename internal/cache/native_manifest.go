package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

// NativeSizeStatus tracks warm completeness for one cell bucket.
type NativeSizeStatus struct {
	Done     int  `json:"done"`
	Total    int  `json:"total"`
	Complete bool `json:"complete,omitempty"`
}

// NativeManifest records per-size native frame cache completeness.
type NativeManifest struct {
	Version  string                     `json:"version"`
	Protocol string                     `json:"protocol"`
	Sizes    map[int]NativeSizeStatus   `json:"sizes"`
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
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(s.nativeManifestPath(), data, 0o600)
}

// EnsureNativeManifest resets the manifest when version/protocol changes.
func (s *Store) EnsureNativeManifest(version, protocol string) error {
	m, err := s.ReadNativeManifest()
	if err != nil {
		return err
	}
	if m.Version == version && m.Protocol == protocol {
		return nil
	}
	m = NativeManifest{
		Version:  version,
		Protocol: protocol,
		Sizes:    map[int]NativeSizeStatus{},
	}
	return s.WriteNativeManifest(m)
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
	if done, err := s.CountNativeSpeciesForCells(cells, m.Protocol); err == nil {
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
	done, err := s.CountNativeSpeciesForCells(cells, m.Protocol)
	if err != nil {
		return err
	}
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
	done, err := s.CountNativeSpeciesForCells(cells, m.Protocol)
	if err != nil {
		return 0, err
	}
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
	if entry.Complete {
		return true
	}
	if entry.Total > 0 && entry.Done >= entry.Total {
		return true
	}
	if entry.Done >= indexTotal {
		return true
	}
	// Fallback: trust on-disk recount when manifest is stale.
	done, err := s.CountNativeSpeciesForCells(cells, m.Protocol)
	if err != nil {
		return false
	}
	return done >= indexTotal
}

// CountNativeSpeciesForCells counts unique species with native frames at cells.
func (s *Store) CountNativeSpeciesForCells(cells int, protocol string) (int, error) {
	w, h := sprite.FrameSizeFor(cells)
	dimTag := fmt.Sprintf("-%dx%d-", w, h)
	protoNeedle := "image"
	if protocol != "" {
		protoNeedle = "image-" + protocol
	}
	dir := filepath.Join(s.root, "native")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	seen := make(map[int]bool)
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
		id := speciesIDFromNativeFilename(name)
		if id > 0 {
			seen[id] = true
		}
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
	for cells := range m.Sizes {
		if _, err := s.RecountNativeSize(cells); err != nil {
			return err
		}
	}
	return nil
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
