package main

// WHICH BUILD THIS WINDOW IS, AND WHICH BUILD IS DOING THE RECORDING.
//
// # Why this is its own read
//
// Because it is the RAIL's question and it was answered by the SCENES view's poll. `Learning`
// returns a demonstration's whole state — its phase, its transcript, its press counts, what it
// learned — and two fields of it paint the build stamp in the rail's foot. So withholding the
// Scenes view left a choice between a poll running two and a half times a second for a surface
// nobody can reach, and a rail that stops saying which build is recording.
//
// That is not a stamp worth losing. Studio spawns the `director.exe` and the `marco.exe` beside
// ITSELF, so a stale sibling is invisible: every binary looks freshly built, the window is
// current, and the recordings come from whenever that director was last made. CLAUDE.md records
// what the invisible version of that cost — *"It cost the owner a day of recordings."* The stamp
// is the one thing in this window that makes it visible.
//
// So the two questions are separated. This one is cheap, answers from the same two readings
// `Learning` answers from — so the rail and the Scenes card cannot disagree about a build — and
// is asked whatever is offered. The Scenes view's query and its painting stop entirely when
// Scenes is withheld.
//
// # The recorder this build cannot read
//
// `recordingNow` has a third answer: a progress stream written by a build whose format this one
// does not understand. `Learning` reports that as a SENTENCE on the Scenes card and does not set
// `Mismatched`, so the rail stayed quiet about it — and a recorder this build cannot read a word
// of is the plainest kind of stale director there is. It is a mismatch here. That is a deliberate
// difference from what the rail showed before, in the one function whose whole job is to say so.
//
// See [[ADR-288-the-rail-keeps-its-build-stamp-when-scenes-is-withheld]].

import "github.com/chaynes-simpleclouds/marco/internal/buildid"

// BuildStatus is the rail's foot: this window's revision, the recorder's, and whether they differ.
type BuildStatus struct {
	// Build is this window's own revision, or "unknown" when the toolchain left no stamp.
	Build string `json:"build"`
	// Recorder is the revision of the recorder that is running, and "" when none is.
	//
	// BOTH VALUES, NEVER A YES/NO. "They differ" is what somebody acts on and the two
	// revisions are what tell them which one to rebuild.
	Recorder string `json:"recorder,omitempty"`
	// Mismatched says the two are known to be different builds.
	//
	// `buildid.Differ` and not `!Same`: an unstamped binary cannot vouch for a match AND
	// cannot be seen to disagree, so warning on one would make every pair that carries no
	// stamp an alarm about a difference nobody observed. The exception is a recorder
	// whose stream this build cannot parse, which is a difference that HAS been observed.
	Mismatched bool `json:"mismatched,omitempty"`
}

// BuildStatus is which build this window is and which build is recording.
//
// IT ASKS THE RECORDER'S STREAM AND NOTHING ELSE. No demonstration is listed, no transcript is
// copied, no phase is worked out — those are the Scenes view's and are `Learning`'s.
func (a *Acts) BuildStatus() BuildStatus {
	out := BuildStatus{Build: a.myBuild()}
	at, _, foreign := a.recordingNow()
	if foreign {
		// A RECORDER THIS BUILD CANNOT READ A WORD OF. Its revision is unknown —
		// the stream it writes is the thing that would have carried it — so there
		// is no second number to show, and the difference is still established.
		out.Mismatched = true
		return out
	}
	out.Recorder = at.Build
	out.Mismatched = buildid.Differ(out.Build, at.Build)
	return out
}
