package main

import (
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// IS MORE PERCEPTION WORTH BUYING RIGHT NOW?
//
// The one place production asks, and it asks observe.EscalationOf rather than deciding. The
// policy names no sensor; this is the caller that happens to hold an expensive one.
//
// # What it reads, and why that is the previous reading
//
// The gate runs INSIDE a collection cycle, and the sufficiency of that cycle does not exist
// until fusion has finished with it. So what it reads is the last reading the session settled
// on — which is correct for the question being asked. This is a BUDGET decision, not an
// evidence one: nothing it returns becomes belief, and a stale answer costs at worst one
// inference that was not needed or one that was.
//
// # IT RUNS INSIDE THE PERCEPTION CYCLE, SO IT MAY TAKE NO LOCK THE CYCLE HOLDS
//
// Both callers ask from within `liveSampler.Sample`, which holds `Runtime.mu` for the whole
// collect-and-fuse so the pinned window cannot move under the providers: `liveSampler.request`
// is evaluated as an argument inside that section, and the shadow provider's own gate is called
// deeper still, from inside `Collect`.
//
// `sync.Mutex` is not reentrant. When `incompleteFor` took `Runtime.mu`, the second sample of
// EVERY observation session blocked forever — Learn, Light Mode, ambient watching, and the fresh
// look a performance takes to find out where it is. The first sample survived only because
// nothing was settled yet, so this returned at `!p.Placed` before reaching the lock, which is
// what made a hang look like a slow start.
//
// Measured on Windows Settings: 14 samples in 12 seconds with the gate bypassed, 1 sample and
// then silence with it. Nothing in an 85-package suite saw it, because every test of this gate
// calls it directly and no test held the cycle's lock.
//
// So: `incompleteSince` has its own mutex, and anything added here must not reach for `mu`.
// Enforced by TestTheSensorGateDoesNotReenterTheCycleLock.
//
// It must never be read the other way round. "The last reading was sufficient" is not a claim
// about the current screen and may not stand in for one.
func (r *Runtime) moreEvidenceIsWorthBuying() bool {
	if r == nil || r.observations == nil {
		return true
	}
	p := r.observations.placeHere()
	if !p.Placed {
		// IGNORANCE, NOT AN ANSWER. No session running, no memory, or nothing settled
		// yet. None of those says the current reading is good enough.
		return true
	}
	// NeedAnswer: the conservative middle. Passive watching would decline more often and
	// acting would demand more, and this gate serves whatever the session is for — which it
	// cannot see from here. Choosing the stricter of the two it might be would spend on
	// background curiosity; choosing the looser would starve a question.
	return observe.EscalationOf(observe.NeedAnswer, observe.SufficiencyOf(p),
		r.incompleteFor(p)).Worth()
}

// incompleteFor is how long the reading has been incomplete, zero when it is not.
//
// Time CORROBORATES here and never classifies — 37D's judgement never looks at a clock, and
// this does not change that. All it decides is whether the cheap remedy has had its chance: a
// page that has been blank for two seconds is not still arriving.
func (r *Runtime) incompleteFor(p observe.Place) time.Duration {
	now := sessionClock.Now()
	r.incompleteSinceMu.Lock()
	defer r.incompleteSinceMu.Unlock()
	if observe.SufficiencyOf(p).State != observe.Incomplete {
		r.incompleteSince = nil
		return 0
	}
	if r.incompleteSince == nil {
		at := now
		r.incompleteSince = &at
		return 0
	}
	return now.Sub(*r.incompleteSince)
}
