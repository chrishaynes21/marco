package main

// THE PRESS BOUNDARY IN GEOMETRY REPLAY MODE.
//
// The last gate before a click goes out. On the vision path it asks eight questions, five of
// which need a look taken at that instant: a screen hypothesis standing, an observation
// generation unmoved, a look that could be taken at all, a reading of that look's own words, and
// a proposal under the approved point.
//
// Those are prerequisites of RECOGNISING a control, and this mode exists for the menus where
// recognition has no answer — a menu drawn over a live 3D world, filed as two different screens
// depending on what moves behind it, with zero words that stay. A boundary demanding them
// refuses every press the mode was granted for, which is what it did: the planner would license
// the press and the boundary would turn it down, so the mode could not send an action.
//
// So geometry replay has its OWN boundary, with its own terms. Four about where a click will
// land, plus the one the mode exists for. The vision path is untouched, and the first test here
// is the one that says so.

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// theDemonstratedWindow is what the fixture's routine was demonstrated in.
var theDemonstratedWindow = workflowstore.WindowGeometry{Width: 1920, Height: 1080}

// aGeometryReplayWorld is the world at the instant of a press in geometry replay mode: the
// window alive, the application right, the generation unmoved, the target in front, the window
// the size it was demonstrated in — and NOTHING ELSE.
//
// No fresh look, no screen, no observation generation, no reading, no proposal. The zero values
// are the point of the fixture: this is a boundary that has to pass with no vision at all.
func aGeometryReplayWorld() (worldNow, workflowrun.Sighting) {
	now := worldNow{
		Alive: true, State: "valid", Generation: 3,
		Front: observe.FrontTarget, Application: "thegame",
		Bounds: directorapi.Rect{X: 20, Y: 40, Width: 1920, Height: 1080},
		Layout: &theDemonstratedWindow,
	}
	planned := workflowrun.Sighting{Generation: 3}
	return now, planned
}

func askGeometry(now worldNow, planned workflowrun.Sighting) (bool, string) {
	return worldMoved("thegame", workflowstore.NormaliseScope("thegame"), now, planned)
}

// A PRESS GOES OUT WITH NO OCR, NO PROPOSALS AND NO LOOK AT ALL.
//
// The whole case. Every vision term is at its zero value, which on the vision path is a refusal
// three times over, and in this mode is simply not a question.
func TestGeometryReplayPressesWithNoReadingAndNoProposal(t *testing.T) {
	now, planned := aGeometryReplayWorld()

	// THE FIXTURE IS THE CLAIM: nothing was looked at.
	if now.Fresh.Took || now.Fresh.Said || now.Fresh.Target || now.Screen != "" ||
		now.Epoch != 0 {
		t.Fatal("the fixture supplies vision evidence and is not posing the question")
	}

	ok, why := askGeometry(now, planned)
	if !ok {
		t.Fatalf("the press boundary refused a geometry replay for want of "+
			"evidence this mode has none of: %s", why)
	}

	// ── AND THE VISION PATH IS UNCHANGED BY THE SAME WORLD ──
	//
	// The same zero values, with the mode off, must still refuse — otherwise this
	// has not added a second boundary, it has weakened the first one.
	vision := now
	vision.Layout = nil
	if ok, _ := askGeometry(vision, planned); ok {
		t.Error("with the mode off, a world with no fresh look at all passed the " +
			"press boundary; the vision path has been weakened rather than " +
			"a second one added")
	}
}

// A WINDOW OF ANOTHER SIZE REFUSES AT THE BOUNDARY, NOT ONLY AT THE PLAN.
//
// Asked twice for the reason every other term here is asked twice: a person authorising a press
// is an unbounded pause in the middle of the one sequence that must not have one, and a window
// can be resized inside it. The point is normalised, so a resized window still produces a
// coordinate — somewhere plausible, in a layout nobody demonstrated.
func TestAResizedWindowRefusesAtThePressBoundary(t *testing.T) {
	now, planned := aGeometryReplayWorld()
	now.Bounds = directorapi.Rect{X: 20, Y: 40, Width: 1280, Height: 720}

	ok, why := askGeometry(now, planned)
	if ok {
		t.Fatal("a press went out into a window half the size of the one this " +
			"was demonstrated in")
	}
	for _, want := range []string{"1920x1080", "1280x720"} {
		if !strings.Contains(why, want) {
			t.Errorf("the refusal is %q and does not name %s", why, want)
		}
	}
}

// AND THE FOUR TERMS ABOUT WHERE A CLICK LANDS ARE ALL STILL ASKED.
//
// The mode replaces the vision terms and NOT these. A click carries no address: it lands in
// whatever window is in front of the pointer at the instant it is sent, so a boundary that
// stopped asking would be one that sends this routine's press to whatever took the foreground.
func TestGeometryReplayStillAsksWhereTheClickWillLand(t *testing.T) {
	for _, c := range []struct {
		name  string
		spoil func(*worldNow)
		says  string
	}{
		{"the window is gone", func(w *worldNow) {
			w.Alive, w.State = false, "lost"
		}, "no longer there"},
		{"another application has the handle", func(w *worldNow) {
			w.Application = "someoneelse"
		}, "the application changed"},
		{"the window was replaced", func(w *worldNow) {
			w.Generation = 9
		}, "the window was replaced"},
		{"something else leads the desktop", func(w *worldNow) {
			w.Front = observe.FrontOther
		}, "does not lead the desktop"},
	} {
		t.Run(c.name, func(t *testing.T) {
			now, planned := aGeometryReplayWorld()
			c.spoil(&now)
			ok, why := askGeometry(now, planned)
			if ok {
				t.Fatalf("the boundary passed with %s", c.name)
			}
			if !strings.Contains(why, c.says) {
				t.Errorf("the refusal is %q and does not say %q", why, c.says)
			}
		})
	}
}

