package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// offering is one settled screen that keeps offering the same controls.
//
// `seen` is how many readings of the screen carried each control, which is the whole variable
// these tests turn: recurrence is what separates a property of the screen from a frame.
func offering(seen int, labels ...string) observe.ShadowTotals {
	st := observe.ScreenState{
		ID: "state_1", Roles: map[string]int{"button": 4},
		Inferences: 12, Episodes: 3, TermObservations: 12, Settled: true,
		Affordances: map[string]observe.AffordanceTally{},
	}
	for _, l := range labels {
		st.Affordances[l] = observe.AffordanceTally{
			Kind: observe.TargetKind("button"), Seen: seen}
	}
	return observe.ShadowTotals{CurrentState: "state_1", States: []observe.ScreenState{st}}
}

func labelsOf(sigs []observe.StructureSignature) []string {
	out := make([]string, 0, len(sigs))
	for _, s := range sigs {
		out = append(out, s.Label)
	}
	return out
}

// ACCEPTANCE A AND B — A STABLE AFFORDANCE IS LEARNED, CLICKED OR NOT.
//
// The whole product claim of 38D step 2, and the second half is the interesting one: nobody
// pressed any of these. A control does not have to be used to be known — what it LEADS TO does,
// and that is a different fact this can never write.
func TestAControlThatKeepsBeingThereBecomesADurableTarget(t *testing.T) {
	totals := offering(3, "Bluetooth & devices", "Network & internet")
	m := recallStub{verdict: observe.MatchSame, subject: "subj_home"}

	got := observe.TargetsToRecord(totals, "settings", m,
		observe.DefaultHypothesisThresholds())
	if len(got) != 2 {
		t.Fatalf("a settled screen offering two controls on every reading produced %v",
			labelsOf(got))
	}
	for _, sig := range got {
		if sig.Subject != observe.SubjectTarget {
			t.Errorf("%q was recorded as %s, not a target", sig.Label, sig.Subject)
		}
		if sig.Place != "subj_home" {
			t.Errorf("%q is scoped to %q rather than the Place it was seen in",
				sig.Label, sig.Place)
		}
	}
}

// ACCEPTANCE C — ONE SIGHTING IS NOT A PROPERTY OF THE SCREEN.
//
// A menu that was open for one reading, a toast, a list still loading, or a transition frame
// carrying the controls of the page being left. The screen is the same screen; what was on it for
// one moment is not what it offers.
//
// Deleting the recurrence rule must fail this.
func TestAControlSeenOnceDoesNotBecomeDurable(t *testing.T) {
	m := recallStub{verdict: observe.MatchSame, subject: "subj_home"}
	got := observe.TargetsToRecord(offering(1, "Update available"), "settings", m,
		observe.DefaultHypothesisThresholds())
	if len(got) != 0 {
		t.Fatalf("a control seen on one reading became durable: %v", labelsOf(got))
	}
}

// ACCEPTANCE E — THE SAME CONTROL AFTER A REFLOW IS THE SAME CONTROL.
//
// Presentation moves. A window is resized, a pane collapses, a button lands in another corner —
// and a durable identity that had moved with it would be a second record of one control that
// nothing will ever reconcile.
//
// The test is structural rather than behavioural on purpose: geometry cannot change the answer
// because no geometry ever reaches the identity. See TargetSignature.
func TestATargetIdentityCarriesNoGeometry(t *testing.T) {
	m := recallStub{verdict: observe.MatchSame, subject: "subj_home"}
	got := observe.TargetsToRecord(offering(3, "Mouse"), "settings", m,
		observe.DefaultHypothesisThresholds())
	if len(got) != 1 {
		t.Fatalf("want one target, got %v", labelsOf(got))
	}
	sig := got[0]
	if sig.Label != "Mouse" || sig.Place != "subj_home" || sig.Kind == "" {
		t.Fatalf("the identity is %+v", sig)
	}
	// Everything a presentation could contribute must be absent. Named individually rather
	// than by a reflection sweep, because the point is that each one was CONSIDERED.
	if sig.Roles != nil || sig.Members != 0 || sig.Envelope != nil {
		t.Errorf("a target identity carries screen shape: roles=%v members=%d envelope=%v. "+
			"A control that reflowed would become a second target.",
			sig.Roles, sig.Members, sig.Envelope)
	}
}

