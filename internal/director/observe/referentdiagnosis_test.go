package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Telling the two causes of `coordinate_mapping_unreliable` apart.
//
// # Why this needed a milestone of its own
//
// A live Explorer teach attempt refused with `coordinate_mapping_unreliable` and no surface could
// say which of two opposite things had happened:
//
//	A. there is no trustworthy frame to convert against  — a fact about the SESSION
//	B. the group resolved and every member sits outside the window — a fact about the PROVIDER
//
// A is a wiring or lifecycle defect and is ours to fix. B is the accessibility tree reporting a
// rectangle that is not inside the window being watched, which is a known perception limitation
// and must stay an honest refusal. Same string, opposite remedies, and guessing between them is
// how the wrong one gets "fixed".
//
// The product sentence stays coarse on purpose. These hold the DIAGNOSIS.

// grounded is the ordinary live geometry every test here starts from, with a group on screen.
//
// Built through the real tracker by [watching], because group membership has rules — persistent in
// exactly one state, cut into vertical runs — and a hand-built group would be testing a shape
// production never forms. The variations below MUTATE this rather than inventing one, so each test
// differs from the working case in exactly the one thing it is about.
func grounded(t *testing.T) (observe.LiveGeometry, observe.StructuralGroup) {
	t.Helper()
	live, onScreen, _ := watching(t)
	live.FrameSequence = 12
	return live, onScreen
}

// pushedOutsideTheWindow moves every member's reference off the left of the frame.
//
// The live Discord shape: an accessibility tree reporting an element whose desktop rectangle is not
// inside the window being watched, so normalising it lands outside 0..1. Applied to the tracks the
// real tracker produced, so everything else about them is exactly what production would hold.
func pushedOutsideTheWindow(live observe.LiveGeometry) observe.LiveGeometry {
	out := make([]observe.ShadowTrack, len(live.Tracks))
	copy(out, live.Tracks)
	for i := range out {
		out[i].Reference.X -= 1.5
	}
	live.Tracks = out
	return live
}

// ── cause A: no trustworthy frame ─────────────────────────────────────────────

func TestAnUnreliableFrameIsDistinguishableFromUnplaceableMembers(t *testing.T) {
	live, group := grounded(t)
	live.Reliable = false
	v := observe.ReferentForSubject(
		observe.Subject{Kind: observe.SubjectGroup, Ref: group.ID},
		observe.ReferentPlace, live)

	if v.Unavailable != observe.ReferentCoordinatesUnreliable {
		t.Fatalf("refusal %q, want %q", v.Unavailable, observe.ReferentCoordinatesUnreliable)
	}
	d := v.Diagnosis
	if d.FrameReliable {
		t.Error("the diagnosis reports a reliable frame for a resolution refused BECAUSE the " +
			"frame was unreliable")
	}
	if !d.FrameAvailable {
		t.Error("frame_available is false although a sample recorded rectangle 12; " +
			"\"nothing has sampled\" and \"a sample ran and its rectangle was unusable\" " +
			"are different defects")
	}
	// The decisive assertion: the member funnel is UNTOUCHED, because the resolver stopped
	// before it. A reader seeing zeros here knows the members were never the problem.
	if d.SubjectResolved || d.MembersTotal != 0 || d.MembersOutsideWindow != 0 {
		t.Errorf("the funnel ran on a resolution that stopped at the frame check: %+v", d)
	}
	if d.Refusal != observe.ReferentCoordinatesUnreliable {
		t.Errorf("the diagnosis records refusal %q", d.Refusal)
	}
}

func TestNoFrameAtAllIsDistinguishableFromAnUnusableOne(t *testing.T) {
	live, group := grounded(t)
	live.Reliable, live.FrameSequence = false, 0
	v := observe.ReferentForSubject(
		observe.Subject{Kind: observe.SubjectGroup, Ref: group.ID},
		observe.ReferentPlace, live)

	if v.Diagnosis.FrameAvailable {
		t.Fatal("frame_available is true although no sample ever recorded a rectangle")
	}
}

// ── cause B: the group resolved and nothing could be placed ───────────────────

