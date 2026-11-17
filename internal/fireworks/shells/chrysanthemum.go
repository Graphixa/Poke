// Adapted from https://github.com/erik-adelbert/firework (MIT)

package shells

import (
	"image/color"
	"iter"
	"time"

	"github.com/Graphixa/Poke/internal/fireworks/firework"
	"github.com/Graphixa/Poke/internal/fireworks/helper"
	"github.com/Graphixa/Poke/internal/fireworks/particle"
	"github.com/Graphixa/Poke/internal/fireworks/vec"
)

const (
	chrysanthemumSpawningDuration = 100 * time.Millisecond
	chrysanthemumActiveDuration   = 100 * time.Millisecond
)

func NewChrysanthemum(o vec.Vec) *firework.Firework {
	ttl := time.Duration(1+helper.JitterInt(10, 1.)) * time.Second
	spawnAfter := time.Duration(1+helper.JitterInt(25, 1.)) * time.Millisecond

	return firework.New(
		o,
		firework.MkForces(0.45, 0.14, 0, nil),
		firework.MkShell(0, NewChrysanthemumSpawner()),
		firework.MkTiming(ttl, spawnAfter),
	)
}

type chrysanthemumSpawner struct {
	LUT
}

func NewChrysanthemumSpawner() *chrysanthemumSpawner {
	return &chrysanthemumSpawner{LUT: PickLut()}
}

func (s *chrysanthemumSpawner) Spawn(o vec.Vec, t time.Time, dt time.Duration) iter.Seq[*particle.Particle] {
	return func(yield func(*particle.Particle) bool) {
		n := helper.JitterInt(20, 1./2)

		for dir := range AllStratifiedCircle(1, n) {
			speed := helper.JitterFloat(155, 1./5)
			v := dir.Scale(speed)

			ttl := time.Duration(helper.JitterInt(1700, 5./17)) * time.Millisecond
			trail := helper.JitterInt(25, 1./5)
			phases := particle.PhaseTiming{
				SpawningEnd: float64(chrysanthemumSpawningDuration) / float64(ttl),
				ActiveEnd:   float64(chrysanthemumActiveDuration) / float64(ttl),
			}

			if !yield(particle.NewWithPhaseTiming(o, v, vec.Vec{}, s.PickColor(), ttl, trail, phases)) {
				return
			}
		}
	}
}

func (s *chrysanthemumSpawner) Gradient(p *particle.Particle) color.RGBA {
	c := p.Color()
	g := PhaseGradient2(p.Life())

	return scaleColor(c, g)
}
