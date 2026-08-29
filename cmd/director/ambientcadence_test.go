package main

import (
	"testing"
	"time"
)

// A QUIET DESKTOP IS WATCHED MORE CHEAPLY, AND A BUSY ONE IS NOT.
//
// # The cost this is about
//
// One accessibility walk of a File Explorer window measured 1.67 seconds — 298 elements, six
// consecutive readings, all identical (37E, `director walk-audit`). The bridge keeps no state
// between walks and subscribes to no UI Automation events, so every reading rebuilds the whole
// subtree from the window root. That is the right shape for a walk and says nothing about how
// often one is worth taking.
//
// Ambient watching already had the answer to how often: `attention` grows from a second to
// eight while nothing changes and resets the moment something does. It governed the gap
// BETWEEN twenty-second sessions and not the cadence inside one, so a settled desktop still
// paid a flat one-second interval for the whole of every session — about seven walks of an
// unchanged Explorer tree, twelve seconds of walking in every twenty.
//
// Passing the same attention through as the session's interval is the whole change. No
// evidence is retained, nothing is cached, and every sample is still a complete fresh walk
// taken at the moment it is reported. There are fewer of them when nothing is happening.
//
// The three assertions below are the three ways this could be got wrong: not applied at all,
// applied to the wrong thing, or applied to a desktop somebody is using.

func TestAQuietDesktopIsWatchedMoreCheaply(t *testing.T) {
	// The supervisor's own backoff, exercised directly. This is the signal the session
	// cadence now reads, so the two cannot drift apart without one of these failing.
	settling := ambientBusy
	for i := 0; i < 6; i++ {
		settling = nextAttention(settling, false)
	}
	if settling != ambientIdle {
		t.Fatalf("attention settled at %v, want %v — the backoff this reuses has changed",
			settling, ambientIdle)
	}

	// A walk of the Explorer window measured at 1.67s. Cycle time is the walk plus the gap
	// the scheduler waits AFTER it completes (see observesession: an overrunning slot
	// re-bases to now+interval rather than queueing a backlog).
	const walk = 1670 * time.Millisecond
	const session = ambientSession

	before := int(session / (walk + ambientBusy))
	after := int(session / (walk + settling))
	if after >= before {
		t.Fatalf("a settled desktop takes %d walks a session and a busy one %d; the "+
			"cadence is not relaxing", after, before)
	}
	t.Logf("unchanged Explorer, %v session, %v walk: %d walks at the busy gap (%v), "+
		"%d at the settled gap (%v)", session, walk, before, ambientBusy, after, settling)
}

// The change reaches ambient watching and nothing else.
//
// A look taken to answer a question sets its own interval, and execution must not get slower
// because the desktop has been quiet. This is the mutation "Observe freshness semantics
// accidentally applied to Perform", made structural: the two intervals are different constants
// reached through different call sites, and freshLook's is not the supervisor's.
func TestTheQuietCadenceDoesNotReachALook(t *testing.T) {
	if freshLookInterval >= ambientIdle {
		t.Errorf("freshLookInterval is %v and the settled ambient gap is %v.\n"+
			"A look exists to answer a question now. If it ever samples as slowly as "+
			"background watching, a Perform waits on a quiet desktop.",
			freshLookInterval, ambientIdle)
	}
	if freshLookInterval > ambientBusy {
		t.Errorf("freshLookInterval is %v, slower than the busiest ambient gap %v",
			freshLookInterval, ambientBusy)
	}
}

// Something happening puts the cadence back immediately.
//
// The saving is only honest if it disappears the instant the desktop is used. A backoff that
// decayed slowly would make Marco least attentive just as somebody started doing something.
func TestAChangeRestoresFullAttentionAtOnce(t *testing.T) {
	settled := ambientIdle
	if got := nextAttention(settled, true); got != ambientBusy {
		t.Errorf("after a change the gap is %v, want %v.\n"+
			"Watching must return to full cadence on the first change, not decay back "+
			"to it — the moment something happens is the moment evidence matters most.",
			got, ambientBusy)
	}
}
