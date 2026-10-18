package observe_test

// TWO READINGS OF ONE BUTTON, AND TWO BUTTONS THAT MERELY LOOK ALIKE.
//
// [[Experiment-050-two-readings-of-one-button]]: the proposer boxed Rocket League's PLAY entry
// as 166x40 and the learned detector as 308x44. The second's centre falls outside the first, so
// `sameControl` — mutual centre containment — says they are different, and the demonstration
// learned two identities for one button. The uncertainty logic noticed, correctly refused to
// resolve it, and left a step that could never run.
//
// What resolves it is not the rectangles. It is the presses: the person clicked (125,480) and
// then (159,499), and BOTH points lie inside BOTH rectangles.

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

func pointAt(x, y float64) observe.PointerAt {
	return observe.PointerAt{X: x / 1920, Y: y / 1080}
}

func boxAt(x0, y0, x1, y1 float64) observe.Region {
	return observe.Region{X: x0 / 1920, Y: y0 / 1080,
		Width: (x1 - x0) / 1920, Height: (y1 - y0) / 1080}
}

// The measured pair, and the two presses that established them.
var (
	proposerPlay = boxAt(26, 468, 192, 508) // 166x40
	detectorPlay = boxAt(59, 469, 367, 513) // 308x44
	firstPlay    = pointAt(125, 480)
	secondPlay   = pointAt(159, 499)
)

// clicks presses each point against the region given for it, as the proposer offered it.
func clicks(t *testing.T, c *observe.ClickTargets, screen observe.VisualScreenID,
	steps ...struct {
		where observe.Region
		at    observe.PointerAt
	}) {

	t.Helper()
	for i, s := range steps {
		obs := c.Press(i+1, s.at, screen, observe.Proposed(
			[]observe.Region{s.where}, i+1, 1, 1))
		if !obs.Landed() {
			t.Fatalf("press %d landed on nothing", i)
		}
	}
}

type step = struct {
	where observe.Region
	at    observe.PointerAt
}

// ONE BUTTON READ TWO WAYS IS ONE CONTROL.
func TestTwoReadingsOfOneButtonBecomeOneControl(t *testing.T) {
	var c observe.ClickTargets
	clicks(t, &c, "visual_1",
		step{proposerPlay, firstPlay},
		step{detectorPlay, secondPlay})

	reg := c.Registry()
	if len(reg) != 1 {
		t.Fatalf("%d target(s); the person pressed one button twice and both "+
			"rectangles cover both presses", len(reg))
	}
	if reg[0].Presses != 2 {
		t.Errorf("the control records %d press(es), want 2", reg[0].Presses)
	}
	// AND IT KEEPS ONE RECTANGLE, the tightest that still covers both presses.
	//
	// Keeping both was measured and is worse: re-acquisition then finds proposals
	// agreeing with each, those describe two different controls, and it refuses as
	// ambiguous on every look.
	if len(reg[0].Where) != 1 {
		t.Fatalf("the control carries %d rectangle(s); two that disagree with each "+
			"other make every re-acquisition ambiguous", len(reg[0].Where))
	}
	if !observe.SameControl(reg[0].Where[0], proposerPlay) {
		t.Errorf("kept %+v, want the tighter of the two readings", reg[0].Where[0])
	}
	if len(reg[0].MaybeOneControlWith) != 0 {
		t.Errorf("the control is still paired with %v", reg[0].MaybeOneControlWith)
	}
}

// AND TWO DIFFERENT ENTRIES STAY TWO CONTROLS, however alike they look.
//
// The owner was exact about this, and right: different controls can navigate to the same place,
// and two menu rows can read the same word. What separates them is that a press on one is not
// inside the other.
func TestTwoDifferentMenuEntriesStayTwoControls(t *testing.T) {
	var c observe.ClickTargets
	play := boxAt(26, 468, 192, 508)
	itemShop := boxAt(26, 514, 192, 554)
	clicks(t, &c, "visual_1",
		step{play, pointAt(109, 488)},
		step{itemShop, pointAt(109, 534)})

	if n := len(c.Registry()); n != 2 {
		t.Fatalf("%d target(s); two rows a person can see are different must stay "+
			"two controls", n)
	}
}

// AND A BAND ACROSS THE WINDOW SWALLOWS NOTHING.
//
// It contains every press on the screen. If containment alone merged, one band would collapse
// every control on a screen into a single identity.
func TestABandAcrossTheWindowDoesNotSwallowAControl(t *testing.T) {
	var c observe.ClickTargets
	band := observe.Region{X: 0, Y: 0, Width: 1, Height: 0.957}
	// THE SECOND PRESS IS INSIDE THE FIRST CONTROL, so only the band's own
	// unusability stands between them. Pressed where nothing but the band was
	// proposed, which is what a look with no small candidate looks like.
	clicks(t, &c, "visual_1",
		step{boxAt(26, 468, 192, 508), pointAt(109, 488)},
		step{band, pointAt(120, 492)})

	if n := len(c.Registry()); n != 2 {
		t.Fatalf("%d target(s); a band that locates nothing is not a reading of a "+
			"menu row", n)
	}
}

