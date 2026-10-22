package workflowstore_test

// GEOMETRY REPLAY: PRESSING THE POINT SOMEBODY CLICKED, IN THE WINDOW THEY CLICKED IT IN.
//
// A MODE, not a fallback. While it was a fallback the window was checked only when recognition
// had already failed, so a run whose screen happened to be recognisable pressed on the
// recognition path's terms and the window could have been any size at all. The licence is for an
// UNCHANGED window; a check that only sometimes runs is not that licence.
//
// # Why the exception exists
//
// Because the recognition route has a case with no answer. A menu drawn over a live 3D world is
// filed as two different screens depending on what is moving behind it, so `Established` never
// settles and waiting does not settle it. Three live trials died there with zero actions sent.
//
// # What must stay true
//
// The point pressed is the PERSON'S CLICK and never the middle of a rectangle something drew
// around it. A run without the licence is unchanged. A window of another size is refused — even
// when the screen IS established, which is the difference between a mode and a fallback. An
// artifact that cannot say what window it was demonstrated in is refused rather than trusted.
// Words that put the control somewhere else still refuse, while no words at all refuse nothing.
// Nobody's agreement and nobody's authorisation is skipped. And the report never describes any
// of this as recognition.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// aMenuOverALiveWorld is one click whose screen can never be established, in a window of a
// recorded size — the shape of the Ark demonstration this exists for.
//
// `clicked` is where the person pressed and `seen` what the proposer drew around it. They are
// separate arguments because the whole question here is what happens when they differ.
func aMenuOverALiveWorld(t *testing.T, clicked observe.PointerAt,
	seen ...observe.Region) workflowstore.Artifact {

	t.Helper()
	at := clicked
	d := workflow.Draft{Name: "ark graphics", Steps: []workflow.Step{{
		From: "visual_1", To: "visual_2", Target: "target_1",
		Cause: observe.CauseClick, Seen: 1,
		Readings: seen, At: &at,
	}}}
	a, err := workflowstore.Save(d, map[observe.VisualScreenID]observe.VisualArrangement{
		"visual_1": {Cells: map[string]int{"0,0": 9}, Total: 9},
		"visual_2": {Cells: map[string]int{"1,1": 7}, Total: 7},
	}, saved)
	if err != nil {
		t.Fatal(err)
	}
	a.Window = &workflowstore.WindowGeometry{Width: 1920, Height: 1080}
	return a
}

// replaying is a session in geometry replay mode: the step agreed to, the session authorised,
// and — deliberately — NO re-acquisition outcome, NO proposals and NO words.
//
// The emptiness is the point. This mode is for a run with no usable vision at all, so a fixture
// that supplied any would be testing something easier than the case it exists for.
func replaying(t *testing.T) (*workflowstore.SessionBinding, workflowstore.Now) {
	t.Helper()
	s := workflowstore.NewSessionBinding()
	if err := s.ConfirmStep(0, "SIMULATED"); err != nil {
		t.Fatal(err)
	}
	return s, workflowstore.Now{
		Screen: "now_9", Epoch: 5, Generation: 1, Authorised: true,
		Layout: workflowstore.LayoutReplay{Licensed: true,
			Window: workflowstore.WindowGeometry{Width: 1920, Height: 1080}},
	}
}

// WITHOUT THE LICENCE, NOTHING CHANGES.
//
// First, because it is the claim the rest of the subsystem depends on. The mode is granted per
// run by whoever holds it; a build where it leaked into the default would be one where every
// saved coordinate had quietly become a replay target.
func TestWithoutTheLicenceAnUnestablishedScreenStillRefuses(t *testing.T) {
	a := aMenuOverALiveWorld(t, observe.PointerAt{X: 0.12, Y: 0.11},
		rect(100, 100, 200, 40))
	s, now := replaying(t)
	now.Layout = workflowstore.LayoutReplay{}

	got := a.StepReadinessAt(0, s, now)
	if got.Stage != workflowstore.StepScreenUnestablished {
		t.Fatalf("step 0 is %q without the fixed-layout licence: %s",
			got.Stage, got.Because)
	}
	if got.Action != nil {
		t.Error("an unlicensed run was given a point to press")
	}
}

