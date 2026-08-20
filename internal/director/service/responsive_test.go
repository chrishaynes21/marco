package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Control-plane responsiveness.
//
//	Control-plane operations must remain responsive even when desktop work is
//	blocked. Long-running work may own a context, but it may not own the service.
//
// These tests hold a command open at a known point with a channel — never a sleep —
// and then exercise status, history, cancellation and a second mutating request from
// other connections. A sleep would make the test both slower and weaker: it would pass
// on a machine where the "blocked" work had actually finished.

// blockingPhase is a phase held open until the test releases it.
//
// Entered closes the moment the phase begins, so a test never has to guess when the
// work has started — the whole reason this is a barrier rather than a delay.
type blockingPhase struct {
	Entered chan struct{}
	Release chan struct{}
}

func newBlockingPhase() *blockingPhase {
	return &blockingPhase{Entered: make(chan struct{}), Release: make(chan struct{})}
}

// Run blocks until released or the context ends.
func (b *blockingPhase) Run(ctx context.Context) error {
	close(b.Entered)
	select {
	case <-b.Release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// controlPlaneBound is how long a control-plane request may take while desktop work is
// blocked. Generous enough not to be flaky on a loaded CI machine, and orders of
// magnitude below the "blocked forever" it is distinguishing from.
const controlPlaneBound = 2 * time.Second

// blockedService starts a service whose command blocks in a named phase.
func blockedService(t *testing.T) (*blockingPhase, string, *fakeRuntime) {
	t.Helper()
	block := newBlockingPhase()
	rt := newFakeRuntime()
	rt.handle = func(ctx context.Context, phrase string, _ func(ProgressPayload)) execute.Outcome {
		if err := block.Run(ctx); err != nil {
			return execute.Outcome{Status: directorapi.ResultCancelled, Message: "cancelled while blocked"}
		}
		return execute.Outcome{Status: directorapi.ResultDone, Message: "released"}
	}
	_, dir := serve(t, rt)
	return block, dir, rt
}

// startBlocked submits a command and waits until it is genuinely blocked.
func startBlocked(t *testing.T, dir string, block *blockingPhase) chan OutcomePayload {
	t.Helper()
	done := make(chan OutcomePayload, 1)
	go func() {
		c := dialQuiet(dir)
		defer c.Close()
		out, _ := c.Execute("click something slow", false, nil)
		done <- out
	}()
	select {
	case <-block.Entered:
	case <-time.After(controlPlaneBound):
		t.Fatal("the command never reached the blocking phase")
	}
	return done
}

// within runs fn on its own connection and fails if it does not finish in time.
func within(t *testing.T, what string, fn func()) {
	t.Helper()
	finished := make(chan struct{})
	go func() { defer close(finished); fn() }()
	select {
	case <-finished:
	case <-time.After(controlPlaneBound):
		t.Fatalf("%s did not return within %s while desktop work was blocked — "+
			"it is waiting behind the command", what, controlPlaneBound)
	}
}

func TestStatusRespondsWhileACommandIsBlocked(t *testing.T) {
	block, dir, _ := blockedService(t)
	done := startBlocked(t, dir, block)
	defer func() { close(block.Release); <-done }()

	within(t, "status", func() {
		c := dialQuiet(dir)
		defer c.Close()
		st, err := c.Status()
		if err != nil {
			t.Errorf("status: %v", err)
			return
		}
		// Not idle. A service that reported "nothing running" while a command was
		// blocked would be actively misleading — the user would conclude the request
		// had been lost.
		if st.Active == nil {
			t.Error("status reported no active command while one was blocked")
		}
	})
}

func TestHistoryRespondsWhileACommandIsBlocked(t *testing.T) {
	block, dir, _ := blockedService(t)
	done := startBlocked(t, dir, block)
	defer func() { close(block.Release); <-done }()

	within(t, "history", func() {
		c := dialQuiet(dir)
		defer c.Close()
		if _, err := c.History(5); err != nil {
			t.Errorf("history: %v", err)
		}
	})
}

func TestCancelRespondsAndReachesTheBlockedWork(t *testing.T) {
	block, dir, _ := blockedService(t)
	done := startBlocked(t, dir, block)

	within(t, "cancel", func() {
		c := dialQuiet(dir)
		defer c.Close()
		res, err := c.Cancel()
		if err != nil {
			t.Errorf("cancel: %v", err)
			return
		}
		if !res.Accepted {
			t.Errorf("cancel was not accepted: %s", res.Message)
		}
	})

	// The blocked work must actually receive it. A cancel that returns promptly and
	// leaves the work running is a cancel in name only.
	select {
	case out := <-done:
		if out.State != CommandCancelled {
			t.Fatalf("state = %s, want cancelled", out.State)
		}
	case <-time.After(controlPlaneBound):
		close(block.Release)
		t.Fatal("cancellation never reached the blocked phase")
	}
}

func TestASecondMutatingCommandIsRefusedPromptlyWhileBlocked(t *testing.T) {
	block, dir, _ := blockedService(t)
	done := startBlocked(t, dir, block)
	defer func() { close(block.Release); <-done }()

	within(t, "the second command", func() {
		c := dialQuiet(dir)
		defer c.Close()
		_, err := c.Execute("click something else", false, nil)
		if err == nil {
			t.Error("a second mutating command was accepted while one was running")
			return
		}
		// BUSY, not a queue. Queueing would mean a user's second request runs at an
		// unpredictable later moment against a screen they are no longer looking at.
		if !contains(err.Error(), "BUSY") {
			t.Errorf("err = %v, want a BUSY refusal", err)
		}
	})
}

func TestCancellationIsNotReportedAsATimeout(t *testing.T) {
	// Two different facts. A timeout says the Director gave up waiting; a cancellation
	// says the user asked. Reporting the first when the second happened invents a
	// fault, and reporting the second when the first happened hides one.
	block, dir, _ := blockedService(t)
	done := startBlocked(t, dir, block)

	c := dialQuiet(dir)
	if _, err := c.Cancel(); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	c.Close()

	select {
	case out := <-done:
		if out.State == CommandTimedOut {
			t.Fatal("a cancellation was classified as a timeout")
		}
		if out.State != CommandCancelled {
			t.Fatalf("state = %s, want cancelled", out.State)
		}
	case <-time.After(controlPlaneBound):
		close(block.Release)
		t.Fatal("the cancelled command never finished")
	}
}

// dialQuiet connects without a *testing.T, for use inside goroutines where t.Fatal
// would be a race.
func dialQuiet(dir string) *Client {
	ep, ok := ReadEndpoint(dir)
	if !ok {
		panic("responsiveness test: no endpoint was published")
	}
	c, err := Dial(ep, 2*time.Second)
	if err != nil {
		panic("responsiveness test could not connect: " + err.Error())
	}
	return c
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
