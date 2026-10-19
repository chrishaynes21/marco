package main

import (
	"context"
	"image"
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/ocr"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/visualstate"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// THE FAST VISUAL PATH, THROUGH THE COMPOSITION PRODUCTION USES.
//
// Experiment-024: 94 samples in 244 seconds — 2.6s each against a configured 500ms — because a
// ScreenParser inference runs synchronously inside the cycle it was offered in. Twelve
// controller presses arrived inside two of those cycles.
//
// So these tests are about one thing above all: that the fast reads actually happen, off the
// cycle, and actually arrive on the sample. A fast path nothing invokes is this repository's
// most expensive recurring failure, and it is worse here than usual — a tracker that silently
// never ran would look exactly like a menu nobody navigated.

// regionPainter answers a region capture with a ground and one bright band.
type regionPainter struct{ band directorapi.Rect }

func (p regionPainter) CaptureRegion(_ context.Context,
	r directorapi.Rect) (ocr.CapturedImage, error) {

	img := image.NewRGBA(image.Rect(0, 0, r.Width, r.Height))
	for y := range r.Height {
		for x := range r.Width {
			img.SetRGBA(x, y, color.RGBA{40, 40, 40, 255})
		}
	}
	// The band, in the captured region's own coordinates.
	for y := p.band.Y - r.Y; y < p.band.Y-r.Y+p.band.Height; y++ {
		if y < 0 || y >= r.Height {
			continue
		}
		for x := p.band.X - r.X; x < p.band.X-r.X+p.band.Width; x++ {
			if x < 0 || x >= r.Width {
				continue
			}
			img.SetRGBA(x, y, color.RGBA{215, 215, 215, 255})
		}
	}
	return ocr.CapturedImage{
		Image: img, Bounds: r, Scale: 1, CapturedAt: time.Now(),
		Transform: ocr.NewTransform(directorapi.Point{X: r.X, Y: r.Y}, 1),
	}, nil
}

// trackingRuntime is a Director whose detector finds unreadable rows and whose region capture
// paints one of them bright.
func trackingRuntime(t *testing.T) *Runtime {
	t.Helper()
	// Fast enough that a test does not wait on a human cadence. Read at construction, so
	// it has to be set before the sampler is built.
	t.Setenv(trackIntervalEnv, "5ms")
	rt := visionShadowRuntime(t, unreadableMenuFixture(), nil,
		paintedFixtureCapture{generation: 1})
	rt.visual = visualstate.New(regionPainter{
		band: directorapi.Rect{X: 700, Y: 300 + markedRow*60, Width: 400, Height: 50},
	})
	return rt
}

// Deleting the tracker's construction or Start in newObservationSampler must fail this.
func TestASessionStartsTheFastVisualPath(t *testing.T) {
	rt := trackingRuntime(t)
	live := rt.newObservationSampler(sessionClock).(*liveSampler)
	defer live.detachNavigation()

	if live.track == nil {
		t.Fatal("a session with an experimental detector and a visual provider started " +
			"no fast visual path. The observation cycle measured 2.6s; a selection read " +
			"taken once per cycle is not a fast read of anything")
	}
	if live.track.stop == nil {
		t.Error("the tracker exists and is not ticking. A capture loop nobody started " +
			"reports nothing, and looks exactly like a menu nobody navigated")
	}
}

// And an ordinary Director does not grow a capture loop it has no use for.
//
// A discovery is the only thing that gives this path geometry, and only the experimental
// detector produces one. `visualstate` refuses an oversized region because "watching screens
// continuously is what this avoids"; the same caution applies to watching a small one for
// nothing.
func TestADirectorWithNoDetectorStartsNoCaptureLoop(t *testing.T) {
	rt := &Runtime{engine: stubEngine{}}
	rt.visual = visualstate.New(regionPainter{})
	if tr := newFastTracker(rt); tr != nil {
		t.Error("a Director with no experimental detector started a fast capture loop")
	}
}

// Deleting the tracker's Stop in detachNavigation must fail this.
func TestEndingASessionStopsTheFastVisualPath(t *testing.T) {
	rt := trackingRuntime(t)
	live := rt.newObservationSampler(sessionClock).(*liveSampler)
	tracker := live.track
	if tracker == nil {
		t.Fatal("no tracker to stop")
	}
	live.detachNavigation()

	if live.track != nil {
		t.Error("the sampler still holds a tracker after the session ended")
	}
	if tracker.stop != nil {
		t.Error("the capture loop was not stopped. A ticker that outlived its session " +
			"would go on photographing a region of a desktop with nothing left to " +
			"report it to")
	}
	// Idempotent: a session may be torn down twice, and the second teardown must not
	// panic on a closed channel. That was a real shutdown bug in the pad source.
	live.detachNavigation()
	tracker.Stop()
}

// THE POSITIVE WIRING TEST: fast reads happen off the cycle and arrive on the sample.
//
// Deleting the Drain in liveSampler.Sample must fail this.
func TestTheFastLooksRideOnTheSample(t *testing.T) {
	rt := trackingRuntime(t)
	live := rt.newObservationSampler(sessionClock).(*liveSampler)
	defer live.detachNavigation()

	// One cycle to establish a discovery. It carries no fast looks: nothing had geometry
	// to read before it ran.
	req := sampleRequest()
	first, err := live.Sample(context.Background(), req)
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	if first.Shadow == nil || len(first.Shadow.Shapes) == 0 {
		t.Fatal("the first cycle found no geometry to track, so this would prove nothing")
	}
	if len(first.Shadow.Track) != 0 {
		t.Errorf("%d fast look(s) on the first cycle, before anything was discovered",
			len(first.Shadow.Track))
	}
	if live.track.geometry() == nil {
		t.Fatal("the cycle found rows and handed the tracker no geometry")
	}

	// The window has to stay where the discovery found it, or the sampler's liveness check
	// correctly invalidates the geometry. `Sample` clears the pin on its way out.
	rt.pinnedWindow = &windowRef1

	// Poll rather than sleep a guess: a fixed sleep either flakes on a loaded machine or
	// wastes a second on an idle one.
	var looks []observe.TrackLook
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		req.Sequence++
		s, err := live.Sample(context.Background(), req)
		if err != nil {
			t.Fatalf("Sample: %v", err)
		}
		rt.pinnedWindow = &windowRef1
		if s.Shadow != nil && len(s.Shadow.Track) > 0 {
			looks = s.Shadow.Track
			break
		}
	}
	if len(looks) == 0 {
		t.Fatal("no fast look reached a sample in five seconds at a 5ms interval. The " +
			"tracker exists, the geometry was handed over, and nothing drained it — a " +
			"complete mechanism production never invokes")
	}

	var read, bright int
	for _, l := range looks {
		if l.Refused != "" {
			t.Errorf("a fast look was refused: %s", l.Refused)
			continue
		}
		read++
		if l.Discovery == 0 || l.DiscoveredAt.IsZero() || l.At.IsZero() {
			t.Errorf("a fast look carries no provenance: %+v", l)
		}
		// NOT BEFORE, rather than STRICTLY AFTER.
		//
		// The claim is that a look cannot have been read before the geometry it was
		// read from existed. Equality is not a violation of that and it happens: on
		// Windows the monotonic clock's granularity is coarse enough that two
		// successive `time.Now()` calls return the identical instant, and this
		// failed intermittently in the full suite with `read` and `discovered`
		// printing the same value down to the monotonic offset. Measured, not
		// supposed — the failing line was
		// `read … m=+113.311121101, discovered … m=+113.311121101`.
		if l.At.Before(l.DiscoveredAt) {
			t.Errorf("a fast look claims to have been read before the geometry was "+
				"discovered: read %v, discovered %v", l.At, l.DiscoveredAt)
		}
		for _, sh := range l.Shades {
			if len(sh) == 2 && sh[0] > 150 {
				bright++
			}
		}
	}
	if read == 0 {
		t.Fatal("every fast look was refused")
	}
	if bright == 0 {
		t.Error("no row read as brightly painted. The capture lit one; if its shading " +
			"does not survive the envelope offset then the fast path is reading " +
			"whatever happens to be at the wrong coordinates, confidently")
	}
}

