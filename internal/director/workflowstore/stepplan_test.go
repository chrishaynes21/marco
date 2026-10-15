package workflowstore_test

// KNOWING THE WHOLE PLAN, AND TRUSTING ONLY THE SCREEN YOU ARE ON.
//
// Step-scoped planning is the change that lets a multi-screen workflow proceed at all, and it is
// therefore the change most able to let one proceed when it should not. Every refusal it can
// give is tested, and so is the one thing it must never become: permission.

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// threeScreens is a -> b -> c, each by a click, with one reading per control.
func threeScreens(t *testing.T) workflowstore.Artifact {
	t.Helper()
	d := workflow.Draft{Name: "open free play"}
	step := func(from, to observe.VisualScreenID, target observe.VisualTargetID,
		r observe.Region) workflow.Step {
		return workflow.Step{
			From: from, To: to, Target: target, Cause: observe.CauseClick, Seen: 2,
			Readings: []observe.Region{r},
		}
	}
	d.Steps = []workflow.Step{
		step("visual_1", "visual_2", "target_1", rect(100, 100, 200, 40)),
		step("visual_2", "visual_3", "target_2", rect(300, 300, 200, 40)),
	}
	a, err := workflowstore.Save(d, map[observe.VisualScreenID]observe.VisualArrangement{
		"visual_1": {Cells: map[string]int{"0,0": 9}, Total: 9},
		"visual_2": {Cells: map[string]int{"1,1": 7}, Total: 7},
		"visual_3": {Cells: map[string]int{"2,2": 5}, Total: 5},
	}, saved)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// found is a matched outcome for one control on one screen.
func found(control string, screen observe.VisualScreenID, r observe.Region) observe.Reacquisition {
	return observe.Reacquisition{
		Target: observe.VisualTargetID(control), Screen: screen,
		Result: observe.ReacquireMatched, Where: r, Controls: 1,
		Because: "a fixture outcome",
	}
}

// ready drives one step to StepPlannable, so each test can spoil exactly one thing.
func ready(t *testing.T, a workflowstore.Artifact) (*workflowstore.SessionBinding, workflowstore.Now) {
	t.Helper()
	s := workflowstore.NewSessionBinding()
	if err := s.Confirm("screen_a", "now_1", 5, 1, "SIMULATED", looked); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfirmStep(0, "SIMULATED"); err != nil {
		t.Fatal(err)
	}
	return s, workflowstore.Now{
		Screen: "now_1", Epoch: 5, Generation: 1, Authorised: true,
		Outcomes: []observe.Reacquisition{
			found("control_1", "now_1", rect(100, 100, 200, 40)),
		},
	}
}

// THE WHOLE POINT: a multi-screen workflow plans its first step without its later
// screens being established, which `Bind` could never allow.
func TestAMultiScreenWorkflowPlansItsFirstStepWithTheRestUnbound(t *testing.T) {
	a := threeScreens(t)
	s, now := ready(t, a)

	// Bind still refuses the workflow as a whole, and that is unchanged on purpose.
	if _, err := a.Bind(s.Binding(5, 1)); err == nil {
		t.Fatal("Bind accepted a workflow with two screens unestablished; its " +
			"all-or-nothing meaning has been weakened")
	}
	got := a.StepReadinessAt(0, s, now)
	if !got.Ready() {
		t.Fatalf("step 0 is %q: %s", got.Stage, got.Because)
	}
	if got.Action == nil || got.Action.Expect != "screen_b" {
		t.Errorf("the action is %+v", got.Action)
	}
	// AND THE NEXT STEP IS NOT PRE-AUTHORISED. Its screen is not established, and
	// knowing the plan is not standing on it.
	next := a.StepReadinessAt(1, s, now)
	if next.Stage != workflowstore.StepScreenUnestablished {
		t.Errorf("step 1 is %q; a later screen has been granted authority early",
			next.Stage)
	}
}

// THE FIRST SCREEN EXPIRES BEFORE THE SECOND IS REACHED — which is what made the old
// contract impossible, and is ordinary here.
func TestTheFirstScreensBindingExpiresAndTheSecondStepStillPlans(t *testing.T) {
	a := threeScreens(t)
	s, _ := ready(t, a)

	// The observation moves: step 0's screen is gone.
	s.Observed(6, 1)
	if _, ok := s.Established("screen_a", 6, 1); ok {
		t.Fatal("the first screen survived the observation moving")
	}
	stale := a.StepReadinessAt(0, s, workflowstore.Now{
		Screen: "now_1", Epoch: 6, Generation: 1, Authorised: true,
		Outcomes: []observe.Reacquisition{found("control_1", "now_1", rect(100, 100, 200, 40))},
	})
	if stale.Stage != workflowstore.StepScreenUnestablished {
		t.Errorf("step 0 is still %q after its observation ended", stale.Stage)
	}

	// Now establish the SECOND screen in the new observation and plan step 1.
	if err := s.Confirm("screen_b", "now_2", 6, 1, "SIMULATED", looked); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfirmStep(1, "SIMULATED"); err != nil {
		t.Fatal(err)
	}
	got := a.StepReadinessAt(1, s, workflowstore.Now{
		Screen: "now_2", Epoch: 6, Generation: 1, Authorised: true,
		Outcomes: []observe.Reacquisition{found("control_2", "now_2", rect(300, 300, 200, 40))},
	})
	if !got.Ready() {
		t.Fatalf("step 1 is %q: %s", got.Stage, got.Because)
	}
}

// A STEP ON AN UNESTABLISHED SCREEN.
func TestAStepOnAnUnestablishedScreenIsRefused(t *testing.T) {
	a := threeScreens(t)
	s := workflowstore.NewSessionBinding()
	if err := s.ConfirmStep(0, "SIMULATED"); err != nil {
		t.Fatal(err)
	}
	got := a.StepReadinessAt(0, s, workflowstore.Now{
		Screen: "now_1", Epoch: 5, Generation: 1, Authorised: true,
		Outcomes: []observe.Reacquisition{found("control_1", "now_1", rect(100, 100, 200, 40))},
	})
	if got.Stage != workflowstore.StepScreenUnestablished {
		t.Fatalf("%q: %s", got.Stage, got.Because)
	}
	if got.Action != nil {
		t.Error("an unestablished step produced an action")
	}
	// AND THE REFUSAL BLAMES THE RIGHT THING. “Nothing is established about this”
	// and “it is established as something else” are different findings, and a
	// reader chasing the second when it was the first learns nothing.
	if !strings.Contains(got.Because, "is not established in this observation") {
		t.Errorf("the refusal says %q, which describes a screen bound to "+
			"something else rather than one nobody has established", got.Because)
	}
}

// THE SCREEN CHANGES BETWEEN RE-ACQUISITION AND LOWERING.
//
// The outcome in hand was computed on a screen that is no longer the one standing, and a
// planner that only checked "is something established" would use it.
func TestAScreenThatChangedSinceReacquisitionIsRefused(t *testing.T) {
	a := threeScreens(t)
	s, now := ready(t, a)
	now.Screen = "now_9" // the screen moved; the binding still names now_1
	got := a.StepReadinessAt(0, s, now)
	if got.Stage != workflowstore.StepScreenUnestablished {
		t.Fatalf("%q: %s", got.Stage, got.Because)
	}
	if !strings.Contains(got.Because, "now_9") {
		t.Errorf("the refusal does not say what is standing instead: %s", got.Because)
	}
}

// A PREVIOUSLY ESTABLISHED SCREEN, REVISITED IN A NEW GENERATION.
func TestARevisitedScreenNeedsEstablishingAgain(t *testing.T) {
	a := threeScreens(t)
	s, _ := ready(t, a)
	s.Observed(6, 1) // away
	s.Observed(7, 1) // and back, same screen, new observation
	got := a.StepReadinessAt(0, s, workflowstore.Now{
		Screen: "now_1", Epoch: 7, Generation: 1, Authorised: true,
		Outcomes: []observe.Reacquisition{found("control_1", "now_1", rect(100, 100, 200, 40))},
	})
	if got.Stage != workflowstore.StepScreenUnestablished {
		t.Fatalf("a revisit reused the earlier answer: %q", got.Stage)
	}
	if !strings.Contains(got.Because, "not evidence now") {
		t.Errorf("the refusal does not say why: %s", got.Because)
	}
}

// A STEP WHOSE CONTROL WAS NEVER SETTLED.
//
// Reported as a clarification, not as a matcher failure — the two send a reader to different
// places, and only one of them has an answer somebody can give.
func TestAStepWithAnUnresolvedClarificationSaysSo(t *testing.T) {
	label := rect(868, 620, 170, 14)
	button := rect(802, 610, 352, 34)
	d := workflow.Draft{Name: "exit"}
	d.Steps = []workflow.Step{{
		From: "visual_1", To: "visual_2", Target: "target_1",
		Cause: observe.CauseClick, Seen: 1,
		Readings: []observe.Region{label}, Containers: []observe.Region{button},
	}}
	a, err := workflowstore.Save(d, nil, saved)
	if err != nil {
		t.Fatal(err)
	}
	s := workflowstore.NewSessionBinding()
	if err := s.Confirm("screen_a", "now_1", 5, 1, "SIMULATED", looked); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfirmStep(0, "SIMULATED"); err != nil {
		t.Fatal(err)
	}
	got := a.StepReadinessAt(0, s, workflowstore.Now{
		Screen: "now_1", Epoch: 5, Generation: 1, Authorised: true,
		Outcomes: []observe.Reacquisition{found("control_1", "now_1", label)},
	})
	if got.Stage != workflowstore.StepNeedsClarification {
		t.Fatalf("%q: %s", got.Stage, got.Because)
	}
	if got.Action != nil {
		t.Error("an unclarified control produced an action")
	}
}

// AN UNCONFIRMED STEP, AND AN UNAUTHORISED ONE. Separate refusals, separate words.
func TestConfirmationAndAuthorisationAreSeparateRefusals(t *testing.T) {
	a := threeScreens(t)

	// Confirmed by nobody.
	s := workflowstore.NewSessionBinding()
	if err := s.Confirm("screen_a", "now_1", 5, 1, "SIMULATED", looked); err != nil {
		t.Fatal(err)
	}
	now := workflowstore.Now{
		Screen: "now_1", Epoch: 5, Generation: 1, Authorised: true,
		Outcomes: []observe.Reacquisition{found("control_1", "now_1", rect(100, 100, 200, 40))},
	}
	if got := a.StepReadinessAt(0, s, now); got.Stage != workflowstore.StepUnconfirmed {
		t.Errorf("unconfirmed step is %q: %s", got.Stage, got.Because)
	}

	// Confirmed, and the session may not act.
	s2, now2 := ready(t, a)
	now2.Authorised = false
	got := a.StepReadinessAt(0, s2, now2)
	if got.Stage != workflowstore.StepUnauthorised {
		t.Fatalf("unauthorised step is %q: %s", got.Stage, got.Because)
	}
	if got.Action != nil {
		t.Error("an unauthorised step produced an action")
	}
	if !strings.Contains(got.Because, "confirmed") {
		t.Errorf("the refusal does not distinguish itself from an unconfirmed one: %s",
			got.Because)
	}
}

// A CONTROL THAT CANNOT BE SEEN IS NOT A CONTROL THAT IS NOT THERE.
func TestAnAmbiguousOutcomeRefusesTheStep(t *testing.T) {
	a := threeScreens(t)
	s, now := ready(t, a)
	now.Outcomes = []observe.Reacquisition{{
		Target: "control_1", Screen: "now_1",
		Result: observe.ReacquireAmbiguous, Controls: 2,
		Because: "two proposals describe different controls",
	}}
	got := a.StepReadinessAt(0, s, now)
	if got.Stage != workflowstore.StepControlUnresolved {
		t.Fatalf("%q: %s", got.Stage, got.Because)
	}
	if !strings.Contains(got.Because, "ambiguous") {
		t.Errorf("the refusal loses the matcher's own verdict: %s", got.Because)
	}
}

// AN OUTCOME ABOUT A DIFFERENT CONTROL IS NOT THIS STEP'S EVIDENCE.
//
// A look offers an outcome per demonstrated control, and taking the first one would press
// whichever control happened to be listed first.
func TestAnOutcomeForAnotherControlIsNotUsed(t *testing.T) {
	a := threeScreens(t)
	s, now := ready(t, a)
	// Only the SECOND step's control is offered, matched and tempting.
	now.Outcomes = []observe.Reacquisition{
		found("control_2", "now_1", rect(300, 300, 200, 40)),
	}
	got := a.StepReadinessAt(0, s, now)
	if got.Stage != workflowstore.StepControlUnresolved {
		t.Fatalf("%q: %s", got.Stage, got.Because)
	}
	if got.Action != nil {
		t.Error("step 0 planned an action from another control's outcome")
	}
}

// A WAIT IS NOT A THING TO PRESS.
func TestAWaitStepIsNotPlannable(t *testing.T) {
	d := workflow.Draft{Name: "with a wait"}
	d.Steps = []workflow.Step{{
		From: "visual_1", To: "visual_2", Cause: observe.CauseAutomatic, Seen: 1,
	}}
	a, err := workflowstore.Save(d, nil, saved)
	if err != nil {
		t.Fatal(err)
	}
	s := workflowstore.NewSessionBinding()
	if err := s.Confirm("screen_a", "now_1", 5, 1, "SIMULATED", looked); err != nil {
		t.Fatal(err)
	}
	got := a.StepReadinessAt(0, s, workflowstore.Now{
		Screen: "now_1", Epoch: 5, Generation: 1, Authorised: true,
	})
	if got.Stage != workflowstore.StepNotRepeatable {
		t.Fatalf("a wait is %q: %s", got.Stage, got.Because)
	}
	if got.Action != nil {
		t.Error("a wait produced an action to press")
	}
	if got.Expect == "" {
		t.Error("a wait has no destination, so nothing could verify its arrival")
	}
}

// PARTIAL PROGRESS IS NEITHER SUCCESS NOR FAILURE, AND SAYS SO.
func TestStoppingPartwayIsItsOwnOutcome(t *testing.T) {
	for _, c := range []struct {
		name     string
		p        workflowstore.Progress
		complete bool
		partial  bool
		says     string
	}{
		{"finished", workflowstore.Progress{Steps: 3, Reached: 3, Stopped: -1},
			true, false, "all 3"},
		{"stopped partway", workflowstore.Progress{Steps: 3, Reached: 2, Stopped: 2,
			Because: "the screen is not established"}, false, true, "STOPPED SAFELY"},
		{"never started", workflowstore.Progress{Steps: 3, Reached: 0, Stopped: 0,
			Because: "the first screen is not established"}, false, false,
			"before the first step"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if c.p.Complete() != c.complete || c.p.Partial() != c.partial {
				t.Errorf("complete=%t partial=%t", c.p.Complete(), c.p.Partial())
			}
			if !strings.Contains(c.p.Summary(), c.says) {
				t.Errorf("summary %q does not say %q", c.p.Summary(), c.says)
			}
		})
	}
}

