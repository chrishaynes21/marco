package main

import (
	"context"
	"image"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/visualstate"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/selection"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// THE FAST HALF OF SLOW DISCOVERY / FAST TRACKING, where the pixels are.
//
// # Why this is off the observation cycle rather than in it
//
// Because the observation cycle is not fast. [[Experiment-024-watching-a-player-navigate-a-real-menu]]
// measured 94 samples in 244 seconds — 2.6s each, against a configured 500ms interval, with
// 360 late samples — because a ScreenParser inference runs synchronously inside the cycle it
// was offered in. Twelve controller presses arrived inside two of those cycles.
//
// So a tracking read taken once per cycle would be taken once per 2.6 seconds and the fast path
// would not be fast. This runs on its own ticker, captures only the envelope the last discovery
// found, and never calls the detector — which is the whole of what "without blocking on
// ScreenParser" means.
//
// # Why it costs almost nothing, measured
//
// A full-window capture is 24ms (measured live, 1920×1080) and measuring nine rows' shading is
// 597µs. A ScreenParser inference is 1.05–1.25s. The envelope is a fraction of the window, so
// the capture is a fraction of the 24ms — but THAT number is not measured yet and the report
// prints the one this path actually achieves rather than an estimate of it.
//
// # Why it is not running most of the time
//
// It ticks only while a discovery exists, and a discovery only exists where the experimental
// detector is configured and proving its target. An ordinary Director never starts it. That is
// deliberate: `visualstate` refuses a region beyond MaxRegionArea because "a region that large
// is a screen, and watching screens continuously is what this avoids", and the same caution
// applies to watching a small region continuously for no reason.

// trackIntervalEnv tunes how often a fast look is taken; trackDisableEnv switches it off.
const (
	trackIntervalEnv = "MARCO_TRACK_INTERVAL"
	trackDisableEnv  = "MARCO_NO_TRACKING"
)

// DefaultTrackInterval is how often the fast path reads pixels.
//
// 250ms, chosen from both ends of the measurement. Above: a capture costs ~24ms for a whole
// window, so even at this rate the machine is spending under a tenth of its time here, and the
// game gets the rest — which is the condition an experiment a player is not supposed to notice
// has to meet. Below: the selection reader needs a couple of frames to confirm a mark has
// moved, so 250ms puts a confirmed selection change about half a second behind the press, which
// is inside what a person reads as responsive.
//
// It is NOT chosen to match a human's pressing speed. The measured burst was twelve presses in
// 3.35 seconds — about one every 280ms — and a tracker at this interval will still miss some of
// those. Reporting the achieved cadence beside the presses observed is how that stays visible
// instead of being assumed away.
const DefaultTrackInterval = 250 * time.Millisecond

// maxPendingLooks bounds the buffer between the ticker and the sampler.
//
// At 250ms a look and 2.6s a cycle, a cycle collects about ten. Sixty leaves room for a cycle
// that took six times as long as the measured one before anything is dropped, and dropping is
// reported rather than silent — see trackerStats.
const maxPendingLooks = 60

// discovery is the geometry one expensive inference established.
//
// Desktop coordinates, because that is what a capture takes and what selection.Measure reads.
// The conversion to window-relative regions happens on the way OUT, in the look, so nothing
// that leaves this file carries a screen coordinate.
type discovery struct {
	sample   int
	at       time.Time
	window   directorapi.WindowID
	bounds   directorapi.Rect
	envelope directorapi.Rect
	rows     []directorapi.Rect
	// generation is the window lifecycle epoch the pixels belonged to. Geometry from two
	// generations describes two different worlds, and the detector's own frames already get
	// this check — see capture.Moved and the provenance guard.
	generation uint64
}

// fastTracker reads the mark inside a discovered list, on its own clock.
//
// # Why it does not look up the live window
//
// Because it runs on its own goroutine and `Runtime.pinnedWindow` is written by the sampler
// under `Runtime.mu` and read by `activeWindow` without it. Reading it from here would be a
// data race, and taking `Runtime.mu` instead would block for the whole 2.6s cycle — which
// would make the fast path tick at exactly the rate it exists to beat.
//
// So liveness is the SAMPLER's job, which is the layer that already holds the lock and already
// knows the window: it calls `invalidate` when the window moved or was replaced. The cost is
// that a window moving between cycles is not noticed until the next one, bounded by
// `observe.TrackStaleness` on the other side. That gap is real — up to one cycle of reads
// against rectangles that have moved — and it is written down rather than papered over. It is
// the same bargain `capture.Moved` strikes for the detector's own frames, at a coarser grain.
type fastTracker struct {
	visual   *visualstate.Provider
	interval time.Duration

	mu      sync.Mutex
	disc    *discovery
	pending []observe.TrackLook
	dropped int

	stop chan struct{}
	done chan struct{}
}

// trackInterval reads the configured cadence.
func trackInterval() time.Duration {
	if v := strings.TrimSpace(os.Getenv(trackIntervalEnv)); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return DefaultTrackInterval
}

// trackingDisabled reports whether the fast path was switched off by hand.
func trackingDisabled() bool {
	v := strings.TrimSpace(os.Getenv(trackDisableEnv))
	return v != "" && v != "0" && !strings.EqualFold(v, "false")
}

// newFastTracker builds one, or returns nil when there is nothing for it to do.
//
// Nil when there is no visual provider and nil when the experimental detector is not
// configured, because a discovery is the only thing that gives this path geometry and only the
// detector produces one. A nil tracker is a no-op at every call site, which is how an ordinary
// Director never grows a capture loop it has no use for.
func newFastTracker(rt *Runtime) *fastTracker {
	if rt == nil || rt.visual == nil || trackingDisabled() {
		return nil
	}
	if rt.shadowVision == nil {
		return nil
	}
	return &fastTracker{visual: rt.visual, interval: trackInterval()}
}

// discovered hands the tracker the geometry an inference just established.
//
// Called on every sample that produced a proven inference, so the geometry the fast path reads
// is never more than one discovery old. A sample that produced nothing leaves the previous
// discovery in place and the staleness bound in observe.TrackLook decides when that stops being
// usable — the tracker does not get to decide that, because the belief is what the freshness is
// relative to.
func (t *fastTracker) discovered(d *discovery) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.disc = d
	t.mu.Unlock()
}

