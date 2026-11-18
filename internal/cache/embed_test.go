package cache_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Graphixa/Poke/internal/cache"
	"github.com/Graphixa/Poke/internal/dataset"
)

func TestUnpackActivatesAndSkipsSameSHA(t *testing.T) {
	root := t.TempDir()
	store := cache.NewStore(root, fixedClock{t: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)})
	gz := packGzip(t, cache.DatasetManifest{
		SchemaVersion: cache.DatasetSchemaVersion,
		Complete:      true,
		SourceSHA:     "abc",
	}, emptyAggregates())
	if err := store.Unpack(context.Background(), gz); err != nil {
		t.Fatal(err)
	}
	id, err := store.ActiveDatasetID()
	if err != nil || id == "" {
		t.Fatalf("active=%q err=%v", id, err)
	}
	man, err := store.ReadDatasetManifest(id)
	if err != nil || man.SourceSHA != "abc" || !man.Complete {
		t.Fatalf("manifest sha=%q complete=%v err=%v", man.SourceSHA, man.Complete, err)
	}
	if n := datasetDirs(t, root); n != 1 {
		t.Fatalf("dataset dirs=%d", n)
	}
	if err := store.Unpack(context.Background(), gz); err != nil {
		t.Fatal(err)
	}
	id2, err := store.ActiveDatasetID()
	if err != nil || id2 != id {
		t.Fatalf("active=%q want %s err=%v", id2, id, err)
	}
	if n := datasetDirs(t, root); n != 1 {
		t.Fatalf("second unpack wrote another id, dirs=%d", n)
	}
}

func TestUnpackIncompleteDoesNotActivate(t *testing.T) {
	root := t.TempDir()
	store := cache.NewStore(root, fixedClock{t: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)})
	const kept = "kept"
	if err := store.EnsureDatasetDir(kept); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteDatasetManifest(cache.DatasetManifest{
		SchemaVersion:  cache.DatasetSchemaVersion,
		DatasetVersion: kept,
		SourceSHA:      "old",
		Complete:       true,
		PokemonCount:   1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.ActivateDataset(kept); err != nil {
		t.Fatal(err)
	}
	gz := packGzip(t, cache.DatasetManifest{
		SchemaVersion: cache.DatasetSchemaVersion,
		Complete:      false,
		SourceSHA:     "nope",
	}, nil)
	if err := store.Unpack(context.Background(), gz); err == nil {
		t.Fatal("expected incomplete embed to fail")
	}
	got, err := store.ActiveDatasetID()
	if err != nil || got != kept {
		t.Fatalf("active=%q err=%v", got, err)
	}
}

func TestUnpackCanceled(t *testing.T) {
	root := t.TempDir()
	store := cache.NewStore(root, fixedClock{t: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)})
	const kept = "kept"
	if err := store.EnsureDatasetDir(kept); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteDatasetManifest(cache.DatasetManifest{
		SchemaVersion:  cache.DatasetSchemaVersion,
		DatasetVersion: kept,
		SourceSHA:      "old",
		Complete:       true,
		PokemonCount:   1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.ActivateDataset(kept); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	gz := packGzip(t, cache.DatasetManifest{
		SchemaVersion: cache.DatasetSchemaVersion,
		Complete:      true,
		SourceSHA:     "abc",
	}, emptyAggregates())
	err := store.Unpack(ctx, gz)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	got, idErr := store.ActiveDatasetID()
	if idErr != nil || got != kept {
		t.Fatalf("active=%q err=%v", got, idErr)
	}
}

func emptyAggregates() map[string][]byte {
	names := []string{
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
	files := make(map[string][]byte, len(names))
	for _, name := range names {
		files[name] = []byte("{}")
	}
	return files
}

func packGzip(t *testing.T, man cache.DatasetManifest, files map[string][]byte) []byte {
	t.Helper()
	if files == nil {
		files = map[string][]byte{}
	}
	raw, err := json.Marshal(man)
	if err != nil {
		t.Fatal(err)
	}
	files[dataset.FileManifest] = raw
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for name, body := range files {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func datasetDirs(t *testing.T, root string) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "datasets"))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			n++
		}
	}
	return n
}
