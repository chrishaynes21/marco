package orchestrator

import (
	"slices"
	"sort"
	"strings"
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
	// Exit is the keys somebody used to leave the chosen window on their way to stopping
	// the recording, whose presses have been taken out of Events.
	//
	// SAID, NEVER SILENT. Leaving something out of a demonstration is exactly the
	// discard this boundary exists to refuse, and the only reason it is allowed here is
	// that these presses are how the recording was ENDED rather than part of it. A person
	// who disagrees can only do so if review tells them. See exitGesture.
	Exit []string
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
	// open is what the boundary stranded and nothing has yet depended on; crossed is what
	// the demonstration carried on past. See EvHeldAcross for why the recorder reports the
	// key and this decides what it meant.
	var open, crossed []string
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
		case recorder.EvHeldAcross:
			if !slices.Contains(open, ev.KeyName) {
				open = append(open, ev.KeyName)
			}
			continue
		}
		// A STRANDED KEY THE DEMONSTRATION CARRIED ON PAST HAS CROSSED, and this is the
		// instant that settles it: something was admitted after the boundary stranded it,
		// so whatever that something meant, it meant it with a key down whose release
		// happened in another window. What is left after the loop was stranded with
		// nothing following it, which is somebody on their way out.
		if len(open) > 0 {
			crossed, open = append(crossed, open...), nil
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
	// A RECORDING THAT IS BEING REFUSED IS SHOWN WHOLE.
	//
	// Nothing is trimmed out of it, because nothing is being saved from it: the person is
	// about to be told it cannot be kept, and the one thing they need in order to give the
	// demonstration again is all of what they did. Taking the way out of a refused
	// recording would also have review saying "these were left out" about a recording that
	// is not going anywhere, which is a sentence about nothing.
	if len(crossed) == 0 && st.Crossing == "" {
		st.Exit, st.Events = exitGesture(st.Events, open, st.Gone)
	}
	if st.Exit == nil {
		crossed = append(crossed, open...)
	}
	// AND A STRANDED KEY THAT WAS NOT THE WAY OUT REFUSES THE SAVE.
	//
	// An `EvCrossed` reason wins when there is one, which keeps the rule the markers
	// already had: it names a gesture that is WRITTEN DOWN wrong — an action taken with
	// something held from outside — where this one names a gesture that is missing a half.
	// The first is the more specific thing to be told.
	// FIRST REASON WINS, which is the rule the markers already had, and it is why `crossed`
	// is not sorted: its order is the order things crossed in — the keys promoted as the
	// demonstration went past them, then whatever the last leaving stranded. The first is
	// the one the person can do something about.
	if st.Crossing == "" && len(crossed) > 0 {
		st.Crossing = crossed[0] + " was held down in the window and let go somewhere " +
			"else, and the demonstration carried on afterwards, so the recording has " +
			"half of a gesture in it"
	}
	return st
}

// exitGesture is the keys somebody used to leave the chosen window, and the recording with their
// presses taken out — or nil, and the recording exactly as it was.
//
// # The one discard this boundary allows, and what bounds it
//
// Leaving a window with a key still down used to refuse the recording outright, which made the
// one obvious way to finish — go back to the window with the Stop button in it — refuse every
// recording it ended. Alt-Tab is a key gesture, so Alt-Tab to Stop stranded Alt and Tab at the
// boundary every single time.
//
// But a held key dropped silently is exactly how `ctrl+click` becomes `click`, which is the
// hazard the whole boundary exists for. So the discard is bounded by three things, and each of
// them is load-bearing:
//
//   - NOTHING FOLLOWS THEM. Everything after the first of these presses must be another of
//     them. Alt-Tab's presses are the last things in the buffer, because what happened after
//     them happened outside the window and was never admitted; a key held through the rest of
//     a demonstration has that demonstration after it, and refuses.
//   - KEYS ONLY. A mouse button stranded at the boundary is a drag that crossed it, and
//     dropping its press would turn a drag into nothing at all. Leaving a window is a keyboard
//     gesture. This one is STRUCTURAL rather than a branch, which is worth saying because it
//     means no single deletion below changes it: presses are identified by `KeyName`, and a
//     click does not have one — the boundary calls it "the left button". The kind check states
//     the intent; the identity is what enforces it.
//   - SOMEBODY LEFT. A key held when the window went AWAY is a gesture the closing cut in half;
//     nobody navigated anywhere, and there is no Stop button they were walking to.
func exitGesture(events []recorder.RecordedEvent, held []string,
	gone bool) ([]string, []recorder.RecordedEvent) {

	if len(held) == 0 || gone {
		return nil, events
	}
	want := map[string]bool{}
	for _, id := range held {
		want[id] = true
	}
	// THE PRESSES OF THEIRS STILL OPEN AT THE END. A key the person pressed and released
	// inside the window is a step, even when the same key was stranded later.
	open := map[string]int{}
	for i, ev := range events {
		if ev.Kind != recorder.EvKey || !want[ev.KeyName] {
			continue
		}
		if ev.Down {
			open[ev.KeyName] = i
		} else {
			delete(open, ev.KeyName)
		}
	}
	if len(open) == 0 {
		return nil, events
	}
	first := len(events)
	for _, i := range open {
		first = min(first, i)
	}
	for _, ev := range events[first:] {
		if ev.Kind != recorder.EvKey || !ev.Down || !want[ev.KeyName] {
			return nil, events
		}
	}
	out := make([]string, 0, len(open))
	for id := range open {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, events[:first]
}

// joinWords is a list in a sentence: "alt and tab", "alt, shift and tab".
func joinWords(in []string) string {
	switch len(in) {
	case 0:
		return ""
	case 1:
		return in[0]
	}
	return strings.Join(in[:len(in)-1], ", ") + " and " + in[len(in)-1]
}