// Start begins ticking.
func (t *fastTracker) Start() {
	if t == nil || t.stop != nil {
		return
	}
	t.stop, t.done = make(chan struct{}), make(chan struct{})
	go t.run(t.stop, t.done)
}

// Stop ends the ticker and waits for it. Idempotent: a session may be torn down twice.
func (t *fastTracker) Stop() {
	if t == nil {
		return
	}
	t.mu.Lock()
	stop, done := t.stop, t.done
	t.stop, t.done = nil, nil
	t.mu.Unlock()
	if stop == nil {
		return
	}
	close(stop)
	<-done
}

func (t *fastTracker) run(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	tick := time.NewTicker(t.interval)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
			t.look()
		}
	}
}

// look takes one fast read, or records why it could not.
//
// A refusal is a LOOK, not a silence. The belief counts them by reason, because "tracking
// produced no readings" has five causes that send a reader to five different places, and a
// tracker that quietly did nothing would be indistinguishable from one that was never wired.
func (t *fastTracker) look() {
	t.mu.Lock()
	d := t.disc
	t.mu.Unlock()
	if d == nil || len(d.rows) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	look := observe.TrackLook{
		Discovery: d.sample, DiscoveredAt: d.at, At: time.Now(),
		Generation: d.generation,
	}
	// The window THE DISCOVERY was about, as the sampler saw it under the lock. Not
	// re-resolved: re-resolving the geometry here would make this path a discovery, which
	// is exactly what it may not be, and re-resolving the window would be the data race
	// described on the type.
	w := directorapi.Window{ID: d.window, Bounds: d.bounds}

	start := time.Now()
	snap, err := t.visual.Capture(ctx, d.envelope, w)
	look.CaptureMS = time.Since(start).Milliseconds()
	if err != nil {
		// The reason is the provider's own, which includes the MaxRegionArea refusal —
		// "a region that large is a screen". A tracker that silently skipped an
		// oversized envelope would look like one watching a screen that never changes.
		look.Refused = err.Error()
		t.offer(look)
		return
	}
	if snap.Image == nil {
		look.Refused = "the capture returned no pixels"
		t.offer(look)
		return
	}

	// MEASURED WHERE THE PIXELS ARE, by the same function the detector's own provider uses
	// and the reader downstream uses. A second implementation would eventually disagree,
	// and the disagreement would read as a selection that moved when nothing did.
	readStart := time.Now()
	look.Rows = make([]observe.Region, 0, len(d.rows))
	look.Shades = make([][]float64, 0, len(d.rows))
	for _, r := range d.rows {
		look.Rows = append(look.Rows, observe.RelativeTo(r, w.Bounds))
		sh := selection.Measure(snap.Image, inSnapshot(r, d.envelope))
		if sh == (selection.Shade{}) {
			// A row that could not be measured carries NOTHING rather than zeros. A
			// rectangle nobody measured and one measured as black must not look
			// alike — the rule the whole shading path is built on.
			look.Shades = append(look.Shades, nil)
			continue
		}
		look.Shades = append(look.Shades, []float64{sh[0], sh[1]})
	}
	look.ReadUS = time.Since(readStart).Microseconds()
	t.offer(look)
}

