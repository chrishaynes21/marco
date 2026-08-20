package observe_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Grounding a whole screen.
//
// The property every test here defends: what is drawn for a place is a PART of that place,
// measured now, belonging to the screen that was named — not the window outline, not the current
// screen, and not a rectangle remembered from when the screen was first seen.

// THE pinning test. A start established two minutes ago is grounded on the screen it was
// established on, not on wherever the user has since wandered.
//
// The mutation this exists for: replacing the state argument with the live account's current
// state. It passes every "does a box appear" check, and it turns the START highlight into a
// confirmation of whatever the user happens to be looking at — the failure mode that makes a
// grounded learn session worse than an ungrounded one, because it looks like agreement.
func TestGroundingAPlaceUsesThePinnedScreenAndNotWhateverIsCurrent(t *testing.T) {
	live := twoScreens(t)

	got := observe.ReferentForPlace("state_start", observe.ReferentLearnStart, live)
	if !got.CanPoint() {
		t.Fatalf("the established screen could not be grounded: %q", got.Unavailable)
	}
	if got.Role != observe.ReferentLearnStart {
		t.Errorf("role %q, want the Learn start", got.Role)
	}
	// Every region belongs to the start screen's structure, and none to the other one.
	allDrawnFrom(t, got.Regions, regionsOfState(t, "state_start", live), "state_start")
	if sharesAnything(got.Regions, regionsOfState(t, "state_elsewhere", live)) {
		t.Fatal("the start was grounded on the screen the user moved to. A highlight that " +
			"follows the user confirms whatever they are looking at, which is the one thing " +
			"showing them the start is supposed to catch")
	}
}

// The two endpoints are grounded on their own screens, not on one shared answer.
func TestTheStartAndTheDestinationAreDifferentScreens(t *testing.T) {
	live := twoScreens(t)

	start := observe.ReferentForPlace("state_start", observe.ReferentLearnStart, live)
	dest := observe.ReferentForPlace("state_elsewhere", observe.ReferentLearnDestination, live)
	if !start.CanPoint() || !dest.CanPoint() {
		t.Fatalf("one endpoint could not be grounded: start=%q destination=%q",
			start.Unavailable, dest.Unavailable)
	}
	if sharesAnything(start.Regions, dest.Regions) {
		t.Fatal("the start and the destination were grounded on the same regions; a person " +
			"shown both would see no difference between where they began and where they ended")
	}
}

// A screen Marco has not settled on is not grounded, and does not silently become one that is.
func TestAnUnsettledScreenIsNotGrounded(t *testing.T) {
	live := twoScreens(t)

	for _, state := range []observe.ScreenStateID{"", observe.ScreenStateUnknown, "state_none"} {
		got := observe.ReferentForPlace(state, observe.ReferentLearnStart, live)
		if got.CanPoint() {
			t.Fatalf("state %q produced %d region(s). With no screen settled there is nothing "+
				"for a highlight to be about", state, len(got.Regions))
		}
		if got.Unavailable != observe.ReferentNotAPart {
			t.Errorf("state %q refused with %q, want %q",
				state, got.Unavailable, observe.ReferentNotAPart)
		}
	}
}

// The description says what the highlight IS, and never that the screen is those controls.
//
// A place is grounded by the structure Marco recognises it by. That is a smaller claim than "the
// start is these eight things", and the wording is the only thing keeping the two apart — a
// surface shown the plain "8 controls" phrasing has no way to tell which claim it is making.
func TestAGroundedScreenSaysItIsWhatTheScreenIsRecognisedBy(t *testing.T) {
	live := twoScreens(t)

	got := observe.ReferentForPlace("state_start", observe.ReferentLearnStart, live)
	if !got.CanPoint() {
		t.Fatalf("nothing to describe: %q", got.Unavailable)
	}
	if !strings.Contains(got.About, "recognise this screen by") {
		t.Errorf("the description reads %q; it must say the highlight is what the screen is "+
			"recognised BY, not that the screen is those controls", got.About)
	}
}

// Grounding a place inherits every rule the one resolver already states.
//
// Specifically the one that matters most here: a container spanning the window is not a part of
// the screen. An accessibility tree hands back the window root beside the controls inside it, and
// a place grounded on the root would outline the whole application — which is the exact gesture
// ReferentNotAPart exists to refuse.
func TestGroundingAPlaceStillRefusesTheWholeWindow(t *testing.T) {
	tracks := []observe.ShadowTrack{
		inState("t_root", "state_start", observe.Region{X: 0, Y: 0, Width: 1, Height: 1}),
		inState("t_root2", "state_start", observe.Region{X: 0, Y: 0, Width: 0.99, Height: 0.99}),
	}
	live := observe.LiveGeometry{
		Application: "code.exe", Window: "window_1", AtInference: 9, Reliable: true,
		Tracks: tracks, States: []observe.ScreenState{{ID: "state_start", Episodes: 3}},
	}
	got := observe.ReferentForPlace("state_start", observe.ReferentLearnStart, live)
	if got.CanPoint() {
		t.Fatalf("outlined the whole window as the start (%d region(s)). A person shown their "+
			"entire application concludes Marco means the entire application", len(got.Regions))
	}
	if got.Unavailable != observe.ReferentNotAPart {
		t.Errorf("refused with %q, want %q", got.Unavailable, observe.ReferentNotAPart)
	}
}

