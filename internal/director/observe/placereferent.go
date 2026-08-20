package observe

import "fmt"

// Pointing at a whole screen, when a whole screen is exactly what cannot be pointed at.
//
// # The tension, stated honestly
//
// [ReferentForSubject] refuses a screen-shaped subject with [ReferentNotAPart], and that rule is
// right: outlining the window says nothing a person did not already know. But teaching establishes
// a START and a DESTINATION, and both of them ARE screens. "Marco has decided where you are
// starting" is the single most consequential thing a teach session does before it asks anybody to
// demonstrate anything, and it is currently invisible.
//
// # What is shown instead, and why it is not a fudge
//
// Not the window. The structure Marco recognises the screen BY — the largest co-occurring group
// belonging to that screen state, which is the same `dominantGroup` notion the hypothesis layer
// already uses to say what a screen is dominated by. It is real live member geometry, resolved
// through the one resolver, and it is a part rather than the whole.
//
// The wording that goes with it has to carry the difference, and the caller owns that: this is
// "what I recognise this screen by", never "the start is these controls". A surface that captioned
// it as the latter would be claiming a precision the highlight does not have.
//
// # It adds no geometry path
//
// It selects a subject and hands it to [ReferentForSubject]. Every rule that file states —
// members not present are omitted, whole-window members are not parts, members outside their own
// window cannot be placed, enclosures are dropped — applies unchanged, because this does not
// reimplement any of them.

// ReferentForPlace resolves the structure that stands for one screen.
//
// The state is PINNED by the caller and is not re-read from what is current. A teaching start
// established two minutes ago must resolve against the screen it was established on; grounding
// that drifted to wherever the user is standing now would confirm whatever they happened to be
// looking at, which is the one thing this must never do.
//
// Deleting the state argument in favour of live.Tracks' current state must fail
// TestGroundingAPlaceUsesThePinnedScreenAndNotWhateverIsCurrent.
func ReferentForPlace(state ScreenStateID, role ReferentRole, live LiveGeometry) VisualReferent {
	// An unsettled screen is never grounded, even if the segmenter happens to have parked
	// structure under it. "Not placed yet" and "placed on the unknown state" are the same fact
	// to a person, and pointing at either would claim Marco had decided something it had not.
	//
	// Deleting this must fail TestStructureParkedUnderTheUnknownScreenIsNeverGrounded.
	settled := state != "" && state != ScreenStateUnknown
	g, ok := standsFor(state, live)
	// Stamped onto whatever the resolver returns, so a refusal says which of the two
	// place-specific steps failed. Both can fail independently and they mean different things:
	// an unsettled screen is "I have not decided where you are", a screen with no group is
	// "I know where you are and nothing on it stands for it".
	stamp := func(v VisualReferent) VisualReferent {
		v.Diagnosis.StateSettled, v.Diagnosis.StandsForGroup = settled, ok
		return v
	}
	if !settled || !ok {
		// Either Marco has not settled on a screen, or the screen it settled on has no
		// structure that stands for it. Both are "there is nothing here to single out".
		//
		// Asked of the one resolver rather than answered here, using a subject no group can
		// match: whether anything is being watched at all, and whether this display's geometry
		// can be trusted, are conditions that file owns. Restating them would be a second
		// place deciding when a referent is unavailable, and the two would drift.
		v := ReferentForSubject(Subject{Kind: SubjectGroup}, role, live)
		if v.Unavailable == ReferentNotOnScreen {
			v.Unavailable, v.Diagnosis.Refusal = ReferentNotAPart, ReferentNotAPart
		}
		return stamp(v)
	}
	v := ReferentForSubject(Subject{Kind: SubjectGroup, Ref: g.ID}, role, live)
	if v.CanPoint() {
		// The description says what the highlight IS. A place grounded by its structure is
		// not the same claim as a question about that structure, and reusing the plain
		// "N controls" wording would let a surface present it as one.
		v.About = fmt.Sprintf("%d %s I recognise this screen by", len(v.Regions),
			plural(len(v.Regions), "control", "controls"))
	}
	return stamp(v)
}

// standsFor is the group that most characterises one screen state.
//
// The same choice `dominantGroup` makes — the largest co-occurring set — reached through the
// public group derivation because that is what [LiveGeometry] carries. A screen with no group at
// all has nothing on it that can stand for it, and that is reported rather than approximated.
func standsFor(state ScreenStateID, live LiveGeometry) (StructuralGroup, bool) {
	var best StructuralGroup
	found := false
	for _, g := range Groups(live.Tracks, live.States) {
		if g.State != state {
			continue
		}
		if !found || len(g.Members) > len(best.Members) {
			best, found = g, true
		}
	}
	return best, found
}
