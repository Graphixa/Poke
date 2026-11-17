package cache_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Graphixa/Poke/internal/cache"
	"github.com/Graphixa/Poke/internal/render/sprite"
)

func TestChangedArtworkDeletesOldNativeFiles(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	ctx := context.Background()
	oldHash := fmt.Sprintf("%064x", 1)
	newHash := fmt.Sprintf("%064x", 99)
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 1}
	if err := store.StoreNamedSpriteWithMeta(ctx, 1, "p1", cache.SpriteKindArtwork, "", oldHash, png); err != nil {
		t.Fatal(err)
	}
	w, h := sprite.FrameSizeFor(sprite.MaxFrameSize)
	oldKey := sprite.NativeCacheKey(oldHash, "sixel", w, h, sprite.DefaultBackground, 64)
	if err := store.StoreNative(ctx, oldKey, []byte("FRAME")); err != nil {
		t.Fatal(err)
	}
	if !store.HasNativeFile(oldKey) {
		t.Fatal("expected native file for old hash")
	}
	if err := store.StoreNamedSpriteWithMeta(ctx, 1, "p1", cache.SpriteKindArtwork, "", newHash, png); err != nil {
		t.Fatal(err)
	}
	if store.HasNativeFile(oldKey) {
		t.Fatal("old native file should be removed after artwork hash change")
	}
}
