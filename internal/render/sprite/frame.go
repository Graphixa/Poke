package sprite

// Fixed sprite frame size in terminal cells. When shown, the panel is always
// this size; when the terminal cannot fit it, the sprite region is hidden.
const (
	FrameWidth  = 24
	FrameHeight = 24
)

// MinInfoWidthBesideSprite is the minimum identity-column width required
// beside the sprite (after the " │ " separator) before the sprite may show.
const MinInfoWidthBesideSprite = 16

// SeparatorWidth is the display width of " │ " between info and sprite.
const SeparatorWidth = 3

// MinRightWidthForSprite is the minimum right-pane content width needed to
// show the fixed frame beside identity info.
const MinRightWidthForSprite = FrameWidth + SeparatorWidth + MinInfoWidthBesideSprite
