package ocr

import (
	"context"
	"errors"
	"image"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The OCR provider's job is to be UNAMBITIOUS, and most of these tests check that it
// stayed that way. It converts engine results into text evidence, places them
// correctly, and refuses what it cannot place. Everything it does not do — decide what
// text means, create an element, assign a role — is not tested here because it is not
// expressible in what this package returns.

func window(id string, r directorapi.Rect) directorapi.Window {
	return directorapi.Window{ID: directorapi.WindowID(id), Application: "testapp",
		Title: "Test", Bounds: r}
}

func rect(x, y, w, h int) directorapi.Rect {
	return directorapi.Rect{X: x, Y: y, Width: w, Height: h}
}

// harness wires a provider over fakes, with a window at the given bounds.
func harness(t *testing.T, w directorapi.Window, results []Result) (*Provider, *FakeEngine, *FakeCapture) {
	t.Helper()
	eng := &FakeEngine{Results: results}
	cap := &FakeCapture{Img: CapturedImage{
		Image:     image.NewRGBA(image.Rect(0, 0, w.Bounds.Width, w.Bounds.Height)),
		Transform: NewTransform(directorapi.Point{X: w.Bounds.X, Y: w.Bounds.Y}, 1),
	}}
	p := New(eng, cap, func(context.Context) (directorapi.Window, bool) { return w, true })
	return p, eng, cap
}

func read(t *testing.T, p *Provider, req observation.Request) ([]observation.Observation, Diagnostics, error) {
	t.Helper()
	return p.Read(context.Background(), req)
}

// ── 1. results become text evidence ───────────────────────────────────────────

func TestAnEngineResultBecomesTextEvidenceAndNothingElse(t *testing.T) {
	w := window("w1", rect(100, 200, 400, 300))
	p, _, _ := harness(t, w, []Result{{
		Text: "Export", Bounds: image.Rect(25, 10, 90, 30), Confidence: 0.92,
		LineID: "l3", WordIndex: 2,
	}})

	obs, diag, err := read(t, p, observation.WithOCR(nil))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(obs) != 1 {
		t.Fatalf("%d observations, want 1", len(obs))
	}
	if diag.Counters.Accepted != 1 {
		t.Errorf("accepted = %d", diag.Counters.Accepted)
	}

	// The KIND is the safety property. A provider that could emit element evidence
	// could invent a control; this one has no way to express that.
	if obs[0].Kind() != observation.TextObservation {
		t.Fatalf("kind = %q, want text — OCR may never emit element evidence", obs[0].Kind())
	}
	tx, ok := obs[0].(observation.Text)
	if !ok {
		t.Fatalf("observation is %T, want observation.Text", obs[0])
	}

	if tx.Content.Raw != "Export" {
		t.Errorf("raw = %q, want the engine's own string untouched", tx.Content.Raw)
	}
	if tx.Content.Comparable != "export" {
		t.Errorf("comparable = %q", tx.Content.Comparable)
	}
	// Image-local (25,10)-(90,30) inside a window at (100,200) is desktop (125,210).
	if got := tx.Box; got != rect(125, 210, 65, 20) {
		t.Errorf("bounds = %+v, want the window origin added", got)
	}
	if tx.WindowID != w.ID || tx.ApplicationID != w.Application {
		t.Errorf("scope = %s/%s, want %s/%s", tx.WindowID, tx.ApplicationID, w.ID, w.Application)
	}
	if tx.LineID != "l3" || tx.WordIndex != 2 {
		t.Errorf("engine granularity was lost: line=%q index=%d", tx.LineID, tx.WordIndex)
	}
	if tx.Score != 0.92 {
		t.Errorf("score = %.2f", tx.Score)
	}
}

func TestRawTextIsNeverRewrittenAndComparableIsNormalised(t *testing.T) {
	w := window("w1", rect(0, 0, 200, 100))
	p, _, _ := harness(t, w, []Result{
		{Text: "  Save   As… ", Bounds: image.Rect(0, 0, 80, 20), Confidence: 0.9},
	})
	obs, _, _ := read(t, p, observation.WithOCR(nil))
	tx := obs[0].(observation.Text)

	// The raw form is what a person sees and what an explanation must quote. Rewriting
	// it would mean showing the user a string their screen does not contain.
	if tx.Content.Raw != "  Save   As… " {
		t.Errorf("raw = %q, want it untouched", tx.Content.Raw)
	}
	if tx.Content.Comparable != "save as" {
		t.Errorf("comparable = %q, want whitespace collapsed and the ellipsis dropped",
			tx.Content.Comparable)
	}
	if !tx.Content.Differs() {
		t.Error("Differs() should report that normalisation changed something")
	}
}

// ── 2. filtering ──────────────────────────────────────────────────────────────

func TestUnusableResultsAreCountedRatherThanSilentlyDropped(t *testing.T) {
	w := window("w1", rect(0, 0, 400, 300))
	// image.Rect takes CORNERS, not x/y/width/height — and normalises, so a
	// carelessly-written "sliver" silently becomes a tall box. Written as corners
	// throughout.
	p, _, _ := harness(t, w, []Result{
		{Text: "Good", Bounds: image.Rect(0, 0, 60, 20), Confidence: 0.9},
		{Text: "   ", Bounds: image.Rect(0, 30, 60, 50), Confidence: 0.9},
		{Text: "Unsure", Bounds: image.Rect(0, 60, 60, 80), Confidence: 0.05},
		{Text: "Sliver", Bounds: image.Rect(0, 90, 60, 91), Confidence: 0.9},
		{Text: "Elsewhere", Bounds: image.Rect(9000, 9000, 9060, 9020), Confidence: 0.9},
	})

	obs, diag, err := read(t, p, observation.WithOCR(nil))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(obs) != 1 {
		t.Fatalf("%d accepted, want 1", len(obs))
	}
	c := diag.Counters
	if c.Accepted != 1 {
		t.Errorf("accepted = %d", c.Accepted)
	}
	if c.RejectedEmpty != 1 {
		t.Errorf("rejected_empty = %d", c.RejectedEmpty)
	}
	if c.RejectedConfidence != 1 {
		t.Errorf("rejected_confidence = %d", c.RejectedConfidence)
	}
	if c.RejectedGeometry != 2 {
		t.Errorf("rejected_geometry = %d, want the sliver and the out-of-image box", c.RejectedGeometry)
	}
	// Every result is accounted for. A discarded observation nobody counted is
	// indistinguishable from an engine that never produced it.
	if c.Total() != 5 {
		t.Errorf("the counters sum to %d, but 5 results went in", c.Total())
	}
}

func TestANonFiniteConfidenceIsRejected(t *testing.T) {
	w := window("w1", rect(0, 0, 200, 100))
	inf := 1.0
	for i := 0; i < 400; i++ {
		inf *= 10 // build an Inf without importing math into the test's expectations
	}
	p, _, _ := harness(t, w, []Result{
		{Text: "Weird", Bounds: image.Rect(0, 0, 50, 20), Confidence: inf},
	})
	obs, diag, _ := read(t, p, observation.WithOCR(nil))
	if len(obs) != 0 {
		t.Errorf("%d accepted; a non-finite confidence cannot be compared to a threshold", len(obs))
	}
	if diag.Counters.RejectedConfidence != 1 {
		t.Errorf("rejected_confidence = %d", diag.Counters.RejectedConfidence)
	}
}

// ── 3. coordinates ────────────────────────────────────────────────────────────

func TestNegativeMonitorCoordinatesSurviveTheTransform(t *testing.T) {
	// A monitor to the LEFT of the primary has negative X. Ordinary, not exceptional —
	// and a transform that clamped it would place every word on the wrong screen.
	w := window("w1", rect(-1920, -200, 800, 600))
	p, _, _ := harness(t, w, []Result{
		{Text: "Left", Bounds: image.Rect(10, 20, 70, 40), Confidence: 0.9},
	})
	obs, _, _ := read(t, p, observation.WithOCR(nil))
	tx := obs[0].(observation.Text)

	if got := tx.Box; got != rect(-1910, -180, 60, 20) {
		t.Errorf("bounds = %+v, want negative desktop coordinates preserved", got)
	}
}

func TestAScaledDisplayTransformsBothAxes(t *testing.T) {
	transform := NewTransform(directorapi.Point{X: 100, Y: 50}, 1.5)
	got := transform.Apply(image.Rect(10, 20, 30, 40))
	// 10*1.5+100 = 115, 20*1.5+50 = 80, 30*1.5+100 = 145, 40*1.5+50 = 110.
	if want := rect(115, 80, 30, 30); got != want {
		t.Errorf("Apply = %+v, want %+v", got, want)
	}

	// And back, so a region expressed in desktop coordinates can be cropped out of an
	// image. Round-tripping is what the region path depends on.
	back := transform.Invert(got)
	if back != image.Rect(10, 20, 30, 40) {
		t.Errorf("Invert = %v, want the original image rect", back)
	}
}

func TestTheIdentityTransformChangesNothing(t *testing.T) {
	r := image.Rect(5, 6, 7, 8)
	got := IdentityTransform().Apply(r)
	if got != (directorapi.Rect{X: 5, Y: 6, Width: 2, Height: 2}) {
		t.Errorf("Apply = %+v", got)
	}
}

func TestARegionCropShiftsCoordinatesBackToTheFullImage(t *testing.T) {
	// The engine sees a crop and reports coordinates within it. Failing to shift them
	// back would place every word near the window's origin — plausible, and wrong.
	w := window("w1", rect(1000, 500, 400, 300))
	p, eng, _ := harness(t, w, []Result{
		{Text: "Inside", Bounds: image.Rect(5, 5, 55, 25), Confidence: 0.9},
	})

	region := rect(1100, 600, 200, 100) // desktop coords → image-local (100,100)-(300,200)
	obs, diag, err := read(t, p, observation.WithOCR(&region))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if eng.Calls != 1 {
		t.Fatalf("engine called %d times", eng.Calls)
	}
	if len(obs) != 1 {
		t.Fatalf("%d observations", len(obs))
	}
	tx := obs[0].(observation.Text)
	// crop origin (100,100) + engine (5,5) + window origin (1000,500) = (1105,605).
	if got := tx.Box; got != rect(1105, 605, 50, 20) {
		t.Errorf("bounds = %+v, want the crop offset applied", got)
	}
	if diag.Region == nil {
		t.Error("the diagnostics do not record that a region was requested")
	}
}

func TestARegionOutsideTheWindowIsRefused(t *testing.T) {
	w := window("w1", rect(0, 0, 200, 200))
	p, _, _ := harness(t, w, []Result{{Text: "x", Bounds: image.Rect(0, 0, 20, 20), Confidence: 1}})

	region := rect(5000, 5000, 100, 100)
	_, diag, err := read(t, p, observation.WithOCR(&region))
	if err == nil {
		t.Fatal("a region that does not overlap the window was accepted")
	}
	if diag.Error == "" {
		t.Error("the diagnostics do not say why")
	}
}

// ── 4. staleness ──────────────────────────────────────────────────────────────

func TestAWindowThatMovedDuringCaptureIsRefusedOutright(t *testing.T) {
	// The failure this check exists to prevent: an image whose pixels came from one
	// place and whose transform describes another. Every word would be placed
	// confidently on whatever is now at those coordinates — evidence that looks right
	// and is wrong, which is worse than no evidence.
	w := window("w1", rect(100, 100, 400, 300))
	p, eng, cap := harness(t, w, []Result{
		{Text: "Moved", Bounds: image.Rect(0, 0, 50, 20), Confidence: 0.9},
	})
	moved := rect(700, 100, 400, 300)
	cap.MoveWindowTo = &moved

	obs, diag, err := read(t, p, observation.WithOCR(nil))
	if err == nil {
		t.Fatal("a stale capture was accepted")
	}
	if len(obs) != 0 {
		t.Errorf("%d observations from a stale capture", len(obs))
	}
	if diag.Counters.RejectedStaleCapture != 1 {
		t.Errorf("rejected_stale_capture = %d", diag.Counters.RejectedStaleCapture)
	}
	if eng.Calls != 0 {
		t.Error("the engine was run on a capture that was already known to be misplaced")
	}
}

func TestACaptureOlderThanTheStalenessBoundIsRefused(t *testing.T) {
	w := window("w1", rect(0, 0, 200, 200))
	p, _, cap := harness(t, w, []Result{{Text: "Old", Bounds: image.Rect(0, 0, 40, 20), Confidence: 1}})
	cap.Img.CapturedAt = time.Now().Add(-time.Hour)

	_, diag, err := read(t, p, observation.WithOCR(nil))
	if err == nil {
		t.Fatal("an hour-old capture was accepted")
	}
	if diag.Counters.RejectedStaleCapture != 1 {
		t.Errorf("rejected_stale_capture = %d", diag.Counters.RejectedStaleCapture)
	}
}

// ── 5. failure isolation ──────────────────────────────────────────────────────

func TestAnUnavailableEngineIsReportedAsUnavailableNotAsEmpty(t *testing.T) {
	// The distinction this whole layer protects. An empty success would make every
	// application look textless and send nobody to install tesseract.
	w := window("w1", rect(0, 0, 200, 200))
	cap := &FakeCapture{}
	p := New(UnavailableEngine{Reason: "tesseract is not installed"}, cap,
		func(context.Context) (directorapi.Window, bool) { return w, true })

	obs, diag, err := read(t, p, observation.WithOCR(nil))
	if err == nil {
		t.Fatal("an unavailable engine returned success")
	}
	if !IsUnavailable(err) {
		t.Errorf("error %v is not recognisable as unavailability", err)
	}
	if len(obs) != 0 {
		t.Errorf("%d observations from an unavailable engine", len(obs))
	}
	if diag.Available {
		t.Error("the diagnostics claim OCR is available")
	}
	if diag.Unavailable == "" {
		t.Error("the diagnostics do not say why it is unavailable")
	}
}

func TestACaptureFailureIsReportedAndDoesNotRunTheEngine(t *testing.T) {
	w := window("w1", rect(0, 0, 200, 200))
	eng := &FakeEngine{}
	cap := &FakeCapture{Err: errors.New("the desktop is locked")}
	p := New(eng, cap, func(context.Context) (directorapi.Window, bool) { return w, true })

	_, diag, err := read(t, p, observation.WithOCR(nil))
	if err == nil {
		t.Fatal("a capture failure returned success")
	}
	if eng.Calls != 0 {
		t.Error("the engine was asked to read an image that does not exist")
	}
	if diag.Error == "" {
		t.Error("the diagnostics do not record the failure")
	}
}

func TestATimeoutBoundsTheEngineRatherThanHangingTheDirector(t *testing.T) {
	w := window("w1", rect(0, 0, 200, 200))
	eng := &FakeEngine{Delay: 5 * time.Second}
	cap := &FakeCapture{}
	p := New(eng, cap, func(context.Context) (directorapi.Window, bool) { return w, true })
	p.Thresholds.Timeout = 50 * time.Millisecond

	start := time.Now()
	_, _, err := read(t, p, observation.WithOCR(nil))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("a hanging engine returned success")
	}
	if elapsed > 2*time.Second {
		t.Errorf("took %s; the timeout did not bound the engine", elapsed)
	}
}

