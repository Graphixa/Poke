// Adapted from https://github.com/erik-adelbert/firework (MIT)
// Show wrapper owned by poke for embedding in the browse TUI.

package fireworks

import (
	"math/rand/v2"
	"time"

	fw "github.com/Graphixa/Pokedex/internal/fireworks/firework"
	"github.com/Graphixa/Pokedex/internal/fireworks/launcher"
	"github.com/Graphixa/Pokedex/internal/fireworks/shells"
	"github.com/Graphixa/Pokedex/internal/fireworks/sym"
	"github.com/Graphixa/Pokedex/internal/fireworks/vec"
)

const stepDT = 30 * time.Millisecond

// Show is a looping mixed firework display sized to the terminal.
type Show struct {
	*launcher.Launcher
	screen []sym.Symbol
	h, w   int
}

var mixedNews = []fw.NewFirework{
	shells.NewPeony,
	shells.NewChrysanthemum,
	shells.NewBrocade,
	shells.NewPalm,
	shells.NewWillow,
	shells.NewGlitter,
	shells.NewKamuro,
}

// NewShow builds a mixed looping show for the given terminal cell size (1 cell = 1 column).
func NewShow(w, h int, now time.Time) *Show {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	s := &Show{
		h:      h,
		w:      w,
		screen: make([]sym.Symbol, h*w),
	}
	s.Launcher = launcher.New(s.spawnMixed(w, h), true)
	s.Trigger(now)
	return s
}

func (s *Show) Resize(w, h int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if w == s.w && h == s.h {
		return
	}
	s.w, s.h = w, h
	if n := h * w; n > cap(s.screen) {
		s.screen = make([]sym.Symbol, n)
	} else {
		s.screen = s.screen[:n]
	}
	now := time.Now()
	s.Init(s.spawnMixed(w, h), true)
	s.Trigger(now)
}

func (s *Show) Step(now time.Time) {
	s.Update(now, stepDT)
}

func (s *Show) Render() string {
	return s.Launcher.Render(s.screen, s.h, s.w)
}

// Paint fills the screen buffer with the current particle frame.
func (s *Show) Paint() {
	s.Launcher.Paint(s.screen, s.h, s.w)
}

// Screen is the painted cell buffer (row-major). Mutate then Stringify.
func (s *Show) Screen() []sym.Symbol { return s.screen }

// Stringify renders the current screen buffer.
func (s *Show) Stringify() string {
	return launcher.Stringify(s.screen, s.h, s.w)
}

func (s *Show) W() int { return s.w }
func (s *Show) H() int { return s.h }

func (s *Show) spawnMixed(w, h int) []*fw.Firework {
	n := 12
	fires := make([]*fw.Firework, 0, n)
	for range n {
		o := vec.Vec{X: float64(w / 2), Y: float64(h / 3)}
		if xSpan := 9*w/20 - w/20; xSpan > 0 {
			o.X = float64(rand.IntN(xSpan) + w/20)
		}
		if ySpan := h/2 - h/6; ySpan > 0 {
			o.Y = float64(rand.IntN(ySpan) + h/6)
		}
		fires = append(fires, mixedNews[rand.IntN(len(mixedNews))](o))
	}
	return fires
}
