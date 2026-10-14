package main

// THE WHOLE PATH, WITH ITS LAST STEP REPLACED BY A NOTEBOOK.
//
// `internal/director/workflowrun` is tested against a `directorapi.MarcoRunner` that writes the
// program down, which proves the coordinator's refusals and its arithmetic and cannot prove that
// the program it produces is legal Marco — the Director may not import the compiler, so from
// inside that package the real front end is out of reach.
//
// This is the other half, here because this is where the halves are allowed to meet. The runner
// is the production `marcorunner`, so the program goes through `lexer.Lex`, `parser.Parse`,
// `graph.Build`, `compile.Compile` and the real runtime; the only substitution is the Host at the
// very bottom, and `recordhost` appends a line instead of pressing a mouse button.
//
// That is the whole design of the seam:
//
//	a dry run that took a shortcut past the compiler would prove that the shortcut works.
//
// So what this establishes is narrow and worth having: the coordinator's lowered step is a
// program Marco accepts, and what arrives at the host is one left click at the point the
// workflow's control was re-acquired at — not a key, not a type, not two clicks.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/platform/marcorunner"
	"github.com/chaynes-simpleclouds/marco/internal/platform/recordhost"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// oneClick is a saved workflow of a single demonstrated press.
func oneClick(t *testing.T) workflowstore.Artifact {
	t.Helper()
	d := workflow.Draft{Name: "enter free play"}
	d.Steps = []workflow.Step{{
		From: "visual_1", To: "visual_2", Target: "target_1",
		Cause: observe.CauseClick, Seen: 2,
		Readings: []observe.Region{{X: 0.4, Y: 0.5, Width: 0.1, Height: 0.04}},
	}}
	a, err := workflowstore.Save(d, map[observe.VisualScreenID]observe.VisualArrangement{
		"visual_1": {Cells: map[string]int{"0,0": 9}, Total: 9},
		"visual_2": {Cells: map[string]int{"1,1": 7}, Total: 7},
	}, time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// answers stands where a person looking at the screen would. SIMULATED.
type answers map[observe.VisualScreenID]string

func (o answers) Establish(ctx context.Context, s *workflowstore.SessionBinding,
	at workflowrun.Sighting) error {

	label, ok := o[at.Screen]
	if !ok {
		return nil
	}
	return s.Confirm(label, at.Screen, at.Epoch, at.Generation, "SIMULATED fixture", at.At)
}

// permits authorises what it is asked, in the observation it was asked in. SIMULATED.
type permits struct {
	epoch, generation uint64
	asked             []workflowrun.Intent
}

func (p *permits) Authorise(ctx context.Context, in workflowrun.Intent) (
	workflowrun.Authorisation, bool) {

	p.asked = append(p.asked, in)
	return workflowrun.Authorisation{
		Step: in.Step, By: "SIMULATED fixture",
		Epoch: p.epoch, Generation: p.generation,
		Expires: time.Now().Add(time.Minute),
	}, true
}

func TestAWorkflowStepReachesTheHostAsOneRealLeftClickThroughTheMarcoCompiler(t *testing.T) {
	art := oneClick(t)

	// THE PRODUCTION RUNNER, with a notebook where the mouse would be.
	book := recordhost.New()
	var marco directorapi.MarcoRunner = marcorunner.New(map[string]runtime.Host{
		"OS": book,
	})

	s := workflowstore.NewSessionBinding()
	if err := s.ConfirmStep(0, "SIMULATED fixture"); err != nil {
		t.Fatal(err)
	}
	grant := &permits{epoch: 5, generation: 1}
	r := workflowrun.New(art, nil, answers{"now_1": "screen_a"}, grant).
		WithExecutor(marcoexec.New(marco), false).
		WithConfirm(func(context.Context, workflowrun.Sighting) (bool, string) {
			return true, ""
		})

	look := workflowrun.Sighting{
		Screen: "now_1", Epoch: 5, Generation: 1,
		Window: "hwnd:1", Front: true, At: time.Now(),
		// An offset window, so a coordinator that dropped the offset would send a
		// different click and this would see it.
		Bounds: directorapi.Rect{X: 100, Y: 50, Width: 1000, Height: 500},
		Outcomes: []observe.Reacquisition{{
			Target: "control_1", Screen: "now_1",
			Result: observe.ReacquireMatched, Controls: 1,
			Where:   observe.Region{X: 0.4, Y: 0.5, Width: 0.1, Height: 0.04},
			Because: "a fixture outcome",
		}},
	}

	got := r.Advance(context.Background(), s, look)
	if got.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("%s — %s", got.Outcome, got.Because)
	}
	// THE PROGRAM COMPILED AND RAN. A compile failure would be StatusCompileFailed, and
	// the point of running it through the real front end is that this can tell them apart.
	if got.Result == nil || got.Result.Status != marcoexec.StatusCompleted {
		t.Fatalf("the program did not complete: %+v", got.Result)
	}

	// AND EXACTLY ONE LEFT CLICK, AT THE CONTROL, ARRIVED AT THE HOST.
	lines := book.Lines()
	if len(lines) != 1 {
		t.Fatalf("%d call(s) reached the host, want 1:\n%s", len(lines),
			strings.Join(lines, "\n"))
	}
	if !strings.HasPrefix(lines[0], "OS's Click") {
		t.Errorf("the host was asked for %q, which is not a click", lines[0])
	}
	for _, want := range []string{"550", "310"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("the click does not carry %s — want the control's centre on "+
				"the desktop: %s", want, lines[0])
		}
	}
	if dropped, over := book.Overflowed(); over {
		t.Errorf("the record overflowed, dropping %d call(s)", dropped)
	}

	// AND THE PERSON WAS SHOWN THAT POINT BEFORE THEY PERMITTED IT.
	if len(grant.asked) != 1 {
		t.Fatalf("somebody was asked %d time(s)", len(grant.asked))
	}
	if at := grant.asked[0].At; at.X != 550 || at.Y != 310 {
		t.Errorf("they authorised a press at %v and (550,310) was sent", at)
	}
}

