package debuglog

import "time"

// Trace logs name.start immediately and name.end with duration when logging
// is enabled. When discarded, both the start log and the closer are no-ops.
func Trace(cat Category, name string) func() {
	l := Global()
	if !l.Enabled() {
		return func() {}
	}
	start := time.Now()
	l.Log(cat, "%s.start", name)
	return func() {
		l.Log(cat, "%s.end dur_us=%d", name, time.Since(start).Microseconds())
	}
}
