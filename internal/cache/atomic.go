package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// schemaVersion 2: per-species dossiers + named sprite folders (see plans/DECISIONS.md).
const schemaVersion = 2

type envelope struct {
	SchemaVersion int             `json:"schema_version"`
	Source        string          `json:"source"`
	FetchedAt     time.Time       `json:"fetched_at"`
	Data          json.RawMessage `json:"data"`
}

type manifest struct {
	SchemaVersion  int       `json:"schema_version"`
	IndexFetchedAt time.Time `json:"index_fetched_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// writeAtomicRename persists with temp+rename only (no fsync). Suitable for
// reconstructable staging files and derived native frames.
func writeAtomicRename(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)

	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return fmt.Errorf("chmod temporary file: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("rename temporary file: %w", err)
	}
	return nil
}

// writeAtomic persists critical metadata with temp+sync+rename.
func writeAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)

	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return fmt.Errorf("chmod temporary file: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("rename temporary file: %w", err)
	}
	return nil
}

func encodeEnvelope(fetchedAt time.Time, value any) ([]byte, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return json.Marshal(envelope{
		SchemaVersion: schemaVersion,
		Source:        "pokeapi",
		FetchedAt:     fetchedAt.UTC(),
		Data:          payload,
	})
}

func decodeEnvelope(data []byte, dest any) (time.Time, error) {
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return time.Time{}, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	if env.SchemaVersion != schemaVersion {
		return time.Time{}, fmt.Errorf("%w: unsupported schema version %d", ErrCorrupt, env.SchemaVersion)
	}
	if err := json.Unmarshal(env.Data, dest); err != nil {
		return time.Time{}, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	return env.FetchedAt, nil
}
