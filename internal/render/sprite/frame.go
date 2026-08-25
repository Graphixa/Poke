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
	MinFrameSize = 24
	// MaxFrameSize is the largest longest-side (cells) browse will show.
	MaxFrameSize = 32
	// FrameStep is the discrete increment between min and max.
	FrameStep = 4
	// CellAspectY is how many cell-heights equal one cell-width in visual space
	// (common monospace ≈ 2). Height = side / CellAspectY.
	CellAspectY = 2
	// FramePad is padding (cells) on each edge of the frame.
	FramePad = 2

	// FrameWidth / FrameHeight are the preferred (max) frame dimensions.
	FrameWidth  = MaxFrameSize
	FrameHeight = MaxFrameSize / CellAspectY
)

// SeparatorWidth is reserved for optional chrome near the sprite region.
const SeparatorWidth = 3

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
	// Snap down to FrameStep grid aligned to MinFrameSize.
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
	out := make([]int, 0, n)
	for s := MinFrameSize; s <= MaxFrameSize; s += FrameStep {
		out = append(out, s)
	}
	return out
}

// OuterFor returns padded outer width×height for a longest-side step.
func OuterFor(side int) (w, h int) {
	fw, fh := FrameSizeFor(side)
	return fw + 2*FramePad, fh + 2*FramePad
}

// FrameOuterWidth is the preferred (max) padded body width.
func FrameOuterWidth() int {
	w, _ := OuterFor(MaxFrameSize)
	return w
}

// FrameOuterHeight is the preferred (max) padded body height.
func FrameOuterHeight() int {
	_, h := OuterFor(MaxFrameSize)
	return h
}

// FrameOuterSize is the larger outer edge at max frame.
func FrameOuterSize() int {
	w, h := FrameOuterWidth(), FrameOuterHeight()
	if h > w {
		return h
	}
	return w
}

// MinTerminalWidth is the smallest terminal width that can host the padded
// *minimum* sprite frame beside a usable list column (~1/3).
func MinTerminalWidth() int {
	needRight, _ := OuterFor(MinFrameSize)
	needRight += 2
	// left ≈ width/3, right ≈ 2*width/3; require right >= needRight.
	w := (needRight*3 + 1) / 2
	const minLeft = 24
	if w < needRight+minLeft {
		w = needRight + minLeft
	}
	return w
}

// MinTerminalHeight is the smallest terminal height that can host the padded
// *minimum* sprite frame plus browse chrome (header / divider / switcher / footer).
func MinTerminalHeight() int {
	const chromeRows = 10
	_, outerH := OuterFor(MinFrameSize)
	return outerH + chromeRows
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
