package workflow_test

// WHAT A DEMONSTRATION TAUGHT, AND MOSTLY WHAT IT DOES NOT AUTHORISE.
//
// The owner's two separate refusals are what most of this file is about: a draft is not a Play,
// and geometric identity is not semantic identity. Both are easy to write code that quietly
// violates — a planner that treats good evidence as a go-ahead looks like a planner that works.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
)

func row(y float64) observe.Region {
	return observe.Region{X: 0.4, Y: y, Width: 0.2, Height: 0.05}
}

func panel() observe.Region {
	return observe.Region{X: 0.35, Y: 0.1, Width: 0.3, Height: 0.6}
}

func centre(r observe.Region) observe.PointerAt {
	return observe.PointerAt{X: r.X + r.Width/2, Y: r.Y + r.Height/2}
}

// demo clicks one row on `from`, arrives at `to`, and returns the correlator and a draft.
func demo(t *testing.T, from, to observe.VisualScreenID, which float64) (
	*observe.ClickTargets, workflow.Draft) {

	t.Helper()
	c := &observe.ClickTargets{}
	all := []observe.Region{panel(), row(0.2), row(0.3), row(0.4)}
	c.Press(1, centre(row(which)), from, observe.Proposed(all, 1, 0, 0))
	c.Arrived(3, observe.VisualTransition{From: from, To: to})
	d := workflow.Learn("enter free play",
		[]observe.VisualScreenID{from, to}, c.Evidence(), c.Registry())
	return c, d
}

// ── the draft ─────────────────────────────────────────────────────────────────

// A DEMONSTRATION BECOMES AN ORDERED, INSPECTABLE DRAFT.
func TestADemonstrationBecomesAnOrderedDraft(t *testing.T) {
	_, d := demo(t, "visual_1", "visual_2", 0.3)
	if len(d.Steps) != 1 {
		t.Fatalf("%d step(s) from one click and one change, want 1: %+v", len(d.Steps), d.Steps)
	}
	s := d.Steps[0]
	if s.From != "visual_1" || s.To != "visual_2" {
		t.Errorf("step reads %s -> %s", s.From, s.To)
	}
	if s.Cause != observe.CauseClick {
		t.Errorf("cause %q, want click", s.Cause)
	}
	if s.Target == "" {
		t.Error("the step names no target, so nothing could be looked for again")
	}
	if len(s.Readings) == 0 {
		t.Error("the step carries no reading of the control: a draft that points at a " +
			"session which will be gone cannot re-establish anything")
	}
	if s.Support() != workflow.SupportGeometric {
		t.Errorf("support %q, want geometric", s.Support())
	}
	// AND THE SINGLE DEMONSTRATION IS NAMED AS A LIMIT, not left for a reader to notice.
	if u := strings.Join(d.Unresolved(), " | "); !strings.Contains(u, "clicked once") {
		t.Errorf("a control clicked once is not reported as unconfirmed: %q", u)
	}
}

// THE DRAFT KEEPS THE ORDER THE DEMONSTRATION WALKED, NOT THE ORDER OF THE EVIDENCE.
//
// `ClickEvidence.Steps` is sorted most-repeated first, which is the right order for reading
// evidence and the wrong order for a workflow: a draft assembled from it would be a set of edges
// in popularity order, and nobody demonstrated that.
func TestTheDraftFollowsTheWalkAndNotTheEvidenceOrder(t *testing.T) {
	c := &observe.ClickTargets{}
	all := []observe.Region{panel(), row(0.2), row(0.3), row(0.4)}
	// Walk 1 -> 2 -> 3, but make 2 -> 3 the more repeated edge so the evidence sorts
	// it first.
	c.Press(1, centre(row(0.2)), "visual_1", observe.Proposed(all, 0, 0, 0))
	c.Arrived(3, observe.VisualTransition{From: "visual_1", To: "visual_2"})
	c.Press(5, centre(row(0.3)), "visual_2", observe.Proposed(all, 0, 0, 0))
	c.Arrived(7, observe.VisualTransition{From: "visual_2", To: "visual_3"})
	c.Press(9, centre(row(0.3)), "visual_2", observe.Proposed(all, 0, 0, 0))
	c.Arrived(11, observe.VisualTransition{From: "visual_2", To: "visual_3"})

	d := workflow.Learn("x", []observe.VisualScreenID{"visual_1", "visual_2", "visual_3"},
		c.Evidence(), c.Registry())
	if len(d.Steps) != 2 {
		t.Fatalf("%d step(s), want 2", len(d.Steps))
	}
	if d.Steps[0].From != "visual_1" || d.Steps[1].From != "visual_2" {
		t.Errorf("the draft runs %s -> %s then %s -> %s; the demonstration walked "+
			"visual_1 first and the evidence sorts visual_2 first",
			d.Steps[0].From, d.Steps[0].To, d.Steps[1].From, d.Steps[1].To)
	}
}

