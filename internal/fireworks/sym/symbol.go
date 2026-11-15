// Adapted from https://github.com/erik-adelbert/firework (MIT)
// Single-width cells so ASCII sparks map 1:1 to terminal columns.

package sym

import (
	"image/color"
	"sync"

	"charm.land/lipgloss/v2"
)

type Symbol struct {
	rune
	color.RGBA
}

func MkSymbol(r rune, c color.RGBA) Symbol {
	return Symbol{rune: r, RGBA: c}
}

func (s Symbol) IsZero() bool {
	return s.rune == 0
}

func (s Symbol) String() string {
	if s.rune == 0 || s.rune == ' ' {
		return " "
	}

	str, ok := renderCache[s]
	if !ok {
		style := getStyle()
		str = style.Foreground(s.RGBA).Render(string(s.rune))
		putStyle(style)
		renderCache[s] = str
	}

	return str
}

var renderCache = make(map[Symbol]string)

var stylePool = sync.Pool{
	New: func() any {
		st := lipgloss.NewStyle()
		return &st
	},
}

func getStyle() *lipgloss.Style {
	return stylePool.Get().(*lipgloss.Style)
}

func putStyle(s *lipgloss.Style) {
	stylePool.Put(s)
}
