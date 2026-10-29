//go:build desktop

// TESTS THAT NEED THE SCREEN, AND THEREFORE DO NOT RUN IN THE SCREEN-FREE STAGE.
//
// `go test -tags desktop ./cmd/director/` runs these, and `gate.ps1` runs them in the stage
// that already takes the desktop. They are out of the default suite for a reason about the
// LEDGER rather than about the tests: their preconditions are facts about the owner's screen
// — a window has to be there, hold still across three looks, and keep a control under the
// same point — and `gate-skips.txt` can express neither state of a skip that depends on the
// environment. An undeclared skip fails the gate and a declaration whose test ran fails it too,
// so the same honest test was a finding with a game on screen and a finding without one. The
// ledger is for a skip that is always a skip; `-tags resolver` and `-tags corpus` are the
// precedent.
//
// What these establish, and what does NOT establish it instead: see the inventory in
// [[ADR-269-a-screen-free-gate-and-the-tests-that-need-the-screen]]. Tests of Director's run
// logic that merely NEEDED a performable act have moved to `anActOverAStatedWindow` and run by
// default; nothing here was replaced by that.

package main

// THE FINAL INPUT BOUNDARY, AGAINST REAL WINDOWS.
//
// `worldMoved` is a pure function and its cases are asserted over fixtures. What a fixture
// cannot say is whether the REAL tracker, the REAL provider and the REAL frontness test agree
// with it — and that is the question, because the check runs immediately before a press and
// everything it consults is live.
//
//	Verify actual foreground/window identity at the final input boundary, not merely when the
//	target is initially acquired. Test a real focus change before input emission.
//
// # How this tests a focus change without causing one
//
// Foregrounding a window of my own would take focus from whatever the person is doing, which is
// the one thing this must not do. So it uses a window that is ALREADY not in front: the desktop
// of a machine with more than one application on it is a machine where the "focus changed away
// from the target" state exists for free.
//
// That is the same state a focus change produces — the target no longer leads the desktop — and
// it is reached through `stillThere`, which is what the coordinator calls as its last act
// before `exec.Do`. Nothing is pressed, nothing is activated, and no window is touched.
//
// Skips where the state does not exist (one window, or none), because a test that invented the
// state would be a fixture test wearing a live test's name.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
)

