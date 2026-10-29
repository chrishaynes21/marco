package main

import (
	"image"
	"image/color"
	"image/draw"
	"path/filepath"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demonstrated"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/structure"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// A PERFORMABLE ACT OVER A WINDOW THIS TEST SUPPLIES, so a test of Director's RUN LOGIC does
// not need the owner's screen.
//
// # What was wrong with asking the desktop
//
// Eleven tests of arithmetic and bookkeeping — a deadline being announced, a question being
// withdrawn, a grant for the wrong Act refusing, a record naming the account it came from —
// had to manufacture a performable act first, and the only way to get one was to photograph
// whatever happened to be in front of the owner. So they inherited every precondition that
// photograph has: the window must exist, hold still across three looks, propose something
// control-shaped, and keep that control under the same point. With a game on screen those
// preconditions fail, and the gate reported eight to ten undeclared skips for a reason not one
// of those tests was about.
//
// A conditional skip cannot be declared either: `gate-skips.txt` fails an undeclared skip AND a
// declaration whose test ran, so an environment-dependent one is a finding both ways. The same
// rule that put the live resolver test behind `-tags resolver`.
//
// # Why the pixels may be drawn and the application may not
//
// Measured, by building the routine from a drawn window while keeping the live window's
// identity: all eleven still passed, with a game animating in front. The run never compares the
// saved arrangement with the screen in these tests — none of them presses anything, and the two
// that get furthest are satisfied by the run REFUSING for want of an answer.
//
// Then measured again with the identity stated as well, and they failed:
//
//	this workflow was demonstrated against "marcoprobe" and the application resolved now is
//	"code"; a workflow may only act on the application it was shown in
//
// That guard is right, and it is why the whole desktop is stated rather than only the picture:
// the application the act was demonstrated against has to be the one the run resolves, and
// `winprovider.DesktopForTest` is what makes it so without borrowing a real window.
//
// The live claims are not replaced by this. They are tagged and run in the stage that takes the
// screen — see [[ADR-269-a-screen-free-gate-and-the-tests-that-need-the-screen]] for the
// inventory tying each one to the claim it establishes.
func anActOverAStatedWindow(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("MARCO_HOME", home)
	t.Setenv("MARCO_ROUTES", filepath.Join(home, "routes"))

	w := theStatedWindow(t)
	img := aDrawnWindow()
	regions := observe.NormaliseRects(img.Bounds(),
		boundsOf(structure.Propose(img, structure.DefaultOptions())))
	// A FAILURE AND NOT A SKIP. The picture is in this file, so a picture production
	// perception finds nothing in is a broken fixture, not an absent precondition — which
	// is the whole difference between this fixture and the live one.
	best, ok := theControlShapedRegion(regions)
	if !ok {
		t.Fatalf("the drawn window proposes %d region(s) and none is control-shaped, "+
			"so this fixture can no longer manufacture an act", len(regions))
	}

	art := anArtifactOver(w, regions, best)
	store := mustOpenStore(t)
	if err := store.SaveSequence(art); err != nil {
		t.Fatal(err)
	}
	src, binds, err := demonstrated.Compose(art, "an act",
		[]demonstrated.Named{{Position: 0, Name: "the control in front"}},
		time.Now(), marcoChecks{})
	if err != nil {
		t.Fatal(err)
	}
	rt := routes.Route{Slug: routes.Slug(art.Name)}
	if err := demonstrated.Save(routesHere(), rt, src, binds, time.Now()); err != nil {
		t.Fatal(err)
	}
	return art.Name
}

// AND THE DRAWN WINDOW IS SOMETHING PRODUCTION PERCEPTION CAN READ.
//
// The numbers in [[ADR-269-a-screen-free-gate-and-the-tests-that-need-the-screen]] were measured
// and then asserted by nothing, which is the shape of a claim that quietly stops being true: a
// change to `structure.Propose` could leave this picture proposing one region instead of four
// and every test above would still pass, because a fixture needs only ONE control-shaped region
// to manufacture an act. It would no longer resemble a window, and the thing it is standing in
// for is a window.
//
// Three looks, because that is what the live fixture requires of a real window, and the point of
// a drawn one is that it passes that bar outright: measured at 1.000 against the 0.55 a run
// requires, where a real window in front of somebody playing a game agreed at 0.339.
func TestTheDrawnWindowIsSomethingPerceptionCanRead(t *testing.T) {
	img := aDrawnWindow()
	var looks [][]observe.Region
	for i := 0; i < 3; i++ {
		looks = append(looks, observe.NormaliseRects(img.Bounds(),
			boundsOf(structure.Propose(img, structure.DefaultOptions()))))
	}
	const want = 4
	if len(looks[0]) != want {
		t.Errorf("the drawn window proposes %d regions, not the %d it is drawn with; "+
			"it no longer resembles the thing it stands in for", len(looks[0]), want)
	}
	// EVERY ONE OF THEM CONTROL-SHAPED, bounded in both axes the way a control is.
	for i, r := range looks[0] {
		if r.Width <= 0 || r.Height <= 0 || r.Width > 0.5 || r.Height > 0.25 {
			t.Errorf("region %d is %.3fx%.3f, which is not control-shaped",
				i+1, r.Width, r.Height)
		}
	}
	// AND IT HOLDS STILL, which is the whole reason it is drawn.
	for i := range looks {
		for j := i + 1; j < len(looks); j++ {
			agree := observe.ArrangementSimilarity(
				observe.ArrangeLook(looks[i]), observe.ArrangeLook(looks[j]))
			if agree < 1.0 {
				t.Errorf("looks %d and %d of a DRAWN window agree at %.3f; "+
					"a picture that cannot agree with itself is not a "+
					"fixture", i+1, j+1, agree)
			}
		}
	}
}

// theStatedWindow is the one window on the desktop these tests run against.
//
// The desktop is stated through `winprovider.DesktopForTest`, which answers the window list,
// the handle lookup and process liveness out of this one window — see its note for why those
// cannot be three separate knobs.
func theStatedWindow(t *testing.T) directorapi.Window {
	t.Helper()
	const (
		handle       uintptr = 0x00A10001
		owner        uint32  = 41001
		behindHandle uintptr = 0x00A10002
		behindOwner  uint32  = 41002
	)
	bounds := winctx.Rect{X: 0, Y: 0, W: drawnWidth, H: drawnHeight}
	// TWO WINDOWS, AND ONE OF THEM IS BEHIND. A desktop with a single window is not a
	// desktop anybody has, and the difference is load-bearing here: `aBackgroundApplication`
	// looks for an application that is NOT in front, so a one-window fixture left
	// `TestGeometryReplayTakesNoLookAtThePressBoundary` skipping for a new reason while
	// looking like it had been fixed. Each application has exactly one window, which is
	// what `anAcquirableApplication` requires.
	winprovider.DesktopForTest(t, []winctx.LiveWindow{{
		Handle:     handle,
		ProcessID:  owner,
		Image:      "statedwindow",
		Title:      "the window in front",
		Bounds:     bounds,
		Visible:    true,
		Foreground: true,
		OnScreen:   true,
	}, {
		Handle:    behindHandle,
		ProcessID: behindOwner,
		Image:     statedWindowBehind,
		Title:     "the window behind",
		Bounds:    winctx.Rect{X: 40, Y: 40, W: 900, H: 600},
		Visible:   true,
		OnScreen:  true,
	}})
	return directorapi.Window{
		ID:          winprovider.WindowID(handle),
		Application: "statedwindow",
		Title:       "the window in front",
		Bounds: directorapi.Rect{X: bounds.X, Y: bounds.Y,
			Width: bounds.W, Height: bounds.H},
	}
}

// statedWindowBehind is the application the stated desktop keeps BEHIND the one in front, for
// the tests that need an application that is not the foreground.
const statedWindowBehind = "windowbehind"

const (
	drawnWidth  = 1280
	drawnHeight = 800
)

// aDrawnWindow is a picture of a window with four controls in it.
//
// Drawn rather than captured, and PRODUCTION PERCEPTION IS STILL WHAT READS IT: the regions
// come from `structure.Propose` exactly as they do from a photograph, so the routine this
// fixture saves is one the production proposer agreed to. Nothing here stubs perception; it
// supplies the pixels and lets perception answer.
//
// Measured: four regions, all four control-shaped, three looks agreeing at 1.000 against the
// 0.55 the run requires. A real window in front of somebody playing a game agreed at 0.339.
func aDrawnWindow() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, drawnWidth, drawnHeight))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{28, 30, 36, 255}},
		image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(180, 120, 1100, 700),
		&image.Uniform{color.RGBA{48, 52, 62, 255}}, image.Point{}, draw.Src)
	for i, r := range []image.Rectangle{
		image.Rect(260, 220, 620, 300),
		image.Rect(260, 340, 620, 420),
		image.Rect(260, 460, 620, 540),
		image.Rect(700, 220, 1020, 300),
	} {
		c := color.RGBA{uint8(200 - i*10), uint8(205 - i*8), 215, 255}
		draw.Draw(img, r, &image.Uniform{c}, image.Point{}, draw.Src)
	}
	return img
}
