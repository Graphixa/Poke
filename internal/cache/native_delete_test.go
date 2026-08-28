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
	key := sprite.NativeCacheKey("named-25-pikachu-artwork", "sixel", 40, 20, sprite.DefaultBackground)
	if err := store.StoreNative(ctx, key, []byte("FRAME")); err != nil {
		t.Fatal(err)
	}
	other := sprite.NativeCacheKey("named-26-raichu-artwork", "sixel", 40, 20, sprite.DefaultBackground)
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
