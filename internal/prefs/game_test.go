package prefs_test

import (
	"path/filepath"
	"testing"

	"github.com/Graphixa/Pokedex/internal/prefs"
)

func TestGameStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.json")
	s := prefs.GameStore{Path: path}
	id, err := s.Load()
	if err != nil || id != 0 {
		t.Fatalf("empty load id=%d err=%v", id, err)
	}
	if err := s.Save(40); err != nil {
		t.Fatal(err)
	}
	id, err = s.Load()
	if err != nil || id != 40 {
		t.Fatalf("id=%d err=%v", id, err)
	}
}
