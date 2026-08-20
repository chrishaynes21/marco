package visualstate

import (
	"context"
	"errors"
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/ocr"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// What these tests protect is a bias, not a behaviour. Every classification here feeds
// a decision about whether repeating a non-idempotent action is safe, and the two ways
// of being wrong are not symmetric:
//
//	calling a change "nothing"  → a retry, and the action applied twice
//	calling nothing a "change"  → the user re-issues one command
//
// So the interesting cases are the ones where the answer is uncertain, and the assertion
// is nearly always that the uncertain answer is the cautious one.

func fill(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

// speckle adds n single-pixel dots of a different colour — the shape of antialiasing
// noise, a blinking caret, or a cursor passing through.
func speckle(img *image.RGBA, n int, c color.RGBA) *image.RGBA {
	out := image.NewRGBA(img.Bounds())
	copy(out.Pix, img.Pix)
	b := out.Bounds()
	for i := 0; i < n; i++ {
		out.Set(b.Min.X+(i*7)%b.Dx(), b.Min.Y+(i*13)%b.Dy(), c)
	}
	return out
}

// block paints a rectangle — the shape of a menu opening or a dialog appearing.
func block(img *image.RGBA, r image.Rectangle, c color.RGBA) *image.RGBA {
	out := image.NewRGBA(img.Bounds())
	copy(out.Pix, img.Pix)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			out.Set(x, y, c)
		}
	}
	return out
}

var (
	white = color.RGBA{240, 240, 240, 255}
	black = color.RGBA{20, 20, 20, 255}
	blue  = color.RGBA{40, 90, 200, 255}
)

func fingerprintOf(t *testing.T, g *GridFingerprinter, img image.Image) Fingerprint {
	t.Helper()
	f, err := g.Fingerprint(img)
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	return f
}

// ── 1. change classification ──────────────────────────────────────────────────

func TestAnIdenticalRegionIsIdentical(t *testing.T) {
	g := NewFingerprinter()
	img := fill(200, 120, white)
	a, b := fingerprintOf(t, g, img), fingerprintOf(t, g, img)

	res := g.Compare(a, b)
	if res.Kind != ChangeIdentical {
		t.Errorf("kind = %q, want identical — this is the ONLY state that permits a retry",
			res.Kind)
	}
	if a.Digest != b.Digest {
		t.Error("two fingerprints of one image have different digests")
	}
}

func TestRenderingNoiseIsNotAMeaningfulChange(t *testing.T) {
	// A blinking caret, a moving cursor, a ticking clock. Treating these as meaningful
	// would block every legitimate retry; the whole guard would stop working.
	g := NewFingerprinter()
	base := fill(400, 300, white)
	noisy := speckle(base, 20, black)

	res := g.Compare(fingerprintOf(t, g, base), fingerprintOf(t, g, noisy))
	if res.Kind.Meaningful() {
		t.Errorf("kind = %q (%.1f%% of cells), want noise to be dismissed: %s",
			res.Kind, res.ChangedCells*100, res.Reason)
	}
}

func TestAMeaningfulChangeIsMeaningful(t *testing.T) {
	g := NewFingerprinter()
	base := fill(400, 300, white)
	// A quarter of the region repainted — a menu, a panel, a loaded page.
	changed := block(base, image.Rect(0, 0, 200, 150), blue)

	res := g.Compare(fingerprintOf(t, g, base), fingerprintOf(t, g, changed))
	if res.Kind != ChangeMeaningful {
		t.Errorf("kind = %q (%.1f%% of cells), want meaningful", res.Kind, res.ChangedCells*100)
	}
	if res.Reason == "" {
		t.Error("the classification has no reason")
	}
}

func TestAResizedRegionCountsAsChanged(t *testing.T) {
	// Averages over different areas are not comparable, and a window that resized has
	// visibly changed anyway. The dangerous answer would be to compare them regardless
	// and get a small number.
	g := NewFingerprinter()
	a := fingerprintOf(t, g, fill(200, 120, white))
	b := fingerprintOf(t, g, fill(300, 120, white))

	if res := g.Compare(a, b); res.Kind != ChangeMeaningful {
		t.Errorf("kind = %q, want a resized region treated as changed", res.Kind)
	}
}

