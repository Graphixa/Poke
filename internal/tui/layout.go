package tui

// Responsive breakpoints from plans/DECISIONS.md.
const (
	minSpriteWidth   = 100
	minSpriteHeight  = 30
	minSummaryWidth  = 88
	minSummaryHeight = 25
	minFooterWidth   = 76
	minFooterHeight  = 22
	minTabWidth      = 64
	minTwoPaneWidth  = 52
	minTwoPaneHeight = 18
	minUsableWidth   = 40
	minUsableHeight  = 12
)

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

func layoutFor(width, height int) LayoutFlags {
	f := LayoutFlags{Width: width, Height: height}
	if width <= 0 || height <= 0 {
		// Before the first WindowSizeMsg, assume a full layout for tests/snapshots.
		f.ShowSprite = true
		f.ShowSummary = true
		f.VerboseFooter = true
		return f
	}
	f.TooSmall = width < minUsableWidth || height < minUsableHeight
	f.OnePane = width < minTwoPaneWidth || height < minTwoPaneHeight
	f.ShowSprite = !f.OnePane && !f.TooSmall && width >= minSpriteWidth && height >= minSpriteHeight
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

func (f LayoutFlags) spriteSide() int {
	rw := f.rightWidth()
	side := rw * 30 / 100
	if side < 8 {
		side = 8
	}
	maxSide := 12
	if side > maxSide {
		side = maxSide
	}
	if side > rw-20 {
		side = rw - 20
	}
	if side < 5 {
		return 0
	}
	return side
}
