package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// THE INSTRUMENT THAT ANSWERS "WHICH PLACE IS THIS", AND WHAT IT MUST NOT BECOME.
//
// 37G's whole claim rests on this command: every same-Place and different-Place result in
// Experiment 017 is a durable subject id printed by `director showing`. An instrument that
// answered from anywhere but the production identity path would make the acceptance a
// measurement of itself.

// It asks the Director. It does not work anything out.
//
// `ObserveShowing` takes one bounded passive look and resolves it with `observe.PlaceNow` against
// the real store, which is the one recogniser. A command that resolved a place itself — from a
// session's own account, from the store directly, from a signature it built — would be a second
// recogniser answering the question the first one exists for, and the two would disagree exactly
// when it mattered.
func TestShowingAsksTheDirectorRatherThanResolvingAPlaceItself(t *testing.T) {
	src := mustReadSource(t, "showingcmd.go")
	if !containsAll(src, "service.ObserveShowing{Application:", "observationRequest") {
		t.Error("showingcmd.go no longer sends the ObserveShowing query.\nThat query is the " +
			"one door onto the production identity path; anything else here would be a " +
			"second recogniser.")
	}
	// CODE, not prose. The comment at the top of that file names PlaceNow deliberately —
	// saying which recogniser answers is the point of it — and a check that could not tell a
	// mention from a call would make the file undocumentable.
	code := withoutComments(src)
	for _, forbidden := range []string{
		"observe.PlaceNow", "observe.CompareStructure", "observe.SignatureOfState",
		"semanticmemory.Open", "Recall(",
	} {
		if strings.Contains(code, forbidden) {
			t.Errorf("showingcmd.go reaches for %s. It is a client: it asks the running "+
				"Director and prints the reply.", forbidden)
		}
	}
	// And it is reachable, or the harness gets "unknown command" and a run that measured
	// nothing.
	if !strings.Contains(mustReadSource(t, "main.go"), `case "showing":`) {
		t.Error("`director showing` is not registered in main.go")
	}
}

// The outcome leads, and an id only ever appears beside a positive identification.
//
// `ShowingView` says it outright: an id beside any other outcome is a bug. A reader who took the
// subject line as the answer would record a match for a look that refused, which is the one way
// an acceptance harness can report a pass it did not get.
func TestShowingNeverPrintsAPlaceForARefusal(t *testing.T) {
	refusals := []service.ShowingView{
		{Application: "settings", Outcome: "unknown",
			Why: "nothing remembered in settings matches what is in front"},
		{Application: "settings", Outcome: "unreadable",
			Why: "accessibility described the window but not the content"},
		{Application: "settings", Outcome: "unobservable", Why: "there is no window to look at"},
		{Application: "settings", Outcome: "unavailable", Why: "no application was named"},
	}
	for _, v := range refusals {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		out := renderShowing(raw)
		if strings.Contains(out, "place  ") {
			t.Errorf("a %s outcome printed a place:\n%s", v.Outcome, out)
		}
		if !strings.HasPrefix(out, v.Outcome) {
			t.Errorf("the outcome does not lead:\n%s", out)
		}
		if v.Why != "" && !strings.Contains(out, v.Why) {
			t.Errorf("the reason was dropped from a %s outcome; a refusal with no account "+
				"is what sends somebody looking for the wrong problem", v.Outcome)
		}
	}

	recognised, err := json.Marshal(service.ShowingView{
		Application: "settings", Outcome: "recognised", Subject: "subj_71727a02470f"})
	if err != nil {
		t.Fatal(err)
	}
	if got := renderShowing(recognised); !strings.Contains(got, "subj_71727a02470f") {
		t.Errorf("a recognised place did not print its subject:\n%s", got)
	}
}

// A look with no application named is refused before anything is asked.
//
// The Director refuses it too, and says so — a screen guard satisfiable by the wrong program is
// worse than no guard. It is refused HERE as well because the two refusals answer different
// people: one is a protocol invariant, the other is somebody who forgot a flag, and being told
// "no application was named" by a service you did not know you were talking to is a worse
// version of the usage line.
func TestShowingRefusesALookWithNoApplication(t *testing.T) {
	// No Director is running in a unit test, so a version that sent the query would fail on
	// the connection — with 1, not 2, and after a socket attempt. The code is 2 because this
	// is a usage error.
	if got := runShowing(nil); got != 2 {
		t.Errorf("runShowing with no --application returned %d, want 2 (a usage refusal "+
			"taken before anything is asked of the Director)", got)
	}
}
