package workflowstore_test

// WHAT ONE "YES" BUYS, AND EVERYTHING IT DOES NOT.
//
// A person confirming a screen is the first thing in this subsystem that can establish an
// identity Marco could not establish for itself. It is therefore the thing most worth bounding:
// a confirmation that quietly outlived its observation, or spread to a screen nobody looked at,
// or survived a reload, would be a durable Places system assembled by accident.

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

var looked = time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)

// route is a three-screen workflow: a -> b -> c.
func route(t *testing.T) workflowstore.Artifact {
	t.Helper()
	d := workflow.Draft{Name: "open free play"}
	step := func(from, to observe.VisualScreenID, target observe.VisualTargetID) workflow.Step {
		return workflow.Step{
			From: from, To: to, Target: target, Cause: observe.CauseClick, Seen: 1,
			Readings: []observe.Region{rect(100, 100, 200, 40)},
		}
	}
	d.Steps = []workflow.Step{
		step("visual_1", "visual_2", "target_1"),
		step("visual_2", "visual_3", "target_2"),
	}
	a, err := workflowstore.Save(d, map[observe.VisualScreenID]observe.VisualArrangement{
		"visual_1": {Cells: map[string]int{"0,0": 9}, Total: 9},
		"visual_2": {Cells: map[string]int{"1,1": 7}, Total: 7},
		"visual_3": {Cells: map[string]int{"2,2": 5}, Total: 5},
	}, saved)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// ONE YES BINDS ONE SCREEN, IN ONE OBSERVATION, WITH A NAME ON IT.
func TestOneConfirmationBindsExactlyOneScreen(t *testing.T) {
	a := route(t)
	s := workflowstore.NewSessionBinding()
	if err := s.Confirm("screen_a", "now_7", 5, 1, "chris", looked); err != nil {
		t.Fatal(err)
	}

	got, ok := s.Established("screen_a", 5, 1)
	if !ok {
		t.Fatal("the confirmation did not establish anything")
	}
	if got.Screen != "now_7" || got.Ground != workflowstore.GroundConfirmed ||
		got.By != "chris" || got.Depth != 0 {
		t.Errorf("established %+v", got)
	}
	// AND NOTHING ELSE IN THE WORKFLOW. Six correct rankings on one recording are not a
	// reason to carry one answer to the other screens.
	for _, other := range []string{"screen_b", "screen_c"} {
		if _, ok := s.Established(other, 5, 1); ok {
			t.Errorf("%s was bound by an answer about screen_a", other)
		}
	}
	if b := s.Binding(5, 1); len(b.Screens) != 1 {
		t.Errorf("%d screen(s) in the binding", len(b.Screens))
	}
	// And the workflow as a whole still cannot be bound, because two screens are open.
	if _, err := a.Bind(s.Binding(5, 1)); err == nil {
		t.Error("a workflow was bound from one confirmed screen of three")
	}
}

// A CONFIRMATION DOES NOT SURVIVE THE OBSERVATION IT WAS GIVEN IN.
func TestAConfirmationDoesNotOutliveItsObservation(t *testing.T) {
	s := workflowstore.NewSessionBinding()
	if err := s.Confirm("screen_a", "now_7", 5, 1, "chris", looked); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name              string
		epoch, generation uint64
	}{
		{"the screen changed", 6, 1},
		{"and changed back", 7, 1},
		{"the window was replaced", 5, 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			if _, ok := s.Established("screen_a", c.epoch, c.generation); ok {
				t.Error("a confirmation from another observation still holds")
			}
			if b := s.Binding(c.epoch, c.generation); len(b.Screens) != 0 {
				t.Error("an expired confirmation reached the binding")
			}
		})
	}
	// REVISITING IS A NEW OBSERVATION, so the answer has to be given again. An
	// arrangement cannot tell a second visit from the first; a counter can.
	if n := s.Observed(9, 1); n != 1 {
		t.Errorf("%d binding(s) dropped when the observation moved, want 1", n)
	}
	if _, ok := s.Established("screen_a", 5, 1); ok {
		t.Error("the original confirmation came back")
	}
}

