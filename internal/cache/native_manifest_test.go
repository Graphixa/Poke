package cache_test

import (
	"context"
	"testing"

	"github.com/Graphixa/Pokedex/internal/cache"
	"github.com/Graphixa/Pokedex/internal/config"
	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

func TestNativeManifestSizeComplete(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)

	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		t.Fatal(err)
	}
	if err := store.InitNativeSizePhase(sprite.MaxFrameSize, 2); err != nil {
		t.Fatal(err)
	}
	if store.NativeSizeComplete(sprite.MaxFrameSize, 2) {
		t.Fatal("expected incomplete before frames")
	}

	bg := sprite.DefaultBackground
	key := sprite.NativeCacheKey("named-25-pikachu-artwork", "sixel", 40, 20, bg)
	if err := svc.StoreNative(context.Background(), key, "FRAME"); err != nil {
		t.Fatal(err)
	}
	key2 := sprite.NativeCacheKey("named-26-raichu-artwork", "sixel", 40, 20, bg)
	if err := svc.StoreNative(context.Background(), key2, "FRAME2"); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkNativeSizeComplete(sprite.MaxFrameSize, 2); err != nil {
		t.Fatal(err)
	}
	if !store.NativeSizeComplete(sprite.MaxFrameSize, 2) {
		t.Fatal("expected complete after two frames")
	}
	if !svc.NativeSizeComplete(sprite.MaxFrameSize, 2) {
		t.Fatal("service wrapper expected complete")
	}
}

func TestInvalidateNativeSpeciesUpdatesManifest(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewStore(dir, nil)
	svc := cache.NewService(store, nil, config.Config{}, nil)

	if err := store.EnsureNativeManifest(sprite.AlgorithmVersion, "sixel"); err != nil {
		t.Fatal(err)
	}
	if err := store.InitNativeSizePhase(sprite.MaxFrameSize, 1); err != nil {
		t.Fatal(err)
	}
	bg := sprite.DefaultBackground
	key := sprite.NativeCacheKey("named-25-pikachu-artwork", "sixel", 40, 20, bg)
	if err := svc.StoreNative(context.Background(), key, "FRAME"); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkNativeSizeComplete(sprite.MaxFrameSize, 1); err != nil {
		t.Fatal(err)
	}
	if err := svc.InvalidateNativeSpecies(25); err != nil {
		t.Fatal(err)
	}
	if store.NativeSizeComplete(sprite.MaxFrameSize, 1) {
		t.Fatal("expected incomplete after invalidate")
	}
	if _, err := store.Native(context.Background(), key); err == nil {
		t.Fatal("expected frame deleted")
	}
}