// A window that moved stops the fast path reading rectangles that moved with it.
//
// Checked here rather than in the ticker because the ticker must not read the live window —
// that field is written by the sampler under a lock the ticker cannot afford to take. So the
// check lives where the data already is, and the refusal travels as a look rather than as a
// silence.
//
// Deleting the liveness switch in liveSampler.Sample must fail this.
func TestAMovedWindowStopsTheFastPathReadingItsGeometry(t *testing.T) {
	rt := trackingRuntime(t)
	live := rt.newObservationSampler(sessionClock).(*liveSampler)
	defer live.detachNavigation()

	req := sampleRequest()
	if _, err := live.Sample(context.Background(), req); err != nil {
		t.Fatalf("Sample: %v", err)
	}
	if live.track.geometry() == nil {
		t.Fatal("no geometry was discovered, so there is nothing to invalidate")
	}

	// The window moves. Everything the discovery remembered is now somewhere else.
	moved := windowRef1
	moved.Bounds.X += 400
	req.Window = moved
	req.Sequence++
	if _, err := live.Sample(context.Background(), req); err != nil {
		t.Fatalf("Sample: %v", err)
	}

	// The geometry for the new position replaces it in the same cycle, which is right: the
	// detector re-ran and found the rows where they now are. What must be true is that the
	// rectangles are the NEW ones.
	d := live.track.geometry()
	if d == nil {
		t.Fatal("the moved cycle discovered nothing")
	}
	if d.bounds.X != moved.Bounds.X {
		t.Errorf("the tracker is still reading geometry from before the window moved "+
			"(bounds at x=%d, window at x=%d). Those rectangles describe pixels that "+
			"are not there, and reading them returns a confident answer about whatever "+
			"is now at those coordinates", d.bounds.X, moved.Bounds.X)
	}

	// And the refusal is reported. It arrives on the FOLLOWING sample because the drain
	// collects the interval that just ended and the invalidation happens after it — which
	// is the honest ordering: the refusal is a thing that happened now, not then.
	rt.pinnedWindow = &moved
	req.Sequence++
	after, err := live.Sample(context.Background(), req)
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	var refused string
	for _, l := range after.Shadow.Track {
		if l.Refused != "" {
			refused = l.Refused
		}
	}
	if refused == "" {
		t.Error("the geometry was dropped and no look said why. A tracker that quietly " +
			"stopped is indistinguishable from one that was never wired")
	}
	if refused != "" && !strings.Contains(refused, "moved") {
		t.Errorf("the refusal does not say the window moved: %s", refused)
	}
}

