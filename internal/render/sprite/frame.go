package sprite

// Browse sprite frame sizing (terminal cells).
//
// Terminal cells are typically ~1:2 (width:height in pixels). A visually square
// sprite therefore uses fewer rows than columns:
//
//	width  = side
//	height = side / CellAspectY
//
// Native sixel buckets are explicit sizes between MinFrameSize and MaxFrameSize.
const (
	// MinFrameSize is the smallest longest-side (cells) browse will show.
	MinFrameSize = 24
	// MaxFrameSize is the largest longest-side (cells) browse will show.
	MaxFrameSize = 40
	// CellAspectY is how many cell-heights equal one cell-width in visual space
	// (common monospace ≈ 2). Height = side / CellAspectY.
	CellAspectY = 2
	// FramePad is visual padding on each edge, in cell-width units. Horizontal
	// pad uses FramePad columns; vertical pad uses FramePad/CellAspectY rows so
	// margins look equal around a 1:1 sprite.
	FramePad  = 4
	FramePadX = FramePad
	FramePadY = FramePad / CellAspectY

	// FrameWidth / FrameHeight are the preferred (max) frame dimensions.
	FrameWidth  = MaxFrameSize
	FrameHeight = MaxFrameSize / CellAspectY

	// SixelCellPxW/H convert terminal cells to sixel pixel targets.
	//
	// These defaults (8×16) approximate common monospace cell geometry. They are
	// not universal: actual cell pixels vary by terminal/font/DPI. Cache identity
	// uses the resulting pixel dimensions, so a future improvement (e.g. CSI 16 t
	// cell-size query) naturally creates distinct artefacts without migration.
	//
	// TODO(terminal): query CSI 16 t / DECSRPP when production capability probing
	// exposes reliable cell pixel dimensions; until then retain this fallback.
	SixelCellPxW = 8
	SixelCellPxH = 16
)

// FrameBuckets lists the only native sixel frame buckets (24 / 32 / 40).
var FrameBuckets = [...]int{24, 32, 40}

// FrameSizeFor returns cell width×height for a longest-side step.
func FrameSizeFor(side int) (w, h int) {
	side = ClampFrameSide(side)
	return side, side / CellAspectY
}

// ClampFrameSide snaps side down to the largest supported bucket that does not
// exceed the requested side, clamped to [MinFrameSize, MaxFrameSize].
func ClampFrameSide(side int) int {
	if side >= MaxFrameSize {
		return MaxFrameSize
	}
	if side <= MinFrameSize {
		return MinFrameSize
	}
	best := MinFrameSize
	for _, s := range FrameBuckets {
		if s <= side {
			best = s
		}
	}
	return best
}

// FrameSteps returns supported longest-side steps from smallest to largest.
func FrameSteps() []int {
	out := make([]int, len(FrameBuckets))
	copy(out, FrameBuckets[:])
	return out
}

// FrameStepsMaxFirst returns FrameSteps from largest to smallest bucket.
func FrameStepsMaxFirst() []int {
	steps := FrameSteps()
	out := make([]int, len(steps))
	for i, s := range steps {
		out[len(steps)-1-i] = s
	}
	return out
}

// OuterFor returns padded outer width×height for a longest-side step.
// Padding is aspect-corrected so visual margins match on ~1:2 cells.
func OuterFor(side int) (w, h int) {
	fw, fh := FrameSizeFor(side)
	return fw + 2*FramePadX, fh + 2*FramePadY
}

// MinListWidth is the smallest list column that stays usable beside the sprite.
const MinListWidth = 32

// MinRightHeaderWidth is enough for name / types · genus without the sprite.
const MinRightHeaderWidth = 36

// RightPanelSharePercent is the preferred share of terminal width for the
// detail/sprite pane (~42%). The list keeps the remaining ~58%.
const RightPanelSharePercent = 42

// BrowseChromeRows is terminal rows outside the sprite body: footer (1) +
// combined-frame borders (2) + name/types header + gap (3) + divider +
// switcher (2). Keep in sync with layout spriteChromeRows.
const BrowseChromeRows = 8

// MinTerminalWidth is the smallest terminal width that can host the padded
// *minimum* sprite frame beside a usable list column.
func MinTerminalWidth() int {
	needRight, _ := OuterFor(MinFrameSize)
	if needRight < MinRightHeaderWidth {
		needRight = MinRightHeaderWidth
	}
	needRight += 2                      // frame borders / gutter
	return needRight + MinListWidth + 1 // + sep
}

// MinTerminalHeight is the smallest terminal height that can host the padded
// *minimum* sprite frame plus browse chrome.
func MinTerminalHeight() int {
	_, outerH := OuterFor(MinFrameSize)
	return outerH + BrowseChromeRows
}

// LargestFrameFitting returns the largest step whose padded outer box fits in
// the given cell budgets, or 0 if even the minimum does not fit.
func LargestFrameFitting(maxOuterW, maxOuterH int) int {
	best := 0
	for _, side := range FrameSteps() {
		ow, oh := OuterFor(side)
		if ow <= maxOuterW && oh <= maxOuterH {
			best = side
		}
	}
	return best
}

// PixelSize converts a cell box to the pixel target used by sixel encoding.
func PixelSize(cellsW, cellsH int) (pxW, pxH int) {
	if cellsW < 1 {
		cellsW = 1
	}
	if cellsH < 1 {
		cellsH = 1
	}
	return cellsW * SixelCellPxW, cellsH * SixelCellPxH
}

// RightPanelWidth picks the right column width: about RightPanelSharePercent of
// the terminal, and never narrower than the padded sprite (list gets the rest).
func RightPanelWidth(termW, spriteSide int) int {
	if termW < 20 {
		return termW / 2
	}
	maxRight := termW - MinListWidth - 1
	if maxRight < MinRightHeaderWidth {
		maxRight = termW / 2
	}
	r := (termW * RightPanelSharePercent) / 100
	if r < MinRightHeaderWidth {
		r = MinRightHeaderWidth
	}
	if spriteSide >= MinFrameSize {
		ow, _ := OuterFor(spriteSide)
		if ow+2 > r {
			r = ow + 2
		}
	}
	if r > maxRight {
		r = maxRight
	}
	if r < 20 {
		r = 20
	}
	return r
}
