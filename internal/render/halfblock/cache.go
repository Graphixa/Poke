package halfblock

import (
	"fmt"
	"image/color"
	"strings"
)

// CacheKey builds a deterministic artefact key from source hash and render options.
// Format: <source-hash>-<width>x<height>-<palette>-<background>
func CacheKey(sourceHash string, opts Options) string {
	bg := opts.Background
	if bg.A == 0 {
		bg = DefaultBackground
	}
	palette := opts.Palette
	if palette == "" {
		palette = PaletteTrueColor
	}
	return fmt.Sprintf("%s-%dx%d-%s-%s",
		sourceHash,
		opts.Width,
		opts.Height,
		string(palette),
		backgroundHex(bg),
	)
}

// ArtefactFilename returns the on-disk name for a cache key.
func ArtefactFilename(key string) string {
	return key + ".ansi"
}

func backgroundHex(c color.RGBA) string {
	return fmt.Sprintf("%02x%02x%02x", c.R, c.G, c.B)
}

// ParseCacheKey validates and splits a cache key produced by CacheKey.
func ParseCacheKey(key string) (sourceHash string, w, h int, palette PaletteMode, bgHex string, err error) {
	// hash may itself contain hyphens only if we keep it hex-only (sha256).
	// Format: <hash>-<w>x<h>-<palette>-<bg>
	parts := strings.Split(key, "-")
	if len(parts) < 4 {
		return "", 0, 0, "", "", fmt.Errorf("halfblock: invalid cache key %q", key)
	}
	bgHex = parts[len(parts)-1]
	palette = PaletteMode(parts[len(parts)-2])
	dim := parts[len(parts)-3]
	sourceHash = strings.Join(parts[:len(parts)-3], "-")
	if _, err := fmt.Sscanf(dim, "%dx%d", &w, &h); err != nil {
		return "", 0, 0, "", "", fmt.Errorf("halfblock: invalid dimensions in key %q", key)
	}
	if sourceHash == "" || w < 1 || h < 1 {
		return "", 0, 0, "", "", fmt.Errorf("halfblock: invalid cache key %q", key)
	}
	return sourceHash, w, h, palette, bgHex, nil
}
