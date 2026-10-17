package service

// NOTHING NEEDED DOING IS A SUCCESS.
//
// Measured on a live Settings window: asked to open the Bluetooth page while already on it,
// Marco said the right thing — "Bluetooth & devices is already the one you are on, so nothing
// needs pressing" — and printed it under `FAILED:` with a non-zero exit status, because
// ResultNothingToDo fell through classify's default. The headline said the opposite of the
// sentence beneath it, which is how a person learns to distrust the answer.

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

func TestARequestThatNeededNothingIsNotReportedAsAFailure(t *testing.T) {
	state, resp := classify(execute.Outcome{
		Status:  directorapi.ResultNothingToDo,
		Message: `"Bluetooth & devices" is already the one you are on, so nothing needs pressing`,
	})
	if state == CommandFailed || resp == ResponseFailed {
		t.Errorf("a request that needed nothing came back as %s/%s", state, resp)
	}
	if state != CommandCompleted || resp != ResponseCompleted {
		t.Errorf("state=%s response=%s, want completed", state, resp)
	}
	// AND WHAT DISTINGUISHES IT IS THE COUNT, which is where the difference belongs: the
	// command completed, and it performed nothing.
	if n := completedActions(execute.Outcome{Status: directorapi.ResultNothingToDo}); n != 0 {
		t.Errorf("%d actions counted for a request that performed none", n)
	}
	// And an ordinary failure is still a failure, so this did not make everything succeed.
	if state, _ := classify(execute.Outcome{Status: directorapi.ResultFailed}); state != CommandFailed {
		t.Errorf("a real failure came back as %s", state)
	}
}