// ACCEPTANCE F — A LABEL THAT LOOKS LIKE A PLACE IS STILL ONLY A LABEL.
//
// The mutation this exists to kill: `Home` offers a control called `Bluetooth & devices`, Marco
// knows a Place called `Bluetooth & devices`, and an edge appears out of two strings agreeing.
//
// Nothing observed anybody arriving anywhere. The words matching is not evidence about a
// transition, and a map that grew edges this way would be confidently wrong about somebody's
// computer in exactly the way the ledger exists to prevent.
//
// The proof is structural, which is the strongest kind available here: what this returns is a
// signature, its type cannot express a relationship, and the store method the write path holds
// can write a subject and nothing else.
func TestALabelMatchingAPlaceNameCreatesNoEdge(t *testing.T) {
	m := recallStub{verdict: observe.MatchSame, subject: "subj_home"}
	got := observe.TargetsToRecord(offering(4, "Bluetooth & devices"), "settings", m,
		observe.DefaultHypothesisThresholds())
	if len(got) != 1 {
		t.Fatalf("want the target, got %v", labelsOf(got))
	}
	for _, sig := range got {
		// A signature has no from, no to and no destination. If any of those ever appear
		// on it, this is the test to revisit first.
		if sig.Subject != observe.SubjectTarget {
			t.Errorf("a label produced a %s rather than a target", sig.Subject)
		}
		if sig.Place != "subj_home" {
			t.Errorf("the target is scoped to %q. A target naming its DESTINATION "+
				"would be an edge wearing a target's type.", sig.Place)
		}
	}
}

// ACCEPTANCE J — A SCREEN NOBODY COULD READ OFFERS NOTHING.
//
// An unsettled state is one still changing shape, which is what a page looks like while it is
// arriving. Remembering what it painted halfway through loading would durably record furniture
// that was never a property of anything.
func TestAnUnsettledScreenOffersNothing(t *testing.T) {
	totals := offering(5, "Sign in")
	totals.States[0].Settled = false
	got := observe.TargetsToRecord(totals, "settings",
		recallStub{verdict: observe.MatchSame, subject: "subj_home"},
		observe.DefaultHypothesisThresholds())
	if len(got) != 0 {
		t.Fatalf("an unsettled screen offered %v", labelsOf(got))
	}
}

// AND A SCREEN MARCO CANNOT PLACE OFFERS NOTHING EITHER.
//
// A target is scoped to a Place; without one there is nothing to scope it to. Writing it against
// a transient state id would produce a durable record nothing could ever resolve.
func TestAnUnrecognisedScreenOffersNothing(t *testing.T) {
	got := observe.TargetsToRecord(offering(5, "Sign in"), "settings",
		recallStub{verdict: observe.MatchDifferent}, observe.DefaultHypothesisThresholds())
	if len(got) != 0 {
		t.Fatalf("a screen with no durable identity offered %v", labelsOf(got))
	}
}

// AND TWO READINGS DISAGREEING ABOUT WHAT A CONTROL IS PRODUCE NOTHING.
//
// The same rule two Actors disagreeing about a place name get. A durable target keyed on the
// wrong kind is a second record of one control, and being seen more often cannot resolve it.
func TestAControlWhoseKindIsDisagreedIsNotRemembered(t *testing.T) {
	totals := offering(5, "Mouse")
	totals.States[0].Affordances["Mouse"] = observe.AffordanceTally{Seen: 9}
	got := observe.TargetsToRecord(totals, "settings",
		recallStub{verdict: observe.MatchSame, subject: "subj_home"},
		observe.DefaultHypothesisThresholds())
	if len(got) != 0 {
		t.Fatalf("a control whose kind two readings disagreed about became durable: %v",
			labelsOf(got))
	}
}

