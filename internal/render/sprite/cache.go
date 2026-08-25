package sprite

import (
	"fmt"
	"strings"
)

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// CacheKey builds a deterministic artefact identity (filesystem-safe):
//
//	<hash>-<source>-<mode>-<w>x<h>-<color>-<version>
//
// Example: 004abc-pixel-chafa-32x32-truecolor-v4
func CacheKey(contentHash string, source SpriteSource, mode string, w, h int, color ColorMode) string {
	if source == "" {
		source = SourcePixel
	}
	if color == "" || color == ColorAuto {
		color = ColorTrueColor
	}
	return fmt.Sprintf("%s-%s-%s-%dx%d-%s-%s",
		contentHash, source, mode, w, h, color, AlgorithmVersion)
}

// ParseCacheKey splits a key produced by CacheKey.
func ParseCacheKey(key string) (hash string, source SpriteSource, mode string, w, h int, color ColorMode, version string, err error) {
	parts := strings.Split(key, "-")
	// hash (hex) + source + mode + dim + color + version = at least 6 segments;
	// hash itself has no hyphens (sha256 hex).
	if len(parts) < 6 {
		return "", "", "", 0, 0, "", "", fmt.Errorf("sprite: invalid cache key %q", key)
	}
	version = parts[len(parts)-1]
	color = ColorMode(parts[len(parts)-2])
	dim := parts[len(parts)-3]
	mode = parts[len(parts)-4]
	source = SpriteSource(parts[len(parts)-5])
	hash = strings.Join(parts[:len(parts)-5], "-")
	if _, err = fmt.Sscanf(dim, "%dx%d", &w, &h); err != nil {
		return "", "", "", 0, 0, "", "", fmt.Errorf("sprite: invalid dimensions in key %q", key)
	}
	if hash == "" || mode == "" || w < 1 || h < 1 {
		return "", "", "", 0, 0, "", "", fmt.Errorf("sprite: invalid cache key %q", key)
	}
	return hash, source, mode, w, h, color, version, nil
}
