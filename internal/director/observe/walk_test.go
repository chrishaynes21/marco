package observe

import "testing"

// The walk is the sequence of places, bridged across the frames nobody can place.
//
// # The live crossings
//
// A Settings demonstration — Home to Bluetooth & devices to Mouse — recorded exactly this:
//
//	state_1 → state_unknown
//	state_unknown → state_2   run 1
//	state_2 → state_unknown
//	state_unknown → state_3   run 1
//
// Two screen changes, four crossings, and not one of them has a placeable state at both ends.
// Reading them pairwise produced an empty walk, which silently restored the single-edge
// behaviour the review lifecycle exists to replace.

// knownPlaces resolves each state to a durable subject of the same name.
type knownPlaces struct{ Memory }

func (knownPlaces) Recall(_ string, sig StructureSignature) Recollection {
	// The fixture's signatures carry the subject in their single role name.
	for role := range sig.Roles {
		return Recollection{Verdict: MatchSame,
			Subject: RememberedSubject{ID: "subj_" + role}}
	}
	return Recollection{Verdict: MatchDifferent}
}

func stateNamed(id ScreenStateID, role string) ScreenState {
	return ScreenState{ID: id, Episodes: 3, TermObservations: 2,
		Roles: map[string]int{role: 4}}
}

func liveCrossings() ShadowTotals {
	return ShadowTotals{
		CurrentState: "state_3",
		States: []ScreenState{
			stateNamed("state_1", "home"),
			stateNamed("state_2", "bt"),
			stateNamed("state_3", "mouse"),
		},
		Crossings: []Crossing{
			{From: "state_1", To: ScreenStateUnknown},
			{From: ScreenStateUnknown, To: "state_2", Run: 1},
			{From: "state_2", To: ScreenStateUnknown},
			{From: ScreenStateUnknown, To: "state_3", Run: 1},
		},
	}
}

// A walk through unplaceable frames still produces its edges, in order.
//
// Reading crossings pairwise instead of bridging must fail this.
func TestAWalkBridgesTheFramesNobodyCanPlace(t *testing.T) {
	grown := []RelationshipRef{
		{From: "subj_bt", To: "subj_mouse"},
		{From: "subj_home", To: "subj_bt"},
	}
	got := DemonstratedWalk(liveCrossings(), "settings", knownPlaces{},
		DefaultHypothesisThresholds(), grown)

	want := []RelationshipRef{
		{From: "subj_home", To: "subj_bt"},
		{From: "subj_bt", To: "subj_mouse"},
	}
	if len(got) != len(want) {
		t.Fatalf("walk has %d edge(s), want %d: %v.\nEvery crossing here passes through an "+
			"unplaceable frame, and an empty walk silently restores single-edge review.",
			len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("edge %d is %v, want %v — the walk is the order it happened in",
				i+1, got[i], want[i])
		}
	}
}

// The order comes from the WALK, not from the order the edges were written.
//
// `grown` above is deliberately given back-to-front; the answer must not follow it.
func TestTheWalkIgnoresTheOrderEdgesWereWrittenIn(t *testing.T) {
	backwards := []RelationshipRef{
		{From: "subj_bt", To: "subj_mouse"},
		{From: "subj_home", To: "subj_bt"},
	}
	got := DemonstratedWalk(liveCrossings(), "settings", knownPlaces{},
		DefaultHypothesisThresholds(), backwards)
	if len(got) == 0 || got[0].From != "subj_home" {
		t.Errorf("the walk followed the written order: %v", got)
	}
}

// An edge nothing walked is not part of this demonstration's sequence.
//
// It stays perfectly good durable knowledge; it is just not what this demonstration is
// evidence for, and reviewing it would be reviewing something nobody showed Marco.
func TestAnIncidentalEdgeIsNotInTheWalk(t *testing.T) {
	grown := []RelationshipRef{
		{From: "subj_home", To: "subj_bt"},
		{From: "subj_bt", To: "subj_mouse"},
		{From: "subj_elsewhere", To: "subj_other"},
	}
	got := DemonstratedWalk(liveCrossings(), "settings", knownPlaces{},
		DefaultHypothesisThresholds(), grown)
	for _, e := range got {
		if e.From == "subj_elsewhere" {
			t.Errorf("an edge no crossing explains entered the walk: %v", got)
		}
	}
	if len(got) != 2 {
		t.Errorf("walk has %d edge(s), want 2: %v", len(got), got)
	}
}

// A place revisited across a gap is one visit, not a step to itself.
func TestAPlaceSeenAgainAcrossAGapIsNotAStep(t *testing.T) {
	tot := liveCrossings()
	tot.Crossings = []Crossing{
		{From: "state_1", To: ScreenStateUnknown},
		{From: ScreenStateUnknown, To: "state_1", Run: 1},
		{From: "state_1", To: ScreenStateUnknown},
		{From: ScreenStateUnknown, To: "state_2", Run: 1},
	}
	got := DemonstratedWalk(tot, "settings", knownPlaces{},
		DefaultHypothesisThresholds(),
		[]RelationshipRef{{From: "subj_home", To: "subj_bt"}})
	if len(got) != 1 || got[0].From != "subj_home" || got[0].To != "subj_bt" {
		t.Errorf("walk = %v, want one Home to Bluetooth step", got)
	}
}
