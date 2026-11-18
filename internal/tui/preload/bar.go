package preload

// Bar is one fraction over every download part.
// A part's share is its item count, so a 1,600-item pass moves the bar
// more than a 1-item pass. Reserved weights hold space until that part
// reports a real total; a non-positive total drops the part (it was skipped).
type Bar struct {
	parts map[string]part
}

type part struct {
	done, total int
}

// NewBar reserves upcoming parts so the fraction cannot hit 1
// before those parts exist. A later Advance replaces the reserve.
func NewBar(reserve map[string]int) *Bar {
	b := &Bar{parts: make(map[string]part, len(reserve))}
	for name, n := range reserve {
		if n > 0 {
			b.parts[name] = part{total: n}
		}
	}
	return b
}

// Advance records one part. total <= 0 removes it.
// Returns items finished and items planned across every part.
func (b *Bar) Advance(name string, done, total int) (sumDone, sumTotal int) {
	if b.parts == nil {
		b.parts = map[string]part{}
	}
	if name == "" {
		return b.sums()
	}
	if total <= 0 {
		delete(b.parts, name)
		return b.sums()
	}
	if done < 0 {
		done = 0
	}
	if done > total {
		done = total
	}
	b.parts[name] = part{done: done, total: total}
	return b.sums()
}

func (b *Bar) sums() (sumDone, sumTotal int) {
	for _, p := range b.parts {
		sumDone += p.done
		sumTotal += p.total
	}
	return sumDone, sumTotal
}