// A STEP THAT IS NOT A CLICK IS NOT SOMETHING TO REPEAT.
//
// Entering Free Play is a load completing: the person clicks, a loading screen appears, and the
// destination arrives with no further input (ADR-174). A planner that pressed something here
// would press at a screen that was going to change anyway.
func TestAnAutomaticStepIsNotRepeatableAndSaysWhy(t *testing.T) {
	c := &observe.ClickTargets{}
	// A change with no press before it, out of a screen Marco clicked its way into:
	// the load-after-a-click shape.
	all := []observe.Region{panel(), row(0.2)}
	c.Press(1, centre(row(0.2)), "visual_1", observe.Proposed(all, 0, 0, 0))
	c.Arrived(2, observe.VisualTransition{From: "visual_1", To: "visual_2"})
	c.Arrived(4, observe.VisualTransition{From: "visual_2", To: "visual_3"})

	d := workflow.Learn("x", []observe.VisualScreenID{"visual_1", "visual_2", "visual_3"},
		c.Evidence(), c.Registry())
	if len(d.Steps) != 2 {
		t.Fatalf("%d step(s), want 2: %+v", len(d.Steps), d.Steps)
	}
	load := d.Steps[1]
	if load.Cause == observe.CauseClick {
		t.Fatalf("the second step is a click; this test needs the load shape: %+v", load)
	}
	if load.Repeatable() {
		t.Error("a step the screen made by itself is reported as something to repeat")
	}
	if load.Support() != workflow.SupportNone {
		t.Errorf("support %q, want none", load.Support())
	}
	if err := (&d).Confirm(1, "chris"); err == nil {
		t.Error("a step with nothing to press accepted a confirmation, which would " +
			"produce a confirmed thing that still cannot run")
	}
	if u := strings.Join(d.Unresolved(), " | "); !strings.Contains(u, "wait and verify") {
		t.Errorf("the draft does not say the response to this step is to wait: %q", u)
	}
}

// ── the two refusals ──────────────────────────────────────────────────────────

// NO STEP CONFIRMS ITSELF, WHATEVER ITS EVIDENCE.
//
// The refusal that makes a draft a draft. `Confirm` is the only thing that sets it, it needs a
// name, and no quantity of geometric support substitutes — because geometric support says a
// control of this shape is in this place and says nothing about what the control means.
func TestNoStepConfirmsItself(t *testing.T) {
	c, d := demo(t, "visual_1", "visual_2", 0.3)
	if d.Steps[0].Support() != workflow.SupportGeometric {
		t.Fatalf("this test needs a step with the best available evidence, got %q",
			d.Steps[0].Support())
	}
	if d.Steps[0].Confirmed() {
		t.Fatal("a freshly learned step is already confirmed")
	}
	if d.Executable() {
		t.Error("a draft nobody has agreed to reports itself executable")
	}
	// AND THE PLANNER REFUSES IT, with the reason rather than a silence.
	regions := []observe.Region{panel(), row(0.2), row(0.3), row(0.4)}
	act, ref := workflow.Plan(d, 0, "visual_1", c.Reacquire("visual_1", regions))
	if act != nil {
		t.Fatalf("an unconfirmed step produced an action at %v", act.At)
	}
	if ref == nil {
		t.Fatal("an unconfirmed step produced neither an action nor a refusal")
	}
	if !strings.Contains(ref.Because, "not been confirmed") {
		t.Errorf("the refusal does not say the step is unconfirmed: %q", ref.Because)
	}
	// AND THE REFUSAL DOES NOT LEAK A RESOLVED POINT. "All it needs is a go-ahead" is a
	// different message from "nobody has agreed to this", and only one of them is true.
	if strings.Contains(ref.Because, "proposal(s) agreeing") {
		t.Errorf("the refusal reports a successful match, which reads as a step that "+
			"is ready and merely unapproved: %q", ref.Because)
	}

	// Confirmed by a person, it plans.
	if err := (&d).Confirm(0, "chris"); err != nil {
		t.Fatalf("confirming a repeatable step: %v", err)
	}
	if !d.Executable() {
		t.Error("every repeatable step is confirmed and the draft is not executable")
	}
	act, ref = workflow.Plan(d, 0, "visual_1", c.Reacquire("visual_1", regions))
	if act == nil {
		t.Fatalf("a confirmed step did not plan: %v", ref)
	}
	if act.Support != workflow.SupportGeometric {
		t.Errorf("the action claims %q support; geometric is the most this subsystem "+
			"can establish", act.Support)
	}
	if act.Expect != "visual_2" {
		t.Errorf("the action expects %q, want visual_2: an action nobody can verify "+
			"is one nobody should take", act.Expect)
	}
}

