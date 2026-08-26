package sprite

import (
	"fmt"
	"image/color"
	"strings"
)

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// BackgroundHash returns a short hex identity for the theme background RGB.
func BackgroundHash(bg color.RGBA) string {
	if bg.A == 0 {
		bg = DefaultBackground
	}
	return fmt.Sprintf("%02x%02x%02x", bg.R, bg.G, bg.B)
}

// CacheKey builds a deterministic artefact identity (filesystem-safe):
//
//	<hash>-<source>-<mode>-<w>x<h>-<color>-<bg>-<version>
//
// Example: 004abc-pixel-chafa-32x16-truecolor-000000-v7
func CacheKey(contentHash string, source SpriteSource, mode string, w, h int, color ColorMode, bg color.RGBA) string {
	if source == "" {
		source = SourcePixel
	}
	if color == "" || color == ColorAuto {
		color = ColorTrueColor
	}
	return fmt.Sprintf("%s-%s-%s-%dx%d-%s-%s-%s",
		contentHash, source, mode, w, h, color, BackgroundHash(bg), AlgorithmVersion)
}

// ParseCacheKey splits a key produced by CacheKey.
func ParseCacheKey(key string) (hash string, source SpriteSource, mode string, w, h int, color ColorMode, bgHash, version string, err error) {
	parts := strings.Split(key, "-")
	// hash + source + mode + dim + color + bg + version = at least 7 segments.
	if len(parts) < 7 {
		return "", "", "", 0, 0, "", "", "", fmt.Errorf("sprite: invalid cache key %q", key)
	}
	version = parts[len(parts)-1]
	bgHash = parts[len(parts)-2]
	color = ColorMode(parts[len(parts)-3])
	dim := parts[len(parts)-4]
	mode = parts[len(parts)-5]
	source = SpriteSource(parts[len(parts)-6])
	hash = strings.Join(parts[:len(parts)-6], "-")
	if _, err = fmt.Sscanf(dim, "%dx%d", &w, &h); err != nil {
		return "", "", "", 0, 0, "", "", "", fmt.Errorf("sprite: invalid dimensions in key %q", key)
	}
	if hash == "" || mode == "" || w < 1 || h < 1 || bgHash == "" {
		return "", "", "", 0, 0, "", "", "", fmt.Errorf("sprite: invalid cache key %q", key)
	}
	return hash, source, mode, w, h, color, bgHash, version, nil
}