func TestCancellationPropagatesToTheEngine(t *testing.T) {
	w := window("w1", rect(0, 0, 200, 200))
	eng := &FakeEngine{Delay: 5 * time.Second}
	p := New(eng, &FakeCapture{}, func(context.Context) (directorapi.Window, bool) { return w, true })

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, _, err := p.Read(ctx, observation.WithOCR(nil))
	if err == nil {
		t.Fatal("a cancelled read returned success")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("took %s; cancellation did not reach the engine", elapsed)
	}
}

// ── 6. trigger policy ─────────────────────────────────────────────────────────

func TestOCRDoesNothingUnlessItIsAskedForByName(t *testing.T) {
	// The trigger policy in one test. An ordinary observation cycle asks for
	// "everything cheap", and a screen capture plus a recognition pass is not that.
	// Reading an empty Sources list as consent would put OCR on every click.
	w := window("w1", rect(0, 0, 200, 200))
	p, eng, cap := harness(t, w, []Result{{Text: "x", Bounds: image.Rect(0, 0, 40, 20), Confidence: 1}})

	obs, err := p.Observe(context.Background(), observation.Request{})
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if len(obs) != 0 {
		t.Errorf("%d observations from an unasked cycle", len(obs))
	}
	if cap.Calls != 0 || eng.Calls != 0 {
		t.Errorf("the screen was captured (%d) and read (%d) without being asked",
			cap.Calls, eng.Calls)
	}

	// And it does run when asked.
	obs, err = p.Observe(context.Background(), observation.WithOCR(nil))
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if len(obs) != 1 {
		t.Errorf("%d observations when OCR was requested by name", len(obs))
	}
}