// AND THE THING IT MUST NEVER BECOME.
//
// `StepPlannable` is a point and an expectation. Nothing in this package lowers it, and nothing
// it returns can be pressed — the lowering path is not reachable from here at all, which the
// package's own import walk holds.
func TestPlannableIsNotPermission(t *testing.T) {
	a := threeScreens(t)
	s, now := ready(t, a)
	got := a.StepReadinessAt(0, s, now)
	if !got.Ready() {
		t.Fatalf("%q", got.Stage)
	}
	// The action carries a point and an expectation, and a support that is geometric —
	// never semantic, because nothing here knows what the control means.
	if got.Action.Support != workflow.SupportGeometric {
		t.Errorf("the action claims %s support", got.Action.Support)
	}
	if got.Action.Expect == "" {
		t.Error("the action has no destination, so nothing could verify it")
	}
	// And the same step on a session that is not authorised is refused, which is the
	// only difference between a plan and a press.
	now.Authorised = false
	if again := a.StepReadinessAt(0, s, now); again.Ready() {
		t.Error("the same step is plannable without authorisation")
	}
	_ = time.Now
}

// THE CONTROL'S POSITION IS KNOWN BEFORE ANYBODY IS ASKED TO PERMIT PRESSING IT.
//
// Authorisation is the LAST refusal, so at the moment somebody is asked for it the step is not
// plannable and `Action` is nil. A caller building the question from `Action` alone shows them a
// press at (0,0) — which is not an explicit authorisation of anything. `Found` carries the point
// from the moment the control is re-acquired, and it is the same point `Action` gets.
func TestTheControlsPositionIsKnownBeforeAuthorisationIsAskedFor(t *testing.T) {
	a := threeScreens(t)
	s, now := ready(t, a)

	now.Authorised = false
	unauthorised := a.StepReadinessAt(0, s, now)
	if unauthorised.Stage != workflowstore.StepUnauthorised {
		t.Fatalf("stage %q — the fixture is not sitting on the authorisation refusal",
			unauthorised.Stage)
	}
	if unauthorised.Action != nil {
		t.Fatal("an unauthorised step has an Action, which is what means plannable")
	}
	if unauthorised.Found == nil {
		t.Fatal("the control's position is unknown at the moment somebody would be " +
			"asked to permit pressing it, so they could only be shown (0,0)")
	}

	now.Authorised = true
	plannable := a.StepReadinessAt(0, s, now)
	if !plannable.Ready() {
		t.Fatalf("stage %q", plannable.Stage)
	}
	// THE SAME POINT, not a second computation of it.
	if *unauthorised.Found != plannable.Action.At {
		t.Errorf("the point somebody authorised (%v) is not the point the step would "+
			"press (%v)", *unauthorised.Found, plannable.Action.At)
	}
	if got := *plannable.Found; got != plannable.Action.At {
		t.Errorf("Found %v and Action.At %v disagree", got, plannable.Action.At)
	}
}

