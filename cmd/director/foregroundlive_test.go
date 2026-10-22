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
	"go/parser"
	"go/token"
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

// aBackgroundApplication finds a real application with a visible window that is NOT in front.
//
// Returns the image name, or skips. Deliberately not the foreground one: this test is about the
// state where the target has lost the desktop, and taking the foreground to create it would
// interrupt whoever is using the machine.
func aBackgroundApplication(t *testing.T, p *winprovider.Provider) string {
	t.Helper()
	all := p.AllCandidates(context.Background())
	if len(all) == 0 {
		t.Skip("no top-level windows on this machine, so there is no live window " +
			"state to check against")
	}
	front := map[string]bool{}
	for _, c := range all {
		if c.Foreground && c.Application != "" {
			front[strings.ToLower(c.Application)] = true
		}
	}
	for _, c := range all {
		img := strings.ToLower(strings.TrimSpace(c.Application))
		if img == "" || c.Minimized || !c.Visible || front[img] {
			continue
		}
		return img
	}
	t.Skip("every windowed application on this machine is in the foreground, so the " +
		"state this checks does not exist here")
	return ""
}

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

// AND NOTHING IN EITHER TEST CAN PRESS OR TAKE THE FOREGROUND.
//
// A live test that could emit input is a live test nobody can run while somebody is using the
// machine, and these are meant to be safe to run during a game.
//
// Asserted over this file's IMPORTS rather than its text — a text scan finds the forbidden
// words in the list that forbids them, which is a test that fails on its own prohibition. What
// must be true is that nothing here can REACH an actuator: both tests drive `stillThere`, which
// answers a question and returns a bool.
func TestTheLiveBoundaryTestsCannotEmitInput(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "foregroundlive_test.go", nil,
		parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, imp := range parsed.Imports {
		for _, forbidden := range []string{
			"marcoexec", "oshost", "internal/driver", "internal/runtime",
			"internal/winctx",
		} {
			if strings.Contains(imp.Path.Value, forbidden) {
				t.Errorf("the live boundary tests import %s; they must "+
					"observe and refuse, never act or take the "+
					"foreground", imp.Path.Value)
			}
		}
	}
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
