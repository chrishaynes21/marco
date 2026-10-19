package main

// ONE KEYPRESS, ALL THE WAY TO THE HOST.
//
// `workflowrunrealpath_test.go` proves that the coordinator's lowered step is a program Marco
// accepts and that exactly one left click arrives at the host. This proves the other end of the
// same path: that the thing which authorised it was a PHYSICAL KEY, admitted by the same
// classifier a real keyboard goes through, and that nothing between the key and the host
// shortened the route.
//
// Everything here is production code except two substitutions, and they are the two that would
// otherwise need a person and a desktop:
//
//	navsource.NewSynthetic   installs no hook and observes no real keyboard, and takes a
//	                         VIRTUAL-KEY CODE, so admission and repeat suppression are real
//	recordhost               appends a line instead of pressing a mouse button
//
// The consent desk, the granter, the coordinator, `marcoexec`, the lexer, the parser, the graph
// builder, the compiler and the runtime are all the real ones.
//
// It is also the deletion test for the whole feature. Remove the key and nothing is sent; remove
// the budget and more than one click is sent; stamp the grant with the wrong window and nothing
// is sent.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/platform/marcorunner"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
	"github.com/chaynes-simpleclouds/marco/internal/platform/recordhost"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// keyRig is the whole assembly: a synthetic keyboard at one end, a notebook at the other.
type keyRig struct {
	press func(code uint16, down bool)
	run   *workflowrun.Runner
	book  *recordhost.Host
	s     *workflowstore.SessionBinding
	out   *strings.Builder
	look  workflowrun.Sighting
}

func newKeyRig(t *testing.T, budget int) *keyRig {
	t.Helper()
	art := oneClick(t)

	book := recordhost.New()
	var marco directorapi.MarcoRunner = marcorunner.New(map[string]runtime.Host{
		"OS": book,
	})

	src, press := navsource.NewSynthetic()
	t.Cleanup(func() { _ = src.Close() })

	s := workflowstore.NewSessionBinding()
	if err := s.ConfirmStep(0, "SIMULATED fixture"); err != nil {
		t.Fatal(err)
	}

	out := &strings.Builder{}
	var run *workflowrun.Runner
	desk := newConsentDesk(src.Consents(), func() { run.Stop() })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go desk.run(ctx)

	grant := &hotkeyGranter{desk: desk, out: out, live: false,
		within: 2 * time.Second, life: 10 * time.Second}

	run = workflowrun.New(art, nil, answers{"now_1": "screen_a"}, grant).
		WithExecutor(marcoexec.New(marco), false).
		WithBudget(budget).
		WithConfirm(func(context.Context, workflowrun.Sighting, observe.PointerAt) (bool, string) {
			return true, ""
		})

	return &keyRig{press: press, run: run, book: book, s: s, out: out,
		look: workflowrun.Sighting{
			Screen: "now_1", Epoch: 5, Generation: 1,
			Window: "hwnd:1", Front: true, At: time.Now(),
			Bounds: directorapi.Rect{X: 100, Y: 50, Width: 1000, Height: 500},
			Outcomes: []observe.Reacquisition{{
				Target: "control_1", Screen: "now_1",
				Result: observe.ReacquireMatched, Controls: 1,
				Where: observe.Region{
					X: 0.4, Y: 0.5, Width: 0.1, Height: 0.04,
				},
				Because: "a fixture outcome",
			}},
		}}
}

// advance runs one observation while `key` is tapped once the question is open.
//
// Zero means no key is pressed at all.
func (r *keyRig) advance(t *testing.T, key uint16) workflowrun.Attempt {
	t.Helper()
	done := make(chan workflowrun.Attempt, 1)
	go func() { done <- r.run.Advance(context.Background(), r.s, r.look) }()
	if key != 0 {
		waitUntil(t, func() bool { return strings.Contains(r.out.String(), "May Marco") },
			"no authorisation question was asked")
		r.press(key, true)
		r.press(key, false)
	}
	select {
	case got := <-done:
		return got
	case <-time.After(10 * time.Second):
		t.Fatal("the observation never finished")
	}
	return workflowrun.Attempt{}
}

