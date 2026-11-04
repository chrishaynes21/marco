package main

import (
	"fmt"
	"os"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/gate"
)

// A DIRECTOR STARTED FOR SOMEBODY STOPS WHEN THEY HAVE GONE AND NOTHING IS USING IT.
//
// The widget starts the Director when it opens (`marco director start --for <pid>`), so that the
// first request does not wait for one. Before that, a Director was started only by a request that
// needed it, and then ran until somebody shut it down, whoever had started it. One started for
// the widget must not outlive it, but must not stop under somebody either. So, with `--for`:
//
//   - nothing happens while the process it was started for is alive;
//   - once that process has exited, the Director stops at the first moment nothing is using it:
//     no command running, no question or confirmation outstanding, no client connected, no
//     observation and no watching;
//   - and the reading and the stop are ONE act, under the lock that admits connections
//     (service.Server.ShutdownIfUnused): a client that arrives as the stop is decided is either
//     admitted first and keeps the Director, or refused as "stopping" before any of its work is
//     accepted. Reading "unused" and then stopping would admit it in between.
//
// Without `--for` nothing changes, so a Director somebody started by hand, or that a request
// started, still runs until it is shut down. And `--for` is only honoured by the Director it
// STARTS: a widget that finds one already running leaves it alone, because it did not start it.
//
// Deleting the call in runServe must fail TestADirectorStartedForAProcessStopsWhenItHasGoneAndNothingUsesIt.

// owner is the process a Director was started for.
type owner struct {
	pid uint32
}

// ownerExit is when the owner has gone. An owner that cannot be identified, because it has
// already exited or this platform cannot say, counts as gone: a Director started for somebody
// who cannot be found must not run on for ever on their account.
func ownerExit(o owner) (<-chan struct{}, error) {
	since, err := gate.StartedAt(o.pid)
	if err != nil {
		return closed(), err
	}
	gone, err := gate.Exited(gate.Identity{PID: o.pid, Since: since})
	if err != nil {
		return closed(), err
	}
	return gone, nil
}

func closed() <-chan struct{} {
	c := make(chan struct{})
	close(c)
	return c
}

// stopWhenUnused waits for gone, then asks tryStop once every `every` until it has stopped the
// service. It returns when it has, or when done closes.
//
// tryStop is the server's ShutdownIfUnused: the reading and the decision are one act under the
// lock that admits connections, so this loop never reads "unused" and stops later.
func stopWhenUnused(gone, done <-chan struct{}, tryStop func() (string, bool),
	every time.Duration, say func(string)) {

	select {
	case <-gone:
	case <-done:
		return
	}
	say("the process this Director was started for has exited; it stops when nothing is using it")
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		if _, stopped := tryStop(); stopped {
			say("nothing was using this Director; it has stopped")
			return
		}
		select {
		case <-t.C:
		case <-done:
			return
		}
	}
}

// runtimeInUse is what the Director is doing on its own that should keep it running.
func (r *Runtime) runtimeInUse() string {
	if r == nil {
		return ""
	}
	if r.observations != nil && r.observations.ActiveID() != "" {
		return "observing"
	}
	if r.AmbientStatus().Watching {
		return "watching"
	}
	return ""
}

func sayStderr(quiet bool) func(string) {
	return func(s string) {
		if !quiet {
			fmt.Fprintln(os.Stderr, "director: "+s)
		}
	}
}
