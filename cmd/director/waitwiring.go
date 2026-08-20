package main

import waitengine "github.com/chaynes-simpleclouds/marco/internal/director/wait/engine"

// ActiveWait reports the wait currently running.
//
// Read-only and cheap: it looks at the engine's own record of what it is doing and
// starts nothing. A diagnostic that observed in order to answer would change the thing
// it was reporting on.
func (r *Runtime) ActiveWait() waitengine.Snapshot {
	if r.waits == nil {
		return waitengine.Snapshot{}
	}
	return r.waits.Active().Snapshot()
}