// AND A STEP WHOSE CONTROL WAS FOUND AS A BAND ACROSS THE WINDOW HAS NO POSITION EITHER.
//
// A different refusal from "not re-acquired", and the difference is what a person should do
// about it. Not re-acquired means look again: the control may be mid-animation. This one never
// resolves — the saved target runs edge to edge, so every future look matching it offers the
// same centre, which is the middle of the window.
//
// Measured rather than imagined. A saved Rocket League workflow carried nine controls and four
// of them were bands of exactly this shape, three covering the window almost entirely. All four
// re-acquired perfectly and all four offered (960,517) on a 1920x1080 window.
func TestAStepWhoseControlIsABandAcrossTheWindowIsNotPlannable(t *testing.T) {
	for _, tc := range []struct {
		name  string
		where observe.Region
	}{
		{"the whole window", observe.Region{X: 0, Y: 0, Width: 1, Height: 0.957}},
		{"the bottom half, full width",
			observe.Region{X: 0, Y: 0.468, Width: 1, Height: 0.461}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := threeScreens(t)
			s, now := ready(t, a)
			now.Authorised = true
			now.Outcomes[0].Where = tc.where

			got := a.StepReadinessAt(0, s, now)
			if got.Stage != workflowstore.StepControlUnlocated {
				t.Fatalf("stage %q, want %q", got.Stage,
					workflowstore.StepControlUnlocated)
			}
			// NO POINT, ANYWHERE. Neither one to press nor one to show
			// somebody while asking them to permit pressing it.
			if got.Action != nil {
				t.Errorf("the step is plannable at %v", got.Action.At)
			}
			if got.Found != nil {
				t.Errorf("the step reports its control at %v", *got.Found)
			}
			if !strings.Contains(got.Because, "no substitute") {
				t.Errorf("the refusal does not say a point will not be "+
					"invented: %s", got.Because)
			}
		})
	}
}

