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
const AlgorithmVersion = "v6"

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
}

// DefaultRenderOptions returns options suitable for the fixed sprite frame.
func DefaultRenderOptions() RenderOptions {
	return RenderOptions{
		Width:          FrameWidth,
		Height:         FrameHeight,
		ColorMode:      ColorAuto,
		AlphaThreshold: 16,
		Source:         SourcePixel,
	}
}
