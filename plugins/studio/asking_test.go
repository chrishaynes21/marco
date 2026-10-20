package main

// A QUESTION OUTLIVES ITS RUNNER AS EVIDENCE, NEVER AS AN INVITATION.
//
// # The defect
//
// Snapshots make a prompt disappear when it is ANSWERED: the event after an answer carries no
// question, so a window rendering the latest state cannot leave a stale one up. They say nothing
// about the other way a question stops being answerable — the process that asked it going away.
//
// A child killed, crashed, or stopped from outside leaves a stream whose last event still carries
// an open question. The surface correctly reported the run as incomplete and then handed the page
// that question anyway, with the keys it accepts and a countdown, and the page drew it. Somebody
// pressing F7 at that is pressing it at nothing — in a window that has just told them a press is
// waiting on their answer.
//
// So there are two fields and they make two different claims. `State.Question` is what the run
// was waiting for when it last said anything, which is part of the account of it and is kept.
// `Asking` is an invitation to press a key, and it is only honest while there is something on the
// other end of the hook to hear it.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
	"github.com/chaynes-simpleclouds/marco/internal/director/runstate"
)

// anOpenQuestion is a press authorisation, pending.
func anOpenQuestion() runstate.Event {
	return runstate.Event{
		Phase: runstate.Asking,
		Interaction: &runstate.Interaction{Index: 1, Of: 2,
			Name: "the graphics tab", Step: 0, Control: "control_1"},
		Question: &runstate.Question{ID: "press-0-1",
			Asks:  "May Marco SEND a left click on that, once?",
			Keys:  []runstate.Key{{Key: "F7", Means: "yes"}},
			Until: time.Now().Add(20 * time.Second)},
	}
}

// WHILE THE RUN IS IN FLIGHT THE QUESTION IS OFFERED, and when the child is gone it is not.
//
// The same stream both times. Nothing about the question changed; what changed is whether there
// is a process listening for the key it names.
func TestAQuestionIsOfferedOnlyWhileSomethingCanHearTheAnswer(t *testing.T) {
	inFlight := aStream(t, anOpenQuestion())
	inFlight.mu.Lock()
	inFlight.busy = true
	inFlight.mu.Unlock()
	w := inFlight.Watch()
	if w.Asking == nil {
		t.Fatal("a run in flight is not offering the question it is waiting on")
	}
	if w.Asking.ID != "press-0-1" || len(w.Asking.Keys) != 1 {
		t.Errorf("the offered question is %+v", w.Asking)
	}

	// ── AND THE CHILD IS GONE, with the same last snapshot.
	gone := aStream(t, anOpenQuestion())
	g := gone.Watch()
	if !g.Incomplete {
		t.Fatalf("a child that exited without a final event reads incomplete=%v",
			g.Incomplete)
	}
	if g.Asking != nil {
		t.Errorf("a question is still offered after the runner exited: %+v",
			g.Asking)
	}
	// AND IT IS STILL THERE AS EVIDENCE. What a run was waiting for when it stopped is
	// part of the account of it, and dropping it would leave a person with "INCOMPLETE"
	// and nothing about where it got to.
	if g.State == nil || g.State.Question == nil {
		t.Fatal("the question the run stopped on was lost")
	}
	if g.State.Question.ID != "press-0-1" {
		t.Errorf("the kept question is %+v", g.State.Question)
	}
}

// AND A RUN THAT ENDED IS NOT WAITING FOR ANYBODY EITHER.
//
// A final event carries no question in production, so this is the predicate being held rather
// than a case that arises: whatever the last snapshot holds, a run with an answer is not asking.
func TestARunThatEndedOffersNothingToAnswer(t *testing.T) {
	ended := aStream(t,
		anOpenQuestion(),
		runstate.Event{Phase: runstate.Ended, Final: true, Outcome: runref.Refused,
			Reason: "nobody answered within 20s",
			Question: &runstate.Question{ID: "press-0-1",
				Asks: "May Marco SEND a left click on that, once?"}},
	)
	ended.mu.Lock()
	ended.busy = true
	ended.mu.Unlock()
	w := ended.Watch()
	if !w.Complete {
		t.Fatalf("a run with a final event reads complete=%v", w.Complete)
	}
	if w.Asking != nil {
		t.Errorf("a finished run is offering %+v", w.Asking)
	}
}

// AND THE PAGE DRAWS THE OFFER FROM THE OFFER.
//
// A source guard, because the rendering is JavaScript and nothing here can run it. It is still
// worth holding: the whole correction above is a second field, and a page that went on reading
// `state.question` for the prompt would pass every test in this file while showing somebody a
// question nothing is listening for.
//
// The keys in particular. They are what invite a press.
func TestThePageOffersTheQuestionItWasGivenToOffer(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(raw)
	if !strings.Contains(page, "const q = w.asking") {
		t.Error("the page no longer takes the answerable question from `asking`, " +
			"so it can offer the keys of a question whose runner has exited")
	}
	// AND THE LAST QUESTION IS STILL SHOWN, as what it is.
	if !strings.Contains(page, "st.question") {
		t.Error("the page no longer shows the question a stopped run was waiting " +
			"on, so an incomplete run says nothing about where it got to")
	}
}
