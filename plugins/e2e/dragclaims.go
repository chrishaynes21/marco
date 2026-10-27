//go:build windows

package main

// WHAT A DRAG'S SCENARIOS DISAGREE ABOUT.
//
// Split from `scenario_drag.go` the way `scenario_geometry.go` keeps its own: a scenario reads as
// an arrangement and a verdict, and the sentences a failure carries are long enough to bury it.
//
// Every one of these returns "" for agreement and a sentence for disagreement, so a scenario
// reads as a list of the ways its claim can be untrue. `firstOf` reports the first.

import (
	"fmt"
	"strings"
)

// theGestureDidNotArrive is the shape: a press first, a release last, movement between.
func theGestureDidNotArrive(got []fixtureEvent) string {
	if len(got) == 0 {
		return "the application received nothing at all, so the drag did not happen"
	}
	if first := got[0]; first.What != "left_down" {
		return fmt.Sprintf("the first thing the application received was %q, and a "+
			"drag begins with its button going down", first.What)
	}
	if last := got[len(got)-1]; last.What != "left_up" {
		return fmt.Sprintf("the last thing the application received was %q, so the "+
			"button this routine pressed is still down as far as this window "+
			"knows", last.What)
	}
	// AND EVERYTHING BETWEEN THEM IS MOVEMENT, which an assertion about the two ends
	// would not catch: two drags in a row satisfy both of the checks above.
	for i, e := range got[1 : len(got)-1] {
		if e.What != "move" {
			return fmt.Sprintf("event %d of the gesture is %q, and everything "+
				"between the press and the release has to be movement — "+
				"anything else means this is more than one gesture", i+2,
				e.What)
		}
	}
	return ""
}

// theTravelWasNotReceived is the claim that makes it a drag rather than two clicks.
//
// AT LEAST ONE INTERMEDIATE POINT, AND NO COUNT. Windows coalesces mouse moves — a message is
// generated from the queue's current position when somebody asks for one — so a gesture that sent
// twenty-four moves can arrive as six, and how many depends on how busy the machine is. A
// scenario asserting a number would be asserting the scheduler, and would fail on a loaded
// machine with nothing wrong.
//
// What coalescing cannot remove is the EXISTENCE of movement: the pointer travelled with the
// button held and the application was told. A replay that teleported between the ends would
// arrive as a press and a release at two places, which is two clicks.
func theTravelWasNotReceived(got []fixtureEvent) string {
	moves := 0
	for _, e := range got {
		if e.What == "move" {
			moves++
		}
	}
	if moves > 0 {
		return ""
	}
	return fmt.Sprintf("the application received %d event(s) and not one of them was "+
		"movement with the button held — so what arrived is a press and a release "+
		"at two places, which is what a teleport looks like and not what a drag "+
		"looks like", len(got))
}

// anEndLandedOutside holds every point of the gesture inside the window it was resolved against.
//
// Every point and not just the ends: a path that left the window and came back would have dragged
// across whatever it crossed, and the fixture's record of each point is the only account of that.
func anEndLandedOutside(got []fixtureEvent, in rect) string {
	for _, e := range got {
		if e.ScreenX < in.X || e.ScreenY < in.Y ||
			e.ScreenX >= in.X+in.W || e.ScreenY >= in.Y+in.H {
			return fmt.Sprintf("%s arrived at screen %d,%d, which is outside the "+
				"%s window it was resolved against — so part of this gesture "+
				"was dragging across whatever is beside it", e.What, e.ScreenX,
				e.ScreenY, in)
		}
	}
	return ""
}

// notInTheMovedWindowThroughout holds the whole gesture against the window as it was MOVED to.
//
// Off each event's own stamped geometry rather than read afterwards, for the reason the press
// scenario gives: a window that moved back between the gesture and the reading would make this
// the same assertion about a different instant.
func notInTheMovedWindowThroughout(got []fixtureEvent, to rect) string {
	for _, e := range got {
		if e.Window != to {
			return fmt.Sprintf("%s arrived while the window was %s, and it was "+
				"moved to %s — so this is not a gesture into the moved "+
				"window", e.What, e.Window, to)
		}
	}
	return ""
}

