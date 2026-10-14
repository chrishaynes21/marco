package main

// The learned detector, beside the fast loop and never inside it.
//
// # Why a second source at all
//
// Because the first one cannot see this kind of control, and that is now measured rather than
// supposed. On Rocket League's Play page the pixel proposer offers nothing that locates the
// Training tile: the only proposal covering it runs 1644x1032 — 82% of the window, which
// `observe.Region.Locates` refuses because the middle of it is the middle of the screen. The
// learned detector puts a box at x 399..754, y 529..752 against a saved reading of
// x 399..754, y 530..751. An IoU of 0.991.
//
// The proposer is not wrong. It resolves text rows and edges, and on the main menu it found the
// PLAY row as a 146px sliver merged with the background beside it while the detector found the
// whole 309px button. They see different kinds of structure, which is why there are two of them.
//
// # Why it cannot be in the loop
//
// Inference takes about 550ms on a 1920x1080 frame, plus the PNG encode to get it there. The
// observation loop runs at 100ms. So a detection is about ten looks behind the pixels it
// describes, and arrives in an observation that may be of a different screen.
//
// That is exactly the problem [[ADR-184-two-speeds-of-looking]] built `Look.Slow` and `admitSlow`
// for, and nothing here re-solves it: candidates carry the epoch and generation of the frame they
// came from, the observer admits or refuses them against the one standing now, and its stats say
// which — `SlowAdmitted`, `SlowStaleEpoch`, `SlowStaleWindow`, `SlowUnusable`.
//
// This file is the part that did not exist: something that runs the detector against LIVE pixels
// and hands the results to that seam. Offering is non-blocking and the queue holds one frame, so
// a busy detector costs detections and never costs cadence.
//
// # What a detection is not
//
// It is a region, and nothing about what the region means. `Candidate.Class` carries the
// detector's word — the Training tile comes back as `image` — and nothing reads it. A detection
// becomes a target only by agreeing with a reading the demonstration recorded, through the same
// `ReacquireStored` and the same ambiguity rules as a pixel proposal. The detector localises;
// correspondence is still somebody else's to establish.

import (
	"context"
	"image"
	"os"
	"sync/atomic"

	"github.com/chaynes-simpleclouds/marco/internal/bridgehost"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/internal/platform/visionclient"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// learnedFrame is one frame handed to the detector, with the observation it came from.
type learnedFrame struct {
	img        image.Image
	look       int
	epoch      uint64
	generation uint64
}

// learnedEyes is the detector running on its own clock.
type learnedEyes struct {
	det  *visionclient.Detector
	work chan learnedFrame
	done chan []observe.Candidate
	// asked and answered count frames offered and detections returned; dropped counts
	// frames skipped because inference was still running, which is the ordinary case and
	// the number that says how far behind the detector is.
	asked, answered, dropped, found atomic.Int64
	// why is why there is no detector, empty when there is one.
	why string
}

// newLearnedEyes starts the detector, or returns one that explains why it cannot run.
//
// The settings are the frozen ones from the benchmark wiring, not new values: a live path that
// swept its own thresholds would be measuring a different detector from the one the decision
// record approved.
func newLearnedEyes(ctx context.Context) *learnedEyes {
	e := &learnedEyes{
		work: make(chan learnedFrame, 1),
		done: make(chan []observe.Candidate, 4),
	}
	bridge := defaultVisionBridge()
	switch {
	case bridge == "":
		e.why = "no vision plugin found — build plugins/vision/build.cmd, or set " +
			"$DIRECTOR_VISION to vision.exe"
		return e
	}
	if _, err := os.Stat(bridge); err != nil {
		e.why = "the vision plugin is not at " + bridge
		return e
	}
	model := screenParserModel()
	if model == "" {
		e.why = "no detector model found — set $MARCO_SCREENPARSER_MODEL"
		return e
	}
	// EVERYTHING ONE CHILD NEEDS, from the one place that already knows it.
	//
	// Hand-rolling this list is how the first attempt loaded a stale runtime: the
	// settings went across and MARCO_ONNXRUNTIME did not, so the plugin found whichever
	// onnxruntime.dll the environment happened to name and refused with "The requested
	// API version [28] is not available ... Current ORT Version is: 1.26.0" — the exact
	// message defaultONNXRuntime was written to prevent, three functions away.
	e.det = visionclient.New(bridgehost.New(bridge).WithEnv(visionChildEnv(model)...))
	go e.run(ctx)
	return e
}

// working reports whether there is a detector at all.
func (e *learnedEyes) working() bool { return e != nil && e.det != nil }

// offer hands the detector a frame, or drops it if inference is still running.
//
// Non-blocking, and dropping is the ordinary case rather than an error: the loop produces ten
// frames a second and the detector answers about twice. What must never happen is the loop
// waiting, so the queue is one deep and the newest frame wins.
func (e *learnedEyes) offer(f learnedFrame) {
	if !e.working() {
		return
	}
	e.asked.Add(1)
	select {
	case e.work <- f:
	default:
		e.dropped.Add(1)
	}
}

// ready drains whatever the detector has finished, without waiting for it.
func (e *learnedEyes) ready() []observe.Candidate {
	if !e.working() {
		return nil
	}
	var out []observe.Candidate
	for {
		select {
		case c := <-e.done:
			out = append(out, c...)
		default:
			return out
		}
	}
}

// run is the worker. One frame at a time, for as long as the run lasts.
func (e *learnedEyes) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case f := <-e.work:
			got, err := e.det.Detect(ctx, vision.Input{Image: f.img})
			if err != nil {
				continue
			}
			e.answered.Add(1)
			e.found.Add(int64(len(got)))
			cands := asCandidates(got, f)
			select {
			case e.done <- cands:
			default: // nobody drained; the next look will have fresher ones anyway
			}
		}
	}
}