// AND A STEP THAT NEVER FOUND ITS CONTROL HAS NO POSITION TO SHOW.
func TestAStepThatCouldNotFindItsControlHasNoPosition(t *testing.T) {
	a := threeScreens(t)
	s, now := ready(t, a)
	now.Outcomes = nil

	got := a.StepReadinessAt(0, s, now)
	if got.Stage != workflowstore.StepControlUnresolved {
		t.Fatalf("stage %q", got.Stage)
	}
	if got.Found != nil {
		t.Errorf("a step whose control was never found reports it at %v", *got.Found)
	}
}

// THE POINT IS THE MIDDLE OF WHAT BOTH DESCRIBE, NOT OF WHAT WAS MATCHED.
//
// Measured. A live look matched Rocket League's PLAY button with a region running from the
// window's left edge to 132px in; the saved reading was 26..192; the button is 62..370. The
// middle of the match was x 66, three pixels inside the button's border, and the click did
// nothing. The middle of the agreed part is x 79, the first letter of PLAY.
func TestThePointIsTheMiddleOfWhatBothTheFileAndTheLookDescribe(t *testing.T) {
	a := threeScreens(t)
	s, now := ready(t, a)
	now.Authorised = true
	saved := a.Controls[0].Readings[0]

	// A MATCH THAT SPILLS PAST THE CONTROL, which is what re-acquisition is generous
	// enough to produce and what makes its middle misleading.
	now.Outcomes[0].Where = observe.Region{
		X: saved.X + saved.Width/2, Y: saved.Y,
		Width: saved.Width, Height: saved.Height,
	}
	got := a.StepReadinessAt(0, s, now)
	if !got.Ready() {
		t.Fatalf("stage %q — %s", got.Stage, got.Because)
	}
	// The agreed part runs from the saved reading's left edge to the match's right edge.
	wantX := saved.X + saved.Width*0.75
	if d := got.Action.At.X - wantX; d > 0.0001 || d < -0.0001 {
		t.Errorf("the point is x %.4f and the middle of the agreed part is %.4f",
			got.Action.At.X, wantX)
	}
	// AND THE MIDDLE OF THE MATCH IS NOT WHERE IT PRESSES.
	mid := now.Outcomes[0].Where.X + now.Outcomes[0].Where.Width/2
	if d := got.Action.At.X - mid; d < 0.0001 && d > -0.0001 {
		t.Error("the point is the middle of the match, which is the thing that missed")
	}
}