// AN EMPTY CONFIRMATION IS NOT A CONFIRMATION.
func TestAConfirmationNeedsSomebodyToHaveGivenIt(t *testing.T) {
	_, d := demo(t, "visual_1", "visual_2", 0.3)
	if err := (&d).Confirm(0, ""); err == nil {
		t.Error("a step was confirmed by nobody; an unattributed confirmation is " +
			"indistinguishable from a default")
	}
	if d.Steps[0].Confirmed() {
		t.Error("the step is confirmed after a refused confirmation")
	}
}

// A DRAFT WITH NOTHING REPEATABLE IS NOT EXECUTABLE.
//
// "Every step of none is confirmed" is true and useless. A planner that read it as permission
// would run an empty workflow and report success — which is the oldest rule in this subsystem:
// delivery is not success.
func TestADraftWithNothingToDoIsNotExecutable(t *testing.T) {
	d := workflow.Draft{Steps: []workflow.Step{
		{From: "visual_1", To: "visual_2", Cause: observe.CauseAutomatic},
	}}
	if d.Executable() {
		t.Error("a draft whose only step is a load reports itself executable")
	}
	if (workflow.Draft{}).Executable() {
		t.Error("an empty draft reports itself executable")
	}
}

// ── the planner's other refusals ──────────────────────────────────────────────

// THE DRAFT MUST BE WHERE IT THINKS IT IS.
func TestAStepIsRefusedOnTheWrongScreen(t *testing.T) {
	c, d := demo(t, "visual_1", "visual_2", 0.3)
	if err := (&d).Confirm(0, "chris"); err != nil {
		t.Fatal(err)
	}
	regions := []observe.Region{panel(), row(0.2), row(0.3), row(0.4)}
	act, ref := workflow.Plan(d, 0, "visual_7", c.Reacquire("visual_7", regions))
	if act != nil {
		t.Fatal("a step demonstrated on visual_1 planned an action on visual_7")
	}
	if !strings.Contains(ref.Because, "demonstrated on visual_1") {
		t.Errorf("the refusal does not name the screen mismatch: %q", ref.Because)
	}
}

// A CONTROL THAT CANNOT BE FOUND, OR COULD BE MORE THAN ONE THING, STOPS THE PLAN.
func TestAnUnfindableOrAmbiguousControlStopsThePlan(t *testing.T) {
	c, d := demo(t, "visual_1", "visual_2", 0.3)
	if err := (&d).Confirm(0, "chris"); err != nil {
		t.Fatal(err)
	}
	// The control is gone: only the panel is on screen.
	act, ref := workflow.Plan(d, 0, "visual_1", c.Reacquire("visual_1", []observe.Region{panel()}))
	if act != nil {
		t.Fatalf("an action was planned with the control off screen, at %v", act.At)
	}
	if ref.Result != observe.ReacquireUnobservable {
		t.Errorf("result %q, want unobservable (%s)", ref.Result, ref.Because)
	}
	// And nothing at all.
	if act, _ := workflow.Plan(d, 0, "visual_1", c.Reacquire("visual_1", nil)); act != nil {
		t.Error("an action was planned from no proposals at all")
	}
}

