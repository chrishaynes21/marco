package main

// WHAT MARCO IS DOING, IN THE WORDS OF THE THING THE PERSON ASKED FOR.
//
// # Why this is not the phase
//
// Because the phase is the runner's vocabulary and it is right for a runner: `starting`,
// `acting`, `verifying`, `paused`. A person who pressed Play on "open the audio settings" is
// owed a sentence about that — "Step 1 of 2 · open the audio settings", "Checking that it
// worked…", "Preparing Ark…" — and the phase beside it is evidence, not the headline.
//
// # Why it is Go and not the page
//
// Same reason as said.go: it is the part that can be wrong. There is one message per situation a
// run can be in, and a page with a switch over them has no way of saying which situations it
// covers. This has a test that walks every one of them.
//
// Nothing here decides anything. Every field it reads was written by the run.

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
	"github.com/chaynes-simpleclouds/marco/internal/director/runstate"
)

// doingOf is the one sentence at the top of the run screen, or "" when the run has ended.
//
// An ended run is said by `saidOf` instead: what it DID is the headline then, and a progress
// line that lingered after the end would be the window's own stale opinion.
func doingOf(application string, st *runstate.Event, active bool) string {
	if st == nil {
		if active {
			return preparing(application)
		}
		return ""
	}
	at := st.Interaction
	switch st.Phase {
	case runstate.Preparing, runstate.Starting:
		// ACQUIRING THE APPLICATION. The run has been authorised and is finding the
		// window, bringing up what it sees with, and taking its first look; none of
		// that is a question and none of it is a press.
		return preparing(application)
	case runstate.Acting:
		return join(stepOf(at), nameOf(at), "")
	case runstate.Verifying:
		if at != nil && at.Verdict != "" {
			// THE ANSWER HAS ARRIVED. Said beside the step rather than
			// instead of it, so the sentence does not jump about between the
			// press and its verdict.
			return join(stepOf(at), nameOf(at), at.Verdict)
		}
		return fmt.Sprintf("Checking that %s worked…", quoted(nameOf(at), "it"))
	case runstate.Paused:
		// NOT PROCEEDING, AND NOT A FAILURE. The reason is the runner's own
		// sentence and says what the person can do; this says where it is and
		// that nothing is being asked of them.
		where := "Paused"
		if s := stepOf(at); s != "" {
			where = "Paused before " + lower(s)
		}
		if st.Reason != "" {
			return join(where, st.Reason)
		}
		return join(where, "waiting for the screen this Act starts on")
	case runstate.Naming, runstate.Agreeing, runstate.Asking:
		// THE ENGINEERING PATH, which a product run never reaches: every press
		// asked for and answered on the keyboard. Said plainly rather than
		// rendered as progress, because somebody is being waited on.
		return "Waiting for an answer on the keyboard"
	}
	return ""
}

// nextOf is what to do about how it ended, or "" when there is nothing to do.
//
// Keyed on the outcome AND on what this surface already knows about the Act, because the
// remedies are different things: an ambiguous target has to be shown again, a missing
// application has to be opened, and a starting screen has to be navigated to. A single
// "try again" over all of them would be the window guessing.
func nextOf(outcome runref.Outcome, waits string) string {
	switch outcome {
	case "":
		return ""
	case runref.Verified:
		return ""
	case runref.Contradicted:
		return "The press went out, so something on your machine moved. If the " +
			"control has moved or been renamed since you showed it, demonstrate " +
			"it again."
	case runref.Unconfirmed:
		return "The press went out and nothing could be read afterwards. " +
			"Demonstrate it again, resting after the click so the result is " +
			"read, and nothing after this interaction was performed."
	case runref.Refused:
		if waits != "" {
			// THE REASON THIS SURFACE ALREADY KNEW. An Act whose starting
			// screen was never read refuses every time, for the same reason,
			// and the remedy is in the demonstration.
			return waits
		}
		return "Nothing was pressed. The sentence above is the run's own reason; " +
			"a run that was waiting for a screen or a window needs that thing " +
			"to be true and then another press of Run."
	case runref.Cancelled:
		return "Nothing was sent after you pressed Stop."
	case runref.Failed:
		return "Technical details has what it said as it stopped."
	case runref.Started:
		return "The run went away without saying how it ended, so nobody can " +
			"speak for what happened. Technical details has the transcript."
	}
	return ""
}

func preparing(application string) string {
	if application == "" {
		return "Preparing…"
	}
	return "Preparing " + application + "…"
}

// stepOf is "Step 1 of 2", or "" when the run could not say which.
func stepOf(at *runstate.Interaction) string {
	if at == nil || at.Index == 0 || at.Of == 0 {
		return ""
	}
	return fmt.Sprintf("Step %d of %d", at.Index, at.Of)
}

// join puts the parts there are beside each other, and nothing where there is nothing.
//
// Every part of this sentence can be missing: a run walking an artifact has no names, a run
// that cannot say which interaction it is on has no numbering, and a phase that speaks for
// itself has no reason. Concatenating them unconditionally produces the separators with the
// words left out, which reads as something broken rather than as something unknown.
func join(parts ...string) string {
	out := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		if out != "" {
			out += " · "
		}
		out += p
	}
	return out
}

// nameOf is what the PERSON called this interaction, never what the machine called it.
//
// `control_2` is available here and is deliberately not used: a window that showed it would be
// showing somebody the machine's word for a thing they had given a word of their own. Empty when
// the run is not performing a named Act, and the caller says what to put there.
func nameOf(at *runstate.Interaction) string {
	if at == nil {
		return ""
	}
	return at.Name
}

func quoted(s, orElse string) string {
	if s == "" {
		return orElse
	}
	return fmt.Sprintf("%q", s)
}

func lower(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]|0x20) + s[1:]
}
