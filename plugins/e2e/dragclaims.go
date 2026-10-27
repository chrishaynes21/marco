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
// AT LEAST ONE POINT THAT IS NEITHER END, AND NO COUNT. Windows coalesces mouse moves — a message
// is generated from the queue's current position when somebody asks for one — so a gesture that
// sent twenty-four moves can arrive as six, and how many depends on how busy the machine is. A
// scenario asserting a number would be asserting the scheduler, and would fail on a loaded
// machine with nothing wrong.
//
// But "a move arrived" is weaker than it reads, which is what the first cut of this got wrong: a
// replay that jumped straight to the far end and let go there sends a move of its own, at the
// destination, and would satisfy a reader that only counted them. The travel is the part BETWEEN
// the ends, so what is required is a received point that is neither of them — which no count can
// stand in for and coalescing cannot remove, because coalescing reports a position and the
// position it reports is one the pointer was at.
//
// # And what this cannot tell apart
//
// A received log with no intermediate point has two readings, and the oracle cannot separate
// them: a gesture that teleported, and an application that never got the processor between the
// press and the release and so was told the pointer's position only once, at the end. The
// sentence names both, because the second is a reason to look at the machine and not at Marco.
func theTravelWasNotReceived(got []fixtureEvent) string {
	if len(got) < 2 {
		return fmt.Sprintf("the application received %d event(s), which is not enough "+
			"to be a gesture at all, let alone one that travelled", len(got))
	}
	ends := [2]fixtureEvent{got[0], got[len(got)-1]}
	moves, between := 0, 0
	for _, e := range got {
		if e.What != "move" {
			continue
		}
		moves++
		if !samePoint(e, ends[0]) && !samePoint(e, ends[1]) {
			between++
		}
	}
	if between > 0 {
		return ""
	}
	if moves == 0 {
		return fmt.Sprintf("the application received %d event(s) and not one of them "+
			"was movement with the button held — so what arrived is a press and a "+
			"release at two places, which is what a teleport looks like and not "+
			"what a drag looks like", len(got))
	}
	return fmt.Sprintf("the application received %d movement(s) with the button held and "+
		"every one of them was at an end of the gesture — the press at %d,%d or the "+
		"release at %d,%d — so no point BETWEEN the ends arrived. A jump to the far "+
		"end and a release there is a teleport with a move event in it, which is what "+
		"this has to tell apart from a drag; the other reading is an application that "+
		"never ran during the travel and was told the pointer's position once, at the "+
		"end, and the log cannot separate the two", moves, ends[0].ScreenX,
		ends[0].ScreenY, ends[1].ScreenX, ends[1].ScreenY)
}

