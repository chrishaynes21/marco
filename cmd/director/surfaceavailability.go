package main

// How much of a watch a press could actually have been answered in.
//
// # The run this exists for
//
// Experiment-067 produced two watches of the same length on the same page. In one the person
// clicked and the whole loop closed; in the other no press arrived at all. The reports were
// indistinguishable apart from the counters — an empty handoff and three false claims — and the
// difference between "nobody clicked" and "somebody clicked into a gap where no question could
// receive it" is the difference between a person's choice and Marco's defect.
//
// The gaps were real and were growing: a clarification cannot accept a press between bounded
// observation sessions, because it is being derived again from a fresh reading, and the
// supervisor's attention curve stretched those gaps to 4s, 8s and 16s as the page sat still.
// Twenty-eight of eighty-five seconds, concentrated at the end.
//
// So this measures the thing the operator was guessing at. It decides nothing, holds nothing open
// and changes no refusal: it watches the same `Open()` the rest of the phase watches and reports
// what it saw.

import (
	"fmt"
	"time"
)

// availability is time a press could have been received, against time it could not.
//
// Sampled rather than event-driven on purpose. The events exist — a rollover and an establishment
// are both recorded — but an account built from them would be an account of what the learner said
// happened, and the question here is what a person at the keyboard would have experienced. A tick
// that asks "could a press land right now" answers that directly, and the tick is one the phase
// was already taking.
type availability struct {
	// WasOpen is what the last tick saw, so a change can be reported as a change.
	WasOpen bool
	// Open and Shut are how long each state has held in total.
	Open time.Duration
	Shut time.Duration
	// LongestShut is the worst single gap, which is the number a person feels. A total of
	// twenty-eight seconds in three gaps and the same total in twenty-eight is not the
	// same experience, and the second is survivable.
	LongestShut time.Duration
	// thisShut is the gap in progress.
	thisShut time.Duration
	// Gaps is how many times the question stopped accepting presses.
	Gaps int
}

// saw records one tick and returns the line to print, empty while nothing changed.
//
// `since` is the tick's own length rather than a clock reading, so the arithmetic is the caller's
// and a test can drive it without waiting.
func (a *availability) saw(open bool, since time.Duration) string {
	if open {
		a.Open += since
	} else {
		a.Shut += since
		a.thisShut += since
		if a.thisShut > a.LongestShut {
			a.LongestShut = a.thisShut
		}
	}
	if open == a.WasOpen {
		return ""
	}
	a.WasOpen = open
	if open {
		a.thisShut = 0
		// THE ONLY PLACE A PERSON IS TOLD THEY MAY CLICK AGAIN. The geometry is in
		// the `established` line printed beside this one; this is the permission.
		return "  → the question is OPEN again: a press now will be received\n"
	}
	a.Gaps++
	return "  → the question is NOT accepting presses just now — its observation session " +
		"ended and it is being derived again from a fresh reading. Wait for the next " +
		"line before clicking.\n"
}

// lines is the account in words.
func (a availability) lines() string {
	total := a.Open + a.Shut
	if total <= 0 {
		return ""
	}
	out := "\nCOULD A PRESS HAVE BEEN RECEIVED, over the whole watch:\n"
	out += fmt.Sprintf("  the question was open .... %s of %s (%d%%)\n",
		a.Open.Round(time.Second), total.Round(time.Second),
		int(100*a.Open/total))
	out += fmt.Sprintf("  in %d gap(s) it was not ... %s, the longest %s\n",
		a.Gaps, a.Shut.Round(time.Second), a.LongestShut.Round(time.Second))
	// AND WHAT THAT MEANS FOR READING THE HANDOFF BELOW, because the two are only
	// honest together.
	switch {
	case a.Gaps == 0:
		out += "  READ THIS AS: the question was open for the whole watch, so an " +
			"absent press is an absent press and not a missed window.\n"
	case a.LongestShut >= 5*time.Second:
		out += fmt.Sprintf("  READ THIS AS: there was a %s stretch in which no press "+
			"could have been received. An absent press is not evidence that "+
			"nobody pressed.\n", a.LongestShut.Round(time.Second))
	default:
		out += "  READ THIS AS: the gaps were short enough that a press is unlikely " +
			"to have fallen in one, though it is not impossible.\n"
	}
	return out
}
