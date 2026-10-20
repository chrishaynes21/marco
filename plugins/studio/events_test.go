package main

// WHAT STUDIO SHOWS COMES FROM THE RUNNER, NOT FROM ITS WORDING.
//
// Studio used to read the prose: it matched on sentences like "May Marco" and "Is that the screen
// you are on?" to work out what was happening. Two defects in one. A window whose behaviour
// depends on the exact wording of a consent prompt breaks when somebody improves the wording —
// silently, and in the direction of showing a person the wrong thing about a press they are about
// to authorise. And prose is not authoritative: the sentence is printed NEAR the decision, not BY
// it, so a reader of prose is always inferring a state something else already knew.
//
// These drive `Watch` over event streams written by hand, which is the only way to be certain
// what the surface is reading. The emission itself — that the runner really says these things at
// these moments — is `cmd/director`'s to prove.

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
	"github.com/chaynes-simpleclouds/marco/internal/director/runstate"
)

// aStream writes an event stream and points a surface at it as though a run had written it.
func aStream(t *testing.T, events ...runstate.Event) *Acts {
	t.Helper()
	a, _ := aStoreWithADemonstration(t)
	path := filepath.Join(t.TempDir(), "run.jsonl")
	w, err := runstate.NewWriter(path, "run-20261019-000000-aaaa", "an act",
		"ark graphics live", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range events {
		w.Say(e)
	}
	w.Close()
	a.mu.Lock()
	a.saying = path
	a.mu.Unlock()
	return a
}

// CHANGING THE WORDS CHANGES NOTHING THE WINDOW DOES.
//
// The same question asked in two completely different sentences. What a person READS differs, and
// everything the window decides from — the phase, that a question is pending, the keys it accepts,
// when it closes — is identical, because none of it was ever read out of the sentence.
func TestRewordingAQuestionChangesNothingStudioDoes(t *testing.T) {
	ask := func(words string) Running {
		a := aStream(t, runstate.Event{
			Phase: runstate.Asking,
			Interaction: &runstate.Interaction{Index: 0, Of: 1,
				Name: "the audio tab", Step: 1},
			Question: &runstate.Question{ID: "press-1-0", Asks: words,
				Keys:  []runstate.Key{{Key: "F7", Means: "yes"}},
				Until: time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)},
		})
		return a.Watch()
	}
	plain := ask("May Marco SEND a left click on that, once?")
	// A COMPLETE REWRITE, in another register and another language's worth of words.
	reworded := ask("Shall I click it? (this really moves the mouse)")

	for _, c := range []struct {
		what string
		a, b any
	}{
		{"phase", plain.State.Phase, reworded.State.Phase},
		{"a question is pending", plain.State.Question != nil,
			reworded.State.Question != nil},
		{"the question's identity", plain.State.Question.ID,
			reworded.State.Question.ID},
		{"the keys it accepts", plain.State.Question.Keys[0].Key,
			reworded.State.Question.Keys[0].Key},
		{"when it closes", plain.State.Question.Until,
			reworded.State.Question.Until},
		{"which interaction", plain.State.Interaction.Name,
			reworded.State.Interaction.Name},
		{"whether it ended", plain.Complete, reworded.Complete},
	} {
		if c.a != c.b {
			t.Errorf("%s differs with the wording: %v vs %v", c.what, c.a, c.b)
		}
	}
	// AND THE WORDS THEMSELVES ARE CARRIED THROUGH VERBATIM, because the window shows
	// the question a person is answering rather than one it composed itself.
	if plain.State.Question.Asks == reworded.State.Question.Asks {
		t.Fatal("the two wordings arrived identical, so this test compares nothing")
	}
}

// A QUESTION THAT HAS BEEN ANSWERED IS GONE.
//
// Snapshots make this true by construction: the event after an answer simply has no question in
// it. A stale prompt is the likeliest thing for a person to answer twice, and the second F7 lands
// on whatever is asked next.
func TestAnAnsweredQuestionDisappearsFromTheState(t *testing.T) {
	a := aStream(t,
		runstate.Event{Phase: runstate.Asking,
			Question: &runstate.Question{ID: "press-1-0", Asks: "May Marco?"}},
		runstate.Event{Phase: runstate.Acting},
	)
	w := a.Watch()
	if w.State == nil {
		t.Fatal("no state was read")
	}
	if w.State.Question != nil {
		t.Fatalf("an answered question is still pending: %+v", w.State.Question)
	}
	if w.State.Phase != runstate.Acting {
		t.Errorf("the phase is %q after the answer", w.State.Phase)
	}
}

