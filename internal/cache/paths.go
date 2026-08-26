package cache

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	// SpriteFileFrontDefault is the PNG filename under sprites/ and official-artwork/.
	SpriteFileFrontDefault = "front_default.png"

	// SpriteKindPixel is the named pixel sprite tree (sprites/).
	SpriteKindPixel = "pixel"
	// SpriteKindArtwork is the official-artwork tree.
	SpriteKindArtwork = "artwork"
)

// PokemonFolderKey returns {id:03d}-{slug} for cache folders.
func PokemonFolderKey(id int, slug string) string {
	return fmt.Sprintf("%03d-%s", id, SanitizeSlug(slug))
}

// SanitizeSlug keeps path-safe slug characters (lowercase letters, digits, hyphen).
func SanitizeSlug(slug string) string {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		return "unknown"
	}
	var b strings.Builder
	b.Grow(len(slug))
	for _, r := range slug {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		case unicode.IsSpace(r), r == '_':
			b.WriteByte('-')
		default:
			// drop path separators and other junk
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "unknown"
	}
	if strings.Contains(out, "..") {
		return "unknown"
	}
	return out
}

// SpriteRelPath returns the cache-relative pixel sprite path for an index hint.
func SpriteRelPath(id int, slug string) string {
	return filepath.ToSlash(filepath.Join("sprites", PokemonFolderKey(id, slug), SpriteFileFrontDefault))
}

// ArtworkRelPath returns the cache-relative official-artwork path.
func ArtworkRelPath(id int, slug string) string {
	return filepath.ToSlash(filepath.Join("official-artwork", PokemonFolderKey(id, slug), SpriteFileFrontDefault))
}
