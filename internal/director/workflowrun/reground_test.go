package workflowrun_test

// Where a control is, when the file cannot say.
//
// The coordinator cannot answer this and must not try: deciding where a control is from the
// inside would be choosing its own click target. So it asks, and what these hold is WHEN it
// asks, how often, and that a refusal is a complete answer.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// band is the measured defect: a reading that re-acquires perfectly and locates nowhere.
func band() observe.Region { return rect(0, 0, 1, 0.957) }

// onABand is the first step's observation with its control matched as a window-spanning band.
func onABand() workflowrun.Sighting { return on("now_1", 5, "control_1", band()) }

// repairs answers with one region, or refuses, and records every question.
type repairs struct {
	where  observe.Region
	why    string
	no     bool
	asked  []string
	steps  []int
	looked []uint64
	stages []workflowstore.StepStage
}

func (p *repairs) Reground(ctx context.Context, step int, control string,
	stage workflowstore.StepStage, at workflowrun.Sighting) (observe.Region, string, bool) {

	p.asked = append(p.asked, control)
	p.steps = append(p.steps, step)
	p.looked = append(p.looked, at.Epoch)
	p.stages = append(p.stages, stage)
	if p.no {
		return observe.Region{}, "", false
	}
	return p.where, p.why, true
}

// A STEP WHOSE CONTROL LOCATES NOTHING IS RE-GROUNDED, AND THEN PERFORMED.
//
// Measured: four of the nine controls in the Rocket League artifact are bands across the window.
// Without this the step is refused forever and the only repair is a developer editing the file.
func TestAControlThatLocatesNothingIsAskedAboutAndThenPressed(t *testing.T) {
	g := newRig(t)
	live := rect(0.177, 0.431, 0.074, 0.031)
	p := &repairs{where: live, why: "the only control whose text is made of words " +
		"from the workflow's own name"}
	g.run.WithRegrounder(p)

	got := g.run.Advance(context.Background(), g.s, onABand())
	if got.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("%s - %s", got.Outcome, got.Because)
	}
	if len(p.asked) != 1 || p.asked[0] != "control_1" {
		t.Fatalf("asked about %v, want control_1 once", p.asked)
	}
	if p.steps[0] != 0 {
		t.Errorf("the question named step %d, and the refusal was step 0", p.steps[0])
	}
	// THE QUESTION CARRIES THE OBSERVATION IT IS ABOUT, or whoever answers is looking
	// for a control on a screen that has gone.
	if p.looked[0] != 5 {
		t.Errorf("the question was asked about epoch %d, and the refusal happened "+
			"in epoch 5", p.looked[0])
	}
	// THE PRESS WENT TO THE RE-GROUNDED REGION.
	want := workflowrun.Desktop(observe.PointerAt{
		X: live.X + live.Width/2, Y: live.Y + live.Height/2}, window)
	if !strings.Contains(g.book.programs[0], "X "+itoa(want.X)+", Y "+itoa(want.Y)) {
		t.Errorf("the program does not press the re-grounded point (%v):\n%s",
			want, g.book.programs[0])
	}
}

// AND THE REPAIR IS REPORTED, WITH ITS EVIDENCE AND THE ARTIFACT UNTOUCHED.
//
// A workflow corrected in place by the run using it would make the next run's evidence depend on
// this one's guess, with the provenance gone by the time anybody looked.
func TestARepairIsReportedWithItsEvidenceAndTheArtifactIsNotTouched(t *testing.T) {
	g := newRig(t)
	live := rect(0.177, 0.431, 0.074, 0.031)
	p := &repairs{where: live, why: "its text is made of words from the workflow's name"}
	g.run.WithRegrounder(p)
	before := g.art.Controls[0].Readings[0]

	if got := g.run.Advance(context.Background(), g.s, onABand()); got.Outcome !=
		workflowrun.OutcomePressed {
		t.Fatalf("%s - %s", got.Outcome, got.Because)
	}
	rep := g.run.Report()
	if len(rep.Repairs) != 1 {
		t.Fatalf("%d repair(s) reported for one re-grounded control", len(rep.Repairs))
	}
	r := rep.Repairs[0]
	if r.Control != "control_1" || r.Step != 0 {
		t.Errorf("the repair names step %d of %q", r.Step, r.Control)
	}
	if r.Where != live {
		t.Errorf("the repair records %+v and the region established was %+v",
			r.Where, live)
	}
	// THE EVIDENCE, IN THE WORDS OF WHOEVER ESTABLISHED IT. A repair with no sentence
	// beside it is a coordinate of unknown provenance.
	if r.Because != p.why {
		t.Errorf("the repair says %q and the regrounder said %q", r.Because, p.why)
	}
	if g.art.Controls[0].Readings[0] != before {
		t.Errorf("the artifact's reading changed from %+v to %+v",
			before, g.art.Controls[0].Readings[0])
	}
	if got := g.run.Repairs(); len(got) != 1 {
		t.Errorf("the runner reports %d repair(s) and the report carries 1", len(got))
	}
}