// THE POINT PRESSED IS THE PERSON'S CLICK, NOT THE MIDDLE OF WHAT A PROPOSER DREW.
//
// The click is off-centre inside a generous proposal, and the proposal's middle lands in a
// DIFFERENT control. That is not a contrived arrangement: proposals spill, and a live look once
// matched PLAY with a region running to the window's left edge whose middle landed three pixels
// inside a button starting at 62.
//
// A replay that pressed the centre would be replaying the segmenter's idea of the control rather
// than the thing the person did — and here it would press the wrong one.
func TestThePointPressedIsTheClickAndNotTheMiddleOfTheProposal(t *testing.T) {
	// A GENEROUS PROPOSAL over two controls side by side. The person clicked the
	// left one, near its left edge; the proposal's middle is inside the RIGHT one.
	sprawl := rect(0.10, 0.10, 0.40, 0.06)
	rightControl := rect(0.30, 0.10, 0.20, 0.06)
	clicked := observe.PointerAt{X: 0.14, Y: 0.13}

	a := aMenuOverALiveWorld(t, clicked, sprawl)
	s, now := replaying(t)

	got := a.StepReadinessAt(0, s, now)
	if !got.Ready() {
		t.Fatalf("step 0 is %q: %s", got.Stage, got.Because)
	}
	if got.Action == nil || got.Found == nil {
		t.Fatal("a plannable step with no point")
	}
	if got.Action.At != clicked || *got.Found != clicked {
		t.Fatalf("the point is %+v and the person clicked %+v",
			got.Action.At, clicked)
	}

	// AND THE MIDDLE OF THE PROPOSAL WOULD HAVE BEEN A DIFFERENT CONTROL.
	//
	// Asserted rather than asserted-about, so this test fails if the fixture ever
	// stops posing the question it was built to pose.
	middle := observe.PointerAt{
		X: sprawl.X + sprawl.Width/2, Y: sprawl.Y + sprawl.Height/2}
	if !inside(middle, rightControl) {
		t.Fatalf("the fixture is no longer a question: the proposal's middle "+
			"%+v is not inside the other control %v", middle, rightControl)
	}
	if inside(clicked, rightControl) {
		t.Fatalf("the fixture is no longer a question: the click %+v is inside "+
			"the other control too", clicked)
	}
}

// inside reports whether a point lies in a region.
func inside(p observe.PointerAt, r observe.Region) bool {
	return p.X >= r.X && p.X <= r.X+r.Width && p.Y >= r.Y && p.Y <= r.Y+r.Height
}

// A PROPOSAL IS NOT REQUIRED.
//
// A press the proposer found nothing under records where it was, and that press is exactly the
// one this mode exists to repeat: on a menu no recogniser can name, the segmenter's silence is
// the ordinary case rather than the exception.
func TestAClickWithNoProposalAtAllIsStillPressable(t *testing.T) {
	clicked := observe.PointerAt{X: 0.4239, Y: 0.2574}
	a := aMenuOverALiveWorld(t, clicked) // no readings at all
	if len(a.Readings("control_1")) != 0 {
		t.Fatal("the fixture has readings and is not posing the question")
	}
	s, now := replaying(t)

	got := a.StepReadinessAt(0, s, now)
	if !got.Ready() {
		t.Fatalf("step 0 is %q with no proposal ever drawn around the click: %s",
			got.Stage, got.Because)
	}
	if got.Action.At != clicked {
		t.Errorf("the point is %+v and the person clicked %+v",
			got.Action.At, clicked)
	}
}

