// Package favourites stores user choices separately from disposable caches.
package favourites

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

type Store struct{ Path string }

type document struct {
	Version int   `json:"version"`
	IDs     []int `json:"species_ids"`
}

func (s Store) Load() ([]int, error) {
	b, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var d document
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	if d.Version != 1 {
		return nil, fmt.Errorf("unsupported favourites version %d", d.Version)
	}
	return normalize(d.IDs)
}

// Save writes a complete temporary document before replacing the previous file.
func (s Store) Save(ids []int) error {
	ids, err := normalize(ids)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(document{Version: 1, IDs: ids}, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".favourites-*")
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

func normalize(ids []int) ([]int, error) {
	ids = slices.Clone(ids)
	for _, id := range ids {
		if id <= 0 {
			return nil, fmt.Errorf("invalid favourite species ID %d", id)
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids), nil
}