// samePoint is two events at one place on screen. Screen coordinates and not client ones: the
// window can move between two events, and a point in a window that moved is a different place.
func samePoint(a, b fixtureEvent) bool {
	return a.ScreenX == b.ScreenX && a.ScreenY == b.ScreenY
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

// stopBracket is the Stop request's place in the fixture's OWN log, and the whole of how this
// harness orders a stop against a release.
//
// Two readings of the fixture's sequence counter, one taken immediately before Stop is pressed
// and one as soon as the press has returned. They bound the request from both sides without
// either clock having to agree with the other: an event numbered at or below `before` was logged
// while the stop had not yet been asked for, and an event above `after` was logged once it had.
// Everything in between arrived during the request, and nothing here can say which came first.
//
// THE RUN RECORD CANNOT DO THIS JOB, which is what the first cut of this used it for. An outcome
// word says what Marco concluded, not when the application was told anything: a stop Marco
// ignored finishes as `performed` and would be excused as "too late", and a stop that landed
// after a completed gesture still records `cancelled` because the run was stopped between steps,
// which would convict a drag nothing interrupted of dropping at its destination. One of those is
// a finding reported as NOT RUN and the other is a false failure, and both come from reading a
// verdict as a timestamp.
type stopBracket struct {
	// before is the last event the fixture had logged when Stop had not yet been pressed.
	before int
	// after is the last it had logged once the press had been made.
	after int
}

func (b stopBracket) String() string {
	return fmt.Sprintf("the log stood at #%d when Stop was requested and #%d once it had "+
		"been", b.before, b.after)
}

// theDragHadAlreadyFinished is whether the stopped-drag scenario exercised a cancellation at all.
//
// A STOP THAT ARRIVED AFTER THE RELEASE HAS EXERCISED NOTHING, and the answer is NOT RUN rather
// than a pass: the gesture completed, and every assertion about where the button came up would be
// true of a drag nothing interrupted. This is the one way this scenario can be accidentally
// green, so it is asked before anything else.
//
// Decided by ORDER and not by inference: the press has to be in the log before Stop was asked
// for, so the button was demonstrably down when the request went in, and the release has to be
// logged after the request was made. An ordering this cannot settle is NOT RUN with the ambiguity
// named — not a pass and not a failure, because what this scenario is about is unestablished.
//
// A log with no release at all returns "" on purpose. That is not an unexercised scenario, it is
// a held button on somebody's desktop, and `theButtonWasNotReleased` reports it as the finding it
// is.
func theDragHadAlreadyFinished(got []fixtureEvent, br stopBracket) string {
	if br.after < br.before {
		return fmt.Sprintf("the fixture's log could not be read around the stop (%s), "+
			"so nothing orders the stop against what the application received",
			br)
	}
	var down, up *fixtureEvent
	for i := range got {
		switch got[i].What {
		case "left_down":
			if down == nil {
				down = &got[i]
			}
		case "left_up":
			up = &got[i]
		}
	}
	if down == nil {
		return fmt.Sprintf("the application received %d event(s) and no press, so "+
			"there was no gesture in flight when the stop went in", len(got))
	}
	if down.Seq > br.before {
		return fmt.Sprintf("the press was logged as #%d and %s, so the button was not "+
			"demonstrably down when the stop was asked for — what the stop "+
			"interrupted is not established", down.Seq, br)
	}
	if up == nil {
		return ""
	}
	if up.Seq <= br.before {
		return fmt.Sprintf("the release was already logged as #%d and %s, so the "+
			"gesture was over before the stop was asked for and no cancellation "+
			"was exercised — everything about where the button came up is true "+
			"of a drag nothing interrupted", up.Seq, br)
	}
	if up.Seq <= br.after {
		return fmt.Sprintf("the release is #%d and %s, so it arrived while the stop "+
			"was being requested and nothing here orders the two. This run cannot "+
			"say whether the cancellation was exercised, and a verdict either way "+
			"would be a guess about which of two things happened first", up.Seq,
			br)
	}
	return ""
}

// theTravelNeverStarted is the other way this scenario can be about nothing, and it is a question
// of SCOPE rather than of timing.
//
// Kept apart from the ordering above because the two were one check and should not have been: a
// gesture that received no movement is a genuine cancellation of a gesture in flight — the button
// was down — but it is not an exercise of "it let go WHERE IT GOT TO", because the pointer never
// got anywhere. So NOT RUN, and the sentence says which of the two it is.
func theTravelNeverStarted(got []fixtureEvent) string {
	for _, e := range got {
		if e.What == "move" {
			return ""
		}
	}
	return fmt.Sprintf("the application received %d event(s) and no movement at all, so "+
		"the stop landed between the press and the first step of the travel. The "+
		"button was down and the cancellation is real, but the pointer never got "+
		"anywhere — so where it let go is not a claim this run can exercise",
		len(got))
}

// theStopWasIgnored is the finding the ordering evidence makes possible.
//
// With the release logged after the stop was asked for, an outcome that is not `cancelled` means
// a stop arrived while a gesture was in flight and the run finished the gesture anyway. That is a
// finding about Marco and not a reason to excuse the scenario — and the first cut of this read it
// as "the stop was too late" and reported NOT RUN, which is the shape of a defect hiding behind
// its own excuse.
//
// Its own sentence rather than `notCancelled`'s, which says nothing arrived afterwards: here
// something did, and it was the rest of a drag.
func theStopWasIgnored(rec runRecord) string {
	if rec.Outcome == "cancelled" {
		return ""
	}
	return fmt.Sprintf("Stop was pressed while the drag's button was demonstrably down "+
		"and the record says %q (%s) — so a run was told to stop with a gesture in "+
		"flight and carried on through it", rec.Outcome, rec.Because)
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

// theReleaseWasNotWhereItGotTo is the defect this scenario is the desktop half of, and it takes
// two claims to say it.
//
// The old gesture broke out of its travel on a stop and then sent the landing move and the
// release anyway, so the thing being dragged was dropped where the routine was GOING rather than
// where the pointer had got to. A stop that still delivers the gesture's whole point is not a
// stop.
//
// "ANYWHERE BUT THE DESTINATION" IS NOT THE CLAIM, which is what the first cut asserted. It
// passes a release at a point the pointer never visited — a corner, the origin, whatever a
// mistaken implementation aimed at — and "it let go somewhere else" is not "it let go where it
// got to". So the release also has to be ON THE TRAVEL: inside the two demonstrated ends and on
// the line between them, which is every point this gesture ever sends and no point it does not.
//
// Both halves are asserted against what was DEMONSTRATED, which the harness knows because it
// seeded it, and not against the last movement received: a release always follows the last move
// wherever either of them is, so reading the log against itself would make this a tautology about
// two adjacent records.
//
// # And what this cannot tell apart
//
// WHICH point of the travel the pointer had reached when the stop landed. Windows coalesces
// movement, so the application's own log does not name the pointer's last position, and the
// instant a stop arrives is not observable from outside the run at all. So the claim is narrowed
// to what the evidence carries — a point on the travel, short of the destination — and the
// passing sentence says that and not more.
func theReleaseWasNotWhereItGotTo(got []fixtureEvent, seed seededRoutine) string {
	if len(got) == 0 || len(seed.at) < 2 {
		return ""
	}
	up := got[len(got)-1]
	if up.What != "left_up" {
		return "" // a gesture with no release: see theButtonWasNotReleased
	}
	// The ends as they stand: this scenario does not move the window, so the demonstrated
	// screen points are where the gesture starts and where a completed drag would end.
	fx, fy := seed.at[0][0], seed.at[0][1]
	dx, dy := seed.at[1][0], seed.at[1][1]
	if up.ScreenX == dx && up.ScreenY == dy {
		return fmt.Sprintf("the run was stopped mid-gesture and the button came up at "+
			"%d,%d — the destination the drag was heading for. A stop has to let "+
			"go where the pointer IS: a release at the destination is a drop, and "+
			"completing the gesture's whole point is not stopping it", dx, dy)
	}
	if !onTheTravel(up.ScreenX, up.ScreenY, fx, fy, dx, dy) {
		return fmt.Sprintf("the button came up at %d,%d, which is not a point on the "+
			"travel from %d,%d to %d,%d. It is not the destination, so this is "+
			"not the drop — but a release has to land where the pointer GOT TO, "+
			"and a point the gesture never passes through is not a place it ever "+
			"reached", up.ScreenX, up.ScreenY, fx, fy, dx, dy)
	}
	return ""
}

// onTheTravel is whether a point is one the gesture passes through.
//
// Inside the box the two ends make, and on the line between them to within a pixel per axis —
// which is the band the gesture's own arithmetic produces, not a tolerance chosen to make
// something pass. It steps `x1 + int((x2-x1)*t)`, so each coordinate is truncated by less than
// one, and the cross product of a point off the exact line by less than one pixel in each axis is
// bounded by the span in the other. Widening that band would start accepting points beside the
// path, which is the thing this exists to refuse.
func onTheTravel(x, y, x1, y1, x2, y2 int) bool {
	if x < min(x1, x2) || x > max(x1, x2) || y < min(y1, y2) || y > max(y1, y2) {
		return false
	}
	cross := (x-x1)*(y2-y1) - (y-y1)*(x2-x1)
	if cross < 0 {
		cross = -cross
	}
	return cross <= abs(x2-x1)+abs(y2-y1)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
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