// AND A STEP WITH NO RECORDED CLICK IS REFUSED, NOT GIVEN ONE.
//
// Every routine recorded before the click was kept is in this position. A point computed from
// the saved rectangle would look exactly like a real one and would be the segmenter's answer
// wearing the person's authority.
func TestAStepWithNoRecordedClickIsRefusedRatherThanGivenOne(t *testing.T) {
	a := aMenuOverALiveWorld(t, observe.PointerAt{X: 0.12, Y: 0.11},
		rect(100, 100, 200, 40))
	a.Steps[0].At = nil
	s, now := replaying(t)

	got := a.StepReadinessAt(0, s, now)
	if got.Stage != workflowstore.StepControlUnlocated {
		t.Fatalf("step 0 is %q with no recorded click: %s", got.Stage, got.Because)
	}
	if got.Action != nil || got.Found != nil {
		t.Fatal("a point was invented for a step that never recorded one")
	}
	if !strings.Contains(got.Because, "will not be invented") {
		t.Errorf("the refusal is %q", got.Because)
	}
}

// THE REPORT NEVER CALLS THIS RECOGNITION.
//
// The one thing Experiment-044 proved costs a person their trust: a weaker kind of evidence
// printed in a stronger kind's sentence.
func TestTheReportSaysNothingWasRecognised(t *testing.T) {
	a := aMenuOverALiveWorld(t, observe.PointerAt{X: 0.12, Y: 0.11},
		rect(0.10, 0.10, 0.20, 0.04))
	s, now := replaying(t)

	got := a.StepReadinessAt(0, s, now)
	if !got.Ready() {
		t.Fatalf("step 0 is %q: %s", got.Stage, got.Because)
	}
	if got.Grounding != workflowstore.GroundingDemonstratedLayout {
		t.Errorf("the grounding is %q; nothing on this look found this control "+
			"and the word for that must not be one of the words for "+
			"something that did", got.Grounding)
	}
	if got.Action.Support != workflow.SupportDemonstratedLayout {
		t.Errorf("the support is %q, which is the word for a control on a "+
			"screen whose arrangement matches — and no arrangement matched",
			got.Action.Support)
	}
	if !strings.Contains(got.Because, "NOTHING was recognised") {
		t.Errorf("the sentence is %q and does not say that nothing was "+
			"recognised", got.Because)
	}
}

// A WINDOW OF ANOTHER SIZE IS REFUSED, NOT SCALED INTO.
func TestAWindowOfAnotherSizeIsRefused(t *testing.T) {
	a := aMenuOverALiveWorld(t, observe.PointerAt{X: 0.12, Y: 0.11},
		rect(100, 100, 200, 40))
	s, now := replaying(t)
	now.Layout.Window = workflowstore.WindowGeometry{Width: 1280, Height: 720}

	got := a.StepReadinessAt(0, s, now)
	if got.Stage != workflowstore.StepLayoutChanged {
		t.Fatalf("step 0 is %q in a window of another size: %s",
			got.Stage, got.Because)
	}
	if got.Action != nil {
		t.Fatal("a point was offered in a window the demonstration never saw")
	}
	for _, want := range []string{"1920x1080", "1280x720"} {
		if !strings.Contains(got.Because, want) {
			t.Errorf("the refusal is %q and does not name %s", got.Because, want)
		}
	}

	// ── AND ONE PIXEL IS A DIFFERENT WINDOW ──
	//
	// There is no tolerance here and that is the decision, not an oversight. A
	// tolerance would be this layer deciding how much change a menu survives,
	// which is the question nobody has measured and the reason the licence is
	// narrow at all. The failure mode it prevents is the one that gets written
	// into a subsystem to make a failing trial pass: widen it until the press
	// goes out, and the refusal that made the exception safe is gone.
	for _, near := range []workflowstore.WindowGeometry{
		{Width: 1919, Height: 1080}, {Width: 1920, Height: 1081},
	} {
		now.Layout.Window = near
		if got := a.StepReadinessAt(0, s, now); got.Stage != workflowstore.StepLayoutChanged {
			t.Errorf("a window of %dx%d is %q against a demonstration made at "+
				"1920x1080; this is an EXACT check and a tolerance here "+
				"is the subsystem deciding how much change a menu "+
				"survives: %s", near.Width, near.Height, got.Stage,
				got.Because)
		}
	}
}