// AN IDENTITY THE SESSION COULD NOT PIN DOWN DOES NOT BECOME AN ACTION.
//
// Two targets on one screen that led to the same destination may be one control read two ways or
// two entries that open the same thing. Pressing one of them on a guess is the failure the whole
// subsystem is shaped against.
func TestAnUncertainIdentityIsNotPlanned(t *testing.T) {
	c := &observe.ClickTargets{}
	a, b := row(0.2), row(0.4)
	all := []observe.Region{panel(), a, b}
	c.Press(1, centre(a), "visual_1", observe.Proposed(all, 0, 0, 0))
	c.Arrived(2, observe.VisualTransition{From: "visual_1", To: "visual_2"})
	c.Press(10, centre(b), "visual_1", observe.Proposed(all, 0, 0, 0))
	c.Arrived(11, observe.VisualTransition{From: "visual_1", To: "visual_2"})

	d := workflow.Learn("x", []observe.VisualScreenID{"visual_1", "visual_2"},
		c.Evidence(), c.Registry())
	if len(d.Steps) == 0 {
		t.Fatal("no step learned")
	}
	if !d.Steps[0].Uncertain {
		t.Fatalf("the step does not record the identity uncertainty the session "+
			"reported: %+v", d.Steps[0])
	}
	if u := strings.Join(d.Unresolved(), " | "); !strings.Contains(u, "may be one control") {
		t.Errorf("the draft does not put the ambiguity in front of anybody: %q", u)
	}
	if err := (&d).Confirm(0, "chris"); err != nil {
		t.Fatal(err)
	}
	if act, ref := workflow.Plan(d, 0, "visual_1", c.Reacquire("visual_1", all)); act != nil {
		t.Errorf("an uncertain identity planned an action at %v (%s)", act.At, act.Because)
	} else if ref == nil {
		t.Error("neither an action nor a refusal")
	}
}

// ── verification ──────────────────────────────────────────────────────────────

// ACTING IS NOT ARRIVING.
//
// Four outcomes, because collapsing them is how a workflow reports success for having pressed
// something. Waiting is not failure — a destination that arrives by a load completing takes
// several looks — and unobserved is neither.
func TestVerificationSeparatesArrivedWaitingElsewhereAndUnobserved(t *testing.T) {
	a := workflow.Action{Step: 0, Expect: "visual_2"}
	cases := []struct {
		now  observe.VisualScreenID
		want workflow.Verdict
	}{
		{"visual_2", workflow.VerdictArrived},
		{"visual_1", workflow.VerdictWaiting},
		{"visual_9", workflow.VerdictElsewhere},
		{observe.VisualScreenUnobserved, workflow.VerdictUnobserved},
		{observe.VisualScreenUnknown, workflow.VerdictUnobserved},
		{"", workflow.VerdictUnobserved},
	}
	for _, c := range cases {
		if got := workflow.Verify(a, "visual_1", c.now); got != c.want {
			t.Errorf("standing on %q after acting: %q, want %q", c.now, got, c.want)
		}
	}
}

// ── the boundary ──────────────────────────────────────────────────────────────

// EVERY WAY A STEP CAN FAIL TO BE REPEATABLE, SEPARATELY.
//
// Four conditions, and a test that only exercises one leaves three guards that can be deleted
// with the suite still green. Each needs its own case because each sends a reader somewhere
// different: a load completing, a press that hit nothing, a reading that was never kept, and a
// destination nobody could name.
func TestEveryWayAStepFailsToBeRepeatable(t *testing.T) {
	readings := []observe.Region{row(0.3)}
	cases := []struct {
		name string
		step workflow.Step
		says string
	}{
		{"a load completing", workflow.Step{
			From: "visual_1", To: "visual_2", Cause: observe.CauseAutomatic,
			Target: "target_1", Readings: readings,
		}, "wait and verify"},
		{"a press that hit nothing", workflow.Step{
			From: "visual_1", To: "visual_2", Cause: observe.CauseClick,
			Readings: readings,
		}, "landed on nothing"},
		{"no reading kept", workflow.Step{
			From: "visual_1", To: "visual_2", Cause: observe.CauseClick,
			Target: "target_1",
		}, "no reading of the control was kept"},
		{"a destination nobody could name", workflow.Step{
			From: "visual_1", To: observe.VisualScreenUnknown,
			Cause: observe.CauseClick, Target: "target_1", Readings: readings,
		}, "could not be verified"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.step.Repeatable() {
				t.Fatalf("%+v is reported repeatable", c.step)
			}
			if c.step.Support() != workflow.SupportNone {
				t.Errorf("support %q, want none", c.step.Support())
			}
			d := workflow.Draft{Steps: []workflow.Step{c.step}}
			if err := (&d).Confirm(0, "chris"); err == nil {
				t.Error("it accepted a confirmation")
			} else if !strings.Contains(err.Error(), c.says) {
				t.Errorf("the refusal says %q, want it to mention %q",
					err.Error(), c.says)
			}
			// AND THE PLANNER REFUSES IT WITH THE SAME REASON, which is a
			// separate claim: an unrepeatable step is also unconfirmed, so a
			// planner that had lost its repeatability check would still refuse
			// — and would say "nobody has agreed to this" about a step with
			// nothing to agree to.
			_, ref := workflow.Plan(d, 0, "visual_1", nil)
			if ref == nil {
				t.Fatal("the planner produced no refusal")
			}
			if !strings.Contains(ref.Because, c.says) {
				t.Errorf("the planner says %q, want it to mention %q",
					ref.Because, c.says)
			}
		})
	}
}

