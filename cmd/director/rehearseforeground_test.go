package main

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The foreground gate, exercised rather than counted.
//
// # What was already held, and what it missed
//
// TestEveryLiveWalkerChecksTheForeground parses this package and requires every function that
// builds a `rehearse.NewLive` to also call `WithForeground`. That is a real property and it caught
// a real defect — there were two composition roots and only one installed the answer.
//
// It asserts that a METHOD IS CALLED. `WithForeground(nil)` calls it, compiles, and passes: the
// AST sees the call and cannot see the argument. And `Live.behind` reads
// `l.real && l.inFront != nil && !l.inFront(ref)`, so a nil answer does not weaken the gate, it
// removes it — the `window_not_in_front` refusal can never fire, and a live rehearsal's real
// keystrokes go to whatever window the person is actually looking at. An independent mutation run
// made exactly that change and both suites stayed green.
//
// So this enters through `Runtime.Observation` — the same request the CLI makes — with the
// platform's answer replaced, and asserts the REFUSAL. Both directions, because a gate that
// refuses unconditionally is not a gate either: it would make every live rehearsal impossible
// while looking, from a single negative test, exactly like a working one.

// oneRun is a Marco runner that records rather than acts.
//
// It stands in for `r.liveMarco` so the walker believes it can reach a computer — `real` is what
// turns the foreground check on — while nothing whatsoever is emitted to a desktop.
type oneRun struct {
	mu   sync.Mutex
	runs int
}

func (o *oneRun) Run(ctx context.Context, name, program string) (directorapi.MarcoResult, error) {
	o.mu.Lock()
	o.runs++
	o.mu.Unlock()
	return directorapi.MarcoResult{}, nil
}

func (o *oneRun) count() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.runs
}

// liveRehearsalAgainstForeground runs one authorized live rehearsal with the platform answering
// `leads` to "is the watched window in front".
func liveRehearsalAgainstForeground(t *testing.T, leads bool) (service.RehearsalView, *oneRun) {
	t.Helper()
	restore := windowLeads
	windowLeads = func(windowref.Ref) bool { return leads }
	t.Cleanup(func() { windowLeads = restore })

	runner := &oneRun{}
	rt := &Runtime{observations: authorizedRegistry(t), liveMarco: runner}
	out, err := rt.Observation(service.ObserveQuery{
		Rehearse: &service.ObserveRehearse{Step: 1, Live: true},
	})
	if err != nil {
		t.Fatalf("the live rehearsal request failed: %v", err)
	}
	view, ok := out.(service.RehearsalView)
	if !ok {
		t.Fatalf("the rehearsal request returned %T", out)
	}
	return view, runner
}

// A LIVE WALKER REFUSES WHEN THE WATCHED WINDOW IS NOT IN FRONT.
func TestALiveWalkerRefusesWhenTheWindowIsNotInFront(t *testing.T) {
	view, runner := liveRehearsalAgainstForeground(t, false)

	if view.Refusal != "window_not_in_front" {
		t.Errorf("a live rehearsal against a window that is NOT in front reported "+
			"refusal=%q terminal=%q attempted=%v.\n"+
			"Emitted input has no address: it goes to whatever window leads the "+
			"desktop. Without this refusal the keystrokes land in the window the "+
			"person is looking at — often the one they just said yes in — and Marco "+
			"then reports honestly that the screen did not respond.",
			view.Refusal, view.Terminal, view.Attempted)
	}
	if view.Attempted {
		t.Error("the refusal came AFTER something was emitted; the gate sits before the " +
			"claim so that nothing is spent and the person can simply click back")
	}
	if runner.count() != 0 {
		t.Errorf("%d program(s) reached the live runner despite the refusal", runner.count())
	}
}

// AND IT DOES NOT REFUSE WHEN THE WINDOW IS IN FRONT.
//
// The negative control. Without it, `behind` returning true unconditionally — a gate welded shut —
// would pass the test above and make every live rehearsal impossible.
func TestALiveWalkerProceedsWhenTheWindowLeads(t *testing.T) {
	view, _ := liveRehearsalAgainstForeground(t, true)

	if view.Refusal == "window_not_in_front" {
		t.Fatal("a live rehearsal was refused for the foreground even though the watched " +
			"window leads the desktop; the gate is welded shut and nothing can ever " +
			"be rehearsed for real")
	}
	if strings.Contains(strings.ToLower(view.Detail), "not in front") {
		t.Errorf("the attempt still complains about the foreground: %q", view.Detail)
	}
}