// AND THE WINDOW IS CHECKED EVEN WHEN RECOGNITION WOULD HAVE SUCCEEDED.
//
// THE DIFFERENCE BETWEEN A MODE AND A FALLBACK, and the gap this closes. While the geometry was
// consulted only after recognition failed, a run whose screen happened to settle took the
// recognition path's terms — which say nothing about the window — and a demonstrated point could
// go out into a window half the size with every check passing.
//
// Here the screen IS established, the control IS re-acquired, and the run still refuses, because
// what this mode licenses is a press in the window the demonstration was made in.
func TestTheWindowIsCheckedEvenWhenTheScreenIsEstablished(t *testing.T) {
	control := rect(0.10, 0.10, 0.20, 0.04)
	a := aMenuOverALiveWorld(t, observe.PointerAt{X: 0.12, Y: 0.11}, control)
	s, now := replaying(t)

	// EVERYTHING THE RECOGNITION PATH WOULD HAVE WANTED, supplied and agreeing.
	if err := s.Confirm("screen_a", "now_1", 5, 1, "SIMULATED", looked); err != nil {
		t.Fatal(err)
	}
	now.Screen = "now_1"
	now.Outcomes = []observe.Reacquisition{found("control_1", "now_1", control)}
	// AND A WINDOW OF ANOTHER SIZE.
	now.Layout.Window = workflowstore.WindowGeometry{Width: 1280, Height: 720}

	got := a.StepReadinessAt(0, s, now)
	if got.Stage != workflowstore.StepLayoutChanged {
		t.Fatalf("step 0 is %q in a window of another size on a screen that WAS "+
			"established — a geometry check that only runs when recognition "+
			"fails is not the licence it claims to be: %s",
			got.Stage, got.Because)
	}
	if got.Action != nil {
		t.Fatal("a demonstrated point was offered in a window the demonstration " +
			"never saw, because something else agreed")
	}
}

// AND A DISPLAY SCALE THAT DIFFERS IS REFUSED, WHILE AN UNKNOWN ONE IS NOT A DIFFERENCE.
func TestADifferentDisplayScaleIsRefusedAndAnUnknownOneIsNotADifference(t *testing.T) {
	a := aMenuOverALiveWorld(t, observe.PointerAt{X: 0.12, Y: 0.11},
		rect(0.10, 0.10, 0.20, 0.04))
	s, now := replaying(t)

	a.Window.Scale = 1
	now.Layout.Window.Scale = 1.5
	if got := a.StepReadinessAt(0, s, now); got.Stage != workflowstore.StepLayoutChanged {
		t.Errorf("step 0 is %q at a different display scale: %s",
			got.Stage, got.Because)
	}

	// THE SAME PIXELS, with the claim removed from one side and then the other.
	// Unknown is a third answer: refusing it would refuse every artifact this
	// build records, and claiming 1 would compare 150% against 100% and call
	// them the same.
	a.Window.Scale = workflowstore.ScaleUnknown
	if got := a.StepReadinessAt(0, s, now); !got.Ready() {
		t.Errorf("step 0 is %q when the demonstration never observed a scale; "+
			"unknown is not a difference: %s", got.Stage, got.Because)
	}
	a.Window.Scale = 1.5
	now.Layout.Window.Scale = workflowstore.ScaleUnknown
	if got := a.StepReadinessAt(0, s, now); !got.Ready() {
		t.Errorf("step 0 is %q when nothing observed the scale now: %s",
			got.Stage, got.Because)
	}
}

