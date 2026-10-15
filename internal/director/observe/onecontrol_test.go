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