// anythingAtAllArrived is the refusal, and it is stricter than `anythingWasPressed`.
//
// NOT ONE EVENT, including not one mouse-DOWN. The guard is read before the gesture and again
// immediately before the button goes down, so an implementation that pressed and then thought
// better of it would leave a down with no up — a button held on somebody's desktop by a routine
// that reported refusing. That is what this is watching for, and a reader counting only presses
// would call it a pass.
func anythingAtAllArrived(got []fixtureEvent, to rect) string {
	if len(got) == 0 {
		return ""
	}
	return fmt.Sprintf("the window was a different size (%dx%d) from the one the drag was "+
		"demonstrated in, and the application received %d event(s) anyway, the first "+
		"being %s. A guarded gesture into a window that is not the one it was shown "+
		"is the hazard the guard exists for, and it would look like a successful run",
		to.W, to.H, len(got), got[0])
}

// theDragHadAlreadyFinished is whether the stopped-drag scenario exercised a cancellation at all.
//
// A STOP THAT ARRIVED AFTER THE RELEASE HAS EXERCISED NOTHING, and the answer is NOT RUN rather
// than a pass: the gesture completed, the record may still say `cancelled` because the run was
// stopped between steps, and every assertion about where the button came up would be true of a
// drag nothing interrupted. This is the one way this scenario can be accidentally green, so it is
// asked before anything else.
//
// From the record's outcome and the fixture's log together, because neither settles it alone: a
// run that reports `performed` was not stopped in time, and a gesture that received no movement
// was stopped before the travel began — which is a different scenario about nothing.
func theDragHadAlreadyFinished(got []fixtureEvent, rec runRecord) string {
	if rec.Outcome == "performed" {
		return fmt.Sprintf("the run finished as %q, so the stop arrived after the "+
			"gesture was over and no cancellation was exercised", rec.Outcome)
	}
	moves := 0
	for _, e := range got {
		if e.What == "move" {
			moves++
		}
	}
	if moves == 0 {
		return fmt.Sprintf("the application received %d event(s) and no movement, so "+
			"the stop landed before the travel began — there was no gesture in "+
			"flight to interrupt", len(got))
	}
	return ""
}

// theButtonWasNotReleased is the promise, on the application's own account.
func theButtonWasNotReleased(got []fixtureEvent) string {
	for _, e := range got {
		if e.What == "left_up" {
			return ""
		}
	}
	return fmt.Sprintf("the run was stopped with the button down and the application "+
		"never received a release, so as far as this window knows the left button "+
		"is still held — and a held left button drags whatever the pointer passes "+
		"over. %d event(s) arrived", len(got))
}

// theReleaseWasAtTheDestination is the defect this scenario is the desktop half of.
//
// The old gesture broke out of its travel on a stop and then sent the landing move and the
// release anyway, so the thing being dragged was dropped where the routine was GOING rather than
// where the pointer had got to. A stop that still delivers the gesture's whole point is not a
// stop.
//
// Asserted against the DEMONSTRATED destination, which the harness knows because it seeded it —
// not against the last movement received, which would make this a tautology about two adjacent
// records in the same log.
func theReleaseWasAtTheDestination(got []fixtureEvent, seed seededRoutine) string {
	if len(got) == 0 || len(seed.at) < 2 {
		return ""
	}
	up := got[len(got)-1]
	if up.What != "left_up" {
		return ""
	}
	// The destination as it stands: this scenario does not move the window, so the
	// demonstrated screen point is where a completed drag would have ended.
	dx, dy := seed.at[1][0], seed.at[1][1]
	if up.ScreenX != dx || up.ScreenY != dy {
		return ""
	}
	return fmt.Sprintf("the run was stopped mid-gesture and the button came up at "+
		"%d,%d — the destination the drag was heading for. A stop has to let go "+
		"where the pointer IS: a release at the destination is a drop, and "+
		"completing the gesture's whole point is not stopping it", dx, dy)
}

// mouseAre is what a failure carries: every mouse event, in order, one per line.
func mouseAre(got []fixtureEvent) string {
	if len(got) == 0 {
		return "nothing"
	}
	var parts []string
	for _, e := range got {
		parts = append(parts, e.String())
	}
	return fmt.Sprintf("%d event(s):\n  %s", len(got), strings.Join(parts, "\n  "))
}

// endsOf is the gesture in one phrase, for a passing line.
func endsOf(got []fixtureEvent) string {
	if len(got) < 2 {
		return "nothing"
	}
	a, b := got[0], got[len(got)-1]
	return fmt.Sprintf("%d,%d → %d,%d", a.ScreenX, a.ScreenY, b.ScreenX, b.ScreenY)
}
