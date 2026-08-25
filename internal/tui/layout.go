package tui

import "github.com/Graphixa/Pokedex/internal/render/sprite"

// Default responsive breakpoints. Sprite mins are overridden by config when set
// on the model (see layoutThresholds). Zero config falls back to frame-derived
// MinTerminalWidth/Height so sprites show on normal terminal sizes.
const (
	// Rows reserved outside the sprite body for header, gap, divider, switcher,
	// and terminal footer when budgeting the scalable frame.
	spriteChromeRows = 10
	minSummaryWidth  = 88
	minSummaryHeight = 25
	minFooterWidth   = 76
	minFooterHeight  = 22
	minTwoPaneWidth  = 52
	minTwoPaneHeight = 18
	minUsableWidth   = 40
	minUsableHeight  = 12
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
		w = sprite.MinTerminalWidth()
	}
	if h <= 0 {
		h = sprite.MinTerminalHeight()
	}
	return w, h
}

// LayoutFlags describe which regions are visible for the current terminal size.
type LayoutFlags struct {
	Width, Height int
	ShowSprite    bool
	ShowSummary   bool
	VerboseFooter bool
	OnePane       bool
	TooSmall      bool
	// SpriteSize is the chosen longest-side step when the sprite panel is shown.
	SpriteSize int
}

func layoutFor(width, height int, th layoutThresholds) LayoutFlags {
	f := LayoutFlags{Width: width, Height: height}
	if width <= 0 || height <= 0 {
		f.ShowSprite = !th.DisableSprite
		f.SpriteSize = sprite.MaxFrameSize
		f.ShowSummary = true
		f.VerboseFooter = true
		return f
	}
	f.TooSmall = width < minUsableWidth || height < minUsableHeight
	f.OnePane = width < minTwoPaneWidth || height < minTwoPaneHeight
	minSW, minSH := th.spriteMin()
	frame := maxSpriteFrame(height, f.rightWidth())
	f.ShowSprite = !th.DisableSprite && !f.OnePane && !f.TooSmall &&
		width >= minSW && height >= minSH && frame >= sprite.MinFrameSize
	if f.ShowSprite {
		f.SpriteSize = frame
	}
	f.ShowSummary = !f.OnePane && !f.TooSmall && width >= minSummaryWidth && height >= minSummaryHeight
	f.VerboseFooter = !f.TooSmall && width >= minFooterWidth && height >= minFooterHeight
	return f
}

// maxSpriteFrame returns the largest discrete longest-side step that fits, or 0.
func maxSpriteFrame(termH, rightW int) int {
	hBudget := termH - spriteChromeRows
	wBudget := rightW - 2
	return sprite.LargestFrameFitting(wBudget, hBudget)
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

// spriteFrameSize returns cell width×height for the chosen step when shown.
func (f LayoutFlags) spriteFrameSize() (w, h int) {
	if !f.ShowSprite || f.SpriteSize < sprite.MinFrameSize {
		return 0, 0
	}
	return sprite.FrameSizeFor(f.SpriteSize)
}