func TestAMissingFingerprintIsTreatedAsChanged(t *testing.T) {
	// Unknown must never read as "nothing happened", because that permits a retry.
	g := NewFingerprinter()
	real := fingerprintOf(t, g, fill(100, 100, white))

	if res := g.Compare(Fingerprint{}, real); !res.Kind.Meaningful() {
		t.Errorf("kind = %q, want an absent before-state treated as changed", res.Kind)
	}
	if res := g.Compare(real, Fingerprint{}); !res.Kind.Meaningful() {
		t.Errorf("kind = %q, want an absent after-state treated as changed", res.Kind)
	}
}

func TestFingerprintingIsDeterministic(t *testing.T) {
	g := NewFingerprinter()
	img := block(fill(300, 200, white), image.Rect(30, 40, 180, 160), blue)

	first := fingerprintOf(t, g, img)
	for i := 0; i < 10; i++ {
		got := fingerprintOf(t, g, img)
		if got.Digest != first.Digest {
			t.Fatalf("run %d produced digest %s, first produced %s", i, got.Digest, first.Digest)
		}
		if g.Compare(first, got).Kind != ChangeIdentical {
			t.Fatalf("run %d does not compare identical to the first", i)
		}
	}
}

// ── 2. overlays ───────────────────────────────────────────────────────────────

func TestAContiguousBlockReadsAsAnOverlayAndScatteredNoiseDoesNot(t *testing.T) {
	// The distinction that makes "clicking File opened a menu" verifiable rather than
	// merely "something happened".
	g := NewFingerprinter()
	base := fill(400, 400, white)

	menu := block(base, image.Rect(20, 40, 160, 300), black)
	overlay, why := OverlayAppeared(fingerprintOf(t, g, base), fingerprintOf(t, g, menu),
		DefaultThresholds())
	if !overlay {
		t.Error("a contiguous block did not read as an overlay")
	}
	if why == "" {
		t.Error("the overlay finding has no reason")
	}

	// A full repaint is not an overlay: nothing was drawn ON something.
	repaint := fill(400, 400, blue)
	if got, _ := OverlayAppeared(fingerprintOf(t, g, base), fingerprintOf(t, g, repaint),
		DefaultThresholds()); got {
		t.Error("a full repaint read as an overlay")
	}

	// Scattered change is not an overlay either.
	noisy := speckle(base, 300, black)
	if got, _ := OverlayAppeared(fingerprintOf(t, g, base), fingerprintOf(t, g, noisy),
		DefaultThresholds()); got {
		t.Error("scattered noise read as an overlay")
	}
}

// ── 3. capture, regions, coordinates ──────────────────────────────────────────

// fakeCapture returns scripted images and records what was asked for.
type fakeCapture struct {
	images    []image.Image
	err       error
	requested []directorapi.Rect
	calls     int
}

func (f *fakeCapture) CaptureRegion(ctx context.Context, r directorapi.Rect) (ocr.CapturedImage, error) {
	f.requested = append(f.requested, r)
	if f.err != nil {
		return ocr.CapturedImage{}, f.err
	}
	if err := ctx.Err(); err != nil {
		return ocr.CapturedImage{}, err
	}
	i := f.calls
	f.calls++
	if i >= len(f.images) {
		i = len(f.images) - 1
	}
	if i < 0 {
		return ocr.CapturedImage{}, errors.New("no scripted image")
	}
	return ocr.CapturedImage{
		Image: f.images[i], Bounds: r, CapturedAt: time.Now(),
		Transform: ocr.NewTransform(directorapi.Point{X: r.X, Y: r.Y}, 1),
	}, nil
}

