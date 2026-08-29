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
// Example: 004abc-pixel-chafa-32x16-truecolor-000000-v10
func CacheKey(contentHash string, source SpriteSource, mode string, w, h int, color ColorMode, bg color.RGBA) string {
	if source == "" {
		source = SourcePixel
	}
	if color == "" || color == ColorAuto {
		color = ColorTrueColor
	}
	version := AlgorithmVersion
	if mode == "chafa" {
		version = AlgorithmVersion + "-" + ChafaRenderVersion
	}
	return fmt.Sprintf("%s-%s-%s-%dx%d-%s-%s-%s",
		contentHash, source, mode, w, h, color, BackgroundHash(bg), version)
}

// ParseCacheKey splits a key produced by CacheKey.
func ParseCacheKey(key string) (hash string, source SpriteSource, mode string, w, h int, color ColorMode, bgHash, version string, err error) {
	parts := strings.Split(key, "-")
	if len(parts) < 7 {
		return "", "", "", 0, 0, "", "", "", fmt.Errorf("sprite: invalid cache key %q", key)
	}
	dimIdx := -1
	for i, p := range parts {
		var tw, th int
		if _, scanErr := fmt.Sscanf(p, "%dx%d", &tw, &th); scanErr == nil && tw > 0 && th > 0 {
			dimIdx = i
			w, h = tw, th
			break
		}
	}
	if dimIdx < 2 || dimIdx+3 >= len(parts) {
		return "", "", "", 0, 0, "", "", "", fmt.Errorf("sprite: invalid cache key %q", key)
	}
	mode = parts[dimIdx-1]
	source = SpriteSource(parts[dimIdx-2])
	hash = strings.Join(parts[:dimIdx-2], "-")
	color = ColorMode(parts[dimIdx+1])
	bgHash = parts[dimIdx+2]
	version = strings.Join(parts[dimIdx+3:], "-")
	if hash == "" || mode == "" || bgHash == "" || version == "" {
		return "", "", "", 0, 0, "", "", "", fmt.Errorf("sprite: invalid cache key %q", key)
	}
	return hash, source, mode, w, h, color, bgHash, version, nil
}

// NativeCacheKey builds a filesystem-safe identity for Kitty/iTerm/Sixel frames:
//
//	<hash>-image[-<proto>]-<w>x<h>-<bg>-<version>
//
// Example: named-25-pikachu-artwork-image-sixel-40x20-313244-v10
func NativeCacheKey(contentHash, proto string, w, h int, bg color.RGBA) string {
	tag := "image"
	if proto != "" {
		tag = "image-" + proto
	}
	pxW, pxH := PixelSize(w, h)
	return fmt.Sprintf("%s-%s-%dx%d-%s-%s",
		contentHash, tag, pxW, pxH, BackgroundHash(bg), AlgorithmVersion)
}

// ParseNativeCacheKey splits a key produced by NativeCacheKey.
func ParseNativeCacheKey(key string) (hash, proto string, w, h int, bgHash, version string, err error) {
	if key == "" || strings.Contains(key, "..") || strings.ContainsAny(key, `/\`) {
		return "", "", 0, 0, "", "", fmt.Errorf("sprite: invalid native cache key %q", key)
	}
	parts := strings.Split(key, "-")
	// hash + image + dim + bg + version = at least 5; proto optional.
	if len(parts) < 5 {
		return "", "", 0, 0, "", "", fmt.Errorf("sprite: invalid native cache key %q", key)
	}
	version = parts[len(parts)-1]
	bgHash = parts[len(parts)-2]
	dim := parts[len(parts)-3]
	if _, err = fmt.Sscanf(dim, "%dx%d", &w, &h); err != nil || w < 1 || h < 1 {
		return "", "", 0, 0, "", "", fmt.Errorf("sprite: invalid dimensions in native key %q", key)
	}
	dimIdx := len(parts) - 3
	imageIdx := -1
	for i := dimIdx - 1; i >= 0; i-- {
		if parts[i] == "image" {
			imageIdx = i
			break
		}
	}
	if imageIdx < 0 || bgHash == "" || version == "" {
		return "", "", 0, 0, "", "", fmt.Errorf("sprite: invalid native cache key %q", key)
	}
	if imageIdx+1 < dimIdx {
		proto = strings.Join(parts[imageIdx+1:dimIdx], "-")
	}
	hash = strings.Join(parts[:imageIdx], "-")
	if hash == "" {
		return "", "", 0, 0, "", "", fmt.Errorf("sprite: invalid native cache key %q", key)
	}
	return hash, proto, w, h, bgHash, version, nil
}
