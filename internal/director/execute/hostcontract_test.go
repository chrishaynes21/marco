package execute

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// TestAHostContractStringIsNotShownToAPerson
//
// # Measured
//
//	marco do "open Documents"
//	FAILED: unsupported: the control does not implement InvokePattern
//
// Every part of that sentence is somebody else's vocabulary. `unsupported:` is a machine
// contract between the C# accessibility host and the Director — its own comment in
// plugins/uia/Uia.cs says why it exists, "so the Director can fall back and RECORD that it did"
// — and `InvokePattern` is a UIA type name. The person asked to open a folder.
//
// # Why it leaked
//
// Because `activate.Unsupported` is used as a BOOLEAN, for deciding whether a fallback is
// allowed, and nothing ever translated the string it recognises. The ladder's own refusals are
// written for a person (`refusalReason` builds "nothing available can open this: …" out of the
// rungs it tried), but this failure happens one step later: the ladder chose the invoke rung
// because the provider offered the control by id, the operation was attempted, and the HOST
// refused at run time. That path had no sentence of its own, so the contract string became one.
//
// The test asserts the absence of the two leaked vocabularies rather than one exact wording,
// because the wording is presentation and the leak is the defect.
func TestAHostContractStringIsNotShownToAPerson(t *testing.T) {
	h := newHarness(
		scene(t0, nil, menuBar()...),
		scene(t0.Add(time.Second), nil, menuBar()...),
		scene(t0.Add(2*time.Second), nil, menuBar()...),
	)
	// Exactly what the C# host returns for a control that offers no Invoke.
	h.operations.err = errors.New("unsupported: the control does not implement InvokePattern")

	out := h.pipeline.Handle(context.Background(), "open File")

	if out.Status == directorapi.ResultDone {
		t.Fatal("a host refusal reported success")
	}
	for _, leak := range []string{"unsupported:", "InvokePattern", "Pattern"} {
		if strings.Contains(out.Message, leak) {
			t.Errorf("the message hands a person %q:\n  %s", leak, out.Message)
		}
	}
	// AND IT STILL SAYS WHICH CONTROL AND WHAT WAS ASKED, or it has replaced one unusable
	// sentence with another.
	if !strings.Contains(out.Message, "File") {
		t.Errorf("the message does not name the control: %q", out.Message)
	}
	if !strings.Contains(strings.ToLower(out.Message), "open") {
		t.Errorf("the message does not name what was asked: %q", out.Message)
	}
	t.Logf("a person sees: %s", out.Message)
}

// TestAnOrdinaryHostFailureIsStillReportedAsItself is the other side.
//
// Only the capability-refusal contract is translated. A bridge that died, a control that
// vanished, a timeout — those are outages, their text is the only account of what went wrong,
// and replacing it with a tidy sentence about the verb would throw away the diagnosis.
func TestAnOrdinaryHostFailureIsStillReportedAsItself(t *testing.T) {
	h := newHarness(
		scene(t0, nil, menuBar()...),
		scene(t0.Add(time.Second), nil, menuBar()...),
		scene(t0.Add(2*time.Second), nil, menuBar()...),
	)
	h.operations.err = errors.New("the accessibility bridge exited unexpectedly")

	out := h.pipeline.Handle(context.Background(), "open File")

	if !strings.Contains(out.Message, "bridge exited unexpectedly") {
		t.Errorf("an outage lost its own account: %q", out.Message)
	}
}
