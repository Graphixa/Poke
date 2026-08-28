package cache_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"image/color"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
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

	key := sprite.CacheKey(hash, sprite.SourcePixel, "chafa", 8, 8, sprite.ColorTrueColor, color.RGBA{A: 255})
	if err := svc.StoreHalfblock(context.Background(), key, "ANSI"); err != nil {
		t.Fatal(err)
	}
	ansi, err := svc.Halfblock(context.Background(), key)
	if err != nil || ansi != "ANSI" {
		t.Fatalf("Halfblock = %q err=%v", ansi, err)
	}

	// Dimension change → different key → miss (invalidation).
	other := sprite.CacheKey(hash, sprite.SourcePixel, "chafa", 10, 10, sprite.ColorTrueColor, color.RGBA{A: 255})
	if _, err := svc.Halfblock(context.Background(), other); err == nil {
		t.Fatal("expected miss for different size key")
	}
	if err := svc.StoreHalfblock(context.Background(), "../escape", "x"); err == nil {
		t.Fatal("expected reject of invalid key")
	}

	nkey := sprite.NativeCacheKey("named-25-pikachu-artwork", "sixel", 40, 20, color.RGBA{R: 0x31, G: 0x32, B: 0x44, A: 255})
	if err := svc.StoreNative(context.Background(), nkey, "FRAME"); err != nil {
		t.Fatal(err)
	}
	frame, err := svc.Native(context.Background(), nkey)
	if err != nil || frame != "FRAME" {
		t.Fatalf("Native = %q err=%v", frame, err)
	}
	if _, err := os.Stat(filepath.Join(root, "native", nkey+".frame")); err != nil {
		t.Fatal(err)
	}
	if err := svc.StoreNative(context.Background(), "../escape", "x"); err == nil {
		t.Fatal("expected reject of invalid native key")
	}

	if err := svc.StoreNamedSpriteWithURL(context.Background(), 25, "pikachu", cache.SpriteKindArtwork,
		sprite.CDNArtworkURL(25), "deadbeef", png); err != nil {
		t.Fatal(err)
	}
	meta, err := store.ReadNamedSpriteMeta(25, "pikachu", cache.SpriteKindArtwork)
	if err != nil || meta.URL != sprite.CDNArtworkURL(25) || meta.SHA256 != "deadbeef" {
		t.Fatalf("meta=%+v err=%v", meta, err)
	}
}
