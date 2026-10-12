package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// A change with no press, out of a screen Marco CLICKED its way into, is automatic.
//
// # The shape Rocket League forced
//
//	Menu A -> user clicks a visible target -> loading -> destination arrives with no further
//	input
//
// The right response to that is to wait and verify. A planner that assumed every edge needs an
// action would try to click its way through a loading screen, and would be wrong about ordinary
// applications and not just games.
//
// Deleting the arrival test must make this unknown, and deleting the automatic branch must make
// it unknown too.
func TestAChangeAfterArrivingByClickIsAutomatic(t *testing.T) {
	var c observe.ClickTargets
	row := []observe.Region{{X: 0.1, Y: 0.4, Width: 0.3, Height: 0.05}}
	// Click into B...
	c.Press(10, observe.PointerAt{X: 0.2, Y: 0.42}, "visual_a", row)
	c.Arrived(16, observe.VisualTransition{From: "visual_a", To: "visual_b"})
	// ...and B becomes C by itself.
	c.Arrived(24, observe.VisualTransition{From: "visual_b", To: "visual_c"})
	c.Close()

	steps := observe.ClickEvidence(c.Evidence()).Steps
	if len(steps) != 2 {
		t.Fatalf("%d step(s), want 2: %+v", len(steps), steps)
	}
	var click, auto *observe.RouteStep
	for i := range steps {
		switch steps[i].Cause {
		case observe.CauseClick:
			click = &steps[i]
		case observe.CauseAutomatic:
			auto = &steps[i]
		}
	}
	if click == nil {
		t.Fatalf("no click-caused step: %+v", steps)
	}
	if auto == nil {
		t.Fatalf("the change out of a screen Marco clicked into is not automatic: %+v",
			steps)
	}
	if auto.From != "visual_b" || auto.To != "visual_c" {
		t.Errorf("the automatic step reads %s -> %s", auto.From, auto.To)
	}
	// AND IT NAMES NO TARGET. A step with a target is an invitation to press something,
	// and nobody pressed anything here.
	if auto.Target != "" {
		t.Errorf("an automatic step names target %q. Nothing was pressed, and naming "+
			"one would invite a planner to press it", auto.Target)
	}
	if auto.Cause.Executable() {
		t.Error("an automatic cause reports itself as executable")
	}
	if !click.Cause.Executable() {
		t.Error("a click cause reports itself as not executable")
	}
	if len(auto.Gaps) != 1 || auto.Gaps[0] != 8 {
		t.Errorf("the automatic step's gap after arriving is %v, want [8]", auto.Gaps)
	}
	if auto.Because == "" {
		t.Error("the automatic step does not say why it was called automatic")
	}

	// AND THE ARRIVAL ONLY EXPLAINS THE SCREEN IT ARRIVED AT. A change out of somewhere
	// else, however soon after, is not this load completing — and an arrival that
	// explained every subsequent change would make almost nothing unknown.
	var d observe.ClickTargets
	d.Press(10, observe.PointerAt{X: 0.2, Y: 0.42}, "visual_a", row)
	d.Arrived(16, observe.VisualTransition{From: "visual_a", To: "visual_b"})
	d.Arrived(20, observe.VisualTransition{From: "visual_x", To: "visual_y"})
	d.Close()
	for _, st := range d.Evidence().Steps {
		if st.From == "visual_x" && st.Cause != observe.CauseUnknown {
			t.Errorf("a change out of visual_x was called %q because Marco had just "+
				"arrived at visual_b. An arrival explains the screen it "+
				"arrived AT", st.Cause)
		}
	}
}

// A change out of a screen Marco did NOT click its way into has an unknown cause.
//
// An animation, a splash, a flash of mid-transition geometry and a real transient menu nobody
// clicked all look the same from here. Calling them automatic would claim a mechanism; calling
// them nothing would lose them.
//
//	Marco should be able to learn that something happened without pretending to know why.
func TestAChangeWithNoArrivalBehindItHasAnUnknownCause(t *testing.T) {
	var c observe.ClickTargets
	c.Arrived(10, observe.VisualTransition{From: "visual_a", To: "visual_b"})
	c.Close()

	steps := c.Evidence().Steps
	if len(steps) != 1 {
		t.Fatalf("%d step(s), want 1", len(steps))
	}
	if steps[0].Cause != observe.CauseUnknown {
		t.Errorf("a change with nothing behind it was called %q", steps[0].Cause)
	}
	if steps[0].Cause.Executable() {
		t.Error("an unknown cause reports itself as executable. Nothing may be run on " +
			"evidence that cannot say what happened")
	}
	if steps[0].Target != "" {
		t.Errorf("an unknown step names target %q", steps[0].Target)
	}
}

// An arrival too long ago does not make the next change automatic.
//
// The bound is the same one a press gets, because it is the same question one step along: how
// long a consequence may take before it stops being a consequence.
func TestAnArrivalTooLongAgoDoesNotExplainTheNextChange(t *testing.T) {
	var c observe.ClickTargets
	row := []observe.Region{{X: 0.1, Y: 0.4, Width: 0.3, Height: 0.05}}
	c.Press(10, observe.PointerAt{X: 0.2, Y: 0.42}, "visual_a", row)
	c.Arrived(16, observe.VisualTransition{From: "visual_a", To: "visual_b"})
	c.Arrived(17+observe.MaxArrivalToAutomaticLooks,
		observe.VisualTransition{From: "visual_b", To: "visual_c"})
	c.Close()

	for _, st := range c.Evidence().Steps {
		if st.From == "visual_b" && st.Cause != observe.CauseUnknown {
			t.Errorf("a change %d looks after arriving was called %q",
				observe.MaxArrivalToAutomaticLooks+1, st.Cause)
		}
	}
}

// The session records the ORDER it settled on screens in.
//
// A route that repeats is a repeated run in that order, and no collection of transitions can
// reconstruct it: counts lose the order and two pairs' occurrences interleave.
//
// Deleting the walk append must fail this.
func TestTheSessionRecordsTheOrderItSettledIn(t *testing.T) {
	var v observe.VisualScreens
	a, b, c := rects(0.1, 6), rects(0.8, 3), rects(0.45, 9)
	settle := func(regions []observe.Region) {
		for range observe.VisualSettleLooks {
			look(t, &v, regions)
		}
	}
	settle(a)
	settle(b)
	settle(a)
	settle(c)

	walk := v.Walk()
	if len(walk) != 4 {
		t.Fatalf("the walk has %d entr(ies) over four settles: %v", len(walk), walk)
	}
	// THE FIRST SETTLE IS IN IT. A route begins somewhere, and a sequence that started at
	// the first CHANGE would be missing the screen the person started on.
	if walk[0] != walk[2] {
		t.Errorf("the walk is %v; returning to the first screen must repeat its id, "+
			"and the first settle must be present", walk)
	}
	if walk[1] == walk[0] || walk[3] == walk[0] || walk[1] == walk[3] {
		t.Errorf("the walk is %v; three arrangements are three screens", walk)
	}
}
