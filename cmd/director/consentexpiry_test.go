package main

// A QUESTION DOES NOT OUTLIVE THE RUN THAT ASKED IT.
//
// The press-authorisation prompt waits up to `--answer` for a key. `--seconds` is the run's own
// limit, and before the deadline went into the context the two were unrelated: the question opened
// before expiry, the desk kept waiting, and a person pressing F7 afterwards was handed a FRESH
// authorisation. None of the runner's boundary checks asks what time it is, so the click could
// still be sent — past the limit the run advertised, with every other guard satisfied.
//
// This is that mechanism on its own, with no window and no clock to wait on: the granter is the
// production one, the desk is the production one, the keys are synthetic, and the context carries
// a deadline that has already passed by the time the key arrives.
//
// The wiring — that `--seconds` reaches this context at all — is held separately by
// `TestPerformWorkflowInstallsItsGates`, which pins `context.WithDeadline(ctx, deadline)` in the
// assembly, and by `TestAnAnswerAfterTheDeadlineAuthorisesNothing`, which drives the real command
// when a foreground window can be read.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// aPressToAuthorise is an intent the desk can describe: one step, one control, one point.
func aPressToAuthorise() workflowrun.Intent {
	return workflowrun.Intent{
		Step: 0, Epoch: 5, Generation: 1,
		Window:  directorapi.WindowID("hwnd:1"),
		Control: "control_1", Screen: "screen_a", Expect: "screen_b",
		At: workflowrun.Point{X: 109, Y: 488},
	}
}

// AN ANSWER THAT ARRIVES AFTER THE RUN'S DEADLINE AUTHORISES NOTHING.
//
// The key is delivered well inside the desk's own `within`, so the only thing that can refuse it
// is the run's deadline — which is the whole point.
func TestAKeyPressedAfterTheRunsDeadlineGrantsNoAuthorisation(t *testing.T) {
	var out strings.Builder
	grant, press := aDeskOverSyntheticKeys(t, &out)

	// A RUN THAT HAS ALREADY USED ITS TIME. `--seconds` has passed; `--answer` has not.
	ctx, cancel := context.WithDeadline(context.Background(),
		time.Now().Add(40*time.Millisecond))
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		// AFTER THE DEADLINE, AND INSIDE THE DESK'S WINDOW. The desk would wait
		// two seconds for this; the run allowed forty milliseconds.
		time.Sleep(300 * time.Millisecond)
		press(navsource.KeyF7, true)
		press(navsource.KeyF7, false)
	}()

	auth, ok := grant.Authorise(ctx, aPressToAuthorise())
	<-done

	if ok {
		t.Fatalf("a key pressed after the run's deadline produced an authorisation "+
			"for step %d by %q", auth.Step, auth.By)
	}
	if auth.By != "" {
		t.Errorf("the refusal still carries an authoriser: %+v", auth)
	}
	// AND THE QUESTION WAS CLOSED, not left listening. `consentDesk.ask` withdraws it
	// on `ctx.Done()`, so the late key is STRAY — which is what keeps it from being
	// taken as the answer to whatever is asked next.
	if !strings.Contains(out.String(), "nobody answered") {
		t.Errorf("the prompt does not report that it closed unanswered:\n%s",
			out.String())
	}
}

// AND THE SAME KEY, IN TIME, STILL AUTHORISES.
//
// The positive control, and it is what makes the test above about the DEADLINE rather than about
// the desk being broken.
func TestAKeyPressedBeforeTheDeadlineStillGrantsAuthorisation(t *testing.T) {
	var out strings.Builder
	grant, press := aDeskOverSyntheticKeys(t, &out)

	ctx, cancel := context.WithDeadline(context.Background(),
		time.Now().Add(2*time.Second))
	defer cancel()

	go func() {
		time.Sleep(40 * time.Millisecond)
		press(navsource.KeyF7, true)
		press(navsource.KeyF7, false)
	}()

	auth, ok := grant.Authorise(ctx, aPressToAuthorise())
	if !ok {
		t.Fatalf("a key pressed inside the deadline was refused:\n%s", out.String())
	}
	if auth.By == "" {
		t.Error("the authorisation names nobody")
	}
}