// A REFUSAL IS A COMPLETE ANSWER AND NOTHING IS PRESSED.
//
// Zero candidates, or two that match equally, and the step stays refused. A regrounder that
// picked the best of several would be choosing Marco's own target on a similarity score.
func TestARegrounderThatRefusesLeavesTheStepRefusedAndSendsNothing(t *testing.T) {
	g := newRig(t)
	p := &repairs{no: true}
	g.run.WithRegrounder(p)

	got := g.run.Advance(context.Background(), g.s, onABand())
	if got.Outcome == workflowrun.OutcomePressed {
		t.Fatal("a step whose control could not be re-grounded was pressed")
	}
	if got.Stage != workflowstore.StepControlUnlocated {
		t.Errorf("stage %q, want %q", got.Stage, workflowstore.StepControlUnlocated)
	}
	if len(g.book.programs) != 0 {
		t.Errorf("%d program(s) reached Marco: %v", len(g.book.programs),
			g.book.programs)
	}
	if len(p.asked) != 1 {
		t.Errorf("asked %d time(s) for one refused step", len(p.asked))
	}
	if got := g.run.Report(); len(got.Repairs) != 0 {
		t.Errorf("%d repair(s) reported for a refusal", len(got.Repairs))
	}
}

// AND NOBODY IS ASKED AGAIN ABOUT A CONTROL THAT HAS BEEN ESTABLISHED.
//
// Once per control, not once per look. The loop runs ten times a second and the question costs
// an OCR pass; more than that, a second answer that differed from the first would mean the run
// pressed two different places for one step.
func TestAControlIsAskedAboutOnceAndNotOncePerLook(t *testing.T) {
	g := newRig(t)
	p := &repairs{where: rect(0.177, 0.431, 0.074, 0.031), why: "SIMULATED"}
	g.run.WithRegrounder(p).WithBudget(1)
	ctx := context.Background()

	for i := 0; i < 4; i++ {
		g.run.Advance(ctx, g.s, onABand())
	}
	if len(p.asked) != 1 {
		t.Errorf("the control was asked about %d time(s) over four looks: %v",
			len(p.asked), p.asked)
	}
	if got := g.run.Report(); len(got.Repairs) != 1 {
		t.Errorf("%d repair(s) reported for one control", len(got.Repairs))
	}
}

// AND A REPAIR THAT DID NOT HELP IS NOT ASKED FOR AGAIN EITHER.
//
// A regrounder can answer with a region that itself locates nothing -- another band -- and the
// step is then refused for exactly the reason it was refused before. Without the memory of
// having asked, the question goes out again on every look for the rest of the run: ten OCR
// passes a second, each one answering the same way, and a report whose repair list grows by one
// entry per observation.
func TestARepairThatDidNotHelpIsNotAskedForAgain(t *testing.T) {
	g := newRig(t)
	// A REPAIR THAT IS ITSELF A BAND, which `Locates` refuses in its turn.
	p := &repairs{where: rect(0, 0.468, 1, 0.461), why: "SIMULATED"}
	g.run.WithRegrounder(p)
	ctx := context.Background()

	for i := 0; i < 4; i++ {
		got := g.run.Advance(ctx, g.s, onABand())
		if got.Outcome == workflowrun.OutcomePressed {
			t.Fatalf("look %d pressed a re-grounded band", i+1)
		}
		if got.Stage != workflowstore.StepControlUnlocated {
			t.Fatalf("look %d: %q - %s", i+1, got.Stage, got.Because)
		}
	}
	// THE STEP STAYS REFUSED FOR THE SAME REASON ON EVERY LOOK, and the question was
	// put once.
	if len(p.asked) != 1 {
		t.Errorf("the control was asked about %d time(s) over four looks in which the "+
			"repair never helped: %v", len(p.asked), p.asked)
	}
	if got := g.run.Report(); len(got.Repairs) != 1 {
		t.Errorf("%d repair(s) reported for one question", len(got.Repairs))
	}
	if len(g.book.programs) != 0 {
		t.Errorf("%d program(s) reached Marco", len(g.book.programs))
	}
}