// AND A MATCH THE SAVED READINGS DO NOT TOUCH IS A CONTRADICTION, not a point.
func TestAMatchTheSavedReadingsDoNotTouchIsRefused(t *testing.T) {
	a := threeScreens(t)
	s, now := ready(t, a)
	now.Authorised = true
	now.Outcomes[0].Where = observe.Region{X: 0.8, Y: 0.8, Width: 0.05, Height: 0.05}

	got := a.StepReadinessAt(0, s, now)
	if got.Ready() {
		t.Fatalf("a match nowhere near the saved readings was plannable at %v",
			got.Action.At)
	}
	if got.Stage != workflowstore.StepControlUnlocated {
		t.Errorf("stage %q, want %q", got.Stage, workflowstore.StepControlUnlocated)
	}
	if !strings.Contains(got.Because, "disagree") {
		t.Errorf("the refusal does not say the two disagree: %s", got.Because)
	}
	if got.Found != nil {
		t.Errorf("a contradiction still reports a position: %v", *got.Found)
	}
}

// AND A WINDOW-SPANNING MATCH IS STILL REFUSED, rather than narrowed into a grounded-looking one.
//
// The order matters: a region that spans the window overlaps EVERY saved reading, so agreeing
// with it first would hand back the saved reading and present a recorded coordinate as something
// this look found. A match that locates nothing corroborates nothing.
func TestAWindowSpanningMatchIsNotNarrowedIntoAGroundedPoint(t *testing.T) {
	a := threeScreens(t)
	s, now := ready(t, a)
	now.Authorised = true
	now.Outcomes[0].Where = observe.Region{X: 0, Y: 0, Width: 1, Height: 0.957}

	got := a.StepReadinessAt(0, s, now)
	if got.Ready() {
		t.Fatalf("a window-spanning match was narrowed into a plannable point at %v",
			got.Action.At)
	}
	if !strings.Contains(got.Because, "full width") {
		t.Errorf("the refusal is not the one about the region: %s", got.Because)
	}
}

// ── a region established for this session, when the file cannot say ───────────

