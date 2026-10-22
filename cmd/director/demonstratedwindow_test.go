package main

// THE WINDOW A DEMONSTRATION WAS MADE IN, RECORDED SO A REPLAY CAN BE REFUSED.
//
// Every reading an artifact carries is normalised to its window, so the arithmetic produces a
// confident-looking point at ANY window size whatever. That is why the size has to be written
// down rather than left to come out in the wash: it is the only thing that can later say whether
// the window a demonstrated point is about to be pressed in is the window it was demonstrated
// in. See [[ADR-248-replaying-a-demonstrated-layout]].
//
// Two claims, and the second is the one that keeps the exception narrow: a demonstration whose
// window was RESIZED partway records nothing at all, because it is not a demonstration of a
// fixed layout and choosing one of the two sizes would be this layer deciding which half of it
// the person meant.

import (
	"image"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// foldInWindows folds one look per window given, through the production fold, and returns what
// the session made of the windows it was looked at in.
func foldInWindows(t *testing.T, windows []directorapi.Rect) visualResult {
	t.Helper()
	obs, err := fastobserve.New(100*time.Millisecond, 8)
	if err != nil {
		t.Fatalf("a 10fps observer: %v", err)
	}
	go func() {
		for range obs.Updates() {
		}
	}()
	t.Cleanup(obs.Close)

	regions := []observe.Region{{X: 0.1, Y: 0.1, Width: 0.16, Height: 0.12}}
	boxes := []image.Rectangle{image.Rect(100, 60, 260, 130)}
	var res visualResult
	for i, w := range windows {
		at := time.Duration(i+1) * 100 * time.Millisecond
		lk := fastobserve.Look{
			At: time.Now().Add(at), Front: true, Regions: regions,
			Window: fastobserve.WindowRef{ID: "w-1", Generation: 1, Bounds: w},
		}
		res.fold(obs, lk, aWindowOfButtons(1000, 600, i+1, boxes, []int{1}),
			at, obs.Screen(), nil, true)
	}
	return res
}

// A DEMONSTRATION IN ONE WINDOW RECORDS THAT WINDOW.
func TestADemonstrationRecordsTheWindowItWasMadeIn(t *testing.T) {
	full := directorapi.Rect{X: 0, Y: 0, Width: 1920, Height: 1080}
	res := foldInWindows(t, []directorapi.Rect{full, full, full, full})

	if res.resized {
		t.Fatal("a demonstration made entirely in one window is recorded as resized")
	}
	if res.window.Width != 1920 || res.window.Height != 1080 {
		t.Fatalf("the window is recorded as %dx%d", res.window.Width, res.window.Height)
	}

	// AND IT REACHES AN ARTIFACT AS A GEOMETRY THAT CAN REFUSE SOMETHING.
	var a workflowstore.Artifact
	if err := a.DemonstratedIn(workflowstore.WindowGeometry{
		Width: res.window.Width, Height: res.window.Height,
		Scale: workflowstore.ScaleUnknown,
	}); err != nil {
		t.Fatal(err)
	}
	if !a.Window.Known() {
		t.Fatal("the recorded window says nothing")
	}
	if same, _ := a.Window.SameAs(workflowstore.WindowGeometry{
		Width: 1280, Height: 720}); same {
		t.Error("a window of another size is called the same layout, so recording " +
			"it refuses nothing")
	}
}

// A WINDOW THAT CHANGED SIZE PARTWAY RECORDS NOTHING.
//
// Not the first size and not the last. Somebody who resized the window mid-demonstration did not
// demonstrate a fixed layout, and the honest consequence is that the fixed-layout route refuses
// this routine outright — which is what `StepLayoutUnrecorded` says, in those words.
func TestADemonstrationAcrossTwoWindowSizesRecordsNeither(t *testing.T) {
	res := foldInWindows(t, []directorapi.Rect{
		{Width: 1920, Height: 1080},
		{Width: 1920, Height: 1080},
		{Width: 1280, Height: 720},
		{Width: 1280, Height: 720},
	})
	if !res.resized {
		t.Fatal("a demonstration that spanned two window sizes is recorded as " +
			"having been made in one")
	}
}

// AND A LOOK WITH NO WINDOW IS NOT A RESIZE.
//
// A failed capture, or a look taken before the window was acquired, carries no rectangle. Taking
// that as a change would report every demonstration with one missed frame as having been resized
// — and the consequence of "resized" is that nothing is recorded, so a false one is a routine
// that can never be replayed and no sentence saying why.
func TestALookWithNoWindowIsNotAResize(t *testing.T) {
	full := directorapi.Rect{X: 0, Y: 0, Width: 1920, Height: 1080}
	res := foldInWindows(t, []directorapi.Rect{full, {}, full})

	if res.resized {
		t.Error("a look that carried no window was counted as a window of " +
			"another size")
	}
	if res.window.Width != 1920 {
		t.Errorf("the window is recorded as %dx%d after a look with none",
			res.window.Width, res.window.Height)
	}
}

// AND THE RECORDER WRITES IT DOWN, UNDER NO PICTURE LICENCE.
//
// A SOURCE GUARD, and it says so: the recording command needs a desktop. Two things are pinned
// and both were decisions.
//
// It is attached OUTSIDE `if *previews`, because a window size is not a picture of anybody's
// screen and a demonstration recorded with no previews still has to be replayable. And the
// resized case is a branch that records nothing rather than a value that happens to be the last
// one seen.
func TestTheRecorderWritesDownTheWindowItWasDemonstratedIn(t *testing.T) {
	cmd, err := os.ReadFile("demonstratevisual.go")
	if err != nil {
		t.Fatalf("reading the recorder: %v", err)
	}
	src := string(cmd)

	if !strings.Contains(src, "a.DemonstratedIn(workflowstore.WindowGeometry{") {
		t.Fatal("the recorder never writes down the window a routine was " +
			"demonstrated in, so no routine it saves can ever be replayed " +
			"on a fixed layout and nothing would say why")
	}
	if !strings.Contains(src, "case res.resized:") {
		t.Error("a demonstration whose window changed size is not a branch of " +
			"its own, so one of the two sizes is being recorded as the " +
			"layout it was demonstrated in")
	}
	// AND THE BRANCH THAT WRITES IT IS REACHABLE FROM WHAT THE SESSION SAW.
	//
	// Pinned separately from the call, because the call and the condition fail
	// differently and only one of them is visible in a diff that looks harmless: a
	// mutation that left `a.DemonstratedIn(...)` exactly where it is and changed the
	// branch above it to one nothing satisfies passed every other test here. The
	// routine saves, the field is nil, and the only symptom is a replay refused for
	// a reason that is not the real one.
	if !strings.Contains(src,
		"case res.window.Width > 0 && res.window.Height > 0:") {
		t.Error("the window the session actually saw does not reach the write, " +
			"so every routine is saved with no window recorded and every " +
			"fixed-layout replay is refused as though the recording " +
			"predated the field")
	}

	// ── AND NOT INSIDE THE PICTURES LICENCE ──
	//
	// Checked by position, because this is the mistake that would look right: the
	// window is kept beside the previews and the evidence, and putting the write
	// beside them too would mean a recording made without `--previews` saved a
	// routine that could never be replayed.
	// Anchored on the call INSIDE the licence rather than on `if *previews {`,
	// because the command has more than one of those and the first is a help line.
	licence := strings.Index(src, "keepPreviews(store, a, draft, res, obs.Clicks())")
	writes := strings.Index(src, "a.DemonstratedIn(")
	if licence < 0 || writes < 0 {
		t.Fatal("the recorder no longer has both of these")
	}
	if writes > licence {
		t.Error("the window a routine was demonstrated in is recorded inside the " +
			"--previews licence; it is a size and not a picture, and a " +
			"recording made without pictures still has to be replayable")
	}
}
