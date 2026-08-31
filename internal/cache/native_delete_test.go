package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

func TestDeleteNativeForSpecies(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	ctx := context.Background()
	key := sprite.NativeCacheKey("named-25-pikachu-artwork", "sixel", 40, 20, sprite.DefaultBackground, 64)
	if err := store.StoreNative(ctx, key, []byte("FRAME")); err != nil {
		t.Fatal(err)
	}
	other := sprite.NativeCacheKey("named-26-raichu-artwork", "sixel", 40, 20, sprite.DefaultBackground, 64)
	if err := store.StoreNative(ctx, other, []byte("OTHER")); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteNativeForSpecies(25); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Native(ctx, key); err == nil {
		t.Fatal("expected miss for species 25")
	}
	if _, err := store.Native(ctx, other); err != nil {
		t.Fatal("expected species 26 frame to remain")
	}
}

func TestDeleteNativeForSpeciesSHA256Key(t *testing.T) {
	root := t.TempDir()
	clock := fixedClock{t: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	store := cache.NewStore(root, clock)
	ctx := context.Background()
	sha25 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	sha26 := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	key25 := sprite.NativeCacheKey(sha25, "sixel", 320, 320, sprite.DefaultBackground, 64)
	key26 := sprite.NativeCacheKey(sha26, "sixel", 320, 320, sprite.DefaultBackground, 64)
	if err := store.StoreNative(ctx, key25, []byte("PIKA")); err != nil {
		t.Fatal(err)
	}
	if err := store.StoreNative(ctx, key26, []byte("RAI")); err != nil {
		t.Fatal(err)
	}
	png := []byte("png-bytes")
	if err := store.StoreNamedSpriteWithMeta(ctx, 25, "pikachu", "artwork", "http://x", sha25, png); err != nil {
		t.Fatal(err)
	}
	if err := store.StoreNamedSpriteWithMeta(ctx, 26, "raichu", "artwork", "http://y", sha26, png); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteNativeForSpecies(25); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Native(ctx, key25); err == nil {
		t.Fatal("expected SHA256 frame for species 25 deleted")
	}
	if _, err := store.Native(ctx, key26); err != nil {
		t.Fatal("expected species 26 SHA256 frame to remain")
	}
}