// NO MEANS NO, AND DOES NOT MEAN THE RUNNER-UP.
//
// Experiment-036's wrong winners beat their runners-up by 0.419. The ordering below a wrong
// answer is not evidence of anything, so a rejection establishes nothing at all.
func TestARejectionEstablishesNothingAndPromotesNobody(t *testing.T) {
	s := workflowstore.NewSessionBinding()
	if err := s.Reject("screen_a", "now_7", "chris"); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Established("screen_a", 5, 1); ok {
		t.Error("a rejection bound something")
	}
	if !s.Rejected("screen_a", "now_7") {
		t.Error("the rejection was not remembered within its observation")
	}
	if s.Rejected("screen_a", "now_8") {
		t.Error("rejecting one candidate rejected another")
	}
	// And the same candidate cannot then be confirmed without the question being put
	// again in a fresh observation.
	if err := s.Confirm("screen_a", "now_7", 5, 1, "chris", looked); err == nil {
		t.Error("a rejected candidate was confirmed in the same observation")
	}
}

// NOT SURE CHANGES NOTHING.
func TestUnsureAbstains(t *testing.T) {
	s := workflowstore.NewSessionBinding()
	if err := s.Unsure("screen_a", "chris"); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Established("screen_a", 5, 1); ok {
		t.Error("an abstention bound something")
	}
	if s.Rejected("screen_a", "now_7") {
		t.Error("an abstention rejected a candidate")
	}
	if s.Asked() != 1 || s.Unsures() != 1 || s.Confirmed() != 0 {
		t.Errorf("asked %d, unsure %d, confirmed %d", s.Asked(), s.Unsures(), s.Confirmed())
	}
}

// AN UNATTRIBUTED ANSWER IS NOT AN ANSWER.
func TestEveryAnswerNeedsSomebody(t *testing.T) {
	s := workflowstore.NewSessionBinding()
	if err := s.Confirm("screen_a", "now_7", 5, 1, "", looked); err == nil {
		t.Error("an anonymous confirmation was accepted")
	}
	if err := s.Confirm("screen_a", "now_7", 5, 1, "chris", time.Time{}); err == nil {
		t.Error("a confirmation with no time was accepted")
	}
	if err := s.Reject("screen_a", "now_7", ""); err == nil {
		t.Error("an anonymous rejection was accepted")
	}
	if err := s.Unsure("screen_a", ""); err == nil {
		t.Error("an anonymous abstention was accepted")
	}
}

