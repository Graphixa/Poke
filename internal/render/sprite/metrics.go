package sprite

import "sync/atomic"

// Counters are process-local render diagnostics. They are cheap atomics and
// never print; debuglog.Trace covers timed events when logging is enabled.
type Counters struct {
	MemHits        atomic.Int64
	DiskHits       atomic.Int64
	ColdRenders    atomic.Int64
	Cancelled      atomic.Int64
	StaleDiscarded atomic.Int64
	HTTPArtwork    atomic.Int64
	PNGReads       atomic.Int64
	PNGDecodes     atomic.Int64
	SixelEncodes   atomic.Int64
}

var stats Counters

// Stats returns the process-wide counter set.
func Stats() *Counters { return &stats }

// ResetStats zeros every counter. Tests and acceptance audits use this.
func ResetStats() {
	stats.MemHits.Store(0)
	stats.DiskHits.Store(0)
	stats.ColdRenders.Store(0)
	stats.Cancelled.Store(0)
	stats.StaleDiscarded.Store(0)
	stats.HTTPArtwork.Store(0)
	stats.PNGReads.Store(0)
	stats.PNGDecodes.Store(0)
	stats.SixelEncodes.Store(0)
}

func noteMemHit()      { stats.MemHits.Add(1) }
func noteDiskHit()     { stats.DiskHits.Add(1) }
func noteCold()        { stats.ColdRenders.Add(1) }
func NoteCancelled()   { stats.Cancelled.Add(1) }
func NoteStale()       { stats.StaleDiscarded.Add(1) }
func NoteHTTPArtwork() { stats.HTTPArtwork.Add(1) }
func notePNGRead()     { stats.PNGReads.Add(1) }
func notePNGDecode()   { stats.PNGDecodes.Add(1) }

// NoteSixelEncode records one DECSIXEL encode. Called from the sixel package.
func NoteSixelEncode() { stats.SixelEncodes.Add(1) }
