package tui

import "github.com/Graphixa/Pokedex/internal/render/sprite"

// Default responsive breakpoints. Sprite mins are overridden by config when set
// on the model (see layoutThresholds).
const (
	defaultMinSpriteWidth  = 110
	defaultMinSpriteHeight = 40
	minSummaryWidth        = 88
	minSummaryHeight       = 25
	minFooterWidth         = 76
	minFooterHeight        = 22
	minTabWidth            = 64
	minTwoPaneWidth        = 52
	minTwoPaneHeight       = 18
	minUsableWidth         = 40
	minUsableHeight        = 12
)

// layoutThresholds holds size gates; zero fields mean defaults.
type layoutThresholds struct {
	MinSpriteWidth  int
	MinSpriteHeight int
	DisableSprite   bool
}

func (t layoutThresholds) spriteMin() (w, h int) {
	w, h = t.MinSpriteWidth, t.MinSpriteHeight
	if w <= 0 {
		w = defaultMinSpriteWidth
	}
	if h <= 0 {
		h = defaultMinSpriteHeight
	}
	return w, h
}

// LayoutFlags describe which regions are visible for the current terminal size.
type LayoutFlags struct {
	Width, Height int
	ShowSprite    bool
	ShowSummary   bool
	VerboseFooter bool
	ShortTabs     bool
	OnePane       bool
	TooSmall      bool
}

func layoutFor(width, height int, th layoutThresholds) LayoutFlags {
	f := LayoutFlags{Width: width, Height: height}
	if width <= 0 || height <= 0 {
		// Before the first WindowSizeMsg, assume a full layout for tests/snapshots.
		f.ShowSprite = !th.DisableSprite
		f.ShowSummary = true
		f.VerboseFooter = true
		return f
	}
	f.TooSmall = width < minUsableWidth || height < minUsableHeight
	f.OnePane = width < minTwoPaneWidth || height < minTwoPaneHeight
	minSW, minSH := th.spriteMin()
	canFitFrame := f.rightWidth() >= sprite.MinRightWidthForSprite
	f.ShowSprite = !th.DisableSprite && !f.OnePane && !f.TooSmall &&
		width >= minSW && height >= minSH && canFitFrame
	f.ShowSummary = !f.OnePane && !f.TooSmall && width >= minSummaryWidth && height >= minSummaryHeight
	f.VerboseFooter = !f.TooSmall && width >= minFooterWidth && height >= minFooterHeight
	f.ShortTabs = width < minTabWidth
	return f
}

func (f LayoutFlags) leftWidth() int {
	if f.Width <= 0 {
		return 40
	}
	w := f.Width / 3
	if w < 24 {
		w = 24
	}
	if w > f.Width-20 {
		w = f.Width - 20
	}
	return w
}

func (f LayoutFlags) rightWidth() int {
	if f.Width <= 0 {
		return 80
	}
	return f.Width - f.leftWidth()
}

// spriteFrameSize returns the fixed frame dimensions when the sprite is shown.
func (f LayoutFlags) spriteFrameSize() (w, h int) {
	if !f.ShowSprite {
		return 0, 0
	}
	return sprite.FrameWidth, sprite.FrameHeight
}