// A TARGET THAT NO LONGER LEADS THE DESKTOP IS REFUSED AT THE BOUNDARY, ON REAL WINDOWS.
//
// The behavioural half of the frontness guard. The target is a real application, acquired by
// the real tracker, with its real generation — and it is not in front, which is the state an
// alt-tab produces between the plan and the press.
func TestTheRealBoundaryRefusesATargetThatLostTheDesktop(t *testing.T) {
	windows := winprovider.New()
	app := aBackgroundApplication(t, windows)

	obs, err := fastobserve.New(100*time.Millisecond, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer obs.Close()
	tracker := windowref.NewTracker(windows)

	v := tracker.Acquire(context.Background(), app)
	if !v.State.OK() {
		t.Skipf("%q could not be acquired (%s), so there is nothing to check at the "+
			"boundary", app, v.State)
	}
	// THE PLAN, made against the window as it actually is. Same generation and epoch, so
	// the refusal below cannot be the window-replaced case or the stale-observation one.
	planned := workflowrun.Sighting{
		Screen: obs.Screen(), Epoch: obs.Epoch(),
		Generation: v.Ref.Generation, Window: v.Ref.ID, Bounds: v.Ref.Bounds,
	}
	// AND THE SCOPE THAT AGREES, so this is the frontness case and not the application
	// one. Taken from the window the tracker resolved, which is the provenance a saved
	// workflow would carry.
	scope := workflowstore.NormaliseScope(v.Ref.Application)

	ok, why := stillThere(tracker, windows, app, scope, nil, noRelook)(
		context.Background(), planned, observe.PointerAt{X: 0.5, Y: 0.5})
	if ok {
		t.Fatalf("a target that does not lead the desktop was cleared to press: %s", why)
	}
	if !strings.Contains(why, "does not lead the desktop") {
		t.Errorf("the refusal is %q; the live frontness test and worldMoved disagree "+
			"about this state", why)
	}
	t.Logf("live refusal for %q: %s", app, why)
}

// AND THE APPLICATION IS CHECKED AT THAT SAME BOUNDARY, FROM OBSERVED PROVENANCE.
//
// The behavioural half of the scope guard. Everything is real and unchanged from the test
// above; the only difference is the workflow's scope, which names an application this window
// does not belong to. That is the race, reached through the real provider:
//
//	Marco identifies Solasta correctly, but the foreground changes to another application
//	before the click. The initial check must not be enough to authorize that later input.
//
// The refusal must be the APPLICATION one and must come before the frontness one, because the
// question "may this press go to this program at all" precedes "is that program in front".
func TestTheRealBoundaryRefusesAnApplicationTheWorkflowDoesNotBelongTo(t *testing.T) {
	windows := winprovider.New()
	app := aBackgroundApplication(t, windows)

	obs, err := fastobserve.New(100*time.Millisecond, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer obs.Close()
	tracker := windowref.NewTracker(windows)

	v := tracker.Acquire(context.Background(), app)
	if !v.State.OK() {
		t.Skipf("%q could not be acquired (%s)", app, v.State)
	}
	planned := workflowrun.Sighting{
		Screen: obs.Screen(), Epoch: obs.Epoch(),
		Generation: v.Ref.Generation, Window: v.Ref.ID, Bounds: v.Ref.Bounds,
	}
	// A WORKFLOW FROM ANOTHER GAME ENTIRELY, which is the file somebody points at the
	// wrong program. Chosen so it cannot coincide with anything on this machine.
	elsewhere := workflowstore.NormaliseScope("arkascended")
	if elsewhere == workflowstore.NormaliseScope(v.Ref.Application) {
		t.Skip("this window belongs to the application the test uses as the wrong one")
	}

	ok, why := stillThere(tracker, windows, app, elsewhere, nil, noRelook)(
		context.Background(), planned, observe.PointerAt{X: 0.5, Y: 0.5})
	if ok {
		t.Fatalf("a workflow for %q was cleared to press into %q: %s",
			string(elsewhere), v.Ref.Application, why)
	}
	if !strings.Contains(why, "changed after the plan was made") {
		t.Errorf("the refusal is %q, and it should say the application changed", why)
	}
	// AND IT NAMES BOTH, so a person can see which file went to which program.
	for _, want := range []string{"arkascended", strings.ToLower(v.Ref.Application)} {
		if !strings.Contains(strings.ToLower(why), want) {
			t.Errorf("the refusal does not mention %q: %s", want, why)
		}
	}
	// AND AN UNSCOPED WORKFLOW IS REFUSED HERE TOO, which is the last place it can be.
	if ok, why := stillThere(tracker, windows, app, "", nil, noRelook)(
		context.Background(), planned, observe.PointerAt{X: 0.5, Y: 0.5}); ok {
		t.Error("an unscoped workflow was cleared to press")
	} else if !strings.Contains(why, "UNSCOPED") {
		t.Errorf("the unscoped refusal at the boundary is %q", why)
	}
	t.Logf("live refusal: %s", why)
}

// noRelook is the fresh look these tests deliberately do not take.
//
// They drive `stillThere` against a REAL desktop to exercise the frontness and scope cases,
// and a capture here would add a second source of failure to a test about neither. What a fresh
// look says about itself is a separate question with its own tests.
//
// It answers as a look that AGREED, and that is what keeps these tests about what they are
// about: the application and frontness cases come before the three fresh ones in `worldMoved`,
// so a stub that refused would let both of these pass on the wrong refusal.
func noRelook(context.Context, windowref.Validation, observe.PointerAt) freshEvidence {
	return freshEvidence{Took: true, Agreed: true, Counted: "settled", Target: true}
}
