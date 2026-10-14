package main

// The second source, and the three things that must stay true of it.
//
// A detection is late, it is a region and not a meaning, and it must never be able to slow the
// loop down. The admission rules themselves live in `fastobserve` and are tested there; what is
// tested here is the part this package added — turning live detections into candidates, and
// letting them be matched only where they still describe the screen in front of Marco.

import (
	"image"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/internal/platform/visionclient"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// frame is a blank image of a given size, for converting detections against.
func frame(w, h int) image.Image {
	return image.NewRGBA(image.Rect(0, 0, w, h))
}

// theTrainingTile is the measured case: a detection of Rocket League's Training tile, and the
// reading the demonstration recorded of the same control.
func theTrainingTile() (vision.Detection, observe.Region) {
	return vision.Detection{
			Class:      "image",
			Bounds:     image.Rect(399, 529, 399+355, 529+223),
			Confidence: 0.39,
		}, observe.Region{
			X: 399.0 / 1920, Y: 530.0 / 1080,
			Width: 355.0 / 1920, Height: 221.0 / 1080,
		}
}

// A DETECTION BECOMES A TARGET ONLY BY AGREEING WITH THE FILE.
//
// The detector says where something is and never what it is: the Training tile comes back classed
// `image`, which is true of the pixels and is not a claim that it is a control. What makes it a
// target is a reading the demonstration recorded agreeing with it, through the same matcher a
// pixel proposal goes through.
func TestADetectionBecomesATargetOnlyByAgreeingWithTheFile(t *testing.T) {
	det, saved := theTrainingTile()
	cands := asCandidates([]vision.Detection{det}, learnedFrame{
		img: frame(1920, 1080), look: 7, epoch: 5, generation: 1,
	})
	if len(cands) != 1 {
		t.Fatalf("%d candidate(s) from one detection", len(cands))
	}
	c := cands[0]
	if c.From != directorapi.SourceVision {
		t.Errorf("the candidate is attributed to %q, not the detector", c.From)
	}
	if c.Class != "image" {
		t.Errorf("the detector's own word was not carried: %q", c.Class)
	}
	if c.Epoch != 5 || c.Generation != 1 || c.FromLook != 7 {
		t.Errorf("the candidate does not carry the look it came from: %+v", c)
	}

	// AND IT AGREES WITH THE SAVED READING, which is the whole of its claim to be this
	// control. Measured on the live Play page: IoU 0.991.
	got, ok := observe.Agreed(c.Region, []observe.Region{saved})
	if !ok {
		t.Fatal("the detection and the saved reading do not agree at all")
	}
	midX := int((got.X + got.Width/2) * 1920)
	midY := int((got.Y + got.Height/2) * 1080)
	if midX != 576 || midY != 640 {
		t.Errorf("the agreed centre is (%d,%d), want (576,640)", midX, midY)
	}

	// AND A DETECTION NOWHERE NEAR IT AGREES WITH NOTHING, so a confident box somewhere
	// else cannot become this control.
	elsewhere := asCandidates([]vision.Detection{{
		Class: "button", Bounds: image.Rect(1500, 800, 1700, 850), Confidence: 0.9,
	}}, learnedFrame{img: frame(1920, 1080), epoch: 5, generation: 1})
	if _, ok := observe.Agreed(elsewhere[0].Region, []observe.Region{saved}); ok {
		t.Error("a detection elsewhere on the screen agreed with this control")
	}
}

// A STALE DETECTION IS NOT MATCHED AGAINST.
//
// Inference takes about ten looks, so a detection routinely arrives in an observation of a
// different screen. `admitSlow` refuses those for the observer's own folding; re-acquisition
// needs the same rule, because a target matched against a rectangle from a screen that has gone
// is a point on a screen that has gone.
func TestAStaleDetectionIsNotMatchedAgainst(t *testing.T) {
	det, _ := theTrainingTile()
	cands := asCandidates([]vision.Detection{det}, learnedFrame{
		img: frame(1920, 1080), epoch: 5, generation: 1,
	})

	if got := fresh(cands, 5, 1); len(got) != 1 {
		t.Fatalf("a detection of the standing screen was dropped: %d region(s)",
			len(got))
	}
	if got := fresh(cands, 6, 1); len(got) != 0 {
		t.Errorf("a detection from observation 5 was matched against observation 6")
	}
	if got := fresh(cands, 5, 2); len(got) != 0 {
		t.Errorf("a detection from window generation 1 was matched against "+
			"generation 2; %d region(s) survived", len(got))
	}
}

// THE LEARNED SOURCE NEVER BLOCKS THE LOOP.
//
// 550ms of inference against a 100ms cadence means the common case is a detector that is still
// busy, and the loop must not wait for it. Dropping is the ordinary outcome and is counted: a
// run that offered 99 frames and answered 8 dropped 89, which is the design working.
func TestTheLearnedSourceNeverBlocksTheLoop(t *testing.T) {
	// A source with a worker that never runs: every offer past the first must be
	// dropped rather than waited on.
	e := &learnedEyes{
		work: make(chan learnedFrame, 1),
		done: make(chan []observe.Candidate, 4),
		// A detector that exists and whose worker is not running, which is the
		// shape of a detector that is busy.
		det: busyDetector,
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			e.offer(learnedFrame{img: frame(8, 8), look: i})
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("offering frames to a busy detector blocked the loop")
	}
	if e.asked.Load() != 200 {
		t.Errorf("%d frame(s) offered, want 200", e.asked.Load())
	}
	if e.dropped.Load() != 199 {
		t.Errorf("%d frame(s) dropped, want 199 — one fits in the queue and the "+
			"rest must not wait", e.dropped.Load())
	}
	// AND DRAINING WHAT IS NOT THERE RETURNS NOTHING, rather than waiting for it.
	if got := e.ready(); len(got) != 0 {
		t.Errorf("%d candidate(s) from a detector that has answered nothing",
			len(got))
	}
}

// AND A SOURCE WITH NO DETECTOR IS INERT, rather than pretending.
func TestASourceWithNoDetectorOffersAndReturnsNothing(t *testing.T) {
	e := &learnedEyes{why: "no vision plugin found"}
	if e.working() {
		t.Fatal("a source with no detector claims to be working")
	}
	e.offer(learnedFrame{img: frame(8, 8)})
	if e.asked.Load() != 0 {
		t.Error("a source with no detector counted a frame it could not look at")
	}
	if got := e.ready(); got != nil {
		t.Errorf("a source with no detector produced %d candidate(s)", len(got))
	}
	if got := e.describe(); got == "" {
		t.Error("a source that is not running does not say so")
	}
}

// busyDetector is a detector whose worker is not running — the shape of one that is busy.
//
// Non-nil so `working()` is true, and never actually asked anything: the test offers frames and
// nothing drains them, which is the case the queue depth exists for.
var busyDetector = visionclient.New(inertHost{})

type inertHost struct{}

func (inertHost) Invoke(runtime.HostCall) (string, runtime.Value, error) {
	return "ok", runtime.Value{}, nil
}
