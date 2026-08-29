package uiaclient

import (
	"sort"
	"sync"
	"time"
)

// COUNT THE WALKS, TIME THEM, AND SAY WHY EACH ONE HAPPENED.
//
// # Why this exists and why it is this small
//
// 37C measured one accessibility walk: 104–120ms on Settings, 1.5 seconds on Explorer. What
// nobody had measured is how many of them one ordinary operation pays for. The bridge holds no
// state between snapshots, subscribes to no UI Automation events, and walks the whole subtree
// from the window root every time — so the answer is entirely a question of how often the
// Director asks, and that question was being reasoned about rather than counted.
//
// So: a counter, a duration, and a purpose. Not a tracing framework. Nothing here samples,
// aggregates over time, writes a file, or has a lifecycle — it is a tally that a diagnostic can
// read and a test can assert on, and if it ever needs a background goroutine it has become the
// wrong thing.
//
// It is OFF unless something asks for it. An always-on counter is a lock on the hot path of the
// slowest thing in perception, and the walk it would be measuring is three orders of magnitude
// more expensive than the measurement — but "cheap enough" is how permanent instrumentation
// gets justified, and the honest position is that nothing in production needs this number.

// WalkPurpose is why a walk was paid for, from a closed set.
//
// Bounded like observe.SufficiencyReason and for the same reason: the useful question is "which
// of a handful of things caused this", and an open string would drift into free text nobody can
// group by.
type WalkPurpose string

const (
	// PurposeUnattributed is a walk whose caller did not say. Not an error — most call
	// sites predate this — but a large count here means the tally cannot answer the
	// question it exists for.
	PurposeUnattributed WalkPurpose = "unattributed"
	// PurposeAmbient is ambient observation: watching, with no authority and no request.
	PurposeAmbient WalkPurpose = "ambient"
	// PurposeSession is a sample inside an observation session — the shape freshLook,
	// Learn and Observe all reach the desktop through.
	PurposeSession WalkPurpose = "session"
	// PurposeCommand is a one-shot reading for an inspect or diagnostic command.
	PurposeCommand WalkPurpose = "command"
)

// WalkTally is what was paid, per purpose.
type WalkTally struct {
	Purpose WalkPurpose
	Walks   int
	Total   time.Duration
	Longest time.Duration
}

var walks struct {
	mu sync.Mutex
	on bool
	// purpose is the label the NEXT walk is attributed to. A single value rather than a
	// context key because the Director drives one accessibility bridge and the walks are
	// serialised through it; a per-call context would be more correct and would also mean
	// threading a parameter through six layers to answer a question that is being asked
	// once, during an audit.
	purpose WalkPurpose
	by      map[WalkPurpose]*WalkTally
}

// CountWalks turns the tally on and clears it, returning a function that turns it off.
//
// Everything is reset on start rather than on stop: a caller that forgets to stop leaks a
// counter and a map, and a caller that reads after stopping gets the run it asked about.
func CountWalks() func() {
	walks.mu.Lock()
	defer walks.mu.Unlock()
	walks.on = true
	walks.purpose = PurposeUnattributed
	walks.by = map[WalkPurpose]*WalkTally{}
	return func() {
		walks.mu.Lock()
		defer walks.mu.Unlock()
		walks.on = false
	}
}

// AttributeWalksTo says what the walks from here on are for.
//
// Returns a function restoring the previous purpose, so a nested attribution — a session
// sampling inside a command, say — puts the outer one back rather than clearing it.
func AttributeWalksTo(p WalkPurpose) func() {
	walks.mu.Lock()
	defer walks.mu.Unlock()
	if !walks.on {
		return func() {}
	}
	previous := walks.purpose
	walks.purpose = p
	return func() {
		walks.mu.Lock()
		defer walks.mu.Unlock()
		walks.purpose = previous
	}
}

// WalksSoFar is the tally, ordered by cost so the expensive purpose reads first.
func WalksSoFar() []WalkTally {
	walks.mu.Lock()
	defer walks.mu.Unlock()
	out := make([]WalkTally, 0, len(walks.by))
	for _, t := range walks.by {
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Total != out[j].Total {
			return out[i].Total > out[j].Total
		}
		return out[i].Purpose < out[j].Purpose
	})
	return out
}

// TotalWalks is how many were paid for, and how long they took altogether.
func TotalWalks() (int, time.Duration) {
	n := 0
	var d time.Duration
	for _, t := range WalksSoFar() {
		n += t.Walks
		d += t.Total
	}
	return n, d
}

// recordWalk tallies one completed walk. A no-op when counting is off.
func recordWalk(took time.Duration) {
	walks.mu.Lock()
	defer walks.mu.Unlock()
	if !walks.on {
		return
	}
	t := walks.by[walks.purpose]
	if t == nil {
		t = &WalkTally{Purpose: walks.purpose}
		walks.by[walks.purpose] = t
	}
	t.Walks++
	t.Total += took
	if took > t.Longest {
		t.Longest = took
	}
}