// AND THE REPAIR REACHES THE PLAN ON EVERY LATER LOOK, not only the one it was made on.
//
// The region is established once and the step is examined again on every observation
// after it. A repair that only reached the plan inside the branch that made it would leave
// the step refused as unlocated from the second look onwards, while the regrounder -- asked
// once per control, correctly -- said nothing more. The run would then report a repair it
// was not using.
func TestARepairSurvivesToTheNextLook(t *testing.T) {
	g := newRig(t)
	p := &repairs{where: rect(0.177, 0.431, 0.074, 0.031), why: "SIMULATED"}
	g.run.WithRegrounder(p)
	// NOBODY AUTHORISES ANYTHING, so the step is examined over and over without ever
	// pressing -- which is what makes the later looks observable.
	g.auth.no = true
	ctx := context.Background()

	first := g.run.Advance(ctx, g.s, onABand())
	if first.Stage != workflowstore.StepUnauthorised {
		t.Fatalf("the first look: %q - %s", first.Stage, first.Because)
	}
	for i := 0; i < 3; i++ {
		got := g.run.Advance(ctx, g.s, onABand())
		if got.Stage == workflowstore.StepControlUnlocated {
			t.Fatalf("look %d is refused as unlocated although the control "+
				"was re-grounded on look 1: %s", i+2, got.Because)
		}
		if got.Stage != workflowstore.StepUnauthorised {
			t.Errorf("look %d: %q - %s", i+2, got.Stage, got.Because)
		}
	}
	if len(p.asked) != 1 {
		t.Errorf("the control was asked about %d time(s)", len(p.asked))
	}
}

// AND A STEP REFUSED FOR ANOTHER REASON IS NOT RE-GROUNDED ON THE LOOK IT IS REFUSED.
//
// The question is asked where the refusal actually happens and nowhere else. Looking for a
// different rectangle because a control was briefly covered, or because nobody has agreed to the
// step yet, would replace a target that works with whatever happened to be visible.
//
// ON THE LOOK, and the qualification is load-bearing for the first case only. A control that was
// not found comes back if it was covered and does not if it moved, and the patience window is
// the only thing that tells those apart — so it IS asked about, once, after the window is spent.
// See TestAControlNeverSeenIsReGroundedOnTheLastLookAndThenPressed. The other two are refusals
// about something other than where the control is, and no amount of waiting makes them askable.
func TestAStepRefusedForAnotherReasonIsNotRegrounded(t *testing.T) {
	for _, tc := range []struct {
		name  string
		spoil func(*rig) workflowrun.Sighting
	}{
		{"the control was not found on this look", func(g *rig) workflowrun.Sighting {
			at := firstStep()
			at.Outcomes = nil
			return at
		}},
		{"the control is where the file says", func(g *rig) workflowrun.Sighting {
			return firstStep()
		}},
		{"the control could not be told from another", func(g *rig) workflowrun.Sighting {
			at := onABand()
			at.Outcomes[0].Result = observe.ReacquireAmbiguous
			at.Outcomes[0].Controls = 2
			at.Outcomes[0].Because = "two controls on this screen match it"
			return at
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newRig(t)
			p := &repairs{where: rect(0.2, 0.4, 0.07, 0.03), why: "SIMULATED"}
			g.run.WithRegrounder(p)

			g.run.Advance(context.Background(), g.s, tc.spoil(g))
			if len(p.asked) != 0 {
				t.Errorf("the regrounder was asked %d time(s): %v",
					len(p.asked), p.asked)
			}
		})
	}
}