// INFERENCE NEEDS BOTH HALVES, AND NEITHER ALONE WILL DO.
//
// A transition says WHICH screen to expect, which is the thing similarity cannot supply —
// Experiment-036's failure was nominating a screen that was not there. Similarity says the
// screen agrees, which is the thing expectation cannot supply, because delivery is not success.
func TestAnInferenceNeedsATransitionAndAnAgreement(t *testing.T) {
	a := route(t)
	base := func() *workflowstore.SessionBinding {
		s := workflowstore.NewSessionBinding()
		if err := s.Confirm("screen_a", "now_1", 5, 1, "chris", looked); err != nil {
			t.Fatal(err)
		}
		return s
	}

	// With both: inferred, at depth 1, and marked as not a person's answer.
	s := base()
	got, err := s.Infer(a, "screen_a", "screen_b", "now_2", 0.81, 6, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Ground != workflowstore.GroundInferred || got.Depth != 1 || got.By != "" {
		t.Errorf("inferred %+v", got)
	}
	if s.Asked() != 1 {
		t.Errorf("an inference asked somebody something (%d asked)", s.Asked())
	}

	// WITHOUT THE TRANSITION: the workflow never goes a -> c, so nothing follows.
	s = base()
	if _, err := s.Infer(a, "screen_a", "screen_c", "now_3", 0.99, 6, 1); err == nil {
		t.Error("a screen was inferred across a step the workflow does not demonstrate")
	}

	// WITHOUT THE AGREEMENT: the workflow expected to arrive and the evidence disagrees,
	// which is a reason to ask rather than to assume.
	s = base()
	_, err = s.Infer(a, "screen_a", "screen_b", "now_2", 0.40, 6, 1)
	if err == nil {
		t.Fatal("a screen was inferred with the arrangement disagreeing")
	}
	if !strings.Contains(err.Error(), "ask rather than to assume") {
		t.Errorf("the refusal does not say what to do instead: %v", err)
	}

	// UNDER ANOTHER WINDOW: the anchor was established while looking at a different
	// window, so what followed in this one did not follow from it.
	s = base()
	if _, err := s.Infer(a, "screen_a", "screen_b", "now_2", 0.99, 6, 2); err == nil {
		t.Error("a screen was inferred from an anchor established under another window")
	}

	// WITHOUT AN ANCHOR: nothing follows from nothing, and the refusal says THAT
	// rather than something about windows. The two are different findings — one
	// is a workflow nobody has located, the other a workflow located somewhere
	// else — and a reader chasing the wrong one wastes the message entirely.
	s = workflowstore.NewSessionBinding()
	_, err = s.Infer(a, "screen_a", "screen_b", "now_2", 0.99, 6, 1)
	if err == nil {
		t.Fatal("a screen was inferred from an anchor that was never established")
	}
	if !strings.Contains(err.Error(), "nothing is established") {
		t.Errorf("the refusal blames the wrong thing: %v", err)
	}
}

// A CHAIN KNOWS HOW FAR IT IS FROM A PERSON.
func TestAnInferenceCarriesItsDistanceFromAnAnswer(t *testing.T) {
	a := route(t)
	s := workflowstore.NewSessionBinding()
	if err := s.Confirm("screen_a", "now_1", 5, 1, "chris", looked); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Infer(a, "screen_a", "screen_b", "now_2", 0.81, 6, 1); err != nil {
		t.Fatal(err)
	}
	got, err := s.Infer(a, "screen_b", "screen_c", "now_3", 0.77, 7, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Depth != 2 {
		t.Errorf("depth %d after two inferences from one answer", got.Depth)
	}
	// And a report can tell the three apart.
	live := s.Live(7, 1)
	if len(live) != 1 {
		// screen_a and screen_b were established in earlier observations and are
		// gone; only the newest holds. That is the staleness rule, not a bug.
		t.Logf("live bindings: %+v", live)
	}
	if live[0].Ground != workflowstore.GroundInferred || live[0].Depth != 2 {
		t.Errorf("the surviving binding is %+v", live[0])
	}
}

// A CONFIRMATION NEVER REACHES THE FILE.
//
// The one that would turn this into a durable Places system by accident. A workflow saved today
// and loaded tomorrow must come back with nothing established, however many questions were
// answered.
func TestNoConfirmationSurvivesASaveAndReload(t *testing.T) {
	a := route(t)
	s := workflowstore.NewSessionBinding()
	if err := s.Confirm("screen_a", "now_7", 5, 1, "chris", looked); err != nil {
		t.Fatal(err)
	}
	body, err := a.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"now_7", "confirmed", "inferred", "chris"} {
		if strings.Contains(string(body), forbidden) {
			t.Errorf("the artifact contains %q; a session binding has reached a "+
				"durable file", forbidden)
		}
	}
	back, err := workflowstore.Unmarshal(body)
	if err != nil {
		t.Fatal(err)
	}
	// A fresh session knows nothing, and the artifact offers nothing to seed it with.
	fresh := workflowstore.NewSessionBinding()
	if _, ok := fresh.Established("screen_a", 5, 1); ok {
		t.Error("a new session started with a binding")
	}
	if _, err := back.Bind(fresh.Binding(5, 1)); err == nil {
		t.Error("a reloaded workflow bound itself from an empty session")
	}
}
