//go:build windows

package main

// THE DRAG SCENARIOS, BEFORE ANY OF THEM HAS RUN.
//
// Four scenarios written against a desktop nobody has pointed them at yet. What can be checked
// without one is the arrangement: that they are gated out of an ordinary run, that they are
// filed where a flag typed for something else will not sweep them up, and that the sentences
// their verdicts rest on say what they claim to.
//
// The gate is the holds' gate and carries the holds' sentence: a scenario nobody has exercised
// must not turn the next ordinary run into a debugging session. When these pass, the gate comes
// off and THIS test changes, by name, in that commit — which is what happened to
// `TestTheHoldScenariosRunLikeAnyOther` and is the shape to copy.

import (
	"strings"
	"testing"
)

// THEY ARE GATED, LABELLED, AND NOT UNDER THE PRESS'S SELECTOR.
//
// Three claims and each has its own failure. A drag that ran in a default run before anybody had
// seen one pass would make the next ordinary run a debugging session; one labelled as anything
// but a desktop scenario would be counted in the wrong total; and one filed under `window/` —
// where two of them would sit naturally, since they are about a window that moved or resized —
// would be selected by `-only window`, which is the guarded press's regression gate.
func TestTheDragScenariosAreGatedUntilTheyHaveRun(t *testing.T) {
	var found int
	for _, s := range scenarios() {
		if !strings.HasPrefix(s.name, "drag/") {
			continue
		}
		found++
		if !s.onlyWhenAsked {
			t.Errorf("%q is in an ordinary run and has never passed on a desktop, "+
				"so the next run anybody makes for another reason becomes a "+
				"debugging session about drags", s.name)
		}
		if s.layer != layerDesktop {
			t.Errorf("%q is labelled %q and it needs a desktop", s.name, s.layer)
		}
		// AND NOT UNDER ANYBODY ELSE'S SELECTOR, asked of EVERY other family
		// rather than of the one that was obviously a risk.
		//
		// `window` was the obvious one and two of these names failed on it: two of
		// the four are about a window that moved or was resized, which is how they
		// want to be named and is exactly what `-only window` means to somebody
		// changing the guarded press. Checking only the obvious collision is how the
		// next one gets through, so this asks about all of them.
		for _, family := range families(t) {
			if family == "drag" {
				continue
			}
			if strings.Contains(s.name, family) {
				t.Errorf("%q contains %q, so `-only %s` — another "+
					"family's regression gate — would select it",
					s.name, family, family)
			}
		}
		if s.about == "" || s.run == nil {
			t.Errorf("%q is not a runnable, described scenario", s.name)
		}
	}
	if found != 4 {
		t.Errorf("%d drag scenario(s) are registered, want 4: the gesture, the moved "+
			"window, the resized window, and the stop", found)
	}
}

// ── AND WHAT THE VERDICTS REST ON ─────────────────────────────────────────────

// aDragLog is a gesture as the fixture would log one: a press, some movement, a release.
func aDragLog(window rect, from, to [2]int, moves int) []fixtureEvent {
	at := func(seq int, what string, x, y int) fixtureEvent {
		return fixtureEvent{Seq: seq, Kind: "mouse", What: what, Control: "Name",
			ScreenX: x, ScreenY: y, Window: window}
	}
	out := []fixtureEvent{at(1, "left_down", from[0], from[1])}
	for i := 1; i <= moves; i++ {
		p := float64(i) / float64(moves+1)
		out = append(out, at(1+i, "move",
			from[0]+int(float64(to[0]-from[0])*p),
			from[1]+int(float64(to[1]-from[1])*p)))
	}
	return append(out, at(2+moves, "left_up", to[0], to[1]))
}

var theTestWindow = rect{X: 100, Y: 100, W: 800, H: 600}

