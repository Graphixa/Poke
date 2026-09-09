package tui

import "github.com/Graphixa/Pokedex/internal/render/sprite"

// Default responsive breakpoints. Sprite mins are overridden by config when set
// on the model (see layoutThresholds). Zero config falls back to frame-derived
// MinTerminalWidth/Height so sprites show on normal terminal sizes.
const (
	// Rows reserved outside the sprite body — must match sprite.BrowseChromeRows
	// (footer + frame borders + header/gap + divider/switcher).
	spriteChromeRows = sprite.BrowseChromeRows
	minFooterWidth   = 76
	minFooterHeight  = 22
	minTwoPaneWidth  = 52
	minTwoPaneHeight = 18
	minUsableWidth   = 40
	minUsableHeight  = 12

	listNamePad         = 3 // extra cells after the longest species name
	listMarkerCols      = 2
	listGapNameDex      = 1
	listDexCols         = 4 // #NNN
	listGapDexTypes     = 2
	listTypeReserve     = 14 // Grass/Poison-sized dual type
	listInnerTrail      = 2
	listShareMinNoTypes = 25
	listShareMaxNoTypes = 30
	listShareWithTypes  = 50
)

// listColumnSpec sizes the browse list pane from content, not leftover sprite space.
type listColumnSpec struct {
	ShowTypes    bool
	MaxNameWidth int
}

func nameColumnWidth(maxName int) int {
	if maxName < 8 {
		maxName = 8
	}
	return maxName + listNamePad
}

func listContentInner(showTypes bool, maxName int) int {
	inner := listMarkerCols + nameColumnWidth(maxName) + listGapNameDex + listDexCols + listInnerTrail
	if showTypes {
		inner += listGapDexTypes + listTypeReserve
	}
	return inner
}

func listPaneWidth(termW int, spec listColumnSpec) int {
	if termW < 20 {
		return termW / 2
	}
	left := listContentInner(spec.ShowTypes, spec.MaxNameWidth) + 1 // combined-frame left border
	if spec.ShowTypes {
		if target := termW * listShareWithTypes / 100; left < target {
			left = target
		}
	} else {
		lo := termW * listShareMinNoTypes / 100
		hi := termW * listShareMaxNoTypes / 100
		if hi < lo {
			hi = lo
		}
		if left < lo {
			left = lo
		} else if left > hi {
			left = hi
			if need := listContentInner(false, spec.MaxNameWidth) + 1; left < need {
				left = need
			}
		}
	}
	maxLeft := termW - sprite.MinRightHeaderWidth - 1
	if maxLeft < 12 {
		maxLeft = termW / 2
	}
	if left > maxLeft {
		left = maxLeft
	}
	if left < 16 {
		left = 16
	}
	if left >= termW {
		left = termW / 2
	}
	return left
}

// layoutThresholds holds size gates; zero fields mean defaults.
type layoutThresholds struct {
	DisableSprite bool
}

// LayoutFlags describe which regions are visible for the current terminal size.
type LayoutFlags struct {
	Width, Height int
	ShowSprite    bool
	VerboseFooter bool
	OnePane       bool
	TooSmall      bool
	// SpriteSize is the chosen longest-side step when the sprite panel is shown.
	SpriteSize int
	// LeftWidth / RightWidth are panel column widths for the combined frame
	// (list sized from names/types; right gets the remainder for the sprite).
	LeftWidth  int
	RightWidth int
}

func layoutFor(width, height int, th layoutThresholds, list listColumnSpec) LayoutFlags {
	f := LayoutFlags{Width: width, Height: height}
	if width <= 0 || height <= 0 {
		// Unknown size: do not pretend a sprite panel exists. Init is a value
		// receiver so it cannot start a load; a fake MaxFrameSize here made the
		// first WindowSize look like a no-op and left the first species blank.
		return f
	}
	f.TooSmall = width < minUsableWidth || height < minUsableHeight
	f.OnePane = width < minTwoPaneWidth || height < minTwoPaneHeight

	bodyH := height - spriteChromeRows
	if bodyH < 1 {
		bodyH = 1
	}
	if !f.OnePane && !f.TooSmall {
		f.LeftWidth = listPaneWidth(width, list)
		f.RightWidth = width - f.LeftWidth - 1
		if f.RightWidth < 12 {
			f.RightWidth = width / 2
			f.LeftWidth = width - f.RightWidth - 1
		}
	}
	maxOuterW := f.RightWidth - 2
	if maxRight := width - 16 - 1; f.RightWidth == 0 {
		maxOuterW = maxRight - 2
	}
	frame := sprite.LargestFrameFitting(maxOuterW, bodyH)
	// Fit-based gate: if a padded frame fits the body, show it. Stale config
	// min width/height must not hide a sprite that has room on screen.
	f.ShowSprite = !th.DisableSprite && !f.OnePane && !f.TooSmall &&
		frame >= sprite.MinFrameSize
	if f.ShowSprite {
		f.SpriteSize = frame
	}
	if f.LeftWidth == 0 && !f.OnePane {
		f.RightWidth = sprite.RightPanelWidth(width, f.SpriteSize)
		f.LeftWidth = width - f.RightWidth - 1
	}

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
	if f.LeftWidth > 0 {
		return f.Width - f.LeftWidth - 1
	}
	return sprite.RightPanelWidth(f.Width, f.SpriteSize)
}
