package target_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/target"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Grounding a request that names no control.
//
// "Scroll down" describes a REGION by what it can do, and the words it contains are not labels:
// there is no control called "down" and none called "top". So the query carries a capability and
// a preference instead of a name, and the resolver has to answer it with the same candidate
// ranking and the same ambiguity handling it gives every other request.
//
// Four things had to be true for that to work, and each was measured false first:
//
//	the query had to COUNT as a description      Constrained() excluded Scrollable, so a
//	                                             scroll refused with "did not describe
//	                                             anything specific enough to look for"
//	the capability had to be an affordance       Any() listed four capabilities and not this
//	                                             one, so a scrollable container afforded
//	                                             nothing and was "not an interactive control"
//	the container could not be scenery           a list is inert BY ROLE — you operate its
//	                                             items — and the penalty for that put it under
//	                                             MinScore, so it vanished with no rejected
//	                                             candidate to explain it
//	focus had to be a preference, not a filter   Notepad's editor scrolls AND holds focus;
//	                                             the fixture's focus starts somewhere that
//	                                             does not scroll while one list does
//
// Two surfaces disagreeing is what settled the last one. Filtering on focus refused the fixture;
// ignoring focus would pick arbitrarily among Notepad's three scrollable regions.

func scrollWorld(t *testing.T, focusedScrolls bool, extraScrollables int) directorapi.WorldState {
	t.Helper()
	mk := func(id, label string, role directorapi.ElementRole, focused bool, patterns string) *directorapi.Element {
		return &directorapi.Element{
			ID: directorapi.ElementID(id), Label: label, Role: role, WindowID: "hwnd:1",
			Enabled: true, Visible: true, Focused: focused, Confidence: 1,
			Bounds:     directorapi.Rect{X: 10, Y: 10, Width: 140, Height: 69},
			Attributes: map[string]any{"patterns": patterns, "native_id": id},
		}
	}
	els := map[directorapi.ElementID]*directorapi.Element{
		// The focused control. Scrollable or not, depending on the surface being modelled.
		"focus": mk("focus", "Name", directorapi.RoleTextField, true,
			map[bool]string{true: "value,scroll", false: "value"}[focusedScrolls]),
		// A scrollable container that is inert by role, like every list and pane.
		"list": mk("list", "Lines", directorapi.RoleList, false, "scroll"),
	}
	for i := 0; i < extraScrollables; i++ {
		id := directorapi.ElementID("extra" + string(rune('a'+i)))
		els[id] = mk(string(id), "", directorapi.RolePane, false, "scroll")
	}
	w := directorapi.WorldState{
		Elements: els,
		Windows:  []directorapi.Window{{ID: "hwnd:1", Application: "fixture", Focused: true}},
		Confidence: directorapi.WorldConfidence{
			ObservationQuality: 1, Coverage: 1, Actionability: 1, Freshness: 1,
		},
	}
	active := directorapi.WindowID("hwnd:1")
	w.ActiveWindow = &active
	return w
}

// scrollQuery is the query the real parser builds, not one written here. A hand-made query would
// test the resolver against a shape nothing produces.
func scrollQuery(t *testing.T) directorapi.ElementQuery {
	t.Helper()
	in := intent.New().Parse("scroll to the bottom")
	if len(in.Targets) == 0 || in.Targets[0].Query == nil {
		t.Fatalf("the parser produced no query for a scroll: %+v", in)
	}
	return *in.Targets[0].Query
}

// TestAScrollFindsTheOnlyScrollableRegion is the fixture's shape: focus is somewhere that does
// not scroll, and exactly one thing does.
func TestAScrollFindsTheOnlyScrollableRegion(t *testing.T) {
	res := target.NewResolver().Resolve(ptr(scrollWorld(t, false, 0)), scrollQuery(t))

	if res.Status != directorapi.ResolutionResolved {
		t.Fatalf("status = %s (%q); the window has exactly one scrollable region and the "+
			"request has one obvious answer", res.Status, res.Explanation)
	}
	if res.Target.Label != "Lines" {
		t.Errorf("resolved to %q, want the scrollable list", res.Target.Label)
	}
}

// TestAScrollPrefersTheRegionYouAreIn is Notepad's shape: several regions scroll and one of them
// holds focus.
func TestAScrollPrefersTheRegionYouAreIn(t *testing.T) {
	res := target.NewResolver().Resolve(ptr(scrollWorld(t, true, 2)), scrollQuery(t))

	if res.Status != directorapi.ResolutionResolved {
		t.Fatalf("status = %s (%q); the focused region scrolls and is the one meant",
			res.Status, res.Explanation)
	}
	if res.Target.Label != "Name" {
		t.Errorf("resolved to %q, want the focused region — \"scroll down\" means the one "+
			"you are in when you are in one", res.Target.Label)
	}
}

// TestSeveralScrollableRegionsAndNoneFocusedIsNotGuessed is the refusal the mandate asked for.
//
// Nothing distinguishes the candidates, so there is no answer to give and asking is the only
// honest move. Choosing the first in reading order would scroll something the person was not
// looking at.
func TestSeveralScrollableRegionsAndNoneFocusedIsNotGuessed(t *testing.T) {
	res := target.NewResolver().Resolve(ptr(scrollWorld(t, false, 2)), scrollQuery(t))

	if res.Status == directorapi.ResolutionResolved {
		t.Errorf("three scrollable regions and no focus resolved to %q; with nothing to "+
			"choose between them this must ask rather than pick",
			res.Target.Label)
	}
}

// TestANonScrollableControlIsNotAScrollTarget keeps the filter honest.
//
// The capability is what identifies the region. A window whose focus is in a text field with no
// scrolling and nothing else scrollable has no answer, and must say so rather than scrolling
// whatever was nearest.
func TestANonScrollableControlIsNotAScrollTarget(t *testing.T) {
	w := scrollWorld(t, false, 0)
	delete(w.Elements, "list")

	res := target.NewResolver().Resolve(&w, scrollQuery(t))
	if res.Status == directorapi.ResolutionResolved {
		t.Errorf("a window with nothing scrollable resolved a scroll to %q",
			res.Target.Label)
	}
}

// TestTheScrollCapabilityIsNotTheScrollItemCapability is the one-letter trap.
//
// `scroll` means this view can be moved; `scrollitem` means this control can be brought into a
// view. On one desktop snapshot 2 elements reported the first and 346 the second, so a substring
// test would make almost everything a scroll target.
func TestTheScrollCapabilityIsNotTheScrollItemCapability(t *testing.T) {
	w := scrollWorld(t, false, 0)
	w.Elements["list"].Attributes["patterns"] = "scrollitem"

	res := target.NewResolver().Resolve(&w, scrollQuery(t))
	if res.Status == directorapi.ResolutionResolved {
		t.Errorf("a control that can be scrolled INTO view was resolved as a region whose "+
			"view can be moved: %q", res.Target.Label)
	}
}

func ptr(w directorapi.WorldState) *directorapi.WorldState { return &w }
