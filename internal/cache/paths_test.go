package cache

import "testing"

func TestPokemonFolderKey(t *testing.T) {
	if got := PokemonFolderKey(25, "pikachu"); got != "025-pikachu" {
		t.Fatalf("got %q", got)
	}
	if got := PokemonFolderKey(6, "Charizard!"); got != "006-charizard" {
		t.Fatalf("sanitize got %q", got)
	}
	if got := SpriteRelPath(25, "pikachu"); got != "sprites/025-pikachu/front_default.png" {
		t.Fatalf("sprite path %q", got)
	}
}

func TestNamedSpriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir, systemClock{})
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0}
	if err := store.StoreNamedSprite(t.Context(), 25, "pikachu", SpriteKindPixel, png); err != nil {
		t.Fatal(err)
	}
	got, err := store.NamedSprite(t.Context(), 25, "pikachu", SpriteKindPixel)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(png) {
		t.Fatalf("mismatch")
	}
}
