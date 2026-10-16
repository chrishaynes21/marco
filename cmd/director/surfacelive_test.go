package main

// THE PRECONDITION CHECKER REFUSES RATHER THAN RELAXES.
//
// The owner's rule for the whole experiment, and the thing this command exists to make true:
//
//	If the live hook is absent or the required foreground evidence is unavailable, stop before
//	execution and report the missing prerequisite rather than weakening a guard.
//
// So what is tested is the refusals and the exit code, because they are what a script and a
// person both read, and because a checker that printed a failure and exited zero would be worse
// than no checker.

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// A DEMONSTRATION IS NEVER LEARNED INTO THE REAL STORE.
//
// The first safeguard, and it is the SAME predicate `reset-test-state` uses: a home this would
// learn into has to be one that can be emptied afterwards. Two opinions about what counts as a
// sandbox would eventually let one command write where the other could not clean up.
func TestADemonstrationIsNeverLearnedIntoTheRealStore(t *testing.T) {
	real := filepath.Join("C:", "Users", "someone", "AppData", "Roaming", "marco")
	for _, tc := range []struct {
		name string
		home string
		why  string
	}{
		{"unset", "", "is not set"},
		{"whitespace", "   ", "is not set"},
		{"the real store", real, "the real store"},
		// THE SAME DIRECTORY BY A LONGER ROUTE. Windows paths, so a guard a slash could
		// step around would not be a guard.
		{"the real store with slashes", strings.ReplaceAll(real, `\`, "/"),
			"the real store"},
		{"the real store in another case", strings.ToUpper(real), "the real store"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := surfaceSandboxCheck(tc.home, real)
			if c.OK {
				t.Fatalf("%q was accepted as a disposable learning record", tc.home)
			}
			if !c.Blocking {
				t.Error("and it does not block the experiment")
			}
			if !strings.Contains(c.Detail, tc.why) {
				t.Errorf("the refusal does not say %q: %s", tc.why, c.Detail)
			}
		})
	}

	// AND A THROWAWAY DIRECTORY IS ACCEPTED, so the guard is not simply always no.
	sandbox := filepath.Join(t.TempDir(), "surface-home")
	if c := surfaceSandboxCheck(sandbox, real); !c.OK {
		t.Fatalf("a throwaway home was refused: %s", c.Detail)
	}
}

// A FALSE PRECONDITION EXITS NON-ZERO, AND THE WHOLE ACCOUNT IS STILL PRINTED.
//
// Which precondition is false IS the answer, so it is printed either way — and the exit code is
// what lets a script refuse to go on. The dogfood script learned this the expensive way: four
// runs walked against a Director that was not watching, because the strip said so and nobody was
// reading the strip.
func TestAFalsePreconditionStopsTheExperiment(t *testing.T) {
	var out bytes.Buffer
	code := reportSurfaceChecks(&out, []surfaceCheck{
		{Name: "a thing that holds", OK: true, Blocking: true, Detail: "fine"},
		{Name: "a thing that does not", Blocking: true, Detail: "the hook is absent"},
		{Name: "a thing nobody measured", Detail: "not measured"},
	}, "click Bluetooth & devices")

	if code == 0 {
		t.Error("a false blocking precondition exited zero, so a script would go on")
	}
	body := out.String()
	for _, want := range []string{
		"a thing that holds", "a thing that does not", "the hook is absent",
		"a thing nobody measured", "click Bluetooth & devices",
		"fix the precondition rather than to relax a guard",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the account does not mention %q:\n%s", want, body)
		}
	}
}

// AND AN UNMEASURED CHECK IS NOT A PASS AND NOT A FAILURE.
//
// The third state matters: "the hook was not watched" and "the hook was watched and saw nothing"
// are different findings, and a checker that collapsed them would let an unmeasured prerequisite
// read as a satisfied one.
func TestAnUnmeasuredPreconditionIsNeitherPassNorFail(t *testing.T) {
	var out bytes.Buffer
	code := reportSurfaceChecks(&out, []surfaceCheck{
		{Name: "measured and true", OK: true, Blocking: true, Detail: "fine"},
		{Name: "not measured", Detail: "re-run with -watch"},
	}, "")
	if code != 0 {
		t.Error("an unmeasured non-blocking check failed the run")
	}
	if !strings.Contains(out.String(), "--?") {
		t.Errorf("an unmeasured check is not marked as such:\n%s", out.String())
	}
}

// AND THE TARGET PHRASE COMES OUT OF THE INSTRUCTION.
func TestTheInstructionsTargetPhrase(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"click Bluetooth & devices", "Bluetooth & devices"},
		{"Click Mouse", "Mouse"},
		{"press Settings", "Settings"},
		{"open Settings", "Settings"},
		{"Bluetooth", "Bluetooth"},
		{"", ""},
	} {
		if got := instructionWords(tc.in); got != tc.want {
			t.Errorf("instructionWords(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// A STAGE IS COLLECTED WITH A WINDOW ALREADY IN IT.
//
// The one ordering fact in the perception stack, and the reason this harness observes twice.
// Every pixel provider is scoped to a window by `Runtime.activeWindow`, which takes its candidate
// from the LAST OBSERVED WORLD — because text has to be fused against elements from a world, and
// photographing a different window from the one those elements came from would compare things
// that were never on screen together.
//
// With accessibility collected that ordering is invisible: the walk reports the window and its
// controls in the same breath. Without it, the first cycle is the one that establishes the window
// and the second is the first that can see anything inside it. The first live run of this
// experiment is what made the point: a foreground of "" and zero Props, on a screen full of
// controls.
//
// What is asserted is the consequence rather than the mechanism. The world that comes back is the
// SECOND one, so whatever the first cycle established is already in it.
func TestAStageIsCollectedWithAWindowAlreadyInIt(t *testing.T) {
	var cycles int
	front := directorapi.WindowID("hwnd:42")
	rt := &Runtime{pipeline: &execute.Pipeline{
		Observe: func(context.Context) (directorapi.WorldState, error) {
			cycles++
			if cycles == 1 {
				// THE FIRST CYCLE, as it really is under a pixel-only
				// configuration: a window has just been enumerated, and
				// nothing has had a chance to look inside one yet.
				return directorapi.WorldState{}, nil
			}
			return directorapi.WorldState{ActiveWindow: &front}, nil
		},
	}}

	w, err := observeStage(context.Background(), rt)
	if err != nil {
		t.Fatalf("observeStage: %v", err)
	}
	if cycles != 2 {
		t.Errorf("%d cycles, want two: the first establishes the window and the second "+
			"is the first that can see inside it", cycles)
	}
	if w.ActiveWindow == nil || *w.ActiveWindow != front {
		t.Errorf("the Stage came back with active window %v, so the warm-up cycle was "+
			"the one reported and every pixel provider had nothing to look at",
			w.ActiveWindow)
	}
}

// AND A PARTIAL FIRST CYCLE IS NOT A FAILURE.
//
// It is the EXPECTED first cycle: there is no window yet, so the providers that need one say so.
// Reporting that as the result of the collection would turn the ordinary case into an error and
// hide the second cycle, which is the one whose failure is a finding.
func TestTheWarmUpCycleIsAllowedToBePartial(t *testing.T) {
	var cycles int
	front := directorapi.WindowID("hwnd:7")
	rt := &Runtime{pipeline: &execute.Pipeline{
		Observe: func(context.Context) (directorapi.WorldState, error) {
			cycles++
			if cycles == 1 {
				return directorapi.WorldState{}, errors.New("no window to read")
			}
			return directorapi.WorldState{ActiveWindow: &front}, nil
		},
	}}

	w, err := observeStage(context.Background(), rt)
	if err != nil {
		t.Fatalf("a partial first cycle was reported as the result: %v", err)
	}
	if w.ActiveWindow == nil {
		t.Error("the Stage has no active window")
	}

	// AND THE SECOND CYCLE'S FAILURE IS REPORTED, because that one is a finding.
	cycles = 0
	rt.pipeline.Observe = func(context.Context) (directorapi.WorldState, error) {
		cycles++
		return directorapi.WorldState{}, errors.New("the collector is gone")
	}
	if _, err := observeStage(context.Background(), rt); err == nil {
		t.Error("a Stage that could not be collected at all came back as a Stage")
	}
}
