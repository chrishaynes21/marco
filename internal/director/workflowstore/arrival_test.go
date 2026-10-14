package workflowstore_test

// A DESTINATION ESTABLISHED IS NOT AN ACTION VERIFIED.
//
// This is the stage that is easiest to over-claim, because it sits where a person would expect
// the word "worked". Everything here is about keeping two sentences apart: the screen is now the
// one the step was going to, and the step's press is why.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// arrivedAt builds a session that has established `label` as `screen` by the given ground.
func arrivedAt(t *testing.T, a workflowstore.Artifact, label string,
	screen observe.VisualScreenID, by Ground) *workflowstore.SessionBinding {

	t.Helper()
	s := workflowstore.NewSessionBinding()
	switch by {
	case workflowstore.GroundConfirmed:
		if err := s.Confirm(label, screen, 9, 1, "SIMULATED", looked); err != nil {
			t.Fatal(err)
		}
	case workflowstore.GroundInferred:
		// An inference needs an anchor the workflow leaves for this label.
		if err := s.Confirm("screen_a", "now_1", 8, 1, "SIMULATED", looked); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Infer(a, "screen_a", label, screen, 0.9, 9, 1); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

// Ground is aliased so the helper reads as the package's own vocabulary.
type Ground = workflowstore.Ground

// A CONFIRMED ARRIVAL IS INDEPENDENT OF THE WORKFLOW.
func TestAConfirmedDestinationIsIndependentEvidence(t *testing.T) {
	a := threeScreens(t)
	s := arrivedAt(t, a, "screen_b", "now_2", workflowstore.GroundConfirmed)

	got := a.DestinationAt(0, s, workflowstore.Now{Screen: "now_2", Epoch: 9, Generation: 1})
	if !got.Established() {
		t.Fatalf("not established: %s", got.Because)
	}
	if !got.Independent() {
		t.Error("a person's confirmation is being treated as workflow-dependent")
	}
	if !strings.Contains(got.Because, "did not consult the workflow") {
		t.Errorf("the reason does not say why it is independent: %s", got.Because)
	}
}

// AN INFERRED ARRIVAL IS ESTABLISHED AND NOT INDEPENDENT.
//
// The circularity the owner named: the workflow said to expect this destination, so the workflow
// cannot then cite it as corroboration of its own step.
func TestAnInferredDestinationIsEstablishedButNotIndependent(t *testing.T) {
	a := threeScreens(t)
	s := arrivedAt(t, a, "screen_b", "now_2", workflowstore.GroundInferred)

	got := a.DestinationAt(0, s, workflowstore.Now{Screen: "now_2", Epoch: 9, Generation: 1})
	if !got.Established() {
		t.Fatalf("not established: %s", got.Because)
	}
	if got.Independent() {
		t.Fatal("an inference drawn from the workflow's own demonstrated transition is " +
			"being counted as independent evidence about that workflow")
	}
	if got.Ground != workflowstore.GroundInferred || got.Depth != 1 {
		t.Errorf("ground %q depth %d", got.Ground, got.Depth)
	}
	if !strings.Contains(got.Because, "same workflow") {
		t.Errorf("the reason does not name the circularity: %s", got.Because)
	}
}

// A DESTINATION NOBODY HAS GROUNDED IS NOT ESTABLISHED.
func TestAnUngroundedDestinationIsNotEstablished(t *testing.T) {
	a := threeScreens(t)
	s := workflowstore.NewSessionBinding()

	got := a.DestinationAt(0, s, workflowstore.Now{Screen: "now_2", Epoch: 9, Generation: 1})
	if got.Established() || got.Independent() {
		t.Fatalf("established=%t independent=%t", got.Established(), got.Independent())
	}
	if !strings.Contains(got.Because, "now_2") {
		t.Errorf("the reason does not say what is standing instead: %s", got.Because)
	}
}

// AN ESTABLISHMENT FROM AN EARLIER OBSERVATION IS NOT AN ARRIVAL.
//
// The destination has to be grounded in the observation AFTER the action boundary. A binding
// from before it would be the workflow checking its arrival against where it already was.
func TestADestinationMustBeEstablishedInTheNewObservation(t *testing.T) {
	a := threeScreens(t)
	s := arrivedAt(t, a, "screen_b", "now_2", workflowstore.GroundConfirmed)

	// The observation moved on; the earlier grounding does not reach forward.
	got := a.DestinationAt(0, s, workflowstore.Now{Screen: "now_2", Epoch: 10, Generation: 1})
	if got.Established() {
		t.Fatal("a destination established in an earlier observation counted as an arrival")
	}
	if !strings.Contains(got.Because, "not established in this observation") {
		t.Errorf("the reason does not say why: %s", got.Because)
	}
}

// AND THE SCREEN STANDING MUST BE THE ONE IT WAS ESTABLISHED AS.
func TestADestinationEstablishedAsAnotherScreenIsNotAnArrival(t *testing.T) {
	a := threeScreens(t)
	s := arrivedAt(t, a, "screen_b", "now_2", workflowstore.GroundConfirmed)

	got := a.DestinationAt(0, s, workflowstore.Now{Screen: "now_9", Epoch: 9, Generation: 1})
	if got.Established() {
		t.Fatal("a destination established as one screen counted as arrival at another")
	}
	if !strings.Contains(got.Because, "now_9") {
		t.Errorf("the reason does not name the screen standing: %s", got.Because)
	}
}

// A STEP WITH NO RECORDED DESTINATION HAS NOTHING TO ARRIVE AT.
func TestAStepWithNoDestinationCannotArrive(t *testing.T) {
	a := threeScreens(t)
	s := arrivedAt(t, a, "screen_b", "now_2", workflowstore.GroundConfirmed)
	if got := a.DestinationAt(99, s, workflowstore.Now{}); got.Established() {
		t.Error("a step outside the workflow arrived somewhere")
	}

	// AND A STEP THE DEMONSTRATION COULD NOT NAME A DESTINATION FOR. The walk went
	// somewhere no recorded change explains, so there is nothing an arrival could
	// be checked against — which is not the same as not having arrived.
	lost := workflow.Draft{Name: "went somewhere"}
	lost.Steps = []workflow.Step{{From: "visual_1", Cause: observe.CauseUnknown, Seen: 1}}
	la, err := workflowstore.Save(lost, nil, saved)
	if err != nil {
		t.Fatal(err)
	}
	ls := workflowstore.NewSessionBinding()
	if err := ls.Confirm("screen_a", "now_1", 9, 1, "SIMULATED", looked); err != nil {
		t.Fatal(err)
	}
	got := la.DestinationAt(0, ls, workflowstore.Now{Screen: "now_1", Epoch: 9, Generation: 1})
	if got.Established() {
		t.Fatal("a step with no recorded destination arrived at one")
	}
	if !strings.Contains(got.Because, "no recorded destination") {
		t.Errorf("the reason does not say why: %s", got.Because)
	}
}

// AND THE SENTENCE THIS WHOLE FILE EXISTS TO REFUSE.
//
// However the destination was established, the action is not verified, and the reason travels
// with the answer so a report cannot print the first and imply the second.
func TestAnActionIsNeverVerified(t *testing.T) {
	a := threeScreens(t)
	for _, by := range []workflowstore.Ground{
		workflowstore.GroundConfirmed, workflowstore.GroundInferred,
	} {
		s := arrivedAt(t, a, "screen_b", "now_2", by)
		got := a.DestinationAt(0, s, workflowstore.Now{
			Screen: "now_2", Epoch: 9, Generation: 1})
		if !got.Established() {
			t.Fatalf("%s: the fixture did not establish the destination", by)
		}
		ok, why := got.Verified()
		if ok {
			t.Errorf("%s: an action was reported as independently verified", by)
		}
		if !strings.Contains(why, "nothing was executed") ||
			!strings.Contains(why, "not by itself proof") {
			t.Errorf("%s: the reason is thin: %q", by, why)
		}
	}
}