// AND A PRESS OUTSIDE THE CONTROL IS NOT ANOTHER READING OF IT.
func TestAPressOutsideTheControlIsNotAnotherReadingOfIt(t *testing.T) {
	var c observe.ClickTargets
	// THE EXISTING CONTROL DOES CONTAIN THE SECOND PRESS, so the first half of the
	// rule passes. What fails is the second: this reading does NOT contain the press
	// that established the control, so the two are not one thing read twice.
	clicks(t, &c, "visual_1",
		step{boxAt(26, 468, 192, 508), pointAt(180, 500)},
		step{boxAt(60, 470, 150, 620), pointAt(100, 490)})

	if n := len(c.Registry()); n != 2 {
		t.Fatalf("%d target(s); the second press is outside the first control", n)
	}
}

// AND THE SAME PLACE ON TWO SCREENS IS TWO CONTROLS.
func TestTheSamePlaceOnTwoScreensIsNotOneControl(t *testing.T) {
	var c observe.ClickTargets
	clicks(t, &c, "visual_1", step{proposerPlay, firstPlay})
	clicks(t, &c, "visual_2", step{detectorPlay, secondPlay})

	if n := len(c.Registry()); n != 2 {
		t.Fatalf("%d target(s); two screens are two places", n)
	}
}

// WHAT MUTUAL CENTRE CONTAINMENT CANNOT DO, MEASURED ON REAL RECTANGLES.
//
// Two limits of `SameRegion`, both found while measuring whether the frames around a press carry
// a better boundary than the press's own frame
// ([[Experiment-084-the-highlight-that-hides-the-label-draws-the-box]]). Neither is a bug: the
// relation is threshold-free and symmetric on purpose, and the alternative was an invented
// overlap fraction this subsystem has a measured history of getting wrong. They are limits, and
// they are written down because the owner asked three separate questions about a learned control
// and this relation is the whole of the answer to the second one:
//
//	Recognition: Can Marco distinguish that button from similar but incorrect controls?
func TestAPanelIsOneControlWithWhicheverButtonSitsOnItsCentre(t *testing.T) {
	// Experiment 082's learned Ark control -- 49%x64% of the window -- and a button of
	// Experiment 080's size placed where a menu's MIDDLE entry would be: on the panel's
	// centre point. Nothing contrived: a pause menu with an odd number of entries has one
	// there by construction.
	panel := observe.Region{X: 0.171875, Y: 0.3611111111111111,
		Width: 0.48541666666666666, Height: 0.6388888888888888}
	midX := panel.X + panel.Width/2
	midY := panel.Y + panel.Height/2
	middle := observe.Region{X: midX - 0.08, Y: midY - 0.024, Width: 0.1625, Height: 0.0481}

	if !observe.Encloses(panel, middle) {
		t.Fatal("the fixture button is not inside the panel")
	}
	// SO THE PANEL AND THAT BUTTON ARE ONE CONTROL, and no comparison of rectangles alone
	// can say which of them was pressed. This is [[Experiment-082-a-control-that-is-half-
	// the-window]] in its smallest form.
	if !observe.SameRegion(panel, middle) {
		t.Errorf("the relation separates a %0.0f%%x%0.0f%% panel from the button on its "+
			"centre; if that is now true the finding has changed and this test should "+
			"be rewritten from the new measurement, not relaxed",
			panel.Width*100, panel.Height*100)
	}

	// AND A BUTTON AWAY FROM THE CENTRE IS CORRECTLY SEPARATED, which is what makes the
	// case above a limit of the relation rather than a claim that it never works.
	edge := observe.Region{X: panel.X + 0.01, Y: panel.Y + 0.01,
		Width: 0.1625, Height: 0.0481}
	if !observe.Encloses(panel, edge) {
		t.Fatal("the fixture edge button is not inside the panel")
	}
	if observe.SameRegion(panel, edge) {
		t.Error("every button inside the panel is one control with it, which would make " +
			"the relation useless rather than limited")
	}
}

// AND THE RELATION IS NOT TRANSITIVE, so a chain of reacquisitions can drift.
//
// Measured on two real proposals for ONE button -- Experiment 050's PLAY entry -- taken from
// adjacent looks of `rl-freeplay-1`: 353x44 at (13,468) on the look of the press, and 90x33 at
// (75,472) on the look after it. Each is one control with the button a person established.
// They are not one control with each other.
func TestTwoReadingsCanEachBeTheButtonWithoutBeingEachOther(t *testing.T) {
	button := observe.Region{X: 26.0 / 1920, Y: 468.0 / 1080,
		Width: 166.0 / 1920, Height: 40.0 / 1080}
	wide := observe.Region{X: 0.007, Y: 0.433, Width: 0.184, Height: 0.041}
	narrow := observe.Region{X: 0.039, Y: 0.437, Width: 0.047, Height: 0.031}

	if !observe.SameRegion(wide, button) || !observe.SameRegion(narrow, button) {
		t.Fatal("the fixture pair are not both readings of the button")
	}
	if observe.SameRegion(wide, narrow) {
		t.Error("the relation is transitive on this triple, so the drift this test " +
			"records cannot happen and the test should be deleted rather than relaxed")
	}
}