// THE SHAPE IS A PRESS, MOVEMENT, AND A RELEASE — AND NOTHING ELSE BETWEEN THEM.
//
// The last part is what the obvious version of this check misses: asserting the first event is a
// press and the last is a release is satisfied by TWO drags in a row, which is a routine doing
// twice what it was asked.
func TestTheGestureShapeIsAskedOfTheWholeLog(t *testing.T) {
	good := aDragLog(theTestWindow, [2]int{200, 200}, [2]int{600, 400}, 5)
	if why := theGestureDidNotArrive(good); why != "" {
		t.Errorf("an ordinary drag is reported as not having arrived: %s", why)
	}
	for _, c := range []struct {
		what string
		got  []fixtureEvent
		says string
	}{
		{"nothing at all", nil, "received nothing at all"},
		{"a release with no press", good[len(good)-1:], "begins with its button"},
		{"a press with no release", good[:len(good)-1], "is still down"},
		{"two gestures in a row", append(append([]fixtureEvent{}, good...), good...),
			"more than one gesture"},
	} {
		t.Run(c.what, func(t *testing.T) {
			why := theGestureDidNotArrive(c.got)
			if why == "" {
				t.Fatalf("%s is reported as an arrived drag", c.what)
			}
			if !strings.Contains(why, c.says) {
				t.Errorf("the sentence is %q and does not say %q", why, c.says)
			}
		})
	}
}

// AND THE TRAVEL IS REQUIRED WITHOUT A COUNT.
//
// Windows coalesces mouse moves, so the number that arrives depends on how busy the machine is —
// a scenario asserting one would be asserting the scheduler and would fail with nothing wrong.
// What cannot be coalesced away is that movement happened at all, and a gesture with none is two
// clicks, which is what a teleport looks like.
func TestTheTravelIsRequiredAndNotCounted(t *testing.T) {
	for _, moves := range []int{1, 3, 24} {
		got := aDragLog(theTestWindow, [2]int{200, 200}, [2]int{600, 400}, moves)
		if why := theTravelWasNotReceived(got); why != "" {
			t.Errorf("a drag whose travel arrived as %d move(s) is reported as "+
				"not having travelled: %s", moves, why)
		}
	}
	// AND NONE AT ALL IS THE ONE THAT FAILS.
	teleport := []fixtureEvent{
		{Seq: 1, Kind: "mouse", What: "left_down", ScreenX: 200, ScreenY: 200},
		{Seq: 2, Kind: "mouse", What: "left_up", ScreenX: 600, ScreenY: 400},
	}
	why := theTravelWasNotReceived(teleport)
	if why == "" {
		t.Fatal("a press and a release at two places is reported as a drag, which " +
			"is the one thing this check exists to tell apart")
	}
	if !strings.Contains(why, "teleport") {
		t.Errorf("the sentence is %q and does not say what arrived instead", why)
	}
}

// AND EVERY POINT HAS TO BE INSIDE THE WINDOW, NOT JUST THE ENDS.
//
// A path that left the window and came back would have dragged across whatever it crossed, and
// the only account of that is the fixture's record of each point.
func TestEveryPointOfTheGestureIsHeldInsideTheWindow(t *testing.T) {
	good := aDragLog(theTestWindow, [2]int{200, 200}, [2]int{600, 400}, 5)
	if why := anEndLandedOutside(good, theTestWindow); why != "" {
		t.Errorf("a gesture entirely inside the window is reported as leaving it: %s",
			why)
	}
	// A MIDDLE POINT OUTSIDE IT, with both ends inside — the case a check on the ends
	// alone would call a pass.
	wandered := append([]fixtureEvent{}, good...)
	wandered[3].ScreenX = theTestWindow.X + theTestWindow.W + 40
	why := anEndLandedOutside(wandered, theTestWindow)
	if why == "" {
		t.Fatal("a gesture that left the window partway is reported as having " +
			"stayed inside it, so a drag across the neighbouring app reads as a " +
			"pass")
	}
	if !strings.Contains(why, "outside") {
		t.Errorf("the sentence is %q", why)
	}
}

