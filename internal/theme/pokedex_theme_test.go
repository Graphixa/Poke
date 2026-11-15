package theme_test

import (
	"github.com/Graphixa/Pokedex/internal/theme"
	"testing"
)

func TestLoadEmbeddedPokedex(t *testing.T) {
	f, err := theme.LoadEmbedded("pokedex")
	if err != nil {
		t.Fatal(err)
	}
	if f.ThemeName != "Pokédex" || f.Style != "dark" || f.Colors.Primary != "#F22B4E" {
		t.Fatalf("unexpected theme: %+v", f)
	}
	if err := theme.Validate(f); err != nil {
		t.Fatal(err)
	}
}