func TestANegativeCoordinateRegionIsCapturedAsAsked(t *testing.T) {
	// A monitor to the left of the primary has negative X. Ordinary, and a capture
	// layer that clamped it would watch the wrong screen entirely.
	cap := &fakeCapture{images: []image.Image{fill(200, 100, white)}}
	p := New(cap)

	region := directorapi.Rect{X: -1920, Y: -200, Width: 200, Height: 100}
	snap, err := p.Capture(context.Background(), region, directorapi.Window{ID: "w1"})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if snap.Region != region {
		t.Errorf("snapshot region = %+v, want %+v", snap.Region, region)
	}
	if len(cap.requested) != 1 || cap.requested[0] != region {
		t.Errorf("requested %+v, want the negative region untouched", cap.requested)
	}
}

func TestAnOversizedRegionIsRefused(t *testing.T) {
	// A "region" the size of the desktop is not a region, and honouring it would turn
	// this into the continuous full-screen analysis the design exists to avoid.
	cap := &fakeCapture{images: []image.Image{fill(10, 10, white)}}
	p := New(cap)
	p.MaxRegionArea = 10_000

	_, err := p.Capture(context.Background(),
		directorapi.Rect{X: 0, Y: 0, Width: 4000, Height: 4000}, directorapi.Window{})
	if err == nil {
		t.Fatal("a desktop-sized region was accepted")
	}
	if cap.calls != 0 {
		t.Error("the screen was captured before the bound was checked")
	}
}

func TestComparingDifferentRegionsIsRefused(t *testing.T) {
	// Two rectangles in different places are not a before and after of anything, and a
	// number computed between them would be a confident answer to no question.
	cap := &fakeCapture{images: []image.Image{fill(100, 100, white)}}
	p := New(cap)
	ctx := context.Background()

	a, _ := p.Capture(ctx, directorapi.Rect{Width: 100, Height: 100}, directorapi.Window{ID: "w1"})
	b, _ := p.Capture(ctx, directorapi.Rect{X: 500, Width: 100, Height: 100}, directorapi.Window{ID: "w1"})

	if _, err := p.Compare(a, b); err == nil {
		t.Error("two different regions were compared as a before and after")
	}
}

func TestCaptureFailureIsAnErrorAndNotAnEmptySuccess(t *testing.T) {
	p := New(&fakeCapture{err: errors.New("the desktop is locked")})
	if _, err := p.Capture(context.Background(),
		directorapi.Rect{Width: 50, Height: 50}, directorapi.Window{}); err == nil {
		t.Fatal("a failed capture returned success")
	}
}

// ── 4. watching a still-changing region ───────────────────────────────────────

func TestAStillChangingRegionIsReportedAsSuchAndBounded(t *testing.T) {
	// The case the structural guard cannot see: a page mid-navigation has an unchanged
	// accessibility tree, so "nothing happened" and "everything is happening" look
	// identical from the structural side.
	base := fill(400, 300, white)
	var frames []image.Image
	for i := 0; i < 12; i++ {
		// Every frame differs MATERIALLY from the last — an animation that never ends.
		// The step has to clear the cell-delta threshold, or the fixture would be
		// testing that slow fades read as noise, which they correctly do.
		c := white
		if i%2 == 1 {
			c = blue
		}
		frames = append(frames, block(base, image.Rect(0, 0, 400, 300), c))
	}
	cap := &fakeCapture{images: frames}
	p := New(cap)
	p.Settle = time.Millisecond
	p.MaxWatchRounds = 3

	ctx := context.Background()
	from, err := p.Capture(ctx, directorapi.Rect{Width: 400, Height: 300}, directorapi.Window{ID: "w1"})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	res, taken, err := p.Watch(ctx, from, directorapi.Window{ID: "w1"})
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	if res.Kind != ChangeStillChanging {
		t.Errorf("kind = %q, want still_changing", res.Kind)
	}
	// Bounded. A spinner would otherwise be watched forever, and the honest answer
	// after a few rounds is already "this is still going".
	if len(taken) != 3 {
		t.Errorf("%d rounds taken, want the bound of 3", len(taken))
	}
}