func TestAGroupWhoseMembersAreOutsideTheWindowSaysSo(t *testing.T) {
	// The live Discord/Explorer shape: an accessibility tree reporting an element whose
	// desktop rectangle is not inside the window being watched, so normalising it lands
	// outside 0..1.
	live, group := grounded(t)
	live = pushedOutsideTheWindow(live)
	v := observe.ReferentForSubject(
		observe.Subject{Kind: observe.SubjectGroup, Ref: group.ID},
		observe.ReferentPlace, live)

	if v.Unavailable != observe.ReferentCoordinatesUnreliable {
		t.Fatalf("refusal %q, want %q", v.Unavailable, observe.ReferentCoordinatesUnreliable)
	}
	d := v.Diagnosis
	// The same product refusal as the test above, and every diagnostic field disagrees.
	if !d.FrameReliable || !d.FrameAvailable {
		t.Errorf("the frame was fine and the diagnosis says otherwise: %+v", d)
	}
	if !d.SubjectResolved {
		t.Error("subject_resolved is false although the group was found and walked")
	}
	if d.MembersPresent == 0 || d.MembersWithRegion != d.MembersPresent {
		t.Errorf("every member was present and sized; funnel says present=%d sized=%d",
			d.MembersPresent, d.MembersWithRegion)
	}
	if d.MembersOutsideWindow != d.MembersWithRegion {
		t.Errorf("members_outside_window = %d of %d sized — this is the count that names "+
			"the provider-geometry defect, and it must account for all of them",
			d.MembersOutsideWindow, d.MembersWithRegion)
	}
	if d.MembersPlaceable != 0 || d.Regions != 0 {
		t.Errorf("something was reported placeable: %+v", d)
	}
}

// The funnel accounts for every member, at every step.
//
// Not decoration. A funnel whose numbers do not add up would send somebody looking for a defect in
// the step that did not lose anything.
func TestTheMemberFunnelAccountsForEveryMember(t *testing.T) {
	live, group := grounded(t)
	v := observe.ReferentForSubject(
		observe.Subject{Kind: observe.SubjectGroup, Ref: group.ID},
		observe.ReferentPlace, live)

	d := v.Diagnosis
	if !v.CanPoint() {
		t.Fatalf("an ordinary control inside its window could not be pointed at: %q %+v",
			v.Unavailable, d)
	}
	if d.MembersTotal < d.MembersPresent || d.MembersPresent < d.MembersWithRegion {
		t.Errorf("the funnel widens as it goes: %+v", d)
	}
	if got := d.MembersWithRegion - d.MembersWholeWindow - d.MembersOutsideWindow; got !=
		d.MembersPlaceable {
		t.Errorf("sized(%d) - whole_window(%d) - outside(%d) = %d, but placeable = %d",
			d.MembersWithRegion, d.MembersWholeWindow, d.MembersOutsideWindow, got,
			d.MembersPlaceable)
	}
	if got := d.MembersPlaceable - d.MembersEnclosing; got != d.Regions {
		t.Errorf("placeable(%d) - enclosing(%d) = %d, but regions = %d",
			d.MembersPlaceable, d.MembersEnclosing, got, d.Regions)
	}
	if d.Refusal != observe.ReferentAvailable {
		t.Errorf("a referent that can point records refusal %q", d.Refusal)
	}
}

// ── the place-specific steps ──────────────────────────────────────────────────

// A screen Marco has not settled on, and a screen with nothing that stands for it, are different.
//
// Both refuse with `not_a_part_of_the_screen`, which is right for a person and useless for a
// reader chasing why no highlight appeared.
func TestGroundingAPlaceSaysWhichOfItsTwoStepsFailed(t *testing.T) {
	live, group := grounded(t)

	unsettled := observe.ReferentForPlace(observe.ScreenStateUnknown,
		observe.ReferentTeachStart, live)
	if unsettled.Diagnosis.StateSettled {
		t.Error("the unknown screen is reported as settled")
	}

	// A settled screen the live geometry holds no group for.
	noGroup := observe.ReferentForPlace(observe.ScreenStateID("state_absent"),
		observe.ReferentTeachStart, live)
	if !noGroup.Diagnosis.StateSettled {
		t.Error("a named screen is reported as unsettled")
	}
	if noGroup.Diagnosis.StandsForGroup {
		t.Error("stands_for_group is true for a screen with no group in live geometry")
	}

	settled := observe.ReferentForPlace(group.State, observe.ReferentTeachStart, live)
	if !settled.Diagnosis.StateSettled || !settled.Diagnosis.StandsForGroup {
		t.Errorf("the fixture's own screen did not resolve: %+v", settled.Diagnosis)
	}
	if !settled.CanPoint() {
		t.Errorf("the fixture's own screen could not be pointed at: %q %+v",
			settled.Unavailable, settled.Diagnosis)
	}
}
