//go:build windows

package main

// SCENARIOS: A DRAG ARRIVES AS A DRAG, FOLLOWS ITS WINDOW, REFUSES A RESIZED ONE, AND LETS GO
// WHERE IT GOT TO.
//
// # What only a desktop can say about a drag
//
// Everything about the ORDER of a drag's events is checkable headlessly and is checked there:
// `internal/oshost` drives the production gesture against a machine that records what it was
// asked for. What no unit test can supply is the application's own account of what ARRIVED —
// whether the travel between the two ends was received as movement with the button held, which is
// the whole difference between a drag and two clicks, and whether the ends landed inside the
// window they were resolved against.
//
// So the fixture logs movement while a button is down, and these four read that log.
//
// # And what is NOT asserted, deliberately
//
//	NOT EVERY POINT, AND NOT A COUNT. Windows coalesces mouse moves: a message is generated
//	from the queue's current position when somebody asks for one, so a gesture that sent
//	twenty-four moves can arrive as six, and the number depends on how busy the machine is.
//	A scenario asserting a count would be asserting the scheduler. What is required is that
//	intermediate movement ARRIVED — at least one point that is neither end — and that the
//	ends came in the right order around it.
//
//	AND NO STUCK BUTTON IS MANUFACTURED. A release the machine refuses is arranged headlessly,
//	where a machine can be told to refuse; doing it on a desktop would mean leaving a real
//	mouse button held down on somebody's computer to see whether the report says so.
//
//	AND A STOP THAT ARRIVED TOO LATE IS NOT A CANCELLATION. If the gesture had finished before
//	the stop landed, the scenario has exercised nothing and says so — NOT RUN rather than a
//	pass. The fixture's own log is what decides: the button has to have been down.
//
// Physical recording capture is not exercised by any of this and never can be: the hooks discard
// injected input, so every one of these seeds a demonstration and plays it. What they establish
// is that a drag ARRIVED as one.
//
// See [[ADR-251-a-press-checks-the-window-it-was-recorded-in]] and
// [[ADR-258-a-chord-is-a-promise-to-let-go]].

import (
	"fmt"
	"time"
)

// theDragControls are the two controls a seeded drag goes between.
//
// Far enough apart that the travel passes `simplify`'s drag threshold several times over, and
// both inside the window, so neither end is the out-of-window case (which the recorder declines
// to measure and `simplify` refuses, headlessly).
var theDragControls = [2]string{"Name", "Save"}

// aDragArrivesAsADrag is the whole gesture, by the application's own account.
func aDragArrivesAsADrag(w *world) result {
	started := time.Now()
	st, err := setUpSeeded(w, func(f *fixtureRun) (seededRoutine, error) {
		return seedDrag(f, w.art, "fixture drag", theDragControls[0],
			theDragControls[1])
	})
	if err != nil {
		return ending(started, err)
	}
	defer st.down()

	seq, records := st.f.lastSeq(), countRecords(st.store)
	if err := st.play(); err != nil {
		return ending(started, err)
	}
	rec, err := waitForRecord(60*time.Second, st.store, records)
	if err != nil {
		st.fail()
		return fail(started, err.Error())
	}
	if err := st.f.drained(30 * time.Second); err != nil {
		st.fail()
		return skip(started, err.Error())
	}
	got, err := st.f.mouse(seq)
	if err != nil {
		return fail(started, err.Error())
	}
	why := firstOf(
		theGestureDidNotArrive(got),
		theTravelWasNotReceived(got),
		anEndLandedOutside(got, st.seed.window),
		marcoSaidSomethingElse(rec, st.seed),
	)
	if why != "" {
		st.fail()
		return fail(started, why, fmt.Sprintf("the fixture received %s", mouseAre(got)),
			fmt.Sprintf("Marco recorded %s", rec))
	}
	return pass(started,
		fmt.Sprintf("the application received the drag as %d events: a press, %d "+
			"movement(s) with the button held, and a release", len(got),
			len(got)-2),
		fmt.Sprintf("it went %s", endsOf(got)),
		fmt.Sprintf("Marco's record: %s", rec))
}

