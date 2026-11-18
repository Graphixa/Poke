package cache

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Graphixa/Poke/internal/config"
	"github.com/Graphixa/Poke/internal/dataset"
)

func TestRefreshSnapshotUsesETagAndSkipsSecondUnpack(t *testing.T) {
	gz := testSnapshot(t, "abc")
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write(gz)
	}))
	t.Cleanup(srv.Close)
	restore := useSnapshot(srv.URL)
	t.Cleanup(restore)

	svc := newSnapshotService(t)
	if err := svc.RefreshSnapshot(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	id, err := svc.store.ActiveDatasetID()
	if err != nil || id == "" {
		t.Fatalf("active=%q err=%v", id, err)
	}
	tag, err := os.ReadFile(filepath.Join(svc.store.Path(), "snapshot.etag"))
	if err != nil || string(tag) != `"v1"` {
		t.Fatalf("etag=%q err=%v", tag, err)
	}
	if err := svc.RefreshSnapshot(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	id2, err := svc.store.ActiveDatasetID()
	if err != nil || id2 != id {
		t.Fatalf("active=%q want %s err=%v", id2, id, err)
	}
	if hits != 2 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestRefreshSnapshotKeepsCacheWhenRepoRejects(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root, nil)
	if err := store.Unpack(context.Background(), testSnapshot(t, "old")); err != nil {
		t.Fatal(err)
	}
	kept, err := store.ActiveDatasetID()
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)
	restore := useSnapshot(srv.URL)
	t.Cleanup(restore)

	svc := NewService(store, nil, config.Default(), nil)
	if err := svc.RefreshSnapshot(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not a gzip"))
	}))
	t.Cleanup(bad.Close)
	SnapshotOverride = bad.URL
	if err := svc.RefreshSnapshot(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	got, err := store.ActiveDatasetID()
	if err != nil || got != kept {
		t.Fatalf("active=%q want %s err=%v", got, kept, err)
	}
}

func TestRefreshSnapshotEmptyCacheUsesEmbed(t *testing.T) {
	gz := testSnapshot(t, "embed")
	prev := embeddedSnapshot
	embeddedSnapshot = func() []byte { return gz }
	t.Cleanup(func() { embeddedSnapshot = prev })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)
	restore := useSnapshot(srv.URL)
	t.Cleanup(restore)

	svc := newSnapshotService(t)
	if err := svc.RefreshSnapshot(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	id, err := svc.store.ActiveDatasetID()
	if err != nil || id == "" {
		t.Fatalf("active=%q err=%v", id, err)
	}
	man, err := svc.store.ReadDatasetManifest(id)
	if err != nil || man.SourceSHA != "embed" {
		t.Fatalf("sha=%q err=%v", man.SourceSHA, err)
	}
}

func newSnapshotService(t *testing.T) *Service {
	t.Helper()
	return NewService(NewStore(t.TempDir(), nil), nil, config.Default(), nil)
}

func useSnapshot(url string) func() {
	prev := SnapshotOverride
	SnapshotOverride = url
	return func() { SnapshotOverride = prev }
}

func testSnapshot(t *testing.T, sha string) []byte {
	t.Helper()
	man, err := json.Marshal(DatasetManifest{
		SchemaVersion: DatasetSchemaVersion,
		Complete:      true,
		SourceSHA:     sha,
		PokemonCount:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{dataset.FileManifest: man}
	for _, name := range embeddedJSON {
		files[name] = []byte("{}")
	}
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