// AND THE REFUSAL IS NOT ONE EVENT, WHICH IS STRICTER THAN NOT ONE PRESS.
//
// The guard is read before the gesture and again immediately before the button goes down, so an
// implementation that pressed and then thought better of it would leave a DOWN with no up — a
// button held on somebody's desktop by a routine that reported refusing. A reader counting only
// presses would call that a pass; a reader counting only complete gestures would too.
func TestARefusedDragIsAssertedOnNotOneEvent(t *testing.T) {
	if why := anythingAtAllArrived(nil, theTestWindow); why != "" {
		t.Errorf("an empty log is reported as having received something: %s", why)
	}
	lonely := []fixtureEvent{{Seq: 1, Kind: "mouse", What: "left_down", ScreenX: 200,
		ScreenY: 200, Window: theTestWindow}}
	if why := anythingAtAllArrived(lonely, theTestWindow); why == "" {
		t.Error("a mouse-down with no release is reported as a refusal, so a routine " +
			"that pressed and regretted it would read as having sent nothing — " +
			"with the button still down")
	}
}

// AND A STOP THAT ARRIVED TOO LATE IS NOT RUN, NOT A PASS.
//
// The one way the stopped-drag scenario can be accidentally green: the gesture finishes, the run
// is stopped between steps and records `cancelled`, and every assertion about where the button
// came up is true of a drag nothing interrupted.
func TestAStopThatArrivedTooLateIsNotACancellation(t *testing.T) {
	mid := aDragLog(theTestWindow, [2]int{200, 200}, [2]int{600, 400}, 3)
	if why := theDragHadAlreadyFinished(mid, runRecord{Outcome: "cancelled"}); why != "" {
		t.Errorf("a stopped gesture that had travelled is reported as having "+
			"finished: %s", why)
	}
	for _, c := range []struct {
		what string
		got  []fixtureEvent
		rec  runRecord
		says string
	}{
		{"the run performed", mid, runRecord{Outcome: "performed"},
			"no cancellation was exercised"},
		{"the stop landed before the travel",
			mid[:1], runRecord{Outcome: "cancelled"},
			"no gesture in flight to interrupt"},
	} {
		t.Run(c.what, func(t *testing.T) {
			why := theDragHadAlreadyFinished(c.got, c.rec)
			if why == "" {
				t.Fatalf("%s is reported as an exercised cancellation", c.what)
			}
			if !strings.Contains(why, c.says) {
				t.Errorf("the sentence is %q and does not say %q", why, c.says)
			}
		})
	}
}

// AND A RELEASE AT THE DESTINATION IS THE DEFECT, ASSERTED AGAINST WHAT WAS DEMONSTRATED.
//
// Against the seeded destination and not against the last movement received, which would make
// the check a tautology about two adjacent records in the same log — a release always follows the
// last move, wherever either of them is.
func TestAReleaseAtTheDestinationIsReported(t *testing.T) {
	seed := seededRoutine{at: [][2]int{{200, 200}, {600, 400}}}
	dropped := aDragLog(theTestWindow, [2]int{200, 200}, [2]int{600, 400}, 3)
	why := theReleaseWasAtTheDestination(dropped, seed)
	if why == "" {
		t.Fatal("a stopped drag that released at its destination is reported as " +
			"having let go where the pointer got to — which is the first defect " +
			"this slice fixed, read as a pass")
	}
	if !strings.Contains(why, "destination") {
		t.Errorf("the sentence is %q", why)
	}
	// AND LETTING GO PARTWAY IS WHAT A STOP LOOKS LIKE.
	partway := append([]fixtureEvent{}, dropped[:len(dropped)-1]...)
	partway = append(partway, fixtureEvent{Seq: 9, Kind: "mouse", What: "left_up",
		ScreenX: 400, ScreenY: 300, Window: theTestWindow})
	if why := theReleaseWasAtTheDestination(partway, seed); why != "" {
		t.Errorf("a release partway along the travel is reported as a drop at the "+
			"destination: %s", why)
	}
}

// families is every scenario prefix registered, which is the vocabulary `-only` selects on.
//
// Read off the scenarios rather than listed, so a family added later is asked about without
// anybody having to remember this test — the pump invariant's lesson, one module over: a list
// would be written against the collisions that existed the day it was written.
func families(t *testing.T) []string {
	t.Helper()
	seen := map[string]bool{}
	var out []string
	for _, s := range scenarios() {
		name, _, found := strings.Cut(s.name, "/")
		if !found || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	if len(out) < 4 {
		t.Fatalf("only %d scenario family/families found (%v), so this is checking "+
			"less than it reads as", len(out), out)
	}
	return out
}