// A RE-GROUNDED CONTROL IS PRESSED WHERE THE SCREEN SAYS, NOT WHERE THE FILE SAYS.
//
// The measured case: `control_3`'s saved reading is the whole window, so the step can never be
// performed however long it is given. A region established live for this session replaces that
// reading, and the point is the middle of THAT.
func TestAControlRegroundedForThisSessionReplacesWhatTheFileSays(t *testing.T) {
	a := threeScreens(t)
	s, now := ready(t, a)
	control := a.Steps[0].Control
	// THE DEFECT: a reading that re-acquires perfectly and locates nowhere.
	now.Outcomes[0].Where = observe.Region{X: 0, Y: 0, Width: 1, Height: 0.957}

	// Unrepaired, the step is refused and has no position at all.
	if got := a.StepReadinessAt(0, s, now); got.Stage !=
		workflowstore.StepControlUnlocated {
		t.Fatalf("the fixture's defect is not the one under test: %q", got.Stage)
	}

	live := observe.Region{X: 0.177, Y: 0.431, Width: 0.074, Height: 0.031}
	now.Reground = map[string]observe.Region{control: live}
	got := a.StepReadinessAt(0, s, now)
	if !got.Ready() {
		t.Fatalf("stage %q — %s", got.Stage, got.Because)
	}
	wantX, wantY := live.X+live.Width/2, live.Y+live.Height/2
	if d := got.Action.At.X - wantX; d > 0.0001 || d < -0.0001 {
		t.Errorf("the point is x %.4f and the middle of the re-grounded region is %.4f",
			got.Action.At.X, wantX)
	}
	if d := got.Action.At.Y - wantY; d > 0.0001 || d < -0.0001 {
		t.Errorf("the point is y %.4f and the middle of the re-grounded region is %.4f",
			got.Action.At.Y, wantY)
	}
}

// AND A REPAIR ANSWERS A CONTROL THAT WAS NOT SEEN AT ALL.
//
// The measured defect: the main menu is bottom-anchored and signed out it loses two entries,
// so PLAY sits 95px below where the demonstration recorded it. Re-acquisition finds nothing
// that agrees with the saved reading, and before this the readiness refused on that before it
// ever consulted the session repairs -- so the repair could only ever widen a control that had
// been found, never place one that had not.
//
// The repair IS a sighting: it was established by looking at this screen. The saved readings
// are the thing that failed.
func TestARepairAnswersAControlThatCouldNotBeSeenAtAll(t *testing.T) {
	a := threeScreens(t)
	s, now := ready(t, a)
	control := a.Steps[0].Control
	// THE DEFECT: looked for, and not found anywhere on this look.
	now.Outcomes[0].Result = observe.ReacquireUnobservable
	now.Outcomes[0].Where = observe.Region{}

	if got := a.StepReadinessAt(0, s, now); got.Stage !=
		workflowstore.StepControlUnresolved {
		t.Fatalf("the fixture is not the defect under test: %q", got.Stage)
	}

	live := observe.Region{X: 0.177, Y: 0.431, Width: 0.074, Height: 0.031}
	now.Reground = map[string]observe.Region{control: live}
	got := a.StepReadinessAt(0, s, now)
	if !got.Ready() {
		t.Fatalf("stage %q -- %s", got.Stage, got.Because)
	}
	wantX, wantY := live.X+live.Width/2, live.Y+live.Height/2
	if d := got.Action.At.X - wantX; d > 0.0001 || d < -0.0001 {
		t.Errorf("the point is x %.4f, and the repair middle is %.4f",
			got.Action.At.X, wantX)
	}
	if d := got.Action.At.Y - wantY; d > 0.0001 || d < -0.0001 {
		t.Errorf("the point is y %.4f, and the repair middle is %.4f",
			got.Action.At.Y, wantY)
	}
}

// ── is what was matched this control ──────────────────────────────────────────

// saying places one word inside a region.
func saying(text string, r observe.Region) observe.ReadWord {
	return observe.ReadWord{Text: text, Where: observe.Region{
		X: r.X + r.Width/4, Y: r.Y + r.Height/4,
		Width: r.Width / 2, Height: r.Height / 2,
	}}
}

// EXPERIMENT-044: A GEOMETRIC MATCH ONTO SOMETHING THAT IS NOT THE CONTROL IS REFUSED.
//
// Measured live: a proposer boxed a patch of pitch inside the recorded rectangle, the
// matcher reported MATCHED because each box contains the other's centre, and the step
// pressed 75px above a PLAY button whose label was read on the same look.
func TestAMatchOntoSomethingElseIsRefusedBeforeItBecomesAPress(t *testing.T) {
	a := threeScreens(t)
	s, now := ready(t, a)
	matched := rect(100, 100, 200, 40)
	elsewhere := rect(100, 300, 200, 40)
	now.Candidates = []observe.Region{matched, elsewhere}
	now.Words = []observe.ReadWord{saying("FREE PLAY", elsewhere)}

	got := a.StepReadinessAt(0, s, now)
	if got.Ready() {
		t.Fatalf("a match onto a region that says nothing was plannable at %+v",
			got.Action)
	}
	if got.Stage != workflowstore.StepControlContradicted {
		t.Fatalf("stage %q -- %s", got.Stage, got.Because)
	}
	// THE PROVENANCE OF THE WORDS IS IN THE REFUSAL. A workflow name agreeing with a
	// label is weaker evidence than a demonstration recording one, and a reader who
	// cannot tell which was used cannot weigh the refusal.
	for _, want := range []string{"open free play", "weaker than somebody pointing"} {
		if !strings.Contains(got.Because, want) {
			t.Errorf("the refusal does not say %q:\n%s", want, got.Because)
		}
	}
}