// AN ARTIFACT THAT CANNOT SAY WHAT WINDOW IT WAS DEMONSTRATED IN IS REFUSED.
//
// The difference between a narrow exception and a hole. The licence permits a demonstrated point
// to be pressed WHILE the window is unchanged; an artifact with no recorded window cannot
// support that sentence however plausible its coordinates look.
func TestAnArtifactWithNoRecordedWindowIsRefused(t *testing.T) {
	a := aMenuOverALiveWorld(t, observe.PointerAt{X: 0.12, Y: 0.11},
		rect(100, 100, 200, 40))
	a.Window = nil
	s, now := replaying(t)

	got := a.StepReadinessAt(0, s, now)
	if got.Stage != workflowstore.StepLayoutUnrecorded {
		t.Fatalf("step 0 is %q with no recorded window: %s", got.Stage, got.Because)
	}
	if got.Action != nil {
		t.Fatal("a point was offered against a window nothing could be checked against")
	}
	if !strings.Contains(got.Because, "fresh demonstration") {
		t.Errorf("the refusal is %q and does not say what would fix it", got.Because)
	}
}

// WORDS THAT PUT THE CONTROL SOMEWHERE ELSE STILL REFUSE.
//
// The licence is for an UNCHANGED layout, so a look on which this control is positively
// SOMEWHERE ELSE is the cheapest evidence there is that it changed — a menu that has scrolled
// still has all its words, in the wrong places.
//
// Asked in the same one direction as everywhere else: text may refuse a match and may never
// propose one. Missing evidence is not contradictory evidence, and no words at all is the whole
// Ark case — a route that refused on silence would refuse exactly the demonstrations it exists
// for.
func TestWordsThatPutTheControlSomewhereElseStillRefuse(t *testing.T) {
	control := rect(0.10, 0.10, 0.20, 0.04)
	moved := rect(0.10, 0.30, 0.20, 0.04)
	a := aMenuOverALiveWorld(t, observe.PointerAt{X: 0.12, Y: 0.11}, control)
	a.Controls[0].Text = []string{"settings"}
	s, now := replaying(t)
	now.Candidates = []observe.Region{control, moved}
	now.Words = []observe.ReadWord{
		{Text: "quit", Where: rect(0.11, 0.108, 0.06, 0.02)},
		{Text: "settings", Where: rect(0.11, 0.308, 0.06, 0.02)},
	}

	got := a.StepReadinessAt(0, s, now)
	if got.Stage != workflowstore.StepControlContradicted {
		t.Fatalf("step 0 is %q where the control's own words are read well below "+
			"the demonstrated point: %s", got.Stage, got.Because)
	}
	if got.Action != nil {
		t.Error("a press was planned onto a point the words contradict")
	}

	silent := now
	silent.Words, silent.Candidates = nil, nil
	if quiet := a.StepReadinessAt(0, s, silent); !quiet.Ready() {
		t.Errorf("step 0 is %q on a look nobody could read a word on, which is "+
			"the case this mode exists for: %s", quiet.Stage, quiet.Because)
	}
}

// AND NOBODY'S AGREEMENT AND NOBODY'S AUTHORISATION IS SKIPPED.
//
// The mode replaces the screen question and the re-acquisition, and nothing else. A route that
// also skipped these would not be a narrow exception to the no-coordinate rule, it would be a
// second execution path with no human in it.
func TestTheLicenceSkipsNeitherAgreementNorAuthorisation(t *testing.T) {
	a := aMenuOverALiveWorld(t, observe.PointerAt{X: 0.12, Y: 0.11},
		rect(0.10, 0.10, 0.20, 0.04))

	unagreed := workflowstore.NewSessionBinding()
	_, now := replaying(t)
	if got := a.StepReadinessAt(0, unagreed, now); got.Stage != workflowstore.StepUnconfirmed {
		t.Errorf("step 0 is %q with nobody having agreed to it: %s",
			got.Stage, got.Because)
	}

	s, unauthorised := replaying(t)
	unauthorised.Authorised = false
	if got := a.StepReadinessAt(0, s, unauthorised); got.Stage != workflowstore.StepUnauthorised {
		t.Errorf("step 0 is %q in a session with no live authorisation: %s",
			got.Stage, got.Because)
	}
}
