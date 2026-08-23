package cache_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/render/halfblock"
)

func TestSpriteAndHalfblockArtefacts(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	svc := cache.NewService(store, &fakeAPI{}, testConfig(), clock)

	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00}
	hash := "deadbeef"
	url := "https://example.test/sprite.png"
	if err := svc.StoreSpriteByURL(context.Background(), url, hash, png); err != nil {
		t.Fatal(err)
	}
	gotHash, gotPNG, err := svc.SpriteByURL(context.Background(), url)
	if err != nil || gotHash != hash || string(gotPNG) != string(png) {
		t.Fatalf("SpriteByURL = %s %q err=%v", gotHash, gotPNG, err)
	}
	if _, err := os.Stat(filepath.Join(root, "sprites", hash+".png")); err != nil {
		t.Fatal(err)
	}

	key := halfblock.CacheKey(hash, halfblock.Options{
		Width: 8, Height: 8, Palette: halfblock.PaletteTrueColor,
	})
	if err := svc.StoreHalfblock(context.Background(), key, "ANSI"); err != nil {
		t.Fatal(err)
	}
	ansi, err := svc.Halfblock(context.Background(), key)
	if err != nil || ansi != "ANSI" {
		t.Fatalf("Halfblock = %q err=%v", ansi, err)
	}

	// Dimension change → different key → miss (invalidation).
	other := halfblock.CacheKey(hash, halfblock.Options{
		Width: 10, Height: 10, Palette: halfblock.PaletteTrueColor,
	})
	if _, err := svc.Halfblock(context.Background(), other); err == nil {
		t.Fatal("expected miss for different size key")
	}
	if err := svc.StoreHalfblock(context.Background(), "../escape", "x"); err == nil {
		t.Fatal("expected reject of invalid key")
	}
}
