//go:build !windows

package recordingnow

import "time"

// processState cannot be answered here: there is no portable way, in the standard library, to read
// when a process started. Always unsure — so a recording cannot be announced and does not start,
// and a marker present is Unknown. Closed, not open.
func processState(uint32) (liveness, time.Time) { return unsure, time.Time{} }
