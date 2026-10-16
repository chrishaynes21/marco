package target

// THE INVARIANT POLICY RELIES ON.
//
// [[ADR-204-a-grounded-target-answers-the-world-level-question-and-nothing-else]] lets a grounded
// target answer two of policy's world-level gates, and policy TRUSTS this package's verdict: it
// does not re-derive what makes an element a legitimate target, because a second copy of that
// rule in the permission layer is how the two come to disagree.
//
// So the guarantee is owed from here. A world full of things that afford nothing must never
// produce a RESOLVED resolution, however well it is observed and however exactly the words match
// — and if that ever stops being true, the gate ADR-204 made conditional becomes conditional on
// nothing.

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// affordsNothing is a well-observed world whose every element is scenery: real rectangles, real
// labels, generic roles. It is the shape a Stage of proposed rectangles has, and also the shape a
// document viewer has.
func affordsNothing() directorapi.WorldState {
	els := map[directorapi.ElementID]*directorapi.Element{}
	for i, label := range []string{"Settings", "Scoreboard", "Resume"} {
		id := directorapi.ElementID(string(rune('a' + i)))
		els[id] = &directorapi.Element{
			ID: id, WindowID: "hwnd:1", Role: directorapi.RoleUnknown, Label: label,
			Bounds:  directorapi.Rect{X: 100, Y: 100 + 40*i, Width: 160, Height: 24},
			Enabled: true, Visible: true, Confidence: 0.99,
		}
	}
	win := directorapi.WindowID("hwnd:1")
	return directorapi.WorldState{
		Elements:     els,
		Windows:      []directorapi.Window{{ID: win, Application: "game", Focused: true}},
		ActiveWindow: &win,
	}
}

func TestNothingThatAffordsNothingEverResolves(t *testing.T) {
	w := affordsNothing()
	r := NewResolver()
	lic := directorapi.OperateLicence{Instruction: "click Settings", Verb: "click"}

	for _, tc := range []struct {
		name string
		res  directorapi.Resolution
	}{
		{"unlicensed", r.Resolve(&w, directorapi.ElementQuery{Text: "settings"})},
		{"licensed", r.ResolveUnder(&w, directorapi.ElementQuery{Text: "settings"},
			lic, "click")},
		{"an exact label", r.Resolve(&w, directorapi.ElementQuery{
			Label: "Settings", ExactLabel: true})},
	} {
		if tc.res.Status == directorapi.ResolutionResolved {
			t.Errorf("%s: a Prop that affords nothing resolved as a target: %+v",
				tc.name, tc.res.Target)
		}
		// AND THE MATCH IS STILL REPORTED, which is the other half of the contract: the
		// refusal has to be able to say what it saw and would not use.
		found := false
		for _, c := range tc.res.Candidates {
			if c.Label == "Settings" && c.Rejected != "" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: the refusal does not name the Prop it rejected", tc.name)
		}
	}
}

// AND SUCH A RESOLUTION DOES NOT GROUND.
//
// The same fact from the other side, so the two halves cannot drift apart: whatever this package
// returns for that world, `GroundedIn` refuses it.
func TestARefusedResolutionDoesNotGround(t *testing.T) {
	w := affordsNothing()
	res := NewResolver().ResolveUnder(&w, directorapi.ElementQuery{Text: "settings"},
		directorapi.OperateLicence{Instruction: "click Settings", Verb: "click"}, "click")
	if ok, why := res.GroundedIn(&w, w.Timestamp); ok {
		t.Error("a refused resolution counts as a grounded target")
	} else if why == "" {
		t.Error("it does not ground and says nothing about why")
	}
}