// A STEP WHOSE TARGET THE MATCHER NEVER MENTIONS IS REFUSED, NOT ASSUMED.
//
// Distinct from an unobservable outcome: there, the matcher looked and said no. Here it offered
// nothing about this target at all — a registry and a plan that disagree about what exists. The
// honest answer is a refusal naming the target, and the dangerous answer is to treat silence as
// absence of objection.
func TestAStepWhoseTargetTheMatcherNeverMentionsIsRefused(t *testing.T) {
	_, d := demo(t, "visual_1", "visual_2", 0.3)
	if err := (&d).Confirm(0, "chris"); err != nil {
		t.Fatal(err)
	}
	// Outcomes for a different target on the same screen, and none for this one.
	got := []observe.Reacquisition{{
		Target: "target_99", Screen: "visual_1", Result: observe.ReacquireMatched,
		Where: row(0.3),
	}}
	act, ref := workflow.Plan(d, 0, "visual_1", got)
	if act != nil {
		t.Fatalf("an action was planned from an outcome about another control: %+v", act)
	}
	if ref.Result != observe.ReacquireUnobservable {
		t.Errorf("result %q, want unobservable: silence about a target is not "+
			"permission to press it", ref.Result)
	}
	if !strings.Contains(ref.Because, "no outcome for") {
		t.Errorf("the refusal does not say the matcher said nothing: %q", ref.Because)
	}
}

// AN OUTCOME COMPUTED ON ANOTHER SCREEN IS NOT EVIDENCE ABOUT THIS STEP.
//
// A target identity is scoped to a screen, and the same id on another screen is another control.
// The outcomes are computed by the caller, so this is the one thing a caller can get wrong in a
// way that looks right.
func TestAnOutcomeFromAnotherScreenIsNotEvidence(t *testing.T) {
	_, d := demo(t, "visual_1", "visual_2", 0.3)
	if err := (&d).Confirm(0, "chris"); err != nil {
		t.Fatal(err)
	}
	got := []observe.Reacquisition{{
		Target: d.Steps[0].Target, Screen: "visual_7",
		Result: observe.ReacquireMatched, Where: row(0.3),
	}}
	act, ref := workflow.Plan(d, 0, "visual_1", got)
	if act != nil {
		t.Fatalf("an action was planned from an outcome computed on visual_7: %+v", act)
	}
	if !strings.Contains(ref.Because, "computed on visual_7") {
		t.Errorf("the refusal does not name the screen mismatch: %q", ref.Because)
	}
}

// A WAIT DOES NOT HOLD UP A DRAFT THAT IS OTHERWISE AGREED TO.
//
// `Executable` skips the steps with nothing to press, and it has to: a load completing can never
// be confirmed, so a version that required every step to be confirmed would report every real
// workflow inexecutable forever. Rocket League's own demonstration has three automatic steps and
// two unexplained ones in twenty-one.
func TestAnAutomaticStepDoesNotBlockAnAgreedDraft(t *testing.T) {
	_, d := demo(t, "visual_1", "visual_2", 0.3)
	d.Steps = append(d.Steps, workflow.Step{
		From: "visual_2", To: "visual_3", Cause: observe.CauseAutomatic,
	})
	if err := (&d).Confirm(0, "chris"); err != nil {
		t.Fatal(err)
	}
	if !d.Executable() {
		t.Error("a draft whose only unconfirmed step is a load completing reports " +
			"itself inexecutable; no real workflow would ever be executable")
	}
}