// ── 7. caching ────────────────────────────────────────────────────────────────

func TestAnUnchangedWindowIsNotCapturedTwiceInQuickSuccession(t *testing.T) {
	w := window("w1", rect(0, 0, 200, 200))
	p, eng, cap := harness(t, w, []Result{{Text: "x", Bounds: image.Rect(0, 0, 40, 20), Confidence: 1}})

	if _, _, err := read(t, p, observation.WithOCR(nil)); err != nil {
		t.Fatalf("Read: %v", err)
	}
	obs, diag, err := read(t, p, observation.WithOCR(nil))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if cap.Calls != 1 || eng.Calls != 1 {
		t.Errorf("captured %d times and read %d times; the second should have been cached",
			cap.Calls, eng.Calls)
	}
	if !diag.FromCache {
		t.Error("the diagnostics do not say the result was cached")
	}
	if len(obs) != 1 {
		t.Errorf("%d observations from the cache", len(obs))
	}
}

func TestAMovedWindowIsNotServedFromCache(t *testing.T) {
	// The cache key includes bounds, because the same window at a different position
	// is showing different content. Serving the old text would place old words on a
	// new screen — the stale-capture failure, arrived at more slowly.
	moved := window("w1", rect(500, 500, 200, 200))
	first := window("w1", rect(0, 0, 200, 200))

	eng := &FakeEngine{Results: []Result{{Text: "x", Bounds: image.Rect(0, 0, 40, 20), Confidence: 1}}}
	cap := &FakeCapture{}
	current := first
	p := New(eng, cap, func(context.Context) (directorapi.Window, bool) { return current, true })

	if _, _, err := read(t, p, observation.WithOCR(nil)); err != nil {
		t.Fatalf("Read: %v", err)
	}
	current = moved
	if _, diag, err := read(t, p, observation.WithOCR(nil)); err != nil {
		t.Fatalf("Read: %v", err)
	} else if diag.FromCache {
		t.Error("a moved window was served from cache")
	}
	if eng.Calls != 2 {
		t.Errorf("the engine ran %d times, want 2", eng.Calls)
	}
}
