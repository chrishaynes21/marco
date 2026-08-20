package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// A control Marco named while watching survives into the demonstration, and into the play.
//
// # The failure this is about
//
// Live, through the UI: two clicks, both resolved to named controls (`pointer_resolved=2`), a
// durable route, and a rehearsal that invoked the control and verified. Then:
//
//	lowering: eligible=no [screen_unnamed, unresolved_pointer_target]
//	step 1: point → expected subj_61ff…, observed subj_61ff… [directly_verified]
//
// `unresolved_pointer_target` reads as "a click with no semantic control behind it", which was
// not true of this demonstration. The label was present the whole way; lowering refused every
// click unconditionally, because a play had no way to name a control durably and a coordinate is
// not something a play can say.
//
// Both halves are now held here: the label arrives, and it is USED.
func TestANamedControlSurvivesIntoTheDemonstration(t *testing.T) {
	got := fromPass(clickedOn("Mouse"))
	if !got.Built {
		t.Fatalf("the demonstration did not build: %s", got.Refusal)
	}
	step := got.Candidate.Steps[0]
	if len(step.Targets) != 1 {
		t.Fatalf("the step carries %d target(s), want 1: %+v.\nThe producer named the "+
			"control the person clicked; if that name is gone by here it was lost in "+
			"capture, and no amount of work in lowering can recover it.",
			len(step.Targets), step)
	}
	if !step.Targets[0].Named() || step.Targets[0].Label != "Mouse" {
		t.Fatalf("the step's target is %+v, want the control called \"Mouse\"",
			step.Targets[0])
	}
}

// And lowering writes it down as a control to press, by name.
//
// THE closure. A click used to be unsayable; it is now a `Control` identified by its label, which
// the host resolves against what is on screen when the play runs. Nothing durable holds an
// accessibility runtime id, because a runtime id means nothing after the tree redraws.
//
// Mutation: drop `Called` from the lowered action, or go back to refusing every NavPoint. Both
// leave the step unsayable and fail here.
func TestALoweredClickCarriesTheControlsName(t *testing.T) {
	got := fromPass(clickedOn("Mouse"))
	j := lowerNamed(t, got.Candidate)

	if len(j.Steps) != 1 || len(j.Steps[0]) != 1 {
		t.Fatalf("lowered to %d step(s): %+v.\nrefusals: %v", len(j.Steps), j.Steps, j.Refusals)
	}
	a := j.Steps[0][0]
	if !a.Invokes() {
		t.Fatalf("the lowered action is %+v; a resolved click must lower to a control to "+
			"press, not to a meaning", a)
	}
	if a.Called != "Mouse" {
		t.Errorf("the play would press %q, want \"Mouse\"", a.Called)
	}
}

// A click Marco could NOT attribute to a control is still refused, and says so honestly.
//
// The control for the test above, and the reason the refusal kept its place in the vocabulary
// rather than being deleted: there really are clicks with no name behind them — nothing under the
// pointer, or a label the admission rule withheld — and inventing a coordinate for them would be
// exactly the thing a play must not say.
func TestAClickWithNoNameIsStillRefused(t *testing.T) {
	tot := twoPlaces()
	tot = moved(tot, "state_1", "state_2", observe.TargetedSequence{
		Intents: []observe.NavIntent{observe.NavPoint},
		Count:   1,
	})
	got := fromPass(tot)
	if !got.Built {
		t.Fatalf("the demonstration did not build: %s", got.Refusal)
	}
	j := lowerNamed(t, got.Candidate)
	if j.Eligible {
		t.Fatal("a click with no control behind it was written down; there is no name to " +
			"write, and a coordinate is not something a play can say")
	}
	if !refusedLowering(j, observe.RefusalNoTargetToName) {
		t.Errorf("refusals are %v, want %q", j.Refusals, observe.RefusalNoTargetToName)
	}
}

// clickedOn is a demonstration whose one action is a click on a named control.
func clickedOn(label string) observe.ShadowTotals {
	tot := twoPlaces()
	return moved(tot, "state_1", "state_2", observe.TargetedSequence{
		Intents: []observe.NavIntent{observe.NavPoint},
		Targets: []observe.SemanticTarget{{Role: "button", Label: label}},
		Count:   1,
	})
}

// refusedLowering reports whether a lowering judgement carries a reason.
func refusedLowering(j observe.LoweringJudgement, want observe.LoweringRefusal) bool {
	for _, r := range j.Refusals {
		if r == want {
			return true
		}
	}
	return false
}

// lowerNamed judges a candidate against a topology whose screens the user has named.
//
// The naming is not what these tests are about — a play cannot be written down without it, and
// without it every case here would refuse for the wrong reason.
func lowerNamed(t *testing.T, c observe.ProcedureCandidate) observe.LoweringJudgement {
	t.Helper()
	top := lowerTopology("subj_a", "subj_b")
	return observe.JudgeLowering(c, rehearsed(t, c, top, "d1"), top, "testgame")
}
