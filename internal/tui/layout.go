package tui

import (
	"github.com/Graphixa/Poke/internal/render/sprite"
	"github.com/Graphixa/Poke/internal/tui/views"
)

// Default responsive breakpoints. Browse sprite visibility uses glance/hero
// PreferredBucket helpers (16 / 40).
const (
	minFooterWidth   = 76
	minFooterHeight  = 22
	minTwoPaneWidth  = 52
	minTwoPaneHeight = 18
	minUsableWidth   = 40
	minUsableHeight  = 12

	listNamePad    = 0 // name column is the longest species name
	listMarkerCols = 4 // "> ★ " / ">   " cursor+star gutter
	listGapNameDex = 1
	listDexCols    = 5 // #NNNN
	listRightPad   = 1 // one cell between dex and the pane edge
	listShareMin   = 25
	listShareMax   = 30

	browseFooterRows      = 1
	browseFrameBorderRows = 2
)

// listColumnSpec sizes the browse list pane from content, not leftover sprite space.
type listColumnSpec struct {
	MaxNameWidth int
}

func nameColumnWidth(maxName int) int {
	if maxName < 8 {
		maxName = 8
	}
	return maxName + listNamePad
}

func listContentInner(maxName int) int {
	return listMarkerCols + nameColumnWidth(maxName) + listGapNameDex + listDexCols + listRightPad
}

func listPaneWidth(termW int, spec listColumnSpec) int {
	if termW < 20 {
		return termW / 2
	}
	left := listContentInner(spec.MaxNameWidth) + 1 // combined-frame left border
	lo := termW * listShareMin / 100
	hi := termW * listShareMax / 100
	if hi < lo {
		hi = lo
	}
	if left < lo {
		left = lo
	} else if left > hi {
		left = hi
		if need := listContentInner(spec.MaxNameWidth) + 1; left < need {
			left = need
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
	// (list sized from names; right gets the remainder for the sprite).
	LeftWidth  int
	RightWidth int
}

func layoutFor(width, height int, th layoutThresholds, list listColumnSpec, browseView BrowseView) LayoutFlags {
	f := LayoutFlags{Width: width, Height: height}
	if width <= 0 || height <= 0 {
		return f
	}
	f.TooSmall = width < minUsableWidth || height < minUsableHeight
	f.OnePane = width < minTwoPaneWidth || height < minTwoPaneHeight

	panelH := height - browseFooterRows
	innerH := panelH - browseFrameBorderRows
	if innerH < 1 {
		innerH = 1
	}

	rightPanel := views.GlanceRightInnerWidth() + 1
	if browseView == BrowseArt {
		rightPanel = views.HeroRightInnerWidth() + 1
	}
	minList := 16
	if !f.OnePane && !f.TooSmall {
		if width-1-rightPanel >= minList {
			f.RightWidth = rightPanel
			f.LeftWidth = width - f.RightWidth - 1
		} else {
			// Terminal too narrow for fixed tile: fall back to list-first split.
			f.LeftWidth = listPaneWidth(width, list)
			f.RightWidth = width - f.LeftWidth - 1
			if f.RightWidth < 12 {
				f.RightWidth = width / 2
				f.LeftWidth = width - f.RightWidth - 1
			}
		}
	}

	rightInnerW := f.RightWidth - 1
	if rightInnerW < 1 {
		rightInnerW = width / 2
	}
	var bucket int
	if browseView == BrowseArt {
		bucket = views.PreferredHeroBucket(rightInnerW, innerH)
	} else {
		bucket = views.PreferredBucket(rightInnerW, innerH)
	}
	f.ShowSprite = !th.DisableSprite && !f.OnePane && !f.TooSmall &&
		bucket >= sprite.SmallestFrameBucket()
	if f.ShowSprite {
		f.SpriteSize = bucket
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
