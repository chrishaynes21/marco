package main

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// A brief screen is explained, not deleted.
//
// The owner's rule:
//
//	A short-lived screen could be a loading state, a genuine transient menu, or a false
//	segmentation. The system should distinguish those outcomes rather than automatically
//	deleting brief states. […] Preserve unknown when evidence is insufficient.
//
// There is no duration threshold anywhere in this, and that is the design: "brief" is the
// symptom, not the cause. A loading screen is brief, so is a menu somebody passed through
// quickly, so is a segmentation artefact. What separates them is what HAPPENED on them.
//
// Deleting any branch of the classification must fail this.
func TestABriefScreenIsExplainedRatherThanDeleted(t *testing.T) {
	r := frameResult{
		VisualSeen: []observe.VisualScreenSeen{
			{ID: "s_place", Looks: 400},
			{ID: "s_load", Looks: 9},
			{ID: "s_flash", Looks: 2},
			{ID: "s_dialog", Looks: 8},
			{ID: "s_dead", Looks: 30},
		},
		Clicks: observe.ClickEvidence{
			Attributed: []observe.ClickTransition{
				{From: "s_place", Target: "t1", To: "s_load", Count: 1},
				{From: "s_dialog", Target: "t2", To: "s_place", Count: 1},
			},
			// A PRESS THAT CHANGED NOTHING still happened on a screen, and it still
			// makes that screen somewhere a person acted. A classification that
			// counted only the presses followed by a change would call this one
			// unexplained — which is the opposite of what the evidence says.
			Inert: []observe.ClickObservation{
				{Look: 500, Screen: "s_dead", Target: "t9",
					Candidates: observe.Proposed([]observe.Region{
						{X: 0.1, Y: 0.1, Width: 0.2, Height: 0.05}}, 0, 0, 0)},
			},
			Steps: []observe.RouteStep{
				// Clicked out of the place, into the loading screen.
				{From: "s_place", To: "s_load", Cause: observe.CauseClick,
					Target: "t1", Count: 1},
				// The loading screen left by itself.
				{From: "s_load", To: "s_dialog", Cause: observe.CauseAutomatic,
					Count: 1},
				// So the dialog was ARRIVED AT automatically, and clicked out of.
				{From: "s_dialog", To: "s_place", Cause: observe.CauseClick,
					Target: "t2", Count: 1},
				// And the flash came and went with nothing to explain it.
				{From: "s_place", To: "s_flash", Cause: observe.CauseUnknown, Count: 1},
				{From: "s_flash", To: "s_place", Cause: observe.CauseUnknown, Count: 1},
			},
		},
	}
	got := map[observe.VisualScreenID]screenVerdict{}
	for _, v := range classifyScreens(r) {
		got[v.ID] = v
	}
	if len(got) != 5 {
		t.Fatalf("%d screen(s) classified of 5; nothing may be dropped for being brief",
			len(got))
	}
	for id, want := range map[observe.VisualScreenID]screenRole{
		"s_place":  rolePlace,
		"s_load":   rolePassage,
		"s_flash":  roleUnexplained,
		"s_dialog": rolePlace,
		// Pressed on, and the press changed nothing. Still a place.
		"s_dead": rolePlace,
	} {
		if got[id].Role != want {
			t.Errorf("%s was classified %q, want %q (%s)",
				id, got[id].Role, want, got[id].Because)
		}
	}
	// THE 8-LOOK DIALOG AND THE 400-LOOK MENU ARE BOTH PLACES, which is the point: the
	// 2-look flash is unexplained for a reason that has nothing to do with being short.
	if got["s_dialog"].Looks >= got["s_place"].Looks {
		t.Fatal("the fixture does not make the dialog the briefer of the two places")
	}
	// And the dialog says HOW it was reached, because a route through one is a wait and
	// then a click, which is two steps in a particular order.
	if !contains(got["s_dialog"].Because, "appeared by itself") {
		t.Errorf("the dialog's explanation is %q and does not say it was arrived at "+
			"automatically", got["s_dialog"].Because)
	}
	// The unexplained one must not be called an artefact. The evidence is insufficient,
	// and saying more than that is the thing this category exists to prevent.
	if !contains(got["s_flash"].Because, "nothing explains") {
		t.Errorf("the unexplained screen's reason is %q", got["s_flash"].Because)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

// The longest repeated run is a contiguous, NON-OVERLAPPING one.
//
// Contiguous because a route is an order, and an edit distance would score two walks that visit
// the same screens in a different order as nearly identical — which is exactly the mistake a
// route must not make. Non-overlapping because an alternating walk would otherwise report its
// own tail as a repetition of its head.
func TestTheLongestRepeatedRunIsContiguousAndDoesNotOverlapItself(t *testing.T) {
	walk := []observe.VisualScreenID{"a", "b", "c", "d", "a", "x", "a", "b", "c", "d", "a"}
	run, at := longestRepeatedRun(walk)
	if len(run) != 5 {
		t.Fatalf("the longest repeated run is %d screen(s) (%v), want 5", len(run), run)
	}
	if at[0] != 0 || at[1] != 6 {
		t.Errorf("the two occurrences start at %v, want [0 6]", at)
	}
	// AN ALTERNATION IS NOT A LONG REPETITION OF ITSELF. `a b a b a` matches itself at
	// every offset, and a rule that allowed the occurrences to overlap would report four
	// — `a b a b` against itself shifted by two. The answer is TWO: `a b` at 0 and at 2,
	// which are the longest two runs that do not share an index.
	alt := []observe.VisualScreenID{"a", "b", "a", "b", "a"}
	if r, at := longestRepeatedRun(alt); len(r) != 2 || at[0] != 0 || at[1] != 2 {
		t.Errorf("a b a b a reports a repeated run of %d at %v (%v); want 2 at [0 2], "+
			"because the two occurrences may not overlap", len(r), at, r)
	}
	if r, _ := longestRepeatedRun([]observe.VisualScreenID{"a", "b", "c"}); len(r) != 0 {
		t.Errorf("a walk with no repetition reports a run of %d", len(r))
	}
	if r, _ := longestRepeatedRun(nil); r != nil {
		t.Errorf("an empty walk reports %v", r)
	}
}

// Dropping transients collapses the walk without inventing a step.
//
// "After accounting for transient states" is the owner's phrase, and the trap is the obvious
// implementation: removing a screen from the middle of `A T A` leaves `A A`, which is not a
// step and would make every collapsed walk report a repetition it never had.
//
// Deleting the consecutive-duplicate guard must fail this.
func TestDroppingTransientsDoesNotInventAStep(t *testing.T) {
	verdicts := []screenVerdict{
		{ID: "a", Role: rolePlace},
		{ID: "b", Role: rolePlace},
		{ID: "t", Role: rolePassage},
		{ID: "u", Role: roleUnexplained},
	}
	drop := map[screenRole]bool{rolePassage: true, roleUnexplained: true}

	// A passage between two looks at one screen must not leave `a a`.
	got := withoutRoles([]observe.VisualScreenID{"a", "t", "a"}, verdicts, drop)
	if len(got) != 1 || got[0] != "a" {
		t.Errorf("a -> t -> a collapsed to %v, want [a]: a screen cannot follow itself", got)
	}
	// A passage BETWEEN two different screens leaves a real step.
	got = withoutRoles([]observe.VisualScreenID{"a", "t", "b"}, verdicts, drop)
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("a -> t -> b collapsed to %v, want [a b]", got)
	}
	// And both kinds are dropped, while places are kept whatever their duration.
	got = withoutRoles([]observe.VisualScreenID{"a", "u", "b", "t", "a"}, verdicts, drop)
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "a" {
		t.Errorf("collapsed to %v, want [a b a]", got)
	}
}

// Two identities are compared on GEOMETRY, and reaching the same place is not part of it.
//
//	Do not merge targets solely because they reach the same destination.
//
// Two different entries of a menu can open the same thing — a "back" on every page proves it —
// so the destination is how a pair is NOTICED and never how it is judged.
//
// Deleting the overlap test or the area test must fail this.
func TestAmbiguousTargetsAreComparedOnGeometryNotOnDestination(t *testing.T) {
	r := frameResult{
		Targets: []observe.TargetSeen{
			{ID: "near_a", Where: []observe.Region{
				{X: 0.10, Y: 0.40, Width: 0.20, Height: 0.05}}},
			{ID: "near_b", Where: []observe.Region{
				{X: 0.11, Y: 0.405, Width: 0.19, Height: 0.052}}},
			{ID: "far", Where: []observe.Region{
				{X: 0.70, Y: 0.80, Width: 0.20, Height: 0.05}}},
			{ID: "panel", Where: []observe.Region{
				{X: 0.05, Y: 0.30, Width: 0.60, Height: 0.40}}},
		},
		Clicks: observe.ClickEvidence{MaybeOneControl: []observe.TargetPair{
			{Screen: "s", A: "near_a", B: "near_b", To: "d"},
			{Screen: "s", A: "near_a", B: "far", To: "d"},
			{Screen: "s", A: "near_a", B: "panel", To: "d"},
		}},
	}
	got := compareTargets(r)
	if len(got) != 3 {
		t.Fatalf("%d comparison(s), want 3", len(got))
	}
	// Two readings of one control: overlapping, comparable, and the verdict says the
	// evidence cannot settle it rather than settling it.
	if !got[0].Overlap || got[0].AreaRatio > 4 {
		t.Errorf("two near-identical rectangles compared as %+v", got[0])
	}
	if !contains(got[0].Verdict, "cannot settle") {
		t.Errorf("the verdict for two near-identical rectangles is %q; it must not "+
			"decide", got[0].Verdict)
	}
	// Two controls that merely lead to the same place: not overlapping, and said so.
	if got[1].Overlap {
		t.Errorf("rectangles at opposite ends of the screen overlap: %+v", got[1])
	}
	if !contains(got[1].Verdict, "TWO CONTROLS") {
		t.Errorf("the verdict for two distant rectangles is %q", got[1].Verdict)
	}
	// A control and the thing containing it: overlapping and wildly different in size.
	if !got[2].Overlap || got[2].AreaRatio <= 4 {
		t.Errorf("a row and the panel containing it compared as %+v", got[2])
	}
	if !contains(got[2].Verdict, "much larger") {
		t.Errorf("the verdict for a row inside a panel is %q", got[2].Verdict)
	}
	// AND THE CENTRE DISTANCE IS A DISTANCE. The x-centre is x+width/2 and the y-centre
	// is y+height/2; using height for both silently reports the wrong number and the
	// verdicts above would still pass, because they key on area.
	// WIDTH AND HEIGHT MUST DIFFER in this fixture, or the bug is invisible: with square
	// rectangles, x+width/2 and x+height/2 are the same number and the wrong one is
	// right by accident.
	wide := frameResult{
		Targets: []observe.TargetSeen{
			{ID: "l", Where: []observe.Region{{X: 0.0, Y: 0.5, Width: 0.2, Height: 0.05}}},
			{ID: "r", Where: []observe.Region{{X: 0.8, Y: 0.5, Width: 0.2, Height: 0.05}}},
		},
		Clicks: observe.ClickEvidence{MaybeOneControl: []observe.TargetPair{
			{Screen: "s", A: "l", B: "r", To: "d"},
		}},
	}
	// Centres at 0.1 and 0.9, so 0.8 apart. With the wrong term it reads 0.725.
	if d := compareTargets(wide)[0].Centres; d < 0.79 || d > 0.81 {
		t.Errorf("two rectangles whose centres are 0.8 apart horizontally measured "+
			"%.3f apart", d)
	}
}