// AND THE MODE IS NOT ENTERED WITHOUT A WINDOW TO CHECK AGAINST.
//
// `replayWindow` is the one place the boundary is told which mode it is in. A run granted the
// mode whose routine never recorded a window must reach the boundary as "not a geometry replay"
// — because the alternative is a geometry boundary with nothing to compare, which would be four
// terms about where a click lands and no term at all about the layout.
//
// The planner has already refused such a step by then. This is here so that a future caller
// cannot arrive by another road.
func TestTheBoundaryIsNotPutInAModeItHasNoWindowFor(t *testing.T) {
	recorded := workflowstore.Artifact{
		Window: &workflowstore.WindowGeometry{Width: 1920, Height: 1080}}
	var unrecorded workflowstore.Artifact

	if w := replayWindow(true, recorded); w == nil {
		t.Error("a granted run with a recorded window is not in geometry replay mode")
	}
	if w := replayWindow(true, unrecorded); w != nil {
		t.Error("a play with no recorded window put the press boundary into " +
			"geometry replay mode, where it would check four things about " +
			"where a click lands and nothing at all about the layout")
	}
	if w := replayWindow(false, recorded); w != nil {
		t.Error("a run that was never granted the mode is in it")
	}

	// AND IT IS A COPY. A boundary holding a pointer into the artifact would
	// compare against whatever a later reader did to that artifact.
	w := replayWindow(true, recorded)
	recorded.Window.Width = 1
	if w.Width != 1920 {
		t.Error("the boundary's window moved when the artifact's did")
	}
}

// AND NO LOOK IS TAKEN AT ALL IN GEOMETRY REPLAY MODE.
//
// Passing the boundary without vision is one claim; not ASKING for it is another, and only the
// second one costs anything. A look at the instant of a press is a window capture, a
// segmentation and — on the path that needs it — an OCR request waited for inside a budget,
// which is hundreds of milliseconds inside the one moment that must not have an unbounded pause
// in it. In this mode nothing it produces is consulted: no screen, no observation generation, no
// reading of its own words, no proposal under the point. A look taken anyway is pure cost on the
// one path that has none of the evidence it gathers.
//
// A LIVE TEST, and it skips where there is no window to acquire — the same limitation and the
// same shape as the boundary tests beside it, because `stillThere` resolves a real window before
// it decides anything.
func TestGeometryReplayTakesNoLookAtThePressBoundary(t *testing.T) {
	// A STATED DESKTOP, because this claim is about Director and not about the
	// screen: the helper below needs an acquirable application, and borrowing the
	// owner's made this skip whenever their machine had none to spare. See
	// `anActOverAStatedWindow`.
	theStatedWindow(t)
	windows := winprovider.New()
	app := aBackgroundApplication(t, windows)
	tracker := windowref.NewTracker(windows)

	v := tracker.Acquire(context.Background(), app)
	if !v.State.OK() {
		t.Skipf("%q could not be acquired (%s), so the boundary never gets as far "+
			"as deciding whether to look", app, v.State)
	}
	planned := workflowrun.Sighting{Generation: v.Ref.Generation, Window: v.Ref.ID}
	scope := workflowstore.NormaliseScope(v.Ref.Application)

	looks := 0
	counted := func(context.Context, windowref.Validation, observe.PointerAt) freshEvidence {
		looks++
		return freshEvidence{}
	}
	// THE WINDOW AS IT ACTUALLY IS, so the geometry term cannot be what decides this.
	layout := &workflowstore.WindowGeometry{
		Width: v.Ref.Bounds.Width, Height: v.Ref.Bounds.Height}

	stillThere(tracker, windows, app, scope, layout, counted)(
		context.Background(), planned, observe.PointerAt{X: 0.5, Y: 0.5})
	if looks != 0 {
		t.Errorf("the press boundary took %d look(s) in geometry replay mode; "+
			"nothing it produces is consulted there, so every one of them is "+
			"a capture, a segmentation and possibly an OCR wait inside the "+
			"instant of the press", looks)
	}

	// ── AND THE VISION PATH STILL LOOKS ──
	//
	// Without this the test passes for a build that stopped looking everywhere,
	// which is the one change that would quietly remove the boundary's ability to
	// see a screen change underneath an authorisation.
	looks = 0
	stillThere(tracker, windows, app, scope, nil, counted)(
		context.Background(), planned, observe.PointerAt{X: 0.5, Y: 0.5})
	if looks != 1 {
		t.Errorf("the vision boundary took %d look(s); it must take exactly one, "+
			"because the cached one is the plan's own and comparing the plan "+
			"against itself is the defect the fresh look exists to close",
			looks)
	}
}