// ACCEPTANCE D — A SWEEP DOES NOT ADMIT THE ROLES A DEMONSTRATION DOES.
//
// # The line this holds, and why it is not the same line ADR-114 drew
//
// AdmittedTargetLabel admits an activatable role — a list item, a tree item, a link — under a
// demonstration licence, and ADR-114 states the limit in the same breath: what makes that text
// admissible is that the person aimed at THAT control, and the gate admits "only what one input
// event's own resolution touched, never a sweep".
//
// This is the sweep. There is no per-element provenance for anything on screen, so the widening
// does not travel. A chat list's rows, a file list's names and a Settings navigation rail are all
// `list_item`, and no rule available here can tell them apart without naming applications.
//
// Making AdmittedAffordanceLabel admit role.Clickable() must fail this.
func TestASweepDoesNotAdmitTheRolesADemonstrationDoes(t *testing.T) {
	const label = "Bluetooth"
	// The CONTROL: a demonstration may keep a list item's name.
	if got := observe.AdmittedTargetLabel(directorapi.RoleListItem, true, label, 1); got != label {
		t.Fatalf("a demonstration was refused a list item's name (%q), so this test "+
			"proves nothing about the difference", got)
	}
	// And a sweep may not.
	if got := observe.AdmittedAffordanceLabel(directorapi.RoleListItem, label, 1); got != "" {
		t.Errorf("a sweep kept a list item's name (%q). A chat list, a file list and a "+
			"navigation rail are the same role, and nothing here can tell them apart.", got)
	}
	// AND A BUTTON IS STILL ADMITTED, or the gate closes by refusing everything.
	if got := observe.AdmittedAffordanceLabel(directorapi.RoleButton, label, 1); got != label {
		t.Errorf("a sweep refused a button's name (%q); the allowlist is the point of "+
			"the gate and a button is on it", got)
	}
}

// AND THE SHAPE FILTER IS UNCONDITIONAL, as it is everywhere else.
//
// Defence in depth: the role allowlist is the primary defence, and a name that looks like a
// handle, a token or a filename is refused whatever role carries it.
func TestASweepStillRefusesTextThatDoesNotLookLikeAControl(t *testing.T) {
	for _, text := range []string{"@someone", "quarterly-report-final-v3.xlsx", "u/1f9a3c"} {
		if got := observe.AdmittedAffordanceLabel(directorapi.RoleButton, text, 1); got != "" {
			t.Errorf("a sweep kept %q as a control name", got)
		}
	}
}

// AND AN AFFORDANCE SURVIVES THE CONSTRUCTORS THAT REBUILD EVIDENCE.
//
// The trap the file records in its own comment: `PlaceName` was added upstream, passed its own
// tests, and was silently dropped by `admissibleTerms`, which rebuilds the struct field by field.
// The chain looked connected from both ends.
func TestAnAffordanceSurvivesTheProductionEvidencePath(t *testing.T) {
	in := observe.SemanticEvidence{
		Observed:    true,
		Affordances: []observe.ObservedAffordance{{Label: "Mouse", Kind: "button"}},
	}
	// Merge is the other rebuild, and it runs on every sample that has two sources.
	merged := in.Merge(observe.SemanticEvidence{Observed: true})
	if len(merged.Affordances) != 1 || merged.Affordances[0].Label != "Mouse" {
		t.Fatalf("merging dropped the affordance: %+v", merged.Affordances)
	}
	if in.Empty() {
		t.Error("evidence carrying only affordances reports itself empty, so a reading " +
			"whose whole finding was what the screen offers is discarded upstream")
	}
}
