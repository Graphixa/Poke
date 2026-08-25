package tui

import "github.com/Graphixa/Pokedex/internal/render/sprite"

// Default responsive breakpoints. Sprite mins are overridden by config when set
// on the model (see layoutThresholds). Zero config falls back to frame-derived
// MinTerminalWidth/Height so sprites show on normal terminal sizes.
const (
	// Rows reserved outside the sprite body — must match sprite.BrowseChromeRows
	// (footer + frame borders + header/gap + divider/switcher).
	spriteChromeRows = sprite.BrowseChromeRows
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
// MinSpriteWidth/Height are retained for config wiring but ShowSprite is
// decided by whether a padded frame fits the body (see layoutFor).
type layoutThresholds struct {
	MinSpriteWidth  int
	MinSpriteHeight int
	DisableSprite   bool
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
	// LeftWidth / RightWidth are panel column widths for the combined frame
	// (list-first: right sized to the sprite, left gets the remainder).
	LeftWidth  int
	RightWidth int
}

func layoutFor(width, height int, th layoutThresholds) LayoutFlags {
	f := LayoutFlags{Width: width, Height: height}
	if width <= 0 || height <= 0 {
		f.ShowSprite = !th.DisableSprite
		f.SpriteSize = sprite.MaxFrameSize
		f.ShowSummary = true
		f.VerboseFooter = true
		f.RightWidth = sprite.RightPanelWidth(120, sprite.MaxFrameSize)
		f.LeftWidth = 120 - f.RightWidth - 1
		return f
	}
	f.TooSmall = width < minUsableWidth || height < minUsableHeight
	f.OnePane = width < minTwoPaneWidth || height < minTwoPaneHeight

	// Detail pane ~42% / list ~58%. Pick the largest sprite that fits the
	// pane width and the real body height (not an overstated chrome budget).
	maxRight := width - sprite.MinListWidth - 1
	if maxRight < sprite.MinRightHeaderWidth {
		maxRight = width / 2
	}
	targetRight := (width * sprite.RightPanelSharePercent) / 100
	if targetRight < sprite.MinRightHeaderWidth {
		targetRight = sprite.MinRightHeaderWidth
	}
	if targetRight > maxRight {
		targetRight = maxRight
	}
	bodyH := height - spriteChromeRows
	if bodyH < 1 {
		bodyH = 1
	}
	frame := sprite.LargestFrameFitting(targetRight-2, bodyH)
	// Fit-based gate: if a padded frame fits the body, show it. Stale config
	// min width/height must not hide a sprite that has room on screen.
	f.ShowSprite = !th.DisableSprite && !f.OnePane && !f.TooSmall &&
		frame >= sprite.MinFrameSize
	if f.ShowSprite {
		f.SpriteSize = frame
	}
	f.RightWidth = sprite.RightPanelWidth(width, f.SpriteSize)
	f.LeftWidth = width - f.RightWidth - 1
	if f.LeftWidth < sprite.MinListWidth && width > sprite.MinListWidth+20 {
		f.LeftWidth = sprite.MinListWidth
		f.RightWidth = width - f.LeftWidth - 1
	}

	f.ShowSummary = !f.OnePane && !f.TooSmall && width >= minSummaryWidth && height >= minSummaryHeight
	f.VerboseFooter = !f.TooSmall && width >= minFooterWidth && height >= minFooterHeight
	return f
}

func (f LayoutFlags) leftWidth() int {
	if f.LeftWidth > 0 {
		return f.LeftWidth
	}
	if f.Width <= 0 {
		return 40
	}
	return f.Width - f.rightWidth() - 1
}

func (f LayoutFlags) rightWidth() int {
	if f.RightWidth > 0 {
		return f.RightWidth
	}
	if f.Width <= 0 {
		return 48
	}
	return sprite.RightPanelWidth(f.Width, f.SpriteSize)
}

// spriteFrameSize returns cell width×height for the chosen step when shown.
func (f LayoutFlags) spriteFrameSize() (w, h int) {
	if !f.ShowSprite || f.SpriteSize < sprite.MinFrameSize {
		return 0, 0
	}
	return sprite.FrameSizeFor(f.SpriteSize)
}