// AND WITH NO AUTHORISATION THE SAME PATH SENDS NOTHING AT ALL.
//
// The same assembly, one thing removed. Without it the test above would establish that the
// coordinator can reach the host and not that anything stands between them.
func TestTheSameRealPathSendsNothingWhenNobodyAuthorisesIt(t *testing.T) {
	art := oneClick(t)
	book := recordhost.New()
	var marco directorapi.MarcoRunner = marcorunner.New(map[string]runtime.Host{"OS": book})

	s := workflowstore.NewSessionBinding()
	if err := s.ConfirmStep(0, "SIMULATED fixture"); err != nil {
		t.Fatal(err)
	}
	r := workflowrun.New(art, nil, answers{"now_1": "screen_a"}, nil).
		WithExecutor(marcoexec.New(marco), false).
		WithConfirm(func(context.Context, workflowrun.Sighting) (bool, string) {
			return true, ""
		})

	got := r.Advance(context.Background(), s, workflowrun.Sighting{
		Screen: "now_1", Epoch: 5, Generation: 1, Window: "hwnd:1", Front: true,
		At: time.Now(), Bounds: directorapi.Rect{X: 100, Y: 50, Width: 1000, Height: 500},
		Outcomes: []observe.Reacquisition{{
			Target: "control_1", Screen: "now_1",
			Result: observe.ReacquireMatched, Controls: 1,
			Where:   observe.Region{X: 0.4, Y: 0.5, Width: 0.1, Height: 0.04},
			Because: "a fixture outcome",
		}},
	})
	if got.Outcome != workflowrun.OutcomeRefused {
		t.Fatalf("%s — %s", got.Outcome, got.Because)
	}
	if lines := book.Lines(); len(lines) != 0 {
		t.Fatalf("%d call(s) reached the host with nobody authorising anything:\n%s",
			len(lines), strings.Join(lines, "\n"))
	}
}
