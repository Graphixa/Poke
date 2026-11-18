// Adapted from https://github.com/erik-adelbert/firework (MIT)

package firework

import (
	"github.com/Graphixa/Poke/internal/fireworks/particle"
	"github.com/Graphixa/Poke/internal/fireworks/vec"
)

type Forces struct {
	g   float64 // gravity
	ar  float64 // air resistance
	spd float64 // max speed

	af func(p *particle.Particle) vec.Vec // additional force
}

func MkForces(gravity, drag, maxSpeed float64, applyForce func(p *particle.Particle) vec.Vec) Forces {
	return Forces{
		g:   gravity,
		ar:  drag,
		spd: maxSpeed,

		af: applyForce,
	}
}
