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

// NativeManifest records native cache version/protocol for upgrade invalidation.
// Runtime cache hits use exact content-addressed key existence on disk, not manifest totals.
type NativeManifest struct {
	Version  string `json:"version"`
	Protocol string `json:"protocol"`
}

func (s *Store) nativeManifestPath() string {
	return filepath.Join(s.root, "native", "manifest.json")
}

func (s *Store) readNativeManifestDisk() (NativeManifest, error) {
	path := s.nativeManifestPath()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return NativeManifest{}, nil
	}
	if err != nil {
		return NativeManifest{}, err
	}
	var m NativeManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return NativeManifest{}, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	return m, nil
}

// ReadNativeManifest loads the native cache manifest when present.
func (s *Store) ReadNativeManifest() (NativeManifest, error) {
	return s.readNativeManifestDisk()
}

// WriteNativeManifest persists the native cache manifest.
func (s *Store) WriteNativeManifest(m NativeManifest) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return writeAtomic(s.nativeManifestPath(), data, 0o600)
}

// EnsureNativeManifest resets the manifest and purges incompatible frames when
// version or protocol changes.
func (s *Store) EnsureNativeManifest(version, protocol string) error {
	m, err := s.readNativeManifestDisk()
	if err != nil {
		return err
	}
	if m.Version == version && m.Protocol == protocol {
		return nil
	}
	if err := s.purgeIncompatibleNativeArtefacts(version); err != nil {
		return err
	}
	return s.WriteNativeManifest(NativeManifest{Version: version, Protocol: protocol})
}

// CountNativeSpeciesForCells counts unique source hashes with native frames at cells.
// Diagnostic/audit only — not used on the browse hot path.
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
		if !isNativeArtefactName(name) {
			continue
		}
		if !strings.Contains(name, dimTag) || !strings.Contains(name, protoNeedle) {
			continue
		}
		key, _ := nativeKeyFromFilename(name)
		hash, proto, fw, fh, _, _, _, parseErr := sprite.ParseNativeCacheKey(key)
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
	key, ok := nativeKeyFromFilename(name)
	if !ok {
		return 0
	}
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

func (s *Service) EnsureNativeManifest(protocol string) error {
	return s.store.EnsureNativeManifest(sprite.AlgorithmVersion, protocol)
}
