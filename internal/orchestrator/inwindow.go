package orchestrator

import (
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
)

// ── THE CLOCK OF A RECORDING THAT WAITED ─────────────────────────────────────

// ResumeSettle is what is left in the clock where an absence was.
//
// # It is a timing policy and NOT a readiness claim
//
// A quarter of a second is what a window that has just come forward costs a person: the
// animation finishes, the first click lands. It establishes NOTHING about the application being
// ready — nothing was asked and nothing was observed — and playback does not activate the window
// here either. It is a pause, chosen because zero would replay a demonstration faster than it
// was given and the real absence would replay the time somebody spent reading their messages.
//
// If a routine needs the window brought forward, that is an Activate, and an Activate is
// something `simplify` emits from a foreground switch that actually happened.
const ResumeSettle = 250 * time.Millisecond

// capture is what the markers in a recording say about how it was made.
type capture struct {
	// Events is the recording with the markers consumed and the clock rewritten.
	//
	// IT CONTAINS NO MARKERS, and that is the compatibility answer rather than a tidy-up.
	// A marker left in a stored recording is a kind an older build does not know, and an
	// older build regenerating from it would ignore the marker, keep the raw timestamps,
	// and produce a routine with a thirty-second pause in it where somebody had looked
	// something up. That is changed behaviour dressed as degradation. So the absence is
	// taken out of the TIMESTAMPS, which every reader of every vintage already interprets
	// the same way, and the recording that lands on disk holds only the kinds it always
	// held. There is nothing in it to reinterpret and therefore no version to refuse.
	Events []recorder.RecordedEvent
	// Crossing is why this recording cannot be saved, or "" when it can.
	Crossing string
	// Away and Absences are what review tells the person: how long they were out of the
	// window and how many times. The leading absence — pressing Record and going to the
	// window — is not one of them, because they had not arrived yet.
	Away     time.Duration
	Absences int
	// Gone is whether the chosen window stopped existing while recording.
	Gone bool
}

// inWindowClock takes the time spent away from the chosen window out of a recording's clock, and
// consumes the capture-state markers into what review needs to know.
//
// # Why the clock and not a wait
//
// Because `simplify.insertWaits` keeps each step's real recorded gap. Leaving the absence in the
// timestamps and dropping a marker beside it means the gap across the absence IS the absence,
// and the routine waits it out on every run. Rewriting the timestamps is the one place that gap
// is decided for every reader at once.
//
// Time spent INSIDE the window is preserved exactly: a person who paused for ten seconds in the
// middle of a demonstration meant to, and the routine waits with them.
func inWindowClock(events []recorder.RecordedEvent, settle time.Duration) capture {
	var st capture
	var shift time.Duration
	var leftAt time.Time
	away := false
	// resume closes an open absence at `at`, moving the clock by what is being taken out.
	resume := func(at time.Time, arrived bool) {
		if !away {
			return
		}
		away = false
		if d := at.Sub(leftAt); d > settle {
			shift += d - settle
		}
		// A LEADING ABSENCE IS NOT ONE. Nothing had been recorded, so the person had
		// not arrived yet: pressing Record and walking to the window is how a
		// recording starts, not an interruption of one. `arrived` separates the two
		// without asking the caller to know which marker came first.
		if arrived {
			st.Away += at.Sub(leftAt)
			st.Absences++
		}
	}
	for _, ev := range events {
		switch ev.Kind {
		case recorder.EvLeft:
			if !away {
				leftAt, away = ev.T, true
			}
			continue
		case recorder.EvEntered:
			resume(ev.T, len(st.Events) > 0)
			continue
		case recorder.EvCrossed:
			// FIRST REASON WINS, the same way the boundary itself keeps the
			// first: the thing that crossed first is the one the person can do
			// something about.
			if st.Crossing == "" {
				st.Crossing = ev.KeyName
			}
			continue
		case recorder.EvTargetGone:
			st.Gone = true
			continue
		}
		// AND AN EVENT IN THE WINDOW ENDS AN ABSENCE, whatever the markers say.
		//
		// Not belt and braces: the app-switch poller writes into the buffer directly,
		// so the Activate for the window somebody has just arrived in is recorded
		// BEFORE the first key or click that produces an `EvEntered`. Closing the
		// absence here as well means that Activate is not left stranded at its raw
		// timestamp with the whole walk to the window in front of it — which would be
		// a long wait before the first step, which is the defect this function exists
		// to prevent.
		resume(ev.T, len(st.Events) > 0)
		ev.T = ev.T.Add(-shift)
		st.Events = append(st.Events, ev)
	}
	// A TRAILING ABSENCE ADDS NOTHING. Somebody who walks away and stops recording from
	// somewhere else has not asked the routine to wait at the end, and there is nothing
	// after the gap for a gap to be between.
	return st
}
