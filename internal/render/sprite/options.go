package sprite

import "image/color"

// RenderMode is a logical sprite rendering class (not a terminal protocol).
type RenderMode string

const (
	ModeAuto        RenderMode = "auto"
	ModeImage       RenderMode = "image"
	ModeChafa       RenderMode = "chafa"
	ModeOff         RenderMode = "off"
	ModePlaceholder RenderMode = "placeholder"
)

// ColorMode selects colour encoding for text-based renderers.
type ColorMode string

const (
	ColorAuto      ColorMode = "auto"
	ColorTrueColor ColorMode = "truecolor"
	Color256       ColorMode = "256"
	ColorMono      ColorMode = "mono"
)

// SpriteSource selects which sprite asset category to prefer.
type SpriteSource string

const (
	SourcePixel   SpriteSource = "pixel"
	SourceArtwork SpriteSource = "artwork"
)

// AlgorithmVersion is embedded in cache keys so artefacts invalidate across
// renderer pipeline changes.
const AlgorithmVersion = "v16"

// DefaultSixelColors is the production sixel palette when unset or unsupported.
const DefaultSixelColors = 64

// EffectiveSixelColors returns 64, 128, or 256. Unsupported values fall back to 64.
func EffectiveSixelColors(n int) int {
	switch n {
	case 64, 128, 256:
		return n
	default:
		return DefaultSixelColors
	}
}

// ChafaRenderVersion bumps when in-process chafa invocation args change.
const ChafaRenderVersion = "lib-v2" // HQ mosaic + 24 samples/cell (poke chafa H/D)

// QualityProfile selects encode/resize quality tradeoffs.
type QualityProfile string

const (
	// QualityInteractive is the browse/cold path (chafa now uses full Mosaic too).
	QualityInteractive QualityProfile = "interactive"
	// QualityPrewarm is the background warm path (may use a heavier resize scaler).
	QualityPrewarm QualityProfile = "prewarm"
)

// RenderOptions control a single sprite conversion.
type RenderOptions struct {
	Width     int
	Height    int
	ColorMode ColorMode
	// BackgroundColor is used when Background is theme (or for letterboxing).
	BackgroundColor color.RGBA
	// AlphaThreshold treats samples below this alpha as transparent (0–255).
	AlphaThreshold uint8
	// Source selects nearest-neighbour (pixel) vs filtered (artwork) resize.
	Source SpriteSource
	// Label is used by the placeholder renderer (Pokémon display name).
	Label string
	// ContentHash identifies the source image for caching.
	ContentHash string
	// Quality selects interactive vs prewarm resize tradeoffs (chafa mosaic is always HQ).
	Quality QualityProfile
	// SixelColors is the sixel palette limit (64 | 128 | 256); zero uses 64.
	SixelColors int
}