// asCandidates normalises detections to the window and stamps them with the look they came from.
//
// The stamp is the whole point. A detection describes pixels that are by now several
// observations old, and `admitSlow` refuses one whose epoch or generation has moved — so the
// numbers travel with the region rather than being assumed to be current.
func asCandidates(got []vision.Detection, f learnedFrame) []observe.Candidate {
	b := f.img.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	if w <= 0 || h <= 0 {
		return nil
	}
	out := make([]observe.Candidate, 0, len(got))
	for _, d := range got {
		box := d.Bounds
		r := observe.Region{
			X: float64(box.Min.X-b.Min.X) / w, Y: float64(box.Min.Y-b.Min.Y) / h,
			Width: float64(box.Dx()) / w, Height: float64(box.Dy()) / h,
		}
		if r.Width <= 0 || r.Height <= 0 {
			continue
		}
		out = append(out, observe.Candidate{
			Region: r,
			From:   directorapi.SourceVision,
			// CARRIED AND NEVER READ. The Training tile comes back as
			// `image`, which is true of the pixels and is not a claim that it
			// is a control.
			Class:      d.Class,
			Score:      d.Confidence,
			FromLook:   f.look,
			Generation: f.generation,
			Epoch:      f.epoch,
		})
	}
	return out
}

// fresh is the detections that describe the observation standing now.
//
// The observer applies this rule to what it folds; re-acquisition needs it too, because a target
// matched against a stale rectangle is a point on a screen that has gone. Same two numbers, asked
// in the one other place that uses them.
func fresh(cands []observe.Candidate, epoch, generation uint64) []observe.Region {
	var out []observe.Region
	for _, c := range cands {
		if c.Epoch == epoch && c.Generation == generation {
			out = append(out, c.Region)
		}
	}
	return out
}

// describe is what the report says about the detector.
func (e *learnedEyes) describe() string {
	if e == nil || e.why != "" {
		if e != nil && e.why != "" {
			return "not running: " + e.why
		}
		return "not running"
	}
	if !e.working() {
		return "not running"
	}
	return "offered " + itoa(e.asked.Load()) + ", answered " + itoa(e.answered.Load()) +
		", dropped while busy " + itoa(e.dropped.Load()) + ", " +
		itoa(e.found.Load()) + " detection(s)"
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}
