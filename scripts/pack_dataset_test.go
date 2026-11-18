package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Graphixa/Poke/internal/dataset"
)

func TestPackCheckRejectsIncomplete(t *testing.T) {
	dir := t.TempDir()
	man := []byte(`{"schema_version":4,"complete":false,"pokemon_count":0}`)
	for _, name := range archiveNames() {
		body := []byte("{}")
		if name == dataset.FileManifest {
			body = man
		}
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "LICENSE.txt"), []byte("bsd"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "dataset.tar.gz")
	if err := pack(dir, out, "abc"); err != nil {
		t.Fatal(err)
	}
	if err := check(out); err == nil {
		t.Fatal("incomplete archive passed the release check")
	}
}

func TestPackCheckAcceptsComplete(t *testing.T) {
	dir := t.TempDir()
	man := []byte(`{"schema_version":4,"complete":true,"pokemon_count":1}`)
	for _, name := range archiveNames() {
		body := []byte("{}")
		if name == dataset.FileManifest {
			body = man
		}
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "LICENSE.txt"), []byte("bsd"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "dataset.tar.gz")
	if err := pack(dir, out, "abc"); err != nil {
		t.Fatal(err)
	}
	if err := check(out); err != nil {
		t.Fatal(err)
	}
}
