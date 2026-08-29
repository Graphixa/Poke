package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImageManifestWriterBatches(t *testing.T) {
	store := NewStore(t.TempDir(), nil)
	w := newImageManifestWriter(store, ImageCacheManifest{
		Version: ImageCacheManifestVersion,
		SpecID:  "test",
		Pokemon: map[int]PokemonImageCacheState{},
	})
	for i := 0; i < 24; i++ {
		w.set(PokemonImageCacheState{SpeciesID: i + 1, SpecID: "test", Complete: true})
	}
	if err := w.maybeFlush(false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.Path(), "native", "image-manifest.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("expected no flush before 25 completions")
	}
	w.set(PokemonImageCacheState{SpeciesID: 25, SpecID: "test", Complete: true})
	if err := w.maybeFlush(false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("expected flush at 25 completions")
	}
}