func TestASettledRegionReportsTheChangeSinceTheACTIONNotSinceTheLastRound(t *testing.T) {
	// A page that finished loading two rounds ago has not changed since the previous
	// round — and has changed enormously since the click. Reporting the former would
	// permit a retry after a navigation, which is exactly the bug this milestone fixes.
	base := fill(400, 300, white)
	loaded := block(base, image.Rect(0, 0, 400, 300), blue)
	cap := &fakeCapture{images: []image.Image{loaded, loaded, loaded}}
	p := New(cap)
	p.Settle = time.Millisecond

	ctx := context.Background()
	from, _ := p.Capture(ctx, directorapi.Rect{Width: 400, Height: 300}, directorapi.Window{ID: "w1"})
	// The first capture returns `loaded`, so re-fingerprint `base` as the before-state.
	fp, _ := p.fp.Fingerprint(base)
	from.Fingerprint = fp

	res, _, err := p.Watch(ctx, from, directorapi.Window{ID: "w1"})
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	if !res.Kind.Meaningful() {
		t.Errorf("kind = %q, want the change measured against the ORIGINAL snapshot", res.Kind)
	}
}

func TestWatchStopsOnCancellation(t *testing.T) {
	cap := &fakeCapture{images: []image.Image{fill(100, 100, white)}}
	p := New(cap)
	p.Settle = 5 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	from, _ := p.Capture(context.Background(),
		directorapi.Rect{Width: 100, Height: 100}, directorapi.Window{})
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	if _, _, err := p.Watch(ctx, from, directorapi.Window{}); err == nil {
		t.Error("a cancelled watch returned success")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("took %s; cancellation did not stop the watch", elapsed)
	}
}

// ── 5. appearance detection says nothing when unsure ──────────────────────────

func TestAFlatContrastingRegionReadsAsHighlighted(t *testing.T) {
	// A blue wash with a white border ring — a selected tab. The border is one grid
	// cell wide (240/24 x 120/24 = 10x5), so the ring the detector samples is the
	// surroundings and the inner grid is the wash. A border that straddled cells would
	// be testing the fixture's arithmetic rather than the detector.
	cap := &fakeCapture{images: []image.Image{
		block(fill(240, 120, white), image.Rect(10, 5, 230, 115), blue),
	}}
	p := New(cap)
	snap, err := p.Capture(context.Background(),
		directorapi.Rect{Width: 240, Height: 120}, directorapi.Window{ID: "w1"})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if score, ok := highlighted(snap); !ok {
		t.Error("a flat contrasting wash did not read as highlighted")
	} else if score <= 0 || score > 0.9 {
		t.Errorf("score = %.2f; an inference from averaged colour must stay well below "+
			"certainty", score)
	}
}

func TestABusyRegionDoesNotReadAsHighlighted(t *testing.T) {
	// The failure mode this guards: a region full of varied content is not highlighted
	// however striking its average, and calling it so would put a false state on a real
	// control.
	cap := &fakeCapture{images: []image.Image{speckle(fill(200, 100, white), 4000, black)}}
	p := New(cap)
	snap, _ := p.Capture(context.Background(),
		directorapi.Rect{Width: 200, Height: 100}, directorapi.Window{ID: "w1"})

	if _, ok := highlighted(snap); ok {
		t.Error("a busy region read as highlighted")
	}
}

func TestUndetectableStatesProduceNoObservationRatherThanAGuess(t *testing.T) {
	// The instruction this package follows most closely. A missing observation costs a
	// piece of state the Director never had; a wrong one puts a confident falsehood
	// into fusion, where it is attached to a real control and believed.
	cap := &fakeCapture{images: []image.Image{fill(120, 40, white)}}
	p := New(cap)
	snap, _ := p.Capture(context.Background(),
		directorapi.Rect{Width: 120, Height: 40}, directorapi.Window{ID: "w1"})

	for _, kind := range []observation.VisualStateKind{
		observation.VisualChecked, observation.VisualPressed, observation.VisualExpanded,
		observation.VisualLoading, observation.VisualProgress,
	} {
		if _, ok := p.detect(kind, snap, Request{}); ok {
			t.Errorf("%s was claimed from a blank region, and it cannot be known", kind)
		}
	}
}