// ── the report ────────────────────────────────────────────────────────────────

// trackedSession is a session whose list was found slowly and whose mark was read quickly.
func trackedSession(t *testing.T) observe.ShadowTotals {
	t.Helper()
	var totals observe.ShadowTotals
	seq := 0
	for _, m := range []int{-1, -1, -1, -1, -1, -1, 0, 0, 0, 1, 1, 1, 2, 2, 2, 3, 3, 3} {
		seq++
		totals.Observe(interfaceMenuSample(seq, 6, m))
	}
	if totals.Interface.Status != observe.InterfaceProvisional {
		t.Fatalf("precondition: no list identified (%s)", totals.Interface.Because)
	}
	disc, at := totals.Interface.Sample, totals.Interface.At
	for i := range 4 {
		s := interfaceSkipped(100 + i)
		for j := range 5 {
			look := observe.TrackLook{
				Discovery: disc, DiscoveredAt: at,
				At:        at.Add(time.Duration(200+j*250) * time.Millisecond),
				CaptureMS: 9,
			}
			for k := range 6 {
				look.Rows = append(look.Rows, observe.Region{
					X: 0.10, Y: 0.40 + float64(k)*0.06, Width: 0.40, Height: 0.05,
				})
				shade := []float64{60, 72}
				if k == 5 {
					shade = []float64{205, 238}
				}
				look.Shades = append(look.Shades, shade)
			}
			s.Shadow.Track = append(s.Shadow.Track, look)
		}
		// And one the sampler refused, so the panel has a refusal to print.
		s.Shadow.Track = append(s.Shadow.Track, observe.TrackLook{
			Discovery: disc, DiscoveredAt: at, At: at.Add(2 * time.Second),
			Refused: "the window moved since the geometry was found", CaptureMS: 2,
		})
		totals.Observe(s)
	}
	return totals
}

// Deleting renderTracking must fail this.
func TestTheReportSaysWhatTheFastPathRead(t *testing.T) {
	var b strings.Builder
	renderShadow(&b, trackedSession(t))
	out := b.String()

	for _, want := range []string{
		"tracking     ",
		"fast look(s)",
		"selection move(s)",
		// The ACHIEVED cadence, not the configured one. The slow path asked for 500ms
		// and got 2.6s and nobody could tell from a report.
		"MEASURED",
		"capture",
		// And the refusal, by name.
		"the window moved since the geometry was found",
		// The authority boundary, in the panel a person actually reads.
		"may never identify a list",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the report does not say %q:\n%s", want, out)
		}
	}
}