// Structure the segmenter has parked under the unknown screen is not a screen.
//
// The state exists, it has members, and every one of them is on screen — so "is there a group for
// this state" answers yes and the referent resolves. That is the whole trap: "not placed yet" and
// "placed on the unknown state" are the same fact to a person, and a highlight for either claims
// Marco decided something it has not.
func TestStructureParkedUnderTheUnknownScreenIsNeverGrounded(t *testing.T) {
	var tracks []observe.ShadowTrack
	for i, r := range []observe.Region{
		{X: 0.05, Y: 0.10, Width: 0.20, Height: 0.05},
		{X: 0.05, Y: 0.20, Width: 0.20, Height: 0.05},
		{X: 0.05, Y: 0.30, Width: 0.20, Height: 0.05},
	} {
		tracks = append(tracks, inState(startTrackID(i), observe.ScreenStateUnknown, r))
	}
	live := observe.LiveGeometry{
		Application: "code.exe", Window: "window_1", AtInference: 12, Reliable: true,
		Tracks: tracks,
		States: []observe.ScreenState{{
			ID: observe.ScreenStateUnknown, Episodes: 3, Inferences: 8}},
	}
	got := observe.ReferentForPlace(observe.ScreenStateUnknown, observe.ReferentLearnStart, live)
	if got.CanPoint() {
		t.Fatalf("grounded %d region(s) on the unknown screen", len(got.Regions))
	}
	if got.Unavailable != observe.ReferentNotAPart {
		t.Errorf("refused with %q, want %q", got.Unavailable, observe.ReferentNotAPart)
	}
}

// Nothing watched is not the same answer as nothing to single out.
func TestGroundingWithNothingWatchedSaysSo(t *testing.T) {
	got := observe.ReferentForPlace("state_start", observe.ReferentLearnStart,
		observe.LiveGeometry{})
	if got.Unavailable != observe.ReferentNothingWatched {
		t.Errorf("refused with %q, want %q", got.Unavailable, observe.ReferentNothingWatched)
	}
}

// ── fixtures ──────────────────────────────────────────────────────────────────

// twoScreens is one session that has seen two distinct screens, each with its own structure.
//
// Built from tracks rather than from a StructuralGroup value, because the resolver re-derives
// groups from tracks and states — a hand-built group would be describing a different session from
// the one the code under test reads.
func twoScreens(t *testing.T) observe.LiveGeometry {
	t.Helper()
	var tracks []observe.ShadowTrack
	for i, r := range []observe.Region{
		{X: 0.05, Y: 0.10, Width: 0.20, Height: 0.05},
		{X: 0.05, Y: 0.20, Width: 0.20, Height: 0.05},
		{X: 0.05, Y: 0.30, Width: 0.20, Height: 0.05},
	} {
		tracks = append(tracks, inState(startTrackID(i), "state_start", r))
	}
	for i, r := range []observe.Region{
		{X: 0.60, Y: 0.10, Width: 0.20, Height: 0.05},
		{X: 0.60, Y: 0.20, Width: 0.20, Height: 0.05},
		{X: 0.60, Y: 0.30, Width: 0.20, Height: 0.05},
	} {
		tracks = append(tracks, inState(elsewhereTrackID(i), "state_elsewhere", r))
	}
	return observe.LiveGeometry{
		Application: "code.exe", Window: "window_1", AtInference: 12, Reliable: true,
		Tracks: tracks,
		States: []observe.ScreenState{
			{ID: "state_start", Episodes: 3, Inferences: 8},
			{ID: "state_elsewhere", Episodes: 2, Inferences: 6},
		},
	}
}

func startTrackID(i int) string     { return "t_start_" + string(rune('a'+i)) }
func elsewhereTrackID(i int) string { return "t_else_" + string(rune('a'+i)) }

// track is one structure that is reliably present in exactly one state — the membership rule
// Groups() applies, restated in the fixture so the group forms for the reason production does.
func inState(id string, state observe.ScreenStateID, r observe.Region) observe.ShadowTrack {
	return observe.ShadowTrack{
		ID: id, Present: true, Reference: r, Seen: 6, Eligible: 6,
		States: []observe.TrackState{{State: state, Seen: 6, Eligible: 6}},
	}
}

// regionsOfState is every region belonging to one screen, read the way the resolver reads it.
func regionsOfState(t *testing.T, state observe.ScreenStateID,
	live observe.LiveGeometry) []observe.Region {

	t.Helper()
	var out []observe.Region
	for _, g := range observe.Groups(live.Tracks, live.States) {
		if g.State != state {
			continue
		}
		for _, id := range g.Members {
			for _, tr := range live.Tracks {
				if tr.ID == id {
					out = append(out, tr.Reference)
				}
			}
		}
	}
	return out
}
