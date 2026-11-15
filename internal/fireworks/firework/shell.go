// Adapted from https://github.com/erik-adelbert/firework (MIT)

package firework

import (
	"iter"
	"time"

	"github.com/Graphixa/Pokedex/internal/fireworks/helper"
	"github.com/Graphixa/Pokedex/internal/fireworks/particle"
	"github.com/Graphixa/Pokedex/internal/fireworks/vec"
)

type ExplosionType int

const (
	Instant ExplosionType = iota
	Sustained
)

type Shell struct {
	step time.Duration

	next time.Time
	last time.Time

	kind ExplosionType
	Spawner
}

func MkShell(step time.Duration, spawner Spawner) Shell {
	kind := Sustained
	if step <= 0 {
		kind = Instant
		step = 0
	}

	return Shell{
		step:    step,
		kind:    kind,
		Spawner: spawner,
	}
}

func (sh *Shell) Reset() {
	sh.next = time.Time{}
	sh.last = time.Time{}
}

func (sh *Shell) Spawn(o vec.Vec, now time.Time, dt time.Duration) iter.Seq[*particle.Particle] {
	seq := helper.EmptySeq[*particle.Particle]()

	if !sh.last.IsZero() && sh.kind == Instant {
		// Already exploded, do nothing.
		return seq
	}

	if sh.last.IsZero() || now.After(sh.next) {
		sh.last = now

		if sh.next = (time.Time{}); sh.kind == Sustained {
			sh.next = now.Add(sh.step)
		}

		seq = sh.Spawner.Spawn(o, now, dt)
	}

	return seq
}
