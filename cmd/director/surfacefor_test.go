package main

// THE TEXT IDENTIFIES THE CONTROL. IT IS NOT NECESSARILY WHAT TO PRESS.
//
// Measured on the Training tile of [[Experiment-049-the-control-it-could-see-and-could-not-name]]:
// the caption's own rectangle is 118x32 and its centre sits 17 px above the bottom edge of the
// 354x221 tile it labels, while the demonstrated press was in the middle of that tile. Worse, it
// was not even stable — which of the two the matcher returned depended on whether the proposer
// happened to box the caption separately on that look, so the press point moved between the
// tile's centre and its lower edge from one look to the next.
//
// The regions below are the measured ones, in window fractions of 1920x1080.

import (
	"context"
	"image"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

var (
	// theSavedTile is the saved reading: 355x221 at (399,530).
	theSavedTile = observe.Region{X: 399.0 / 1920, Y: 530.0 / 1080,
		Width: 355.0 / 1920, Height: 221.0 / 1080}
	// theCaptionBox is what OCR read TRAINING in: 118x32 at (402,718).
	theCaptionBox = observe.Region{X: 402.0 / 1920, Y: 718.0 / 1080,
		Width: 118.0 / 1920, Height: 32.0 / 1080}
	// theTileOnThisLook is the same tile proposed again, a pixel or two different.
	theTileOnThisLook = observe.Region{X: 400.0 / 1920, Y: 529.0 / 1080,
		Width: 354.0 / 1920, Height: 222.0 / 1080}
	// theCaptionRow is the band the caption sits in: 353x34 at (396,719). It contains
	// the caption and is NOT the tile.
	theCaptionRow = observe.Region{X: 396.0 / 1920, Y: 719.0 / 1080,
		Width: 353.0 / 1920, Height: 34.0 / 1080}
)

// withSavedControl is a regrounder over an artifact whose control_1 was demonstrated at `saved`.
func withSavedControl(t *testing.T, saved []observe.Region,
	candidates []observe.Region) *regrounder {

	t.Helper()
	d := workflow.Draft{Name: "enter free play"}
	d.Steps = []workflow.Step{{
		From: "visual_1", To: "visual_2", Target: "target_1",
		Cause: observe.CauseClick, Seen: 2, Readings: saved,
		Text: []string{"training"},
	}}
	a, err := workflowstore.SaveWith(d,
		map[observe.VisualScreenID]observe.VisualArrangement{
			"visual_1": {Cells: map[string]int{"0,0": 9}, Total: 9},
			"visual_2": {Cells: map[string]int{"1,1": 7}, Total: 7},
		}, nil, time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return &regrounder{art: a, candidates: candidates}
}

// THE PRESS GOES TO THE SURFACE THE DEMONSTRATION ESTABLISHED.
func TestAControlIdentifiedByItsCaptionIsPressedOnTheSurfaceAroundIt(t *testing.T) {
	g := withSavedControl(t, []observe.Region{theSavedTile},
		[]observe.Region{theCaptionBox, theTileOnThisLook, theCaptionRow})

	press, surfaced := g.surfaceFor("control_1", theCaptionBox)
	if !surfaced {
		t.Fatal("the caption was pressed although the tile around it is on this " +
			"look and agrees with the demonstrated control")
	}
	if !observe.SameControl(press, theSavedTile) {
		t.Fatalf("pressing %+v, which is not a reading of the demonstrated control",
			press)
	}
	// AND THE POINT IS THE TILE'S MIDDLE, not seventeen pixels off its bottom edge.
	y := (press.Y + press.Height/2) * 1080
	if y < 600 || y > 680 {
		t.Errorf("the press point is at y %.0f; the demonstrated press was at 640 "+
			"and the caption's centre is at 734", y)
	}
}

// AND WHEN NOTHING AROUND IT AGREES, THE TEXT IS PRESSED AND THE CALLER IS TOLD.
//
// Measured: on the look where the page was changing, the tile was not proposed at all and the
// only thing containing the caption was the caption's own row.
func TestWithNoAgreeingSurfaceTheTextItselfIsPressed(t *testing.T) {
	g := withSavedControl(t, []observe.Region{theSavedTile},
		[]observe.Region{theCaptionBox, theCaptionRow})

	press, surfaced := g.surfaceFor("control_1", theCaptionBox)
	if surfaced {
		t.Fatalf("a surface was claimed at %+v; the caption's own row is not a "+
			"reading of a 355x221 tile", press)
	}
	if press != theCaptionBox {
		t.Errorf("pressing %+v, want the identifying text itself", press)
	}
}

// AND A CONTROL WITH NO SAVED READING HAS NO DEMONSTRATED SHAPE TO PRESERVE.
func TestWithNoSavedReadingTheTextItselfIsPressed(t *testing.T) {
	g := withSavedControl(t, nil, []observe.Region{theCaptionBox, theTileOnThisLook})

	press, surfaced := g.surfaceFor("control_1", theCaptionBox)
	if surfaced || press != theCaptionBox {
		t.Errorf("pressing %+v (surfaced %v) for a control the demonstration never "+
			"recorded a rectangle for", press, surfaced)
	}
}

// AND A BAND ACROSS THE WINDOW IS NEVER A SURFACE, whatever it contains.
func TestAFullWindowBandIsNotAnInteractionSurface(t *testing.T) {
	band := observe.Region{X: 0, Y: 0, Width: 1, Height: 0.957}
	g := withSavedControl(t, []observe.Region{band},
		[]observe.Region{theCaptionBox, band})

	press, surfaced := g.surfaceFor("control_1", theCaptionBox)
	if surfaced {
		t.Fatalf("a band across the window was chosen as the thing to press: %+v",
			press)
	}
}

// AND THE SMALLEST AGREEING SURFACE WINS, not the largest.
//
// Two proposals of one tile, a pixel or two apart, both agree with the saved reading by mutual
// centre containment. Taking the larger would creep outwards every time a proposer read the tile
// generously, which is the drift this subsystem has paid for before.
func TestTheSmallestAgreeingSurfaceIsTheOnePressed(t *testing.T) {
	bigger := observe.Region{X: 397.0 / 1920, Y: 528.0 / 1080,
		Width: 360.0 / 1920, Height: 227.0 / 1080}
	g := withSavedControl(t, []observe.Region{theSavedTile},
		[]observe.Region{theCaptionBox, bigger, theTileOnThisLook})

	press, surfaced := g.surfaceFor("control_1", theCaptionBox)
	if !surfaced {
		t.Fatal("no surface was found although two candidates agree")
	}
	if press != theTileOnThisLook {
		t.Errorf("pressing %+v; the smaller of the two agreeing surfaces is %+v",
			press, theTileOnThisLook)
	}
}

// AND THE WHOLE PATH DOES IT, not just the helper.
//
// Entered through `Reground`, which is what the coordinator calls. Deleting the `surfaceFor`
// call from it must fail this: the region handed back is what gets pressed.
func TestRegroundingACaptionHandsBackTheSurfaceToPress(t *testing.T) {
	g := withSavedControl(t, []observe.Region{theSavedTile},
		[]observe.Region{theCaptionBox, theTileOnThisLook})
	var out strings.Builder
	g.out = &out
	// OCR READS THE CAPTION, in window pixels, as the live reader hands them over.
	g.words = func(ctx context.Context) ([]readWord, image.Rectangle, bool) {
		return []readWord{{Text: "TRAINING", X: 402, Y: 718, W: 118, H: 32}},
			image.Rect(0, 0, 1920, 1080), true
	}

	where, why, ok := g.Reground(context.Background(), 1, "control_1",
		workflowstore.StepControlUnresolved, workflowrun.Sighting{})
	if !ok {
		t.Fatalf("the control was not re-grounded: %s", out.String())
	}
	if !observe.SameControl(where, theSavedTile) {
		t.Fatalf("re-grounded to %+v, which is not a reading of the demonstrated "+
			"control. The caption identifies it; it is not what to press.\n%s",
			where, out.String())
	}
	for _, want := range []string{"identified by its text at", "pressing the surface around it"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the report does not say %q:\n%s", want, out.String())
		}
	}
	if !strings.Contains(why, "agrees with the demonstrated control") {
		t.Errorf("the reason does not say what the press rests on: %s", why)
	}
}

// THE ROTATING NEWS PANEL IS NEVER PLAY.
//
// [[Experiment-050-two-readings-of-one-button]]: the live run's re-grounder matched the saved
// label "play" against three regions, because Rocket League's news panel was advertising
// "Play 10 Competitive 2v2 Doubles Matches". It refused, which was right and left a step that
// could never run. A saved label is not unique; the demonstration's own geometry is what says
// which occurrence is the control.
func TestTheRotatingNewsPanelIsNeverTheDemonstratedControl(t *testing.T) {
	// The PLAY entry as the demonstration recorded it, and as this look proposes it.
	saved := observe.Region{X: 26.0 / 1920, Y: 468.0 / 1080,
		Width: 166.0 / 1920, Height: 40.0 / 1080}
	onThisLook := observe.Region{X: 30.0 / 1920, Y: 470.0 / 1080,
		Width: 160.0 / 1920, Height: 38.0 / 1080}
	// The news panel, far away, carrying the same word.
	banner := observe.Region{X: 1520.0 / 1920, Y: 600.0 / 1080,
		Width: 300.0 / 1920, Height: 40.0 / 1080}

	d := workflow.Draft{Name: "enter free play"}
	d.Steps = []workflow.Step{{
		From: "visual_1", To: "visual_2", Target: "target_1",
		Cause: observe.CauseClick, Seen: 2,
		Readings: []observe.Region{saved}, Text: []string{"play"},
	}}
	a, err := workflowstore.SaveWith(d,
		map[observe.VisualScreenID]observe.VisualArrangement{
			"visual_1": {Cells: map[string]int{"0,0": 9}, Total: 9},
			"visual_2": {Cells: map[string]int{"1,1": 7}, Total: 7},
		}, nil, time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	g := &regrounder{art: a, candidates: []observe.Region{onThisLook, banner}}
	f, ferr := os.CreateTemp(t.TempDir(), "r*.txt")
	if ferr != nil {
		t.Fatal(ferr)
	}
	t.Cleanup(func() { f.Close() })
	g.out = f
	g.words = func(context.Context) ([]readWord, image.Rectangle, bool) {
		return []readWord{
			{Text: "PLAY", X: 42, Y: 476, W: 50, H: 20},
			{Text: "Play 10 Competitive 2v2 Doubles Matches",
				X: 1530, Y: 608, W: 280, H: 24},
		}, image.Rect(0, 0, 1920, 1080), true
	}

	where, _, ok := g.Reground(context.Background(), 0, "control_1",
		workflowstore.StepControlUnresolved, workflowrun.Sighting{})
	if !ok {
		b, _ := os.ReadFile(f.Name())
		t.Fatalf("the control was not re-grounded although the menu entry is on "+
			"this look and agrees with where it was demonstrated:\n%s%s",
			out.String(), b)
	}
	if !observe.SameControl(where, saved) {
		t.Fatalf("re-grounded to %+v, which is not where PLAY was demonstrated. "+
			"The news banner carries the word and is not the control.", where)
	}
	mid := observe.PointerAt{X: where.X + where.Width/2, Y: where.Y + where.Height/2}
	if mid.X*1920 > 400 {
		t.Errorf("the press would land at x %.0f, which is out in the news panel",
			mid.X*1920)
	}
}

// AND WHEN THE DEMONSTRATION LEFT NO USABLE RECTANGLE, THE WORDS STILL DECIDE.
//
// [[ADR-193-a-control-re-grounded-from-the-workflows-own-name]] exists for controls saved as
// bands across the window. Requiring those to corroborate a text match against a rectangle that
// names nowhere would refuse every repair it was built to make.
func TestAControlSavedAsABandIsStillRepairableFromItsWords(t *testing.T) {
	band := observe.Region{X: 0, Y: 0, Width: 1, Height: 0.957}
	onThisLook := observe.Region{X: 0.3, Y: 0.4, Width: 0.1, Height: 0.04}

	d := workflow.Draft{Name: "free play"}
	d.Steps = []workflow.Step{{
		From: "visual_1", To: "visual_2", Target: "target_1",
		Cause: observe.CauseClick, Seen: 1, Readings: []observe.Region{band},
	}}
	a, err := workflowstore.SaveWith(d,
		map[observe.VisualScreenID]observe.VisualArrangement{
			"visual_1": {Cells: map[string]int{"0,0": 9}, Total: 9},
			"visual_2": {Cells: map[string]int{"1,1": 7}, Total: 7},
		}, nil, time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	f, ferr := os.CreateTemp(t.TempDir(), "r*.txt")
	if ferr != nil {
		t.Fatal(ferr)
	}
	t.Cleanup(func() { f.Close() })
	g := &regrounder{art: a, out: f, candidates: []observe.Region{onThisLook}}
	g.words = func(context.Context) ([]readWord, image.Rectangle, bool) {
		return []readWord{{Text: "FREE PLAY", X: 590, Y: 440, W: 100, H: 24}},
			image.Rect(0, 0, 1920, 1080), true
	}

	where, _, ok := g.Reground(context.Background(), 0, "control_1",
		workflowstore.StepControlUnlocated, workflowrun.Sighting{})
	if !ok {
		t.Fatal("a control saved as a band is no longer repairable from its words")
	}
	if where != onThisLook {
		t.Errorf("re-grounded to %+v, want the region the words were read in", where)
	}
}