// AND A MATCH THAT SAYS WHAT IT SHOULD IS PLANNABLE, AND SAYS SO.
func TestACorroboratedMatchIsPlannableAndReportedAsCorroborated(t *testing.T) {
	a := threeScreens(t)
	s, now := ready(t, a)
	matched := rect(100, 100, 200, 40)
	now.Candidates = []observe.Region{matched}
	now.Words = []observe.ReadWord{saying("FREE PLAY", matched)}

	got := a.StepReadinessAt(0, s, now)
	if !got.Ready() {
		t.Fatalf("stage %q -- %s", got.Stage, got.Because)
	}
	if got.Grounding != workflowstore.GroundingCorroborated {
		t.Errorf("grounding %q, want it to say the words agreed", got.Grounding)
	}
	if got.Identity.Said != "FREE PLAY" {
		t.Errorf("it was read as %q", got.Identity.Said)
	}
}

// AND A MATCH NOTHING CONTRADICTS IS PLANNABLE, AND IS NOT CALLED CORROBORATED.
//
// The case that keeps every unlabelled control in every application pressable: an icon, a
// label drawn outside the clickable rectangle, an OCR that missed. Missing evidence is not
// contradictory evidence -- and it is not corroboration either, so the report must not
// describe a geometric coincidence in the words it uses for an identity.
func TestAnUnlabelledMatchIsPlannableAndReportedAsGeometryOnly(t *testing.T) {
	for _, tc := range []struct {
		name  string
		words []observe.ReadWord
	}{
		{"nothing was read at all", nil},
		{"words, none of them about this control", []observe.ReadWord{
			saying("GARAGE", rect(100, 300, 200, 40))}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := threeScreens(t)
			s, now := ready(t, a)
			now.Candidates = []observe.Region{rect(100, 100, 200, 40),
				rect(100, 300, 200, 40)}
			now.Words = tc.words

			got := a.StepReadinessAt(0, s, now)
			if !got.Ready() {
				t.Fatalf("stage %q -- %s", got.Stage, got.Because)
			}
			if got.Grounding != workflowstore.GroundingGeometric {
				t.Errorf("grounding %q, want geometry alone", got.Grounding)
			}
		})
	}
}

// AND TWO CANDIDATES NAMED THE SAME REFUSE THE MATCH AND CHOOSE NEITHER.
func TestCompetingCandidatesRefuseTheMatchAndChooseNeither(t *testing.T) {
	a := threeScreens(t)
	s, now := ready(t, a)
	one, two := rect(100, 300, 200, 40), rect(100, 400, 200, 40)
	now.Candidates = []observe.Region{rect(100, 100, 200, 40), one, two}
	now.Words = []observe.ReadWord{saying("FREE PLAY", one), saying("FREE PLAY", two)}

	got := a.StepReadinessAt(0, s, now)
	if got.Stage != workflowstore.StepControlContradicted {
		t.Fatalf("stage %q -- %s", got.Stage, got.Because)
	}
	if got.Found != nil || got.Action != nil {
		t.Errorf("a refused match offered a point anyway: %+v %+v",
			got.Found, got.Action)
	}
	if !strings.Contains(got.Because, "nothing chooses between them") {
		t.Errorf("the refusal reads as though it had an answer:\n%s", got.Because)
	}
}

// AND A REPAIRED REGION IS NOT ASKED TO CONFIRM ITSELF.
//
// The repair established it by reading this screen. Asking the same words whether they agree
// would be one source voting twice, and `GroundingRegrounded` already says what the evidence
// was -- including that it is the workflow agreeing with itself.
func TestARepairedRegionIsNotCheckedAgainstTheWordsThatMadeIt(t *testing.T) {
	a := threeScreens(t)
	s, now := ready(t, a)
	live := rect(100, 300, 200, 40)
	now.Reground = map[string]observe.Region{a.Steps[0].Control: live}
	// Words that would contradict any geometric match, and must not be consulted.
	now.Candidates = []observe.Region{live, rect(100, 500, 200, 40)}
	now.Words = []observe.ReadWord{saying("FREE PLAY", rect(100, 500, 200, 40))}

	got := a.StepReadinessAt(0, s, now)
	if !got.Ready() {
		t.Fatalf("stage %q -- %s", got.Stage, got.Because)
	}
	if got.Grounding != workflowstore.GroundingRegrounded {
		t.Errorf("grounding %q, want the repair named as the evidence", got.Grounding)
	}
}

