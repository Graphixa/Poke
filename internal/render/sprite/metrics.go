package sprite

import "sync/atomic"

// Counters are process-local render diagnostics. They are cheap atomics and
// never print; debuglog.Trace covers timed events when logging is enabled.
type Counters struct {
	MemHits          atomic.Int64
	DiskHits         atomic.Int64
	ColdRenders      atomic.Int64
	Cancelled        atomic.Int64
	StaleDiscarded   atomic.Int64
	NeighbourRenders atomic.Int64
	HTTPArtwork      atomic.Int64
	PNGReads         atomic.Int64
}

var stats Counters

// Stats returns the process-wide counter set.
func Stats() *Counters { return &stats }

func noteMemHit()      { stats.MemHits.Add(1) }
func noteDiskHit()     { stats.DiskHits.Add(1) }
func noteCold()        { stats.ColdRenders.Add(1) }
func NoteCancelled()   { stats.Cancelled.Add(1) }
func NoteStale()       { stats.StaleDiscarded.Add(1) }
func NoteNeighbour()   { stats.NeighbourRenders.Add(1) }
func NoteHTTPArtwork() { stats.HTTPArtwork.Add(1) }
func notePNGRead()     { stats.PNGReads.Add(1) }
