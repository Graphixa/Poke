package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image/color"
	"sort"
	"strconv"
	"strings"

	"github.com/Graphixa/Pokedex/internal/render/sprite"
)

// ImageCacheManifestVersion is the on-disk aggregate image-cache schema.
const ImageCacheManifestVersion = "1"

// ImageCacheSpec is the identity of generated image output. Any field that
// affects derived frames must be included so SpecID changes invalidate work.
type ImageCacheSpec struct {
	Version         string
	RendererVersion string
	Protocol        string
	RequiredSizes   []int
	SourceMode      string
	Background      string
	PaletteLimit    int
}

// DefaultImageCacheSpec builds the current native/sixel image-cache identity.
func DefaultImageCacheSpec(protocol, sourceMode string, bg color.RGBA) ImageCacheSpec {
	if sourceMode == "" {
		sourceMode = string(sprite.SourceArtwork)
	}
	if bg.A == 0 {
		bg = sprite.DefaultBackground
	}
	sizes := sprite.FrameSteps()
	return ImageCacheSpec{
		Version:         ImageCacheManifestVersion,
		RendererVersion: sprite.AlgorithmVersion,
		Protocol:        protocol,
		RequiredSizes:   append([]int(nil), sizes...),
		SourceMode:      sourceMode,
		Background:      sprite.BackgroundHash(bg),
	}
}

// ID returns a stable identity for this specification.
func (s ImageCacheSpec) ID() string {
	sizes := append([]int(nil), s.RequiredSizes...)
	sort.Ints(sizes)
	parts := make([]string, len(sizes))
	for i, n := range sizes {
		parts[i] = strconv.Itoa(n)
	}
	raw := fmt.Sprintf("%s|%s|%s|%s|%s|%d|%s",
		s.Version, s.RendererVersion, s.Protocol, s.SourceMode, s.Background, s.PaletteLimit,
		strings.Join(parts, ","))
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:16])
}

// BackgroundRGBA reconstructs the letterbox colour encoded in Background.
func (s ImageCacheSpec) BackgroundRGBA() color.RGBA {
	hexStr := strings.TrimPrefix(strings.TrimSpace(s.Background), "#")
	if len(hexStr) != 6 {
		return sprite.DefaultBackground
	}
	var r, g, b uint8
	if _, err := fmt.Sscanf(hexStr, "%02x%02x%02x", &r, &g, &b); err != nil {
		return sprite.DefaultBackground
	}
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

// nativeKey returns the canonical SHA-256 native cache key for a variant.
func (s ImageCacheSpec) nativeKey(sourceHash string, cells int) string {
	w, h := sprite.FrameSizeFor(cells)
	return sprite.NativeCacheKey(sourceHash, s.Protocol, w, h, s.BackgroundRGBA())
}
