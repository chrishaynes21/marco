package main

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/platform/screenhost"
)

// The Director's answer to "what place is showing right now", asked from outside this process.
//
// # Why anybody asks
//
// Every learned play's generated Marco opens with `do Screen's Showing with "<place>"`. The
// process that RUNS a play is `marco`, and `marco` cannot see. A learned play with intact
// provenance is delegated here and never meets that problem; edit it and it is an ordinary play,
// which takes the local runner and used to refuse at its own first line. This verb is how the
// local runner gets an answer, and every test below is about the answer being an answer rather
// than an assumption.

// ── the wiring: the verb reaches the fresh look ───────────────────────────────

// A SHOWING query is answered, through the ordinary observation door.
//
// Entered through `Runtime.Observation` — the production request handler — rather than by calling
// `ShowingNow` directly, because the thing that can rot is the ROUTING: a verb nothing dispatches
// is code that exists and is never invoked, which this repository has three recorded cases of.
//
// The place is established from the live session's own settled signature, so recognition here is
// the real resolver agreeing with real evidence rather than a fixture agreeing with itself.
//
// Deleting the `case q.Showing != nil` arm in Observation must fail this.
func TestTheDirectorAnswersWhichPlaceIsShowing(t *testing.T) {
	g, store := watchedRegistry(t)
	here := watchNow(t, g, store, "settings")
	rt := &Runtime{observations: g}

	answer, err := rt.Observation(service.ObserveQuery{
		Showing: &service.ObserveShowing{Application: "settings"},
	})
	if err != nil {
		t.Fatalf("asking which place is showing: %v", err)
	}
	view, ok := answer.(service.ShowingView)
	if !ok {
		t.Fatalf("a SHOWING query was answered with a %T. Something else on this query "+
			"claimed it, which is how a guard comes to be satisfied by a session listing.",
			answer)
	}
	if view.Outcome != string(screenhost.Recognised) {
		t.Fatalf("outcome %q (%s), want %q — a live session is watching settings and it "+
			"resolves to %q, so the look CAN be taken",
			view.Outcome, view.Why, screenhost.Recognised, here)
	}
	if view.Subject != here {
		t.Fatalf("the place showing is reported as %q, want %q", view.Subject, here)
	}
	if view.Application != "settings" {
		t.Errorf("the answer says it is about %q; a caller cannot tell an answer to its own "+
			"question from an answer to somebody else's", view.Application)
	}
}

// ── the refusals, which are the half that may never be weakened ───────────────

// A LOOK, NOT A LOOKUP.
//
// `placeNowSubject` answers from the newest FINISHED session when none is running — the right rule
// for "what is Marco talking about", the wrong one for "where is the Audience standing". Answering
// a play's entry condition from it would let a guard pass on a screen the person walked away from,
// which is worse than having no guard: the play would begin, confidently, in the wrong place.
//
// The premise is asserted first — the retired session really can resolve to a durable place — so
// this fails for the reason it claims rather than because nothing resolved today.
//
// Mutation: replace the `freshPlace` call in ShowingNow with `r.observations.placeNowSubject()`.
// That is history answering a question about now, and it must fail here.
func TestShowingNowTakesAFreshLookRatherThanReadingHistory(t *testing.T) {
	rt, _, a, _ := namingRuntime(t)
	standingOn(rt, observe.TermAudio)

	if got := rt.observations.placeNowSubject(); got != a {
		t.Fatalf("the retired session resolves to %q, want %q — the premise of this test is "+
			"that history CAN answer, so that refusing it means something", got, a)
	}

	view, err := rt.ShowingNow(context.Background(), service.ObserveShowing{Application: "settings"})
	if err != nil {
		t.Fatalf("asking: %v", err)
	}
	if view.Subject == a {
		t.Fatalf("a play's guard would be satisfied by %q, which is where a FINISHED session "+
			"left off. Nobody looked; the person may have walked away an hour ago.", a)
	}
	if view.Outcome == string(screenhost.Recognised) {
		t.Fatalf("outcome is %q with subject %q — a recognition nobody looked for",
			view.Outcome, view.Subject)
	}
}

// NO APPLICATION IS A REFUSAL, NOT "WHATEVER IS IN FRONT".
//
// A play's entry condition is about the application the play is in. Answering it about whichever
// window happens to be foremost would make the guard satisfiable by the wrong program — a screen
// guard that can be passed by accident is worse than none, because it reads as protection.
//
// Deleting the empty-application arm must fail this.
func TestShowingNowRefusesWhenNoApplicationWasNamed(t *testing.T) {
	g, store := watchedRegistry(t)
	here := watchNow(t, g, store, "settings")
	rt := &Runtime{observations: g}

	view, err := rt.ShowingNow(context.Background(), service.ObserveShowing{})
	if err != nil {
		t.Fatalf("asking: %v", err)
	}
	if view.Subject != "" {
		t.Fatalf("an unscoped question was answered with %q (settings is standing on %q). "+
			"Nobody said which application, so nobody asked about that one.", view.Subject, here)
	}
	if view.Outcome != string(screenhost.Unavailable) {
		t.Errorf("outcome %q, want %q — no application named is Marco being unable to check, "+
			"not Marco looking and not recognising anything", view.Outcome, screenhost.Unavailable)
	}
}

// "I COULD NOT LOOK" IS NOT "I LOOKED AND IT WAS DIFFERENT".
//
// Somebody demonstrating in another program means the look cannot be taken at all: one observation
// session runs at a time, and cancelling theirs to answer a question is not this request's to do.
// The two must stay apart, because they call for opposite fixes — one is "finish what you were
// doing", the other is "teach Marco this screen".
//
// Reverting Unobservable to Unknown here must fail this.
func TestShowingNowSaysItCouldNotLookRatherThanThatItDidNotRecognise(t *testing.T) {
	g, store := watchedRegistry(t)
	elsewhere := watchNow(t, g, store, "testgame")
	rt := &Runtime{observations: g}

	view, err := rt.ShowingNow(context.Background(), service.ObserveShowing{Application: "settings"})
	if err != nil {
		t.Fatalf("asking: %v", err)
	}
	if view.Subject == elsewhere {
		t.Fatalf("a play in settings was told it is standing on %q, which is where somebody "+
			"is demonstrating in testgame. Live evidence, wrong window.", elsewhere)
	}
	if view.Outcome != string(screenhost.Unobservable) {
		t.Fatalf("outcome %q (%s), want %q", view.Outcome, view.Why, screenhost.Unobservable)
	}
	if !strings.Contains(view.Why, "testgame") {
		t.Errorf("the reason reads %q, which does not say what is actually in the way", view.Why)
	}
	if strings.HasPrefix(strings.TrimSpace(view.Why), ":") {
		t.Errorf("the reason reads %q — freshPlace's leading separator leaked into a field "+
			"that is a sentence, not a fragment", view.Why)
	}
}
