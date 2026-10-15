package observe_test

// WHAT A DEMONSTRATED CLICK REMEMBERS ABOUT WHAT IT CLICKED.
//
// [[Experiment-049-the-control-it-could-see-and-could-not-name]] is the measurement behind every
// case here. The live run pressed PLAY, recognised the Play page, and then refused the Training
// step with `"TRAINING"` sitting in its own candidate listing — because the demonstration had
// recorded a rectangle and no name.
//
// The rules under test are the ones that make a name safe to learn: it comes from inside the
// control's own rectangle, a band across the window learns nothing, and one look is never asked
// to settle it alone.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// word is one phrase read at one place.
func word(text string, x, y, w, h float64) observe.ReadWord {
	return observe.ReadWord{Text: text,
		Where: observe.Region{X: x, Y: y, Width: w, Height: h}}
}

// theTile is the Training tile as the learned detector boxed it, and theCaption the word that
// sits inside it — both taken from the recording.
var (
	theTile    = observe.Region{X: 0.208, Y: 0.491, Width: 0.185, Height: 0.205}
	theCaption = word("TRAINING", 0.213, 0.673, 0.054, 0.015)
	elsewhere  = word("PLAY OFFLINE", 0.411, 0.673, 0.079, 0.015)
	theWindow  = observe.Region{X: 0, Y: 0, Width: 1, Height: 0.957}
)

// A LABEL COMES FROM INSIDE THE CONTROL'S OWN RECTANGLE.
func TestAControlIsNamedByTheWordsInsideIt(t *testing.T) {
	got := observe.LabelOf(theTile, []observe.ReadWord{theCaption, elsewhere})
	if len(got) != 1 || got[0] != "training" {
		t.Fatalf("the tile was named %v; the word inside it is TRAINING and the one "+
			"beside it is not", got)
	}
}

// AND A BAND ACROSS THE WINDOW LEARNS NOTHING AT ALL.
//
// The measured case is `control_3`, the Free Play control, saved as 0,0 1.000x0.957. A
// full-window hypothesis contains every word on the screen, and a rule that named it from them
// would teach Marco that the control is called everything.
func TestAFullWindowHypothesisLearnsNoName(t *testing.T) {
	got := observe.LabelOf(theWindow, []observe.ReadWord{theCaption, elsewhere})
	if len(got) != 0 {
		t.Fatalf("a band across the window was named %v; it locates nothing, so a "+
			"word inside it says nothing about what was clicked", got)
	}
}

// AND A LOOK NOBODY READ NAMES NOTHING, which is not the same as a control with no name.
func TestALookNobodyReadNamesNothing(t *testing.T) {
	if got := observe.LabelOf(theTile, nil); got != nil {
		t.Errorf("a look with no reading produced the name %v", got)
	}
}

// ONE LOOK IS NEVER ASKED TO SETTLE WHAT A CONTROL SAYS.
//
// The press that started this landed on the one look where OCR had stopped reading the word
// PLAY, because a highlighted menu entry renders differently. So a word is kept when it was read
// on at least TextCommon of the control's looks, and a word read once is this reader's noise.
func TestAControlKeepsTheWordsMostOfItsLooksAgreedOn(t *testing.T) {
	var c observe.ClickTargets
	// A press establishes the control, then the screen is read five times.
	clickOn(t, &c, "visual_1", theTile, 0.3, 0.55)
	for i := 0; i < 4; i++ {
		c.Reading("visual_1", []observe.ReadWord{theCaption})
	}
	// One look reads something else inside the same rectangle.
	c.Reading("visual_1", []observe.ReadWord{word("TRAINNG", 0.213, 0.673, 0.054, 0.015)})

	got := labelIn(t, &c)
	if got != "training" {
		t.Errorf("the control was learned as %q; TRAINING was read on four looks of "+
			"five and the misreading on one", got)
	}
}

// AND A CONTROL READ ONLY BEFORE IT WAS CLICKED IS STILL NAMED.
//
// Measured: PLAY was read on seventeen looks of the main menu and on NONE of the looks around
// the press, because the entry was highlighted by the time the pointer reached it. A rule that
// asked only at the moment of the press learns nothing here.
func TestAControlReadBeforeItWasClickedIsStillNamed(t *testing.T) {
	var c observe.ClickTargets
	for i := 0; i < 4; i++ {
		c.Reading("visual_1", []observe.ReadWord{theCaption})
	}
	// AND THEN IT IS CLICKED, on a look nobody could read.
	clickOn(t, &c, "visual_1", theTile, 0.3, 0.55)
	c.Reading("visual_1", nil)

	if got := labelIn(t, &c); got != "training" {
		t.Errorf("the control was learned as %q, although every look before the "+
			"press read TRAINING inside it", got)
	}
}

// AND A READING OF ANOTHER SCREEN NAMES NOTHING ON THIS ONE.
func TestAReadingOfAnotherScreenDoesNotNameThisControl(t *testing.T) {
	var c observe.ClickTargets
	clickOn(t, &c, "visual_1", theTile, 0.3, 0.55)
	for i := 0; i < 4; i++ {
		c.Reading("visual_9", []observe.ReadWord{theCaption})
	}
	if got := labelIn(t, &c); got != "" {
		t.Errorf("a control on visual_1 was named %q by readings of visual_9", got)
	}
}

// press clicks inside `where`, with `where` as the only proposal containing it.
func clickOn(t *testing.T, c *observe.ClickTargets, screen observe.VisualScreenID,
	where observe.Region, x, y float64) {

	t.Helper()
	obs := c.Press(1, observe.PointerAt{X: x, Y: y}, screen,
		observe.Proposed([]observe.Region{where}, 1, 1, 1))
	if !obs.Landed() {
		t.Fatalf("the press at %.3f,%.3f landed on nothing", x, y)
	}
}

// labelIn is the single target's learned name, as the registry publishes it.
func labelIn(t *testing.T, c *observe.ClickTargets) string {
	t.Helper()
	reg := c.Registry()
	if len(reg) != 1 {
		t.Fatalf("%d target(s) in the registry, want 1", len(reg))
	}
	return strings.Join(reg[0].Text, " ")
}

// THE TWO DIRECTIONS A LABEL CAN BE MATCHED IN, AND WHY THEY ARE NOT ONE RULE.
func TestWhichWayACandidateHasToBeNamedDependsOnWhereTheWordsCameFrom(t *testing.T) {
	for _, c := range []struct {
		name         string
		said, wanted []string
		demonstrated bool
		want         bool
	}{{
		name: "a demonstrated label is found inside a richer reading",
		said: []string{"custom", "training"}, wanted: []string{"training"},
		demonstrated: true, want: true,
	}, {
		name: "and the same pair fails the workflow-title rule",
		said: []string{"custom", "training"}, wanted: []string{"training"},
		demonstrated: false, want: false,
	}, {
		name: "a title names a control whose label is made of its words",
		said: []string{"free", "play"}, wanted: []string{"enter", "free", "play"},
		demonstrated: false, want: true,
	}, {
		name: "a demonstrated label is not found in a reading that lacks it",
		said: []string{"quick", "play"}, wanted: []string{"training"},
		demonstrated: true, want: false,
	}} {
		t.Run(c.name, func(t *testing.T) {
			if got := observe.NamesControl(c.said, c.wanted,
				c.demonstrated); got != c.want {
				t.Errorf("NamesControl(%v, %v, %v) = %v, want %v",
					c.said, c.wanted, c.demonstrated, got, c.want)
			}
		})
	}
}
