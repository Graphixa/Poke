package sprite

// Preferred (maximum) sprite frame size in terminal cells. Browse may use a
// smaller bucketed frame when the terminal cannot fit this size.
const (
	FrameWidth  = 40
	FrameHeight = 40
	// MinFrameSize is the smallest square frame browse will show.
	MinFrameSize = 16
)

// MinInfoWidthBesideSprite is the minimum identity-column width required
// beside the sprite (after the " │ " separator) before the sprite may show.
const MinInfoWidthBesideSprite = 16

// SeparatorWidth is the display width of " │ " between info and sprite.
const SeparatorWidth = 3

// MinRightWidthForSprite is the minimum right-pane content width needed to
// show the smallest sprite frame beside identity info.
const MinRightWidthForSprite = MinFrameSize + SeparatorWidth + MinInfoWidthBesideSprite

// MinRightWidthForFrame is the right-pane width needed for a square frame of size n.
func MinRightWidthForFrame(n int) int {
	if n < MinFrameSize {
		n = MinFrameSize
	}
	return n + SeparatorWidth + MinInfoWidthBesideSprite
}