// THE CONTROL'S OWN SAVED TEXT WINS OVER THE WORKFLOW'S NAME, AND THE REPORT SAYS WHICH.
//
// No demonstration writes `Control.Text` yet, and the preference is a rule rather than a
// convenience: a fallback nothing could ever be preferred over would quietly become the only
// rule there is.
func TestSavedControlTextIsPreferredOverTheWorkflowName(t *testing.T) {
	a := threeScreens(t)
	for i := range a.Controls {
		if a.Controls[i].Label == a.Steps[0].Control {
			a.Controls[i].Text = []string{"FREE PLAY"}
		}
	}
	words, from, _ := a.ExpectedWords(a.Steps[0].Control)
	if strings.Join(words, " ") != "free play" {
		t.Errorf("matching against %v", words)
	}
	if !strings.Contains(from, "demonstration read on this control") {
		t.Errorf("the provenance is %q", from)
	}

	// AND THE SAVED TEXT IS WHAT CONTRADICTS, not the name that would also have
	// matched. "open free play" covers CUSTOM TRAINING no better, so the two are
	// separated by a control whose text the name does NOT contain.
	for i := range a.Controls {
		if a.Controls[i].Label == a.Steps[0].Control {
			a.Controls[i].Text = []string{"CUSTOM TRAINING"}
		}
	}
	s, now := ready(t, a)
	matched, other := rect(100, 100, 200, 40), rect(100, 300, 200, 40)
	now.Candidates = []observe.Region{matched, other}
	now.Words = []observe.ReadWord{saying("CUSTOM TRAINING", other)}

	got := a.StepReadinessAt(0, s, now)
	if got.Stage != workflowstore.StepControlContradicted {
		t.Fatalf("stage %q -- %s", got.Stage, got.Because)
	}
	if !strings.Contains(got.Because, "demonstration read on this control") {
		t.Errorf("the refusal does not name the stronger evidence:\n%s",
			got.Because)
	}
}

// AND IT IS NOT NARROWED AGAINST THE READINGS IT EXISTS TO REPLACE.
//
// The agreement rule asks "what do the file and this look both describe", and both of its inputs
// are suspect in exactly this case: the saved reading is the broken thing, and narrowing a good
// live region against a window-spanning one would hand the window-spanning one straight back.
func TestARegroundedRegionIsNotNarrowedAgainstTheReadingItReplaces(t *testing.T) {
	a := threeScreens(t)
	s, now := ready(t, a)
	control := a.Steps[0].Control
	now.Outcomes[0].Where = observe.Region{X: 0, Y: 0, Width: 1, Height: 0.957}

	// A live region NOWHERE NEAR the saved reading, which is the ordinary case when the
	// saved reading is a band: it overlaps everything, so "disagrees with the file" says
	// nothing about whether the repair is right.
	saved := a.Controls[0].Readings[0]
	live := observe.Region{X: saved.X + saved.Width + 0.3, Y: saved.Y + 0.3,
		Width: 0.05, Height: 0.03}
	now.Reground = map[string]observe.Region{control: live}

	got := a.StepReadinessAt(0, s, now)
	if !got.Ready() {
		t.Fatalf("a re-grounded region was refused for disagreeing with the reading "+
			"it replaces: %q — %s", got.Stage, got.Because)
	}
	if d := got.Action.At.X - (live.X + live.Width/2); d > 0.0001 || d < -0.0001 {
		t.Errorf("the point is x %.4f, which is not the middle of the re-grounded "+
			"region (%.4f)", got.Action.At.X, live.X+live.Width/2)
	}
}

// AND A REPAIR THAT LOCATES NOTHING IS THE DEFECT IT WAS MEANT TO FIX.
//
// The one rule a repair does NOT skip. A re-grounded band across the window would be the same
// unpressable step with a better provenance, and provenance is not a position.
func TestARepairThatNamesAnotherBandAcrossTheWindowIsRefused(t *testing.T) {
	a := threeScreens(t)
	s, now := ready(t, a)
	control := a.Steps[0].Control
	now.Outcomes[0].Where = observe.Region{X: 0, Y: 0, Width: 1, Height: 0.957}
	now.Reground = map[string]observe.Region{
		control: {X: 0, Y: 0.468, Width: 1, Height: 0.461},
	}

	got := a.StepReadinessAt(0, s, now)
	if got.Ready() {
		t.Fatalf("a re-grounded band was plannable at %v", got.Action.At)
	}
	if got.Stage != workflowstore.StepControlUnlocated {
		t.Errorf("stage %q, want %q", got.Stage, workflowstore.StepControlUnlocated)
	}
	if got.Found != nil {
		t.Errorf("a refused repair still reports a position: %v", *got.Found)
	}
	// AND THE REFUSAL SAYS WHICH OF THE TWO IT IS ABOUT, or a reviewer reads it as the
	// original defect and goes looking in the artifact.
	if !strings.Contains(got.Because, "re-grounded") {
		t.Errorf("the refusal does not say the repair is the thing that failed: %s",
			got.Because)
	}
}

// AND A REPAIR FOR ANOTHER CONTROL DOES NOT MOVE THIS ONE.
func TestARepairIsAboutOneControl(t *testing.T) {
	a := threeScreens(t)
	s, now := ready(t, a)
	saved := a.Controls[0].Readings[0]
	now.Reground = map[string]observe.Region{
		"control_9": {X: 0.8, Y: 0.8, Width: 0.05, Height: 0.03},
	}

	got := a.StepReadinessAt(0, s, now)
	if !got.Ready() {
		t.Fatalf("stage %q — %s", got.Stage, got.Because)
	}
	if d := got.Action.At.X - (saved.X + saved.Width/2); d > 0.0001 || d < -0.0001 {
		t.Errorf("the point is x %.4f and another control's repair is at 0.8..0.85; "+
			"this step's control is at %.4f", got.Action.At.X, saved.X+saved.Width/2)
	}
}