// inSnapshot places a desktop rectangle inside the captured envelope's image.
//
// The snapshot's image starts at the envelope's top-left, so the offset is a subtraction. Done
// here rather than trusted: a row measured at the wrong offset returns a confident reading of
// whatever happens to be at those coordinates, which is worse than no reading.
func inSnapshot(row, envelope directorapi.Rect) image.Rectangle {
	return image.Rect(
		row.X-envelope.X, row.Y-envelope.Y,
		row.X-envelope.X+row.Width, row.Y-envelope.Y+row.Height)
}

// offer buffers a look for the next sample, dropping the oldest when the buffer is full.
func (t *fastTracker) offer(look observe.TrackLook) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.pending) >= maxPendingLooks {
		// THE OLDEST goes. A full buffer means the cycle is slower than the ticker, and
		// what a belief needs in that case is the most recent reads: an old one describes
		// a mark that has already moved on.
		t.pending = t.pending[1:]
		t.dropped++
	}
	t.pending = append(t.pending, look)
}

// invalidate drops the current geometry and says, as a look, why.
//
// Called by the SAMPLER, which is the layer that holds the lock and knows the live window.
// A look rather than a silent clear: "tracking stopped reporting" and "tracking reported that
// the window moved" are different findings, and a tracker that quietly stopped would be
// indistinguishable from one that was never wired.
func (t *fastTracker) invalidate(d *discovery, why string) {
	if t == nil || d == nil {
		return
	}
	t.mu.Lock()
	if t.disc != nil {
		t.disc = nil
	}
	t.mu.Unlock()
	t.offer(observe.TrackLook{
		Discovery: d.sample, DiscoveredAt: d.at, At: time.Now(),
		Generation: d.generation, Refused: why,
	})
}

// geometry is the discovery the fast path is currently reading, nil when there is none.
func (t *fastTracker) geometry() *discovery {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.disc
}

// Drain takes the looks accumulated since the last sample.
func (t *fastTracker) Drain() []observe.TrackLook {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	out := t.pending
	t.pending = nil
	return out
}

// Dropped is how many looks the buffer discarded, cumulative.
func (t *fastTracker) Dropped() int {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.dropped
}

// discoveryFrom builds the geometry a fast look will read, from one proven inference.
//
// The candidate geometry is every region and every kept shape that carried a MEASUREMENT.
// Deliberately both: on the game this was built for almost nothing has a name, and the whole
// point of [[ADR-154-geometry-is-evidence-even-when-the-words-are-not]] is that a nameless
// rectangle is still a rectangle. A row with no shading is left out, because the fast path's
// only job is to compare shadings and a row it cannot measure contributes nothing to that.
//
// Returns nil when there is nothing worth tracking, which leaves the PREVIOUS discovery in
// place: a single sparse inference should not throw away usable geometry, and the staleness
// bound is what decides when old geometry stops being usable.
func discoveryFrom(sample int, at time.Time, sh *observe.ShadowSample,
	w directorapi.Window, generation uint64) *discovery {

	if sh == nil || !sh.Ran || !sh.TargetProven || w.Bounds.Width <= 0 || w.Bounds.Height <= 0 {
		return nil
	}
	var rows []directorapi.Rect
	add := func(r observe.Region, shade []float64) {
		if len(shade) != 2 {
			return
		}
		rows = append(rows, absoluteIn(r, w.Bounds))
	}
	for _, r := range sh.Regions {
		add(r.Region, r.Shade)
	}
	for _, s := range sh.Shapes {
		add(s.Region, s.Shade)
	}
	if len(rows) < 2 {
		// One rectangle is not a list, and tracking a mark inside a list of one is not a
		// question. Two is the smallest thing a mark can be handed between.
		return nil
	}
	env := rows[0]
	for _, r := range rows[1:] {
		env = union(env, r)
	}
	return &discovery{
		sample: sample, at: at, window: w.ID, bounds: w.Bounds,
		generation: generation, envelope: env, rows: rows,
	}
}

// absoluteIn is the inverse of observe.RelativeTo.
func absoluteIn(r observe.Region, window directorapi.Rect) directorapi.Rect {
	return directorapi.Rect{
		X:      window.X + int(r.X*float64(window.Width)+0.5),
		Y:      window.Y + int(r.Y*float64(window.Height)+0.5),
		Width:  int(r.Width*float64(window.Width) + 0.5),
		Height: int(r.Height*float64(window.Height) + 0.5),
	}
}

// union is the smallest rectangle containing both.
func union(a, b directorapi.Rect) directorapi.Rect {
	x0, y0 := min(a.X, b.X), min(a.Y, b.Y)
	x1, y1 := max(a.X+a.Width, b.X+b.Width), max(a.Y+a.Height, b.Y+b.Height)
	return directorapi.Rect{X: x0, Y: y0, Width: x1 - x0, Height: y1 - y0}
}