// aDragFollowsItsMovedWindow is the permissive half, and both ends have to follow.
func aDragFollowsItsMovedWindow(w *world) result {
	started := time.Now()
	st, err := setUpSeeded(w, func(f *fixtureRun) (seededRoutine, error) {
		return seedDrag(f, w.art, "fixture drag", theDragControls[0],
			theDragControls[1])
	})
	if err != nil {
		return ending(started, err)
	}
	defer st.down()

	was := st.seed.window
	to := rect{X: was.X + 240, Y: was.Y + 120, W: was.W, H: was.H}
	if err := place(st.f.hwnd, to.X, to.Y, to.W, to.H); err != nil {
		return fail(started, err.Error())
	}
	moved, saw := until(10*time.Second, func() (bool, string) {
		held, err := st.f.read()
		if err != nil {
			return false, err.Error()
		}
		return held.Window == to, held.Window.String()
	})
	if !moved {
		return skip(started, fmt.Sprintf("the fixture's window would not move to %s "+
			"(it says %s), so there is no moved window to drag into", to, saw))
	}
	w.art.step("the window moved from %s to %s", was, to)

	seq, records := st.f.lastSeq(), countRecords(st.store)
	if err := st.play(); err != nil {
		return ending(started, err)
	}
	rec, err := waitForRecord(60*time.Second, st.store, records)
	if err != nil {
		st.fail()
		return fail(started, err.Error())
	}
	if err := st.f.drained(30 * time.Second); err != nil {
		st.fail()
		return skip(started, err.Error())
	}
	got, err := st.f.mouse(seq)
	if err != nil {
		return fail(started, err.Error())
	}
	why := firstOf(
		theGestureDidNotArrive(got),
		theTravelWasNotReceived(got),
		anEndLandedOutside(got, to),
		// AND IT HAPPENED IN THE MOVED WINDOW, off the events' own records rather than
		// read afterwards: a window that moved back in between would make this the
		// same assertion about a different instant.
		notInTheMovedWindowThroughout(got, to),
		marcoSaidSomethingElse(rec, st.seed),
	)
	if why != "" {
		st.fail()
		return fail(started, why, fmt.Sprintf("the fixture received %s", mouseAre(got)),
			fmt.Sprintf("Marco recorded %s", rec))
	}
	return pass(started,
		fmt.Sprintf("the window moved %+d,%+d and the drag still went %s inside it",
			to.X-was.X, to.Y-was.Y, endsOf(got)),
		fmt.Sprintf("Marco's record: %s", rec))
}

// aDragIntoAResizedWindowIsRefused is the half that matters more, and the claim is an absence:
// not one event, which includes not one mouse-DOWN.
//
// Which is the ordering this scenario exists for. The guard is checked before the gesture and
// again immediately before the button goes down — so "refused" here means the application
// received no press, and an implementation that pressed and then thought better of it would show
// up as a down with no up.
func aDragIntoAResizedWindowIsRefused(w *world) result {
	started := time.Now()
	st, err := setUpSeeded(w, func(f *fixtureRun) (seededRoutine, error) {
		return seedDrag(f, w.art, "fixture drag", theDragControls[0],
			theDragControls[1])
	})
	if err != nil {
		return ending(started, err)
	}
	defer st.down()

	was := st.seed.window
	to := rect{X: was.X, Y: was.Y, W: was.W + 90, H: was.H + 60}
	if err := place(st.f.hwnd, to.X, to.Y, to.W, to.H); err != nil {
		return fail(started, err.Error())
	}
	resized, saw := until(10*time.Second, func() (bool, string) {
		held, err := st.f.read()
		if err != nil {
			return false, err.Error()
		}
		return held.Window.W == to.W && held.Window.H == to.H, held.Window.String()
	})
	if !resized {
		return skip(started, fmt.Sprintf("the fixture's window would not change size "+
			"to %dx%d (it says %s), so there is no resized window to refuse",
			to.W, to.H, saw))
	}
	w.art.step("the window went from %dx%d to %dx%d", was.W, was.H, to.W, to.H)

	seq, records := st.f.lastSeq(), countRecords(st.store)
	if err := st.play(); err != nil {
		return ending(started, err)
	}
	rec, err := waitForRecord(60*time.Second, st.store, records)
	if err != nil {
		st.fail()
		return fail(started, err.Error())
	}
	if err := st.f.drained(30 * time.Second); err != nil {
		st.fail()
		return skip(started, err.Error())
	}
	got, err := st.f.mouse(seq)
	if err != nil {
		return fail(started, err.Error())
	}
	why := firstOf(
		anythingAtAllArrived(got, to),
		notRefused(rec),
		refusedWithoutSaying(rec),
	)
	if why != "" {
		st.fail()
		return fail(started, why, fmt.Sprintf("the fixture received %s", mouseAre(got)),
			fmt.Sprintf("Marco recorded %s", rec))
	}
	return pass(started,
		fmt.Sprintf("the window went %dx%d → %dx%d and the routine stopped as %q, "+
			"sending nothing at all — not even a press it then regretted",
			was.W, was.H, to.W, to.H, rec.Outcome),
		fmt.Sprintf("it said: %s", rec.Because))
}

