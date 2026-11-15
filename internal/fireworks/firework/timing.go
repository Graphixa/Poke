// Adapted from https://github.com/erik-adelbert/firework (MIT)

package firework

import "time"

type Timing struct {
	t0              time.Time
	ttl, spawnAfter time.Duration
}

func MkTiming(ttl, spawnAfter time.Duration) Timing {
	return Timing{
		ttl:        ttl,
		spawnAfter: spawnAfter,
	}
}

func (t *Timing) Reset() {
	t.t0 = time.Time{}
}
