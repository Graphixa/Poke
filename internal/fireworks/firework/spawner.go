// Adapted from https://github.com/erik-adelbert/firework (MIT)

package firework

import (
	"image/color"
	"iter"
	"time"

	"github.com/Graphixa/Poke/internal/fireworks/particle"
	"github.com/Graphixa/Poke/internal/fireworks/vec"
)

type Spawner interface {
	Spawn(o vec.Vec, t time.Time, dt time.Duration) iter.Seq[*particle.Particle]
	Gradient(p *particle.Particle) color.RGBA
}
