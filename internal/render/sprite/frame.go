package sprite

// Browse sprite frame sizing (terminal cells).
//
// Terminal cells are typically ~1:2 (width:height in pixels). A visually square
// sprite therefore uses fewer rows than columns:
//
//	FrameWidth  = MaxFrameSize
//	FrameHeight = MaxFrameSize / CellAspectY
//
// Tunable: change MaxFrameSize to grow/shrink the art; FramePad is empty margin
// on each edge. The image is shrunk uniformly to fit — never stretched.
const (
	// MaxFrameSize is the longest side of the sprite in cells (usually width).
	MaxFrameSize = 28
	// CellAspectY is how many cell-heights equal one cell-width in visual space
	// (common monospace ≈ 2). Height = MaxFrameSize / CellAspectY.
	CellAspectY = 2
	// FramePad is padding (cells) on each edge of the frame.
	FramePad = 2

	FrameWidth   = MaxFrameSize
	FrameHeight  = MaxFrameSize / CellAspectY // visual 1:1 with CellAspectY
	MinFrameSize = MaxFrameSize
)

// SeparatorWidth is reserved for optional chrome near the sprite region.
const SeparatorWidth = 3

// FrameOuterWidth is the body reservation width including horizontal pad.
func FrameOuterWidth() int {
	return FrameWidth + 2*FramePad
}

// FrameOuterHeight is the body reservation height including vertical pad.
func FrameOuterHeight() int {
	return FrameHeight + 2*FramePad
}

// FrameOuterSize is the larger outer edge (used for square budget checks).
func FrameOuterSize() int {
	w, h := FrameOuterWidth(), FrameOuterHeight()
	if h > w {
		return h
	}
	return w
}
