package localizetrial

// THE EVALUATION RUN.
//
// One hundred and twenty cases, two methods, one target demonstrated once. The parameters were
// frozen against the setup examples before any of this was scored; see protocol.go and
// setup_test.go. Nothing here was changed after the first evaluation run — had it been, this
// would be a tuning run and the evaluation would start again with fresh seeds.
//
// The numbers asserted below are what the run MEASURED, recorded so that a change to the fixture
// or to either method has to be noticed and explained rather than absorbed. They are not a
// target: had the experiment's answer been no, the number recorded here would say no.

import (
	"image"
	"strings"
	"testing"
)

// What the evaluation measured, on the run of record.
const (
	supportedCases   = 70 // 60 positive + 10 window-movement
	unsupportedCases = 50 // 10 each: absent, covered, partly covered, reflow, geometry

	// THE WHOLE-WINDOW BASELINE. It never once selected the demonstrated instance:
	// with two controls that look alike it either refuses as ambiguous, or — when
	// the demonstrated one is gone and only the decoy is left — selects the decoy
	// with no doubt at all. That second number is the finding.
	wholeRight = 0
	wholeWrong = 30

	// THE LOCAL REGION. Everything supported, nothing unsupported, no wrong instance.
	localRight   = supportedCases
	localRefused = unsupportedCases
)

func TestTheEvaluation(t *testing.T) {
	results := Evaluate(Cases(), Frozen)
	t.Log("\n" + Report(results))
	t.Log("\n" + Reasons(results))

	// ── THE WHOLE-WINDOW BASELINE ──
	right, of := Supported(results, "whole-window")
	if of != supportedCases {
		t.Fatalf("the evaluation has %d supported cases, not %d", of,
			supportedCases)
	}
	if right != wholeRight {
		t.Errorf("the baseline selected the demonstrated instance in %d of %d "+
			"supported cases; the run of record says %d", right, of,
			wholeRight)
	}
	if got := WrongInstances(results, "whole-window"); got != wholeWrong {
		t.Errorf("the baseline chose a different instance %d times; the run of "+
			"record says %d", got, wholeWrong)
	}

	// ── THE LOCAL REGION ──
	//
	// The three parts of the pass criterion, stated and checked separately
	// because they are three different failures.
	right, of = Supported(results, "local region")
	if right != localRight || of != supportedCases {
		t.Errorf("the local region selected the demonstrated instance in %d of "+
			"%d supported cases; the run of record says %d of %d",
			right, of, localRight, supportedCases)
	}
	refused, unsupported := Unsupported(results, "local region")
	if refused != localRefused || unsupported != unsupportedCases {
		t.Errorf("the local region refused %d of %d unsupported cases; the run "+
			"of record says %d of %d", refused, unsupported,
			localRefused, unsupportedCases)
	}
	if got := WrongInstances(results, "local region"); got != 0 {
		t.Errorf("the local region chose a different instance %d times, and "+
			"the pass criterion is zero", got)
	}
}

// THE LOCAL METHOD NEVER ANSWERS WITH A POINT OUTSIDE THE NEIGHBOURHOOD IT WAS GIVEN.
//
// "No fallback search elsewhere" is a property of the method and not of the fixture, so it is
// checked over every case rather than inferred from the fact that none of them went wrong. A
// future change that added a widening search would pass the table above on this fixture and fail
// here.
func TestTheLocalMethodNeverAnswersOutsideItsNeighbourhood(t *testing.T) {
	_, targets := Demonstration()
	for _, c := range Cases() {
		look := Render(c.Scene)
		target := targets[c.Want]
		v := Local(target, look, Frozen)
		if !v.Found {
			continue
		}
		d := v.Chose.Sub(target.At)
		if abs(d.X) > Frozen.Tolerance || abs(d.Y) > Frozen.Tolerance {
			t.Errorf("%s seed %d: the demonstrated point is %v and it chose "+
				"%v, which is outside the %dpx neighbourhood",
				c.Kind, c.Seed, target.At, v.Chose, Frozen.Tolerance)
		}
	}
}

