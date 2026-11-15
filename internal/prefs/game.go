// Package prefs stores durable user choices separate from disposable caches.
package prefs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const gameDocVersion = 1

// GameStore persists the dossier game-version preference.
type GameStore struct{ Path string }

type gameDoc struct {
	Version   int `json:"version"`
	VersionID int `json:"version_id"` // PokéAPI version id; 0 = unset
}

// Load returns the saved version id, or 0 if unset/missing.
func (s GameStore) Load() (int, error) {
	b, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var d gameDoc
	if err := json.Unmarshal(b, &d); err != nil {
		return 0, err
	}
	if d.Version != gameDocVersion {
		return 0, fmt.Errorf("unsupported game prefs version %d", d.Version)
	}
	if d.VersionID < 0 {
		return 0, fmt.Errorf("invalid version id %d", d.VersionID)
	}
	return d.VersionID, nil
}

// Save writes the confirmed game version id. versionID 0 clears the preference (All).
func (s GameStore) Save(versionID int) error {
	if versionID < 0 {
		return fmt.Errorf("invalid version id %d", versionID)
	}
	if versionID == 0 {
		err := os.Remove(s.Path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	b, err := json.MarshalIndent(gameDoc{Version: gameDocVersion, VersionID: versionID}, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".game-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(append(b, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), s.Path)
}
