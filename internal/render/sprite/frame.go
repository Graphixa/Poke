package sprite

// Browse sprite frame sizing (terminal cells).
//
// Terminal cells are typically ~1:2 (width:height in pixels). A visually square
// sprite therefore uses fewer rows than columns:
//
//	width  = side
//	height = side / CellAspectY
//
// The longest side scales in discrete steps between MinFrameSize and
// MaxFrameSize. The image is shrunk uniformly to fit — never stretched.
const (
	// MinFrameSize is the smallest longest-side (cells) browse will show.
	MinFrameSize = 20
	// MaxFrameSize is the largest longest-side (cells) browse will show.
	MaxFrameSize = 40
	// FrameStep is the discrete increment between min and max.
	FrameStep = 4
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
)

// FrameSizeFor returns cell width×height for a longest-side step.
// side is clamped into [MinFrameSize, MaxFrameSize] and snapped down to FrameStep.
func FrameSizeFor(side int) (w, h int) {
	side = ClampFrameSide(side)
	return side, side / CellAspectY
}

// ClampFrameSide snaps side into the allowed step range.
func ClampFrameSide(side int) int {
	if side < MinFrameSize {
		return MinFrameSize
	}
	if side > MaxFrameSize {
		return MaxFrameSize
	}
	off := side - MinFrameSize
	side = MinFrameSize + (off/FrameStep)*FrameStep
	if side < MinFrameSize {
		return MinFrameSize
	}
	return side
}

// FrameSteps returns longest-side steps from min to max inclusive.
func FrameSteps() []int {
	n := ((MaxFrameSize - MinFrameSize) / FrameStep) + 1
	if n < 1 {
		return []int{MinFrameSize}
	}
	steps := make([]int, 0, n)
	for s := MinFrameSize; s <= MaxFrameSize; s += FrameStep {
		steps = append(steps, s)
	}
	return steps
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