// A MOVED WINDOW CHANGES NOTHING, BECAUSE NOTHING STORED IS A SCREEN COORDINATE.
//
// Tested on its own, as asked. The same client pixels at a different place on the virtual screen
// must produce the same client-relative answer and a different screen point — which is the
// difference between storing where the control is in its window and storing where it was on
// somebody's desktop.
func TestAMovedWindowChangesNothing(t *testing.T) {
	_, targets := Demonstration()
	target := targets[LeftApply]
	still := Render(Scene{Seed: 4100, Banner: Quiet})
	fixed := Local(target, still, Frozen)
	if !fixed.Found {
		t.Fatalf("the control was not found in a still window: %s", fixed.Why)
	}
	for _, at := range []image.Point{{X: 0, Y: 0}, {X: 900, Y: 40},
		{X: 37, Y: 611}, {X: 1600, Y: 900}} {
		moved := Render(Scene{Seed: 4100, Banner: Quiet, Origin: at})
		got := Local(target, moved, Frozen)
		if !got.Found {
			t.Errorf("the window at %v: %s", at, got.Why)
			continue
		}
		if got.Chose != fixed.Chose {
			t.Errorf("the window at %v was answered with client point %v, "+
				"and the same window at the origin with %v", at,
				got.Chose, fixed.Chose)
		}
		// AND THE SCREEN POINT MOVES WITH THE WINDOW, which is the other
		// half of the same claim: the answer is client-relative, and
		// turning it into something to press is where the window's
		// position is used.
		want := fixed.Chose.Add(at)
		if on := onScreen(moved, got.Chose); on != want {
			t.Errorf("on screen that is %v and the window moved to %v",
				on, want)
		}
	}
}

// AND A DISPLAY SCALE CHANGE IS REFUSED EVEN WHEN THE PIXELS ARE IDENTICAL.
//
// Its own test because a rendered case cannot isolate it: a real scale change re-renders
// everything larger, so the evaluation's scale cases are refused on size before the scale is
// ever consulted. Here the pixels are held identical and only the claim differs, which is the
// only way to see that the scale is checked at all.
func TestADifferentDisplayScaleIsRefused(t *testing.T) {
	_, targets := Demonstration()
	target := targets[LeftApply]
	look := Render(Scene{Seed: 4200, Banner: Quiet})
	if v := Local(target, look, Frozen); !v.Found {
		t.Fatalf("the control was not found at the demonstrated scale: %s",
			v.Why)
	}
	look.Scale = 1.25
	v := Local(target, look, Frozen)
	if v.Found {
		t.Error("the same pixels at a different display scale were accepted")
	}
	if !strings.Contains(v.Why, "scale") {
		t.Errorf("the refusal does not say it is about the scale: %q", v.Why)
	}
}

// FINDING THE CONTROL IS NOT EVIDENCE THAT PRESSING IT WORKED.
//
// The two are measured separately throughout, and this is the statement of why. The fixture
// knows what a click at a point would do; the method knows where it would click. A localization
// that is right about the point says nothing whatever about the result, and an experiment that
// reported one number for both would be making exactly the claim this project refuses to make.
func TestLocalizationIsNotVerification(t *testing.T) {
	_, targets := Demonstration()
	target := targets[LeftApply]
	look := Render(Scene{Seed: 4300, Banner: Quiet})

	v := Local(target, look, Frozen)
	if !v.Found {
		t.Fatalf("the control was not found: %s", v.Why)
	}
	// WHAT THE FIXTURE SAYS A CLICK THERE WOULD DO — which is a different question,
	// answered by a different thing, and the only reason it is asked here is to
	// show that the two instances ARE distinguishable by result.
	on, hit := look.Hit(v.Chose)
	if !hit || on != LeftApply {
		t.Fatalf("a click at the chosen point lands on %v (hit %v), not on the "+
			"instance that was demonstrated", on, hit)
	}
	if other, _ := look.Hit(mustCentre(t, look, RightApply)); other == on {
		t.Fatal("the two instances are not distinguishable by what a click on " +
			"them does, so no case could tell a right answer from a " +
			"wrong one")
	}
	// AND THE SAME POINT IN A LOOK WHERE THE CONTROL IS GONE LANDS ON NOTHING.
	// The localization above was correct about that look and says nothing about
	// this one: a stored point is not a promise that the control is still there.
	gone := LeftApply
	after := Render(Scene{Seed: 4300, Banner: Quiet, Drop: &gone})
	if _, still := after.Hit(v.Chose); still {
		t.Error("the control was removed and a click at the chosen point " +
			"still lands on one")
	}
}

func mustCentre(t *testing.T, l Look, of Instance) image.Point {
	t.Helper()
	at, ok := l.Centre(of)
	if !ok {
		t.Fatalf("the fixture has no %v", of)
	}
	return at
}
