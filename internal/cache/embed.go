package cache

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/Graphixa/Poke/internal/dataset"
)

// embeddedJSON is the aggregate files a snapshot must contain.
// bootstrap.json is a crawl report and is not required.
var embeddedJSON = []string{
	dataset.FileBrowse,
	dataset.FilePokemon,
	dataset.FileSpecies,
	dataset.FileLearnsets,
	dataset.FileMoves,
	dataset.FileEvolutions,
	dataset.FileEncounters,
	dataset.FileVersions,
	dataset.FileVersionGroups,
	dataset.FileTypes,
	dataset.FileAbilities,
	dataset.FileForms,
}

// Unpack gunzips a dataset tar into a new snapshot directory and activates it.
// A complete active snapshot with the same source_sha is left unchanged.
// Validation failures return before any active pointer write.
func (s *Store) Unpack(ctx context.Context, gz []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	files, err := readTarGz(gz)
	if err != nil {
		return fmt.Errorf("cache: embedded dataset: %w", err)
	}
	raw, ok := files[dataset.FileManifest]
	if !ok {
		return fmt.Errorf("cache: embedded dataset: missing manifest")
	}
	var man DatasetManifest
	if err := json.Unmarshal(raw, &man); err != nil {
		return fmt.Errorf("cache: embedded dataset: %w", err)
	}
	if err := usableEmbed(man, files); err != nil {
		return err
	}
	if s.embeddedCurrent(man.SourceSHA) {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	id, err := s.unusedDatasetID()
	if err != nil {
		return err
	}
	man.DatasetVersion = id
	body, err := json.Marshal(man)
	if err != nil {
		return err
	}
	files[dataset.FileManifest] = body
	for name, data := range files {
		if err := ctx.Err(); err != nil {
			_ = s.RemoveDatasetDir(id)
			return err
		}
		if err := writeAtomicRename(filepath.Join(s.datasetDir(id), name), data, 0o600); err != nil {
			_ = s.RemoveDatasetDir(id)
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		_ = s.RemoveDatasetDir(id)
		return err
	}
	if err := s.ActivateDataset(id); err != nil {
		_ = s.RemoveDatasetDir(id)
		return err
	}
	return nil
}

func usableEmbed(man DatasetManifest, files map[string][]byte) error {
	if man.SchemaVersion != dataset.SchemaVersion {
		return fmt.Errorf("cache: embedded dataset: schema %d", man.SchemaVersion)
	}
	if !man.Complete || (man.SourceSHA == "" && man.PokemonCount == 0) {
		return fmt.Errorf("cache: embedded dataset: incomplete")
	}
	for _, name := range embeddedJSON {
		if _, ok := files[name]; !ok {
			return fmt.Errorf("cache: embedded dataset: missing %s", name)
		}
	}
	return nil
}

func (s *Store) embeddedCurrent(sha string) bool {
	if sha == "" {
		return false
	}
	id, err := s.ActiveDatasetID()
	if err != nil {
		return false
	}
	man, err := s.ReadDatasetManifest(id)
	if err != nil || !man.Complete || !supportedDatasetSchema(man.SchemaVersion) {
		return false
	}
	return man.SourceSHA == sha
}

func (s *Store) unusedDatasetID() (string, error) {
	base := DatasetIDForTime(s.clock.Now())
	id := base
	for n := 2; n < 1000; n++ {
		_, err := os.Stat(s.datasetDir(id))
		if errors.Is(err, os.ErrNotExist) {
			return id, nil
		}
		if err != nil {
			return "", err
		}
		id = fmt.Sprintf("%s-%d", base, n)
	}
	return "", fmt.Errorf("cache: no free dataset id")
}

func readTarGz(gz []byte) (map[string][]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	tr := tar.NewReader(zr)
	files := map[string][]byte{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue
		}
		name := path.Clean(hdr.Name)
		if name != path.Base(name) || name == "." || name == ".." {
			return nil, fmt.Errorf("bad archive path %q", hdr.Name)
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		files[name] = body
	}
	return files, nil
}
