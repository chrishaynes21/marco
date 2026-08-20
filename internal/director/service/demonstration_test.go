package service

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The demonstration commands over the wire.
//
// These drive a real recorder and a real store through the real server, because what is
// worth proving here is the ROUTING and the ORDER: that an action reaches the right
// method, that extraction installs nothing, and that approval is a separate request a
// client has to make deliberately.

// demoService serves a fake runtime whose store lives in a temp directory.
func demoService(t *testing.T) *Client {
	t.Helper()
	rt := newFakeRuntime()
	rt.dir = t.TempDir()
	_, dir := serve(t, rt)
	return dial(t, dir)
}

func TestARecordingSessionStartsAndStops(t *testing.T) {
	c := demoService(t)

	started, err := c.Demonstration(DemonstrationPayload{Action: DemoStart})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if started.Recording == nil || started.Recording.Status != demo.Recording {
		t.Fatalf("nothing is recording: %+v", started)
	}

	active, err := c.Demonstration(DemonstrationPayload{Action: DemoActive})
	if err != nil {
		t.Fatalf("active: %v", err)
	}
	if active.Recording == nil || active.Recording.ID != started.Recording.ID {
		t.Fatalf("the open session is not the one that was started: %+v", active)
	}

	stopped, err := c.Demonstration(DemonstrationPayload{Action: DemoStop})
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if stopped.Demonstration == nil || stopped.Demonstration.Status == demo.Recording {
		t.Fatalf("the session did not end: %+v", stopped)
	}

	// And it was stored: a demonstration that existed only in memory would be gone the
	// moment the user went to look at it.
	list, err := c.Demonstration(DemonstrationPayload{Action: DemoList})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Demonstrations) != 1 {
		t.Fatalf("%d stored demonstrations", len(list.Demonstrations))
	}
}

func TestASecondStartIsRefusedOverTheWire(t *testing.T) {
	c := demoService(t)
	if _, err := c.Demonstration(DemonstrationPayload{Action: DemoStart}); err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, err := c.Demonstration(DemonstrationPayload{Action: DemoStart}); err == nil {
		t.Fatal("a second session was opened over an open one")
	}
}

func TestStoppingWithNoSessionSaysSoRatherThanInventingOne(t *testing.T) {
	c := demoService(t)
	if _, err := c.Demonstration(DemonstrationPayload{Action: DemoStop}); err == nil {
		t.Fatal("stopping with nothing recording produced a demonstration")
	}
}

func TestAnUnknownActionIsAnErrorRatherThanADefault(t *testing.T) {
	c := demoService(t)
	_, err := c.Demonstration(DemonstrationPayload{Action: "start-recording-please"})
	if err == nil {
		t.Fatal("an unrecognised action was accepted")
	}
	if !strings.Contains(err.Error(), "demonstration") {
		t.Errorf("the error does not say what was wrong: %v", err)
	}
}

func TestExtractingAnEmptySessionRefusesRatherThanProposing(t *testing.T) {
	c := demoService(t)
	started, _ := c.Demonstration(DemonstrationPayload{Action: DemoStart})
	_, _ = c.Demonstration(DemonstrationPayload{Action: DemoStop})

	out, err := c.Demonstration(DemonstrationPayload{
		Action: DemoExtract, ID: string(started.Recording.ID),
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if out.Extraction == nil {
		t.Fatal("no extraction came back at all")
	}
	if out.Extraction.Candidate != nil {
		t.Fatalf("a procedure was proposed from an empty session: %+v",
			out.Extraction.Candidate)
	}
	if out.Extraction.Refusal == "" {
		t.Error("the refusal has no reason")
	}
}

func TestApprovingSomethingThatWasNeverExtractedFails(t *testing.T) {
	c := demoService(t)
	if _, err := c.Demonstration(DemonstrationPayload{
		Action: DemoApprove, ID: "demo-does-not-exist",
	}); err == nil {
		t.Fatal("a procedure was approved from a demonstration that does not exist")
	}
}

func TestTheDemonstrationCommandsAnswerWhileACommandRuns(t *testing.T) {
	// The control-plane property, over the wire. A recording command that blocked behind
	// desktop work could not be used to stop a recording of that work.
	rt := newFakeRuntime()
	rt.dir = t.TempDir()
	release := make(chan struct{})
	rt.handle = func(context.Context, string, func(ProgressPayload)) execute.Outcome {
		<-release
		return execute.Outcome{Status: directorapi.ResultDone}
	}
	_, dir := serve(t, rt)

	runner := dial(t, dir)
	done := make(chan struct{})
	go func() {
		_, _ = runner.Execute("click Save", false, nil)
		close(done)
	}()

	watcher := dial(t, dir)
	if _, err := watcher.Demonstration(DemonstrationPayload{Action: DemoActive}); err != nil {
		t.Fatalf("a demonstration query blocked behind a running command: %v", err)
	}
	close(release)
	<-done
}
