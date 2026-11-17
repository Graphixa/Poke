// Adapted from https://github.com/erik-adelbert/firework (MIT)

package launcher

import (
	"image/color"
	"iter"
	"math"
	"strings"

	"github.com/Graphixa/Poke/internal/fireworks/sym"
	"github.com/Graphixa/Poke/internal/fireworks/vec"
)

// Paint rasterizes active particles into screen (clears first).
func (l *Launcher) Paint(screen []sym.Symbol, h, w int) {
	if len(screen) < h*w {
		return
	}

	idx := func(x cell) int {
		return w*x.r + x.c
	}

	clear(screen)

	for i, fwk := range l.Actives {
		for j, trail := range l.AllActiveTrails(i) {
			var (
				ii   int
				ok   bool
				α, ω vec.Vec
			)

			p := fwk.Particles[j]
			next, close := iter.Pull2(trail)
			_, α, ok = next()
			if !ok {
				close()
				continue
			}

			pointDrawn := false

			for n := p.TrailLen(); n > 0; {
				if ii, ω, ok = next(); !ok {
					break
				}

				d := float64(n-ii) / float64(n)
				allCells := allDDA(α, ω)

				for cell := range allInside(allCells, h, w) {
					rgba := fwk.Gradient(p)
					iii := idx(cell)

					if bright(screen[iii].RGBA) <= bright(rgba) {
						screen[iii] = sym.MkSymbol(getrune(p, d), rgba)
						pointDrawn = true
					}
				}

				α = ω
			}

			if !pointDrawn {
				c := cell{
					r: int(math.Round(α.Y)),
					c: int(math.Round(α.X)),
				}

				if c.inside(h, w) {
					iii := idx(c)

					if screen[iii].IsZero() {
						rgba := fwk.Gradient(p)
						screen[iii] = sym.MkSymbol(getrune(p, 0), rgba)
					}
				}
			}

			close()
		}
	}
}

// Stringify turns a painted screen into a terminal string.
func Stringify(screen []sym.Symbol, h, w int) string {
	if len(screen) < h*w {
		return ""
	}
	var sb strings.Builder
	sb.Grow(h * w)
	for i := range h {
		lo, hi := i*w, (i+1)*w
		for _, s := range screen[lo:hi:hi] {
			sb.WriteString(s.String())
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func (l *Launcher) Render(screen []sym.Symbol, h, w int) string {
	l.Paint(screen, h, w)
	return Stringify(screen, h, w)
}

func allInside(allCells iter.Seq[cell], h, w int) iter.Seq[cell] {
	return func(yield func(cell) bool) {
		for c := range allCells {
			if c.inside(h, w) && !yield(c) {
				return
			}
		}
	}
}

// bright calculates the brightness of a color as the average
// of its RGB components.
func bright(c color.RGBA) uint8 {
	return uint8((uint16(c.R) + uint16(c.G) + uint16(c.B)) / 3)
}
