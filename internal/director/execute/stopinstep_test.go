package execute

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/edit"
	editproviders "github.com/chaynes-simpleclouds/marco/internal/director/edit/providers"
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// cancellingEditor is the stop key arriving while a step is in flight.
//
// The existing cancellation tests cancel BETWEEN steps — before the program starts, or after one
// finished — and the step loop checks ctx.Err() at the top of each iteration, so both are caught.
// Neither covers the window this models: the stop lands while an operation is mid-flight, the
// operation comes back with an error because the context under it died, and the loop then reads
// that error as the step's own verdict.
type cancellingEditor struct {
	cancel func()
	calls  int
}

func (c *cancellingEditor) Apply(_ context.Context, _ editproviders.Target, _ edit.Operation) (edit.Outcome, error) {
	c.calls++
	c.cancel()
	return edit.Outcome{}, errors.New("the input went nowhere")
}

// TestAStopDuringAStepIsCancelledNotFailed is the third instance of one rule.
//
// Marco already holds it in two other places — TestARejectedConfirmationIsCancelledNotFailed and
// TestAStoppedRequestIsCancelledAndNotMisunderstood — and the rule is the same each time: when
// the context is gone, the cancellation is the reason for whatever happened next, and reporting
// the downstream symptom instead sends a person to fix something that was never wrong.
//
// Measured on a real desktop before this existed. `type Marco into the Name field and click Save`,
// stopped while the typing was in flight: nothing was typed, Save was never pressed, and the
// person who had just pressed stop was told the request FAILED.
//
// Why that is worse than untidy: `failed` is the one word that invites a retry, and a person who
// stopped a half-done sequence is the last person who should be nudged to run it again.
func TestAStopDuringAStepIsCancelledNotFailed(t *testing.T) {
	h, _, vals := captureHarness(t, fieldScenes(8)...)
	vals.value, vals.known = "Alice", true

	ctx, cancel := context.WithCancel(context.Background())
	ed := &cancellingEditor{cancel: cancel}
	x, ok := h.pipeline.Executor.(*Executor)
	if !ok {
		t.Fatalf("the harness executor is %T; this test needs the real one", h.pipeline.Executor)
	}
	x.Editor = ed

	prog, err := program.Decompose(`type "hello" and then type "world"`, testIntent)
	if err != nil {
		t.Fatalf("decompose: %v", err)
	}
	if len(prog.Steps) != 2 {
		t.Fatalf("the request decomposed into %d steps; this test needs two", len(prog.Steps))
	}

	out := h.pipeline.RunProgram(ctx, prog, program.Context{}, 0)

	if out.Status == directorapi.ResultDone {
		t.Fatal("a cancelled program reported success")
	}
	if out.Status != directorapi.ResultCancelled {
		t.Errorf("a program stopped during step 1 came back as %s, want cancelled.\n\n"+
			"message: %q\n\n"+
			"The step's own error is a true sentence about an operation whose context had "+
			"just been killed, and the wrong one for somebody who pressed stop. `failed` is "+
			"also the one word that invites a retry.", out.Status, out.Message)
	}
	// AND THE SECOND STEP NEVER RAN. Cancellation that reports itself correctly but keeps
	// going would be the worse bug of the two.
	if ed.calls != 1 {
		t.Errorf("the editor was called %d times; the step after the stop must not run", ed.calls)
	}
	// AND THE REASON IS NOT HIDDEN. Relabelling the status must not throw away what the step
	// said, or a genuine failure that happened to coincide with a stop becomes invisible.
	if !strings.Contains(out.Message, "went nowhere") {
		t.Errorf("the message dropped the step's own account: %q", out.Message)
	}
}

// TestAnUncancelledStepFailureIsStillAFailure is the other side, and the reason the check is on
// the context rather than on the error.
//
// A step that simply did not work, with nobody having stopped anything, keeps `failed`. Without
// this, "cancelled" would become the word for every program that went wrong — which is the same
// class of lie in the opposite direction, and a much easier one to ship by accident.
func TestAnUncancelledStepFailureIsStillAFailure(t *testing.T) {
	h, _, vals := captureHarness(t, fieldScenes(8)...)
	vals.value, vals.known = "Alice", true

	ed := &cancellingEditor{cancel: func() {}} // fails, cancels nothing
	x, ok := h.pipeline.Executor.(*Executor)
	if !ok {
		t.Fatalf("the harness executor is %T", h.pipeline.Executor)
	}
	x.Editor = ed

	prog, err := program.Decompose(`type "hello" and then type "world"`, testIntent)
	if err != nil {
		t.Fatalf("decompose: %v", err)
	}
	out := h.pipeline.RunProgram(context.Background(), prog, program.Context{}, 0)

	if out.Status == directorapi.ResultCancelled {
		t.Fatal("a step that failed with nobody stopping it was reported as cancelled")
	}
	if out.Status == directorapi.ResultDone {
		t.Fatal("a failed step reported success")
	}
}