// A DRAFT CARRIES WHAT THE DEMONSTRATION READ ON THE CONTROL IT PRESSED.
//
// The live step of [[Experiment-049-the-control-it-could-see-and-could-not-name]] refused with
// the word TRAINING in its own candidate listing, because nothing between the press and the file
// carried the control's name. This is the link in that chain.
func TestADraftCarriesTheNameOfTheControlThatWasPressed(t *testing.T) {
	c := &observe.ClickTargets{}
	all := []observe.Region{panel(), row(0.2), row(0.3), row(0.4)}
	label := observe.ReadWord{Text: "TRAINING",
		Where: observe.Region{X: 0.42, Y: 0.31, Width: 0.05, Height: 0.01}}
	// READ FIRST, CLICKED AFTER, which is the order the recording actually has.
	for i := 0; i < 3; i++ {
		c.Reading("visual_1", []observe.ReadWord{label})
	}
	c.Press(1, centre(row(0.3)), "visual_1", observe.Proposed(all, 1, 0, 0))
	c.Arrived(3, observe.VisualTransition{From: "visual_1", To: "visual_2"})

	d := workflow.Learn("enter free play",
		[]observe.VisualScreenID{"visual_1", "visual_2"}, c.Evidence(), c.Registry())
	if len(d.Steps) != 1 {
		t.Fatalf("%d step(s) learned, want 1", len(d.Steps))
	}
	got := strings.Join(d.Steps[0].Text, " ")
	if got != "training" {
		t.Fatalf("the step's control is named %q; the demonstration read TRAINING "+
			"inside the row it pressed", got)
	}
	if d.Steps[0].TextLooks == 0 {
		t.Error("the draft does not say how many looks contributed the name")
	}
}

// AND A CONTROL NOBODY READ SAYS SO, so a reader can tell "unlabelled" from "not looked at".
func TestADraftSaysWhenNothingWasReadOnTheControl(t *testing.T) {
	_, d := demo(t, "visual_1", "visual_2", 0.3)
	if len(d.Steps) != 1 {
		t.Fatalf("%d step(s) learned, want 1", len(d.Steps))
	}
	if len(d.Steps[0].Text) != 0 {
		t.Fatalf("a control nobody read is named %v", d.Steps[0].Text)
	}
	var said bool
	for _, u := range d.Steps[0].Unresolved {
		if strings.Contains(u, "nothing was read on this control") {
			said = true
		}
	}
	if !said {
		t.Errorf("the draft does not record that the control was never read: %v",
			d.Steps[0].Unresolved)
	}
}

// ── the same control, demonstrated on two separate visits ────────────────────

