// Adapted from https://github.com/erik-adelbert/firework (MIT)
// Show wrapper owned by poke for embedding in the browse TUI.

package fireworks

import (
	"math/rand/v2"
	"time"

	fw "github.com/Graphixa/Poke/internal/fireworks/firework"
	"github.com/Graphixa/Poke/internal/fireworks/launcher"
	"github.com/Graphixa/Poke/internal/fireworks/particle"
	"github.com/Graphixa/Poke/internal/fireworks/shells"
	"github.com/Graphixa/Poke/internal/fireworks/sym"
	"github.com/Graphixa/Poke/internal/fireworks/vec"
)

const (
	stepDT      = 30 * time.Millisecond
	burstMin    = 2
	burstMax    = 3
	refillEvery = 20 // ~600ms between new shells
)

// Show is a looping mixed firework display sized to the terminal.
type Show struct {
	*launcher.Launcher
	screen     []sym.Symbol
	h, w       int
	burstN     int
	sinceSpawn int
}

// Radial shells only.
var mixedNews = []fw.NewFirework{
	shells.NewPeony,
	shells.NewPeony,
	shells.NewChrysanthemum,
	shells.NewChrysanthemum,
	shells.NewBrocade,
	shells.NewGlitter,
	shells.NewKamuro,
}

// NewShow builds a mixed show for the given terminal cell size (1 cell = 1 column).
func NewShow(w, h int, now time.Time) *Show {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	tuneScale(w, h)
	s := &Show{
		h:      h,
		w:      w,
		burstN: burstCount(w, h),
		screen: make([]sym.Symbol, h*w),
	}
	// Start with one shell; Step tops up slowly.
	s.Launcher = launcher.New(s.spawnBurst(1), false)
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
	s.burstN = burstCount(w, h)
	tuneScale(w, h)
	if n := h * w; n > cap(s.screen) {
		s.screen = make([]sym.Symbol, n)
	} else {
		s.screen = s.screen[:n]
	}
	now := time.Now()
	s.Init(s.spawnBurst(1), false)
	s.sinceSpawn = 0
	s.Trigger(now)
}

func (s *Show) Step(now time.Time) {
	s.Update(now, stepDT)
	s.sinceSpawn++
	if len(s.Actives) < s.burstN && s.sinceSpawn >= refillEvery {
		f := s.newRandom()
		s.Activate(f)
		f.Trigger(now)
		s.sinceSpawn = 0
	}
}

func (s *Show) Render() string {
	return s.Launcher.Render(s.screen, s.h, s.w)
}

func (s *Show) Paint() {
	s.Launcher.Paint(s.screen, s.h, s.w)
}

func (s *Show) Screen() []sym.Symbol { return s.screen }

func (s *Show) Stringify() string {
	return launcher.Stringify(s.screen, s.h, s.w)
}

func (s *Show) W() int { return s.w }
func (s *Show) H() int { return s.h }

func (s *Show) spawnBurst(n int) []*fw.Firework {
	fires := make([]*fw.Firework, 0, n)
	for range n {
		fires = append(fires, s.newRandom())
	}
	return fires
}

func (s *Show) newRandom() *fw.Firework {
	return mixedNews[rand.IntN(len(mixedNews))](randomOrigin(s.w, s.h))
}

func randomOrigin(w, h int) vec.Vec {
	marginX := max(2, w/12)
	marginY := max(2, h/8)
	xSpan := w - 2*marginX
	ySpan := h - 2*marginY
	o := vec.Vec{X: float64(w / 2), Y: float64(h / 3)}
	if xSpan > 0 {
		o.X = float64(rand.IntN(xSpan) + marginX)
	}
	if ySpan > 0 {
		o.Y = float64(rand.IntN(ySpan) + marginY)
	}
	return o
}

func burstCount(w, h int) int {
	n := w * h / 1200
	if n < burstMin {
		n = burstMin
	}
	if n > burstMax {
		n = burstMax
	}
	return n
}

func tuneScale(w, h int) {
	// Bigger than before so radial spokes read as starbursts (g uses s²).
	m := min(w, h)
	s := float64(m) / 90.0 * 0.55
	if s < 0.42 {
		s = 0.42
	}
	if s > 0.72 {
		s = 0.72
	}
	particle.WorldScale = s
}