// THE PREPARATION DEADLINE IS THE RUNNER'S, AND STUDIO ONLY COUNTS TO IT.
//
// A window may compute and show the time remaining. It may not decide that preparation is over:
// the run ends its own preparation, and a front end acting on its own arithmetic would be
// deciding something it is not in a position to know.
func TestThePreparationDeadlineComesFromTheRunner(t *testing.T) {
	ends := time.Now().Add(45 * time.Second)
	a := aStream(t, runstate.Event{Phase: runstate.Preparing, Preparation: ends,
		Question: &runstate.Question{ID: "start", Asks: "ready?"}})
	w := a.Watch()
	if w.State.Preparation.IsZero() {
		t.Fatal("no preparation deadline was carried")
	}
	if got := w.State.Preparation.Sub(ends); got > time.Second || got < -time.Second {
		t.Errorf("the deadline moved by %s on the way through", got)
	}
	// AND IT IS STILL PREPARING. Nothing in the surface advances the phase; only an
	// event does.
	if w.State.Phase != runstate.Preparing {
		t.Errorf("the phase is %q", w.State.Phase)
	}
}

// A RUN THAT ENDED SAYS SO, AND ONE THAT VANISHED IS INCOMPLETE.
//
// Different facts, reported apart. A refusal is an answer; a child that disappeared without a
// final event is a run nobody can speak for, and showing that as a failure would be inventing a
// verdict.
func TestARunWithoutAFinalEventIsIncomplete(t *testing.T) {
	ended := aStream(t,
		runstate.Event{Phase: runstate.Acting},
		runstate.Event{Phase: runstate.Ended, Final: true,
			Outcome: runref.Refused, Reason: "nobody agreed to step 0"},
	)
	w := ended.Watch()
	if !w.Complete || w.Incomplete {
		t.Fatalf("a run that ended reads complete=%v incomplete=%v", w.Complete,
			w.Incomplete)
	}
	if w.Outcome != string(runref.Refused) || w.Reason == "" {
		t.Errorf("the outcome reads %q because %q", w.Outcome, w.Reason)
	}

	vanished := aStream(t, runstate.Event{Phase: runstate.Acting})
	v := vanished.Watch()
	if v.Complete || !v.Incomplete {
		t.Fatalf("a run that vanished reads complete=%v incomplete=%v", v.Complete,
			v.Incomplete)
	}
	if v.Outcome != "" {
		t.Errorf("a run nobody can speak for was given the outcome %q", v.Outcome)
	}
}

// AND THE EVENT OUTRANKS THE SURFACE'S OWN BOOKKEEPING.
//
// Both know the Act and the mode. The run's own account was written by the thing that decided,
// and a disagreement means the surface is behind — so the event wins.
func TestTheRunsOwnAccountOutranksTheSurfacesBookkeeping(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	path := filepath.Join(t.TempDir(), "run.jsonl")
	w, err := runstate.NewWriter(path, "run-20261019-000000-bbbb", "what it really is",
		"ark graphics live", true)
	if err != nil {
		t.Fatal(err)
	}
	w.Say(runstate.Event{Phase: runstate.Acting})
	w.Close()
	// THE SURFACE BELIEVES SOMETHING ELSE, as it would in the window between starting a
	// run and the run's first word.
	a.mu.Lock()
	a.act, a.live, a.reference, a.saying = "what the page thought", false, "r_old", path
	a.mu.Unlock()

	got := a.Watch()
	if got.Act != "what it really is" {
		t.Errorf("the Act reads %q", got.Act)
	}
	if !got.Live {
		t.Error("a live run is reported as dry")
	}
	if got.Reference != "run-20261019-000000-bbbb" {
		t.Errorf("the reference reads %q", got.Reference)
	}
}

// AND BEFORE THE RUN HAS SAID ANYTHING, THE SURFACE'S OWN ANSWER STANDS.
//
// The window between starting a child and its first event is real, and a surface that showed
// nothing at all there would look broken for the moment that matters most.
func TestBeforeTheFirstEventTheSurfaceStillSaysWhatItStarted(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	path := filepath.Join(t.TempDir(), "empty.jsonl")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	a.act, a.live, a.busy, a.saying = "just started", true, true, path
	a.mu.Unlock()
	w := a.Watch()
	if w.Act != "just started" || !w.Live || !w.Active {
		t.Fatalf("the surface forgot what it started: %+v", w)
	}
	if w.State != nil {
		t.Error("it read a state out of an empty stream")
	}
	// AND IT IS NOT INCOMPLETE YET, because the run has not gone anywhere.
	if w.Incomplete {
		t.Error("a run still in flight was called incomplete")
	}
}