// ONE BUTTON CLICKED ON TWO VISITS IS ONE CONTROL, AND THE CAVEAT GOES AWAY.
//
// The validation the owner asked for before any pixel action:
//
//	Capture the same Settings button twice across separate visits to the Ark menu. Verify
//	whether the artifact recognizes the repeated control as one identity.
//
// This is that shape, deterministically, so a live run that reports two controls is read as
// evidence about Ark's menu rather than about this code. The walk leaves and comes back:
//
//	visual_1 --click Settings--> visual_2 --Escape--> visual_1 --click Settings--> visual_2
//
// Three things have to hold together, and each would fail differently live:
//
//   - the revisited screen is the SAME screen, so the second press is offered the same
//     identity to merge into — which is the demonstration's own job and is NOT what this
//     test can prove about Ark;
//   - the two presses merge, which is `sameControl` and is proven on real measured
//     rectangles by [[Experiment-050-two-readings-of-one-button]];
//   - the pair (visual_1, visual_2) occurring twice becomes two steps rather than one,
//     which is ADR-174's rule and the reason `Learn` walks occurrences.
//
// And the caveat that stands on every single-press control -- *clicked once, so nothing
// independent confirms that this control can be found again* -- must be GONE, because that
// sentence is the whole reason this demonstration is being asked for.
func TestOneControlClickedOnTwoVisitsIsOneControlWithoutTheCaveat(t *testing.T) {
	c := &observe.ClickTargets{}
	all := []observe.Region{panel(), row(0.2), row(0.3), row(0.4)}
	settings := row(0.3)

	c.Press(1, centre(settings), "visual_1", observe.Proposed(all, 1, 0, 0))
	c.Arrived(3, observe.VisualTransition{From: "visual_1", To: "visual_2"})
	// BACK, BY A KEY. No press reaches the correlator, so this leg has no control --
	// which is the owner's third question: the intervening interaction must not become a
	// target of its own.
	c.Arrived(6, observe.VisualTransition{From: "visual_2", To: "visual_1"})
	c.Press(8, centre(settings), "visual_1", observe.Proposed(all, 8, 0, 0))
	c.Arrived(10, observe.VisualTransition{From: "visual_1", To: "visual_2"})

	reg := c.Registry()
	if len(reg) != 1 {
		t.Fatalf("%d control(s); one button on two visits to one screen is one "+
			"identity: %+v", len(reg), reg)
	}
	if reg[0].Presses != 2 || !reg[0].Repeated() {
		t.Fatalf("%s records %d press(es), repeated=%v", reg[0].ID, reg[0].Presses,
			reg[0].Repeated())
	}

	d := workflow.Learn("ark twice",
		[]observe.VisualScreenID{"visual_1", "visual_2", "visual_1", "visual_2"},
		c.Evidence(), reg)
	if len(d.Steps) != 3 {
		t.Fatalf("%d step(s) from a walk of four screens, want 3: %+v", len(d.Steps),
			d.Steps)
	}
	first, back, second := d.Steps[0], d.Steps[1], d.Steps[2]
	if first.Cause != observe.CauseClick || second.Cause != observe.CauseClick {
		t.Errorf("the two visits are caused by %q and %q, want click twice",
			first.Cause, second.Cause)
	}
	// THE SECOND VISIT NAMES THE SAME CONTROL. A draft whose two legs named different
	// targets would replay as two buttons and read, in the report, as a near miss.
	if first.Target != second.Target || first.Target == "" {
		t.Errorf("visit 1 pressed %q and visit 2 pressed %q", first.Target,
			second.Target)
	}
	// AND THE WAY BACK IS NOT A CONTROL.
	if back.Cause == observe.CauseClick || back.Target != "" {
		t.Errorf("the Escape leg was attributed to %q by %q", back.Target, back.Cause)
	}

	if u := strings.Join(d.Unresolved(), " | "); strings.Contains(u, "clicked once") {
		t.Errorf("a control clicked on two visits is still reported as clicked "+
			"once: %q", u)
	}
}

// TWO VISITS OF ONE PAIR TAKE THE EVIDENCE OF THAT VISIT, NOT OF THE FIRST.
//
// ADR-174 measured a step that was a click on one pass and automatic on the other, which is why
// `Learn` walks the occurrences of a pair rather than taking its first. A draft that copied the
// first occurrence onto every later visit would describe a click where nothing was pressed --
// and it would look right, because the pair and the order would both be correct.
func TestEachVisitOfAPairTakesThatVisitsEvidence(t *testing.T) {
	c := &observe.ClickTargets{}
	all := []observe.Region{panel(), row(0.2), row(0.3), row(0.4)}

	c.Press(1, centre(row(0.3)), "visual_1", observe.Proposed(all, 1, 0, 0))
	c.Arrived(3, observe.VisualTransition{From: "visual_1", To: "visual_2"})
	c.Arrived(6, observe.VisualTransition{From: "visual_2", To: "visual_1"})
	// THE SECOND VISIT HAPPENED BY ITSELF. Nothing was pressed, so this leg has no
	// control and the right response to it is to wait.
	c.Arrived(9, observe.VisualTransition{From: "visual_1", To: "visual_2"})

	d := workflow.Learn("ark twice",
		[]observe.VisualScreenID{"visual_1", "visual_2", "visual_1", "visual_2"},
		c.Evidence(), c.Registry())
	if len(d.Steps) != 3 {
		t.Fatalf("%d step(s) from a walk of four screens, want 3: %+v", len(d.Steps),
			d.Steps)
	}
	var clicked, otherwise int
	for _, s := range d.Steps {
		if s.From != "visual_1" || s.To != "visual_2" {
			continue
		}
		if s.Cause == observe.CauseClick {
			clicked++
			continue
		}
		otherwise++
		if s.Target != "" {
			t.Errorf("a leg caused by %q names the control %q", s.Cause, s.Target)
		}
	}
	if clicked != 1 || otherwise != 1 {
		t.Errorf("the two visits of visual_1 -> visual_2 came out %d click(s) and %d "+
			"otherwise; one of each was demonstrated", clicked, otherwise)
	}
}