// AND A REPAIR IS NOT PERMISSION.
//
// Re-grounding is asked before anybody's agreement is consulted, because `Locates` is asked
// before it: a band refuses at the point where the step has no position at all, which is earlier
// than any question about who said yes. That ordering is right -- where a control is, is a
// perception question, and the answer actuates nothing by itself -- and it would be a serious
// defect if establishing a region also satisfied the sentence that permits pressing it.
func TestARepairDoesNotStandInForSomebodyAgreeingOrAuthorising(t *testing.T) {
	for _, tc := range []struct {
		name  string
		spoil func(*rig)
		stage workflowstore.StepStage
	}{
		{"nobody agreed to this step", func(g *rig) {
			g.s = workflowstore.NewSessionBinding()
			if err := g.s.Confirm("screen_a", "now_1", 5, 1, "SIMULATED",
				looked); err != nil {
				t.Fatal(err)
			}
		}, workflowstore.StepUnconfirmed},
		{"nobody authorised the press", func(g *rig) {
			g.auth.no = true
		}, workflowstore.StepUnauthorised},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newRig(t)
			p := &repairs{where: rect(0.177, 0.431, 0.074, 0.031),
				why: "SIMULATED"}
			g.run.WithRegrounder(p)
			tc.spoil(g)

			got := g.run.Advance(context.Background(), g.s, onABand())
			if got.Outcome == workflowrun.OutcomePressed {
				t.Fatal("a re-grounded control was pressed with nobody " +
					"having agreed to it")
			}
			if len(g.book.programs) != 0 {
				t.Errorf("%d program(s) reached Marco", len(g.book.programs))
			}
			// THE REGION WAS ESTABLISHED -- that is the point -- and the step
			// is refused anyway, at the stage that names what is missing.
			if len(p.asked) != 1 {
				t.Errorf("the regrounder was asked %d time(s)", len(p.asked))
			}
			if got.Stage != tc.stage {
				t.Errorf("stage %q, want %q", got.Stage, tc.stage)
			}
		})
	}
}

// AND A RUN WITH NO REGROUNDER REFUSES THE STEP RATHER THAN INVENTING A POINT.
func TestARunWithNoRegrounderStillRefusesAControlThatLocatesNothing(t *testing.T) {
	g := newRig(t)
	g.run.WithDwell(time.Millisecond)

	got := g.run.Advance(context.Background(), g.s, onABand())
	if got.Outcome == workflowrun.OutcomePressed {
		t.Fatal("a control that locates nothing was pressed with nobody to re-ground it")
	}
	if got.Stage != workflowstore.StepControlUnlocated {
		t.Errorf("stage %q, want %q", got.Stage, workflowstore.StepControlUnlocated)
	}
	if len(g.book.programs) != 0 {
		t.Errorf("%d program(s) reached Marco", len(g.book.programs))
	}
}

// notFound is an observation on which the control was looked for and not seen.
//
// The measured shape: the main menu is bottom-anchored and signed out it loses two entries,
// so PLAY sits 95px below where the demonstration recorded it. The recorded rectangle still
// locates something -- the arena behind the menu -- so this is not StepControlUnlocated.
func notFound(lk workflowrun.Sighting) workflowrun.Sighting {
	lk.Outcomes[0].Result = observe.ReacquireUnobservable
	lk.Outcomes[0].Where = observe.Region{}
	lk.Outcomes[0].Controls = 0
	lk.Outcomes[0].Because = "no proposal on this look agrees with any reading of this " +
		"control, though a region that contained the original press is still here"
	return lk
}

// A CONTROL THAT WAS BRIEFLY NOT SEEN IS NOT RE-GROUNDED WHILE THE RUN IS STILL WAITING.
//
// The exclusion the original rule defended, and it survives: a control mid-animation or
// covered by something passing over it is refused exactly this way and comes back a look
// later. Asking then would replace a target that works with whatever happened to be visible.
func TestAControlNotSeenYetIsNotReGroundedWhileThereIsStillPatience(t *testing.T) {
	g := newRig(t)
	p := &repairs{where: rect(0.177, 0.431, 0.074, 0.031), why: "somewhere else"}
	g.run.WithRegrounder(p).WithPatience(4)

	for i := 0; i < 4; i++ {
		got := g.run.Advance(context.Background(), g.s, notFound(firstStep()))
		if got.Outcome != workflowrun.OutcomeWaiting {
			t.Fatalf("look %d: %s -- %s", i, got.Outcome, got.Because)
		}
		if len(p.asked) != 0 {
			t.Fatalf("look %d: re-grounded after %d look(s); a control that is "+
				"briefly covered must be waited for", i, i+1)
		}
	}
	if g.book.clicks() != 0 {
		t.Errorf("%d press(es) reached Marco", g.book.clicks())
	}
}

