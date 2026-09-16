package sprite

// Browse sprite frame sizing (terminal cells).
//
// Terminal cells are typically ~1:2 (width:height in pixels). A visually square
// sprite therefore uses fewer rows than columns:
//
//	width  = side
//	height = side / CellAspectY
//
// Native sixel buckets are explicit sizes (16 glance … 40 hero).
const (
	// MinFrameSize is the classic terminal-minimum longest side (helpers /
	// MinTerminal*). Encode buckets may be smaller (see FrameBuckets / glance).
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
)

// SixelCellPxW/H convert terminal cells to sixel pixel targets.
//
// Defaults match typical Windows Terminal / Cascadia geometry (10×20). App
// startup measures the real cell size via XTWINOPS (CSI 16 t) and overrides
// through SetCellPixelSize before the engine or any warmer starts — encoding
// with a guessed cell height taller than reality paints sixel past the art
// box (the browse overpaint bug). Cache identity includes the resulting pixel
// dimensions, so changed cell sizes key distinct artefacts without migration.
var (
	SixelCellPxW = 10
	SixelCellPxH = 20
)

// SetCellPixelSize overrides the cell→pixel conversion with a measured value.
// Call once at startup before rendering begins; out-of-range values are
// ignored (keeps the safe default on bogus terminal replies).
func SetCellPixelSize(pxW, pxH int) {
	if pxW < 3 || pxW > 40 || pxH < 6 || pxH > 80 {
		return
	}
	SixelCellPxW = pxW
	SixelCellPxH = pxH
}

// FrameBuckets lists native sixel frame buckets (16 glance / 24 / 32 / 40 hero).
var FrameBuckets = [...]int{16, 24, 32, 40}

// SmallestFrameBucket is the smallest supported encode/layout bucket.
func SmallestFrameBucket() int { return FrameBuckets[0] }

// BrowseWarmBuckets is the default warm phase order: glance then hero.
func BrowseWarmBuckets() []int {
	return []int{FrameBuckets[0], MaxFrameSize}
}

// FrameSizeFor returns cell width×height for a longest-side step.
// Side is snapped to a native bucket (16 / 24 / 32 / 40).
func FrameSizeFor(side int) (w, h int) {
	side = ClampFrameSide(side)
	return side, side / CellAspectY
}

// ClampFrameSide snaps side down to the largest supported bucket that does not
// exceed the requested side. Values below the smallest bucket snap up to it.
func ClampFrameSide(side int) int {
	minBucket := FrameBuckets[0]
	if side >= MaxFrameSize {
		return MaxFrameSize
	}
	if side <= minBucket {
		return minBucket
	}
	best := minBucket
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

// RightPanelSharePercent is a fallback share for the sprite pane when list
// sizing is not available.
const RightPanelSharePercent = 42

// BrowseChromeRows is terminal rows outside the minimum art *card* content:
// footer, outer frame borders, and the name/types header. The pinned art card
// itself includes its own borders and 1-cell pad (see views.ArtCardOuter).
const BrowseChromeRows = 5

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

// NormalizeBuckets clamps, de-dupes, and drops sizes below the smallest bucket.
func NormalizeBuckets(buckets []int) []int {
	seen := make(map[int]bool, len(buckets))
	out := make([]int, 0, len(buckets))
	for _, b := range buckets {
		b = ClampFrameSide(b)
		if b < SmallestFrameBucket() || seen[b] {
			continue
		}
		seen[b] = true
		out = append(out, b)
	}
	return out
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
	if spriteSide >= SmallestFrameBucket() {
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