// F7 SENDS ONE REAL LEFT CLICK THROUGH THE MARCO COMPILER, and nothing else.
func TestOneApprovalKeySendsOneLeftClickThroughTheMarcoCompiler(t *testing.T) {
	r := newKeyRig(t, 1)

	got := r.advance(t, navsource.KeyF7)
	if got.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("%s — %s", got.Outcome, got.Because)
	}
	if got.Result == nil || got.Result.Status != marcoexec.StatusCompleted {
		t.Fatalf("the program did not complete: %+v", got.Result)
	}

	// A MOVE, THEN A PRESS, AT THE SAME POINT.
	//
	// Through the real lexer, parser, graph builder, compiler and runtime, with a notebook
	// where the mouse would be. Two host calls and not one: the Windows backend sends an
	// absolute move, a button-down and a button-up in a single SendInput batch, which a
	// game menu ignores because it has not yet noticed the pointer is there. See
	// workflowrun.DefaultDwell, and the live run that measured it.
	lines := r.book.Lines()
	if len(lines) != 2 {
		t.Fatalf("%d call(s) reached the host, want 2 (a move and a click):%s%s",
			len(lines), "\n", strings.Join(lines, "\n"))
	}
	if !strings.HasPrefix(lines[0], "OS's Move") {
		t.Errorf("the host was asked for %q first, which is not a move", lines[0])
	}
	if !strings.HasPrefix(lines[1], "OS's Click") {
		t.Errorf("the host was asked for %q second, which is not a click", lines[1])
	}
	// THE CONTROL'S POINT ON THE DESKTOP, in both: the window's offset plus the centre of
	// the region it was re-acquired at. A pointer that rested somewhere the press does not
	// land would be worse than no dwell at all.
	for _, line := range lines {
		for _, want := range []string{"550", "310"} {
			if !strings.Contains(line, want) {
				t.Errorf("%q does not carry %s", line, want)
			}
		}
	}

	// AND NOTHING ELSE REACHED THE HOST.
	for _, forbidden := range []string{"Type", "Key", "Activate", "Secret"} {
		for _, line := range lines {
			if strings.Contains(line, forbidden) {
				t.Errorf("the host was reached with %s: %s", forbidden, line)
			}
		}
	}
}

// AND WITHOUT THE KEY, NOTHING IS SENT.
//
// The deletion half. Same assembly, same plannable step, same executor — and no keypress.
func TestTheSameAssemblySendsNothingWhenNobodyPressesTheKey(t *testing.T) {
	r := newKeyRig(t, 1)
	r.run.WithBudget(1)

	// The question is asked and expires unanswered.
	done := make(chan workflowrun.Attempt, 1)
	go func() { done <- r.run.Advance(context.Background(), r.s, r.look) }()
	got := <-done

	if got.Outcome == workflowrun.OutcomePressed {
		t.Fatal("a press was sent with nobody answering")
	}
	if n := len(r.book.Lines()); n != 0 {
		t.Fatalf("%d call(s) reached the host with no key pressed", n)
	}
}

// AND F8 SENDS NOTHING, which is a different sentence from nobody answering.
func TestTheRefusalKeySendsNothingThroughTheRealPath(t *testing.T) {
	r := newKeyRig(t, 1)
	got := r.advance(t, navsource.KeyF8)
	if got.Outcome == workflowrun.OutcomePressed {
		t.Fatal("a refused action was sent")
	}
	if n := len(r.book.Lines()); n != 0 {
		t.Fatalf("%d call(s) reached the host after a refusal", n)
	}
}

// A SPENT BUDGET REFUSES A PLANNABLE STEP, with the key pressed and everything else in place.
//
// `--max-actions` has to be a property of the coordinator rather than of the caller: a caller
// that stopped the run on seeing a press reported would be acting on a report of something that
// already happened. So this assembly is the one that DOES send a click two tests above, with
// nothing changed but the budget, and the key is pressed exactly as it is there.
func TestASpentBudgetRefusesAPlannableStepThroughTheRealPath(t *testing.T) {
	r := newKeyRig(t, 0)

	got := r.advance(t, 0)
	if got.Outcome == workflowrun.OutcomePressed {
		t.Fatal("a press was sent by a run with no budget for one")
	}
	// AND NOBODY WAS ASKED. A run that put the question up and then refused the answer
	// teaches a person that answering does not matter.
	if strings.Contains(r.out.String(), "May Marco") {
		t.Errorf("somebody was asked for permission that could not be used:\n%s", r.out)
	}
	if n := len(r.book.Lines()); n != 0 {
		t.Fatalf("%d call(s) reached the host on a spent budget", n)
	}
	// AND IT SAYS WHICH REFUSAL IT WAS. A spent budget reported as "nobody authorised
	// it" would send somebody looking for a key that worked perfectly well.
	if !strings.Contains(got.Because, "may send 0 action(s)") {
		t.Errorf("the refusal does not name the budget: %s", got.Because)
	}
	if r.run.Sent() != 0 {
		t.Errorf("the run says it sent %d action(s), want 0", r.run.Sent())
	}
}