// Deleting the tracked line in renderInterface must fail this.
func TestTheReportSaysWhenASelectionWasTracked(t *testing.T) {
	var b strings.Builder
	renderShadow(&b, trackedSession(t))
	out := b.String()

	if !strings.Contains(out, "read by a FAST look") {
		t.Errorf("the report presents a tracked selection as if the discovery had read "+
			"it. A fast observation presented as contemporaneous with an old discovery "+
			"is the defect this whole path had to be built around:\n%s", out)
	}
	if !strings.Contains(out, "not by the discovery") {
		t.Error("the report does not say which look produced the selection")
	}
}

// A walker built when no session is running must not leave a ticker behind.
//
// `walker()` falls back to `newWalkerSampler`, and nothing ever detaches what that returns —
// the registry only stops a SESSION's sampler. A capture loop nobody stops goes on
// photographing a region of a desktop with nothing left to report it to.
//
// Deleting the Stop in newWalkerSampler must fail this.
func TestAWalkerStartsNoCaptureLoopNobodyWillStop(t *testing.T) {
	rt := trackingRuntime(t)

	// The precondition that makes the test mean something: an ordinary SESSION sampler on
	// this runtime does start a ticker, so a walker's not having one is a decision rather
	// than an accident of the configuration.
	session := rt.newObservationSampler(sessionClock).(*liveSampler)
	if session.track == nil || session.track.stop == nil {
		t.Fatal("this runtime starts no tracker for a session either, so the test would " +
			"pass for the wrong reason")
	}
	session.detachNavigation()

	walker := rt.newWalkerSampler().(*liveSampler)
	defer walker.detachNavigation()
	if walker.track != nil {
		t.Error("a walker's sampler is holding a fast visual tracker. Nothing in the " +
			"tree will ever stop it, and it captures a region of the desktop four " +
			"times a second")
	}
}

// A SPARSE INFERENCE DOES NOT THROW AWAY USABLE GEOMETRY.
//
// The measured shape of the live run: during 150 seconds of navigation the detector admitted
// nothing on 56 of 72 looks and a single row on the other 16. If every sparse inference cleared
// the discovery, the fast path would have had geometry for a few seconds in four minutes — and
// the staleness bound, not the detector's good luck, is what should decide when old rectangles
// stop being usable.
func TestASparseInferenceLeavesThePreviousGeometryInPlace(t *testing.T) {
	rt := trackingRuntime(t)
	live := rt.newObservationSampler(sessionClock).(*liveSampler)
	defer live.detachNavigation()

	req := sampleRequest()
	if _, err := live.Sample(context.Background(), req); err != nil {
		t.Fatalf("Sample: %v", err)
	}
	first := live.track.geometry()
	if first == nil {
		t.Fatal("the first cycle discovered nothing")
	}

	// A detector that finds one row, and then one that finds none at all. Neither is a
	// list, and neither is a reason to forget where the list was.
	for _, dets := range [][]vision.Detection{unreadableMenuFixture()[:1], nil} {
		rt2 := visionShadowRuntime(t, dets, nil, paintedFixtureCapture{generation: 1})
		sparse := discoveryFrom(req.Sequence+1, time.Now(),
			mustSample(t, rt2).Shadow, pinnedAsWindow(windowRef1), 1)
		if sparse != nil {
			t.Errorf("%d detection(s) produced a discovery with %d row(s); one "+
				"rectangle is not a list and a mark inside a list of one is not a "+
				"question", len(dets), len(sparse.rows))
		}
	}

	// And the sampler left the original in place rather than clearing it.
	req.Sequence++
	rt.pinnedWindow = &windowRef1
	if _, err := live.Sample(context.Background(), req); err != nil {
		t.Fatalf("Sample: %v", err)
	}
	if d := live.track.geometry(); d == nil {
		t.Error("the geometry was cleared. observe.TrackStaleness is what decides when " +
			"old rectangles stop being usable, not whether one inference happened to " +
			"be sparse")
	}
}

// pinnedAsWindow is the window a pinned reference describes, the way activeWindow builds it.
func pinnedAsWindow(r windowref.Ref) directorapi.Window {
	return directorapi.Window{ID: r.ID, Application: r.Application, Bounds: r.Bounds}
}

// mustSample runs one cycle on a runtime and returns the sample.
func mustSample(t *testing.T, rt *Runtime) observe.Sample {
	t.Helper()
	live := rt.newObservationSampler(sessionClock).(*liveSampler)
	defer live.detachNavigation()
	s, err := live.Sample(context.Background(), sampleRequest())
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	return s
}