// aStoppedDragLetsGoWhereItGotTo is the desktop half of the first defect this slice fixed.
//
// The old gesture broke out of its travel on a stop and then sent the landing move and the
// release anyway, so the thing being dragged was dropped at the DESTINATION rather than where the
// pointer had got to. Headless tests hold that against a machine that records; this holds it
// against the application's own account of where the button came up.
//
// AND A STOP THAT ARRIVED AFTER THE GESTURE FINISHED HAS EXERCISED NOTHING. The fixture's log is
// what decides: a release that arrived before the stop was sent is a completed drag, and this
// reports NOT RUN rather than a pass. That is the one way this scenario can be accidentally
// green, so it is the one thing it checks first.
func aStoppedDragLetsGoWhereItGotTo(w *world) result {
	started := time.Now()
	st, err := setUpSeeded(w, func(f *fixtureRun) (seededRoutine, error) {
		return seedDrag(f, w.art, "fixture drag", theDragControls[0],
			theDragControls[1])
	})
	if err != nil {
		return ending(started, err)
	}
	defer st.down()

	seq, records := st.f.lastSeq(), countRecords(st.store)
	if err := st.play(); err != nil {
		return ending(started, err)
	}
	// THE STOP GOES IN ONCE THE BUTTON IS DEMONSTRABLY DOWN, by the application's own
	// account and not by a sleep. A stop timed by a guess is the difference between this
	// scenario and a scenario about nothing.
	down, arrived := st.f.waitForMouse(45*time.Second, seq, "left_down")
	if !arrived {
		st.fail()
		return skip(started, "the application never received the drag's press, so "+
			"there was no gesture in flight to stop")
	}
	w.art.step("the button went down on %q at screen %d,%d; stopping the run",
		down.Control, down.ScreenX, down.ScreenY)
	if err := st.s.press("cancelBtn2"); err != nil {
		// A STOP THAT CANNOT BE RAISED IS NOT A FINDING ABOUT DRAGS, and this says
		// which of the two it is rather than reporting a gesture that ignored Stop.
		// `internal/stopsignal.Raise` has been seen to fail with Access is denied on
		// this platform and has never been reproduced as a test failure.
		st.fail()
		return fail(started, fmt.Sprintf("Studio's Stop could not be pressed while "+
			"the drag was in flight, so what the stop would have done with the "+
			"gesture is not established: %v", err))
	}
	rec, err := waitForRecord(60*time.Second, st.store, records)
	if err != nil {
		st.fail()
		return fail(started, err.Error())
	}
	if err := st.f.drained(30 * time.Second); err != nil {
		st.fail()
		return skip(started, err.Error())
	}
	got, err := st.f.mouse(seq)
	if err != nil {
		return fail(started, err.Error())
	}
	// ── AND WHETHER THIS EXERCISED A CANCELLATION AT ALL ──
	if why := theDragHadAlreadyFinished(got, rec); why != "" {
		return skip(started, why)
	}
	why := firstOf(
		theButtonWasNotReleased(got),
		theReleaseWasAtTheDestination(got, st.seed),
		notCancelled(rec),
	)
	if why != "" {
		st.fail()
		return fail(started, why, fmt.Sprintf("the fixture received %s", mouseAre(got)),
			fmt.Sprintf("Marco recorded %s", rec))
	}
	up := got[len(got)-1]
	return pass(started,
		fmt.Sprintf("the run was stopped with the button down and it came up at "+
			"%d,%d, which is not the destination", up.ScreenX, up.ScreenY),
		fmt.Sprintf("the gesture arrived as %s", endsOf(got)),
		fmt.Sprintf("Marco recorded %q: %s", rec.Outcome, rec.Because))
}
