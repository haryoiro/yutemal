package ui

import "github.com/mattn/go-runewidth"

// Pre-calculated string widths for commonly used strings
var (
	// Player constants
	MusicEmojiWidth     = runewidth.StringWidth("🎵 ")
	SeparatorWidth      = runewidth.StringWidth(" - ")
	TimeFormatWidth     = runewidth.StringWidth("--:--")

	// Common UI elements
	EllipsisWidth       = runewidth.StringWidth("...")
	SpaceWidth          = 1

	// Progress bar
	ProgressEmptyChar   = "─"
	ProgressFilledChar  = "━"
)