// AND ONE THAT WAS NEVER SEEN AT ALL IS ASKED ABOUT, ONCE, BEFORE THE RUN GIVES UP.
//
// The defect this fixes, measured live: step 0 was refused unobservable on all 60 looks of
// its window and the run stopped, while the repair that exists for exactly this -- a control
// grounded by what it SAYS -- was never asked, because the control still located something.
func TestAControlNeverSeenIsReGroundedOnTheLastLookAndThenPressed(t *testing.T) {
	g := newRig(t)
	live := rect(0.177, 0.431, 0.074, 0.031)
	p := &repairs{where: live, why: "the only control on this screen whose text is " +
		"made of words from the workflow's own name"}
	g.run.WithRegrounder(p).WithPatience(3)

	ctx := context.Background()
	// THE WHOLE WINDOW IS SPENT WAITING FIRST, because a control that comes back is not
	// a control that moved, and only the window apart tells them apart.
	for i := 0; i < 3; i++ {
		got := g.run.Advance(ctx, g.s, notFound(firstStep()))
		if got.Outcome != workflowrun.OutcomeWaiting {
			t.Fatalf("look %d: %s -- %s", i, got.Outcome, got.Because)
		}
		if len(p.asked) != 0 {
			t.Fatalf("re-grounded on look %d, with the window not yet spent", i)
		}
	}

	// AND THE LOOK THE RUN WOULD OTHERWISE STOP ON IS THE ONE THAT ASKS.
	got := g.run.Advance(ctx, g.s, notFound(firstStep()))
	if len(p.asked) != 1 {
		t.Fatalf("asked %d time(s), want exactly once", len(p.asked))
	}
	if got.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("%s -- %s", got.Outcome, got.Because)
	}
	if n := g.book.clicks(); n != 1 {
		t.Fatalf("%d press(es) reached Marco, want 1", n)
	}
	want := workflowrun.Desktop(observe.PointerAt{
		X: live.X + live.Width/2, Y: live.Y + live.Height/2}, window)
	if !strings.Contains(g.book.programs[0], "X "+itoa(want.X)+", Y "+itoa(want.Y)) {
		t.Errorf("the press did not go to the re-grounded point (%v):\n%s",
			want, g.book.programs[0])
	}
}

// AND THE REASON IT IS BEING ASKED REACHES WHOEVER ANSWERS.
//
// Two refusals arrive here and they are different findings: a rectangle recorded wrong, and
// a control that has moved. A repair that described itself as the first would send a reader
// to the artifact to look at a rectangle that is perfectly well-formed.
func TestTheRepairIsToldWhichRefusalItIsAnswering(t *testing.T) {
	for _, tc := range []struct {
		name string
		look workflowrun.Sighting
		// looks is how many observations it takes to reach the question, which is
		// itself the difference: a band is asked about at once, a control that was
		// not found is asked about only once waiting has been given up on.
		looks int
		want  workflowstore.StepStage
	}{
		{"a band across the window", onABand(), 1,
			workflowstore.StepControlUnlocated},
		{"a control that moved", notFound(firstStep()), 2,
			workflowstore.StepControlUnresolved},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newRig(t)
			p := &repairs{no: true}
			g.run.WithRegrounder(p).WithPatience(1)
			for i := 0; i < tc.looks; i++ {
				g.run.Advance(context.Background(), g.s, tc.look)
			}
			if len(p.stages) != 1 {
				t.Fatalf("asked %d time(s), want once", len(p.stages))
			}
			if p.stages[0] != tc.want {
				t.Errorf("told %q, want %q", p.stages[0], tc.want)
			}
		})
	}
}

// AND AN ORDINARY STEP NEVER CONSULTS THE REPAIR AT ALL.
//
// Every plannable step passes through 5a refused as unauthorised, because authorisation is
// deliberately asked for last. A rule that re-grounded whatever refusal it met would look for
// a new rectangle before any step was ever pressed, and the target the demonstration
// established would never be used.
func TestAStepThatIsMerelyUnauthorisedIsNeverReGrounded(t *testing.T) {
	g := newRig(t)
	p := &repairs{where: rect(0.177, 0.431, 0.074, 0.031), why: "somewhere else"}
	g.run.WithRegrounder(p)

	got := g.run.Advance(context.Background(), g.s, firstStep())
	if got.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("%s -- %s", got.Outcome, got.Because)
	}
	if len(p.asked) != 0 {
		t.Errorf("a step whose control was found was re-grounded anyway, about %v",
			p.asked)
	}
}

// itoa keeps the expected program text free of fmt in a test about coordinates.
func itoa(n int) string {
	neg := n < 0
	if neg {
		n = -n
	}
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	if neg {
		return "-" + string(d)
	}
	return string(d)
}
