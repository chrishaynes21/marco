// Package fastobserve is the fast local visual observer: it notices that the screen changed.
//
// # The division of labour, and why it has to be two things
//
// Marco has two perception paths and until now they shared a clock by accident of wiring.
// [[ADR-175-no-settling-rule-survives-a-four-second-cadence]] measured what that costs: the
// screen model needs about ten looks a second, the AI discovery path samples at ~3.8 seconds,
// and at that rate the model settles on 0 to 3 of nine screens while SAMPLING 5 to 6 of them.
// The screens were seen and the rule discarded them. Click attribution dies even earlier —
// 13/5/3 click/automatic/unknown at 10fps becomes 0/0/12 at 3.8s.
//
// So:
//
//	fast local observer   notices that the screen CHANGED, and what was clicked   <- here
//	slow semantic path    says what the screen IS, names it, plans over it
//
// [[ADR-177-acquisition-is-a-frame-interval-not-a-cost]] made this affordable: a full-resolution
// look costs about 50ms — acquisition 16.7, conversion 2.1, proposer 31.7 — so twenty a second
// is available where ten is needed, with no change to the capture path.
//
// # What this package may not do
//
// It may not act, and it may not remember. A visual screen is a count of rectangles in nine
// boxes; the durable store takes a composition of roles a provider vouched for. Nothing here
// reaches the store, the place establisher, or anything that can touch a desktop — held by the
// boundary tests in this package, which walk its imports rather than trusting it.
//
// It also may not BLOCK. A fast observer that waited for a slow consumer would run at the slow
// consumer's rate, which is the defect this package exists to fix. Publication is a non-blocking
// send to a bounded channel and a dropped update is counted.
package fastobserve

import (
	"fmt"
	"sort"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// SettleWindow is how long a change must persist before it is a screen.
//
// # A duration, because that is what the rule always was
//
// `observe.VisualSettleLooks` is three, and it was chosen against a 10fps recording — three
// looks spanning about 200ms from the first to the third. The number of LOOKS is how that
// duration is counted at a particular rate; it is not the rule. An observer that inherited the
// count without the rate would damp for eleven seconds at the AI's cadence, which is
// ADR-175's finding.
//
// So the observer is given its interval and derives the count, and a replay at 10fps and a live
// observer at 10fps damp identically because they are counting the same 200ms.
const SettleWindow = 200 * time.Millisecond

// MinSettleLooks is the fewest looks that can damp anything.
//
// Two. One look is no damping at all: ADR-175 measured settle=1 on the real recording at 113
// promotions over 900 looks — a promotion every eight looks, with 72 of the causes unknown. A
// transition storm looks like evidence, which makes it worse than seeing nothing.
//
// So an observer whose interval is too coarse to fit two looks in the settle window REFUSES to
// start rather than quietly running undamped. That refusal is the guard against inheriting a
// slow cadence by accident: at 3.8 seconds the arithmetic gives one, and one is not allowed.
const MinSettleLooks = 2

// SettleLooksFor is how many looks the settle window holds at a given interval.
//
// The span from the first look of a run to its last is (n-1) intervals, so n is one more than
// the number of intervals that fit. At 100ms that is 3, which is exactly the rule the offline
// measurements were taken under.
func SettleLooksFor(interval time.Duration) int {
	if interval <= 0 {
		return observe.VisualSettleLooks
	}
	return 1 + int(SettleWindow/interval)
}

// WindowRef is which window a look was of, and where it was.
//
// Provenance travels with every observation, because the alternative is a screen hypothesis
// nobody can place: two looks at two different windows are not two looks at one screen, and a
// transition between them is a navigation nobody made.
type WindowRef struct {
	ID         directorapi.WindowID `json:"window_id"`
	Generation uint64               `json:"generation"`
	Bounds     directorapi.Rect     `json:"bounds"`
}

// Look is one observation offered to the observer.
//
// The pixels are already gone by the time one of these exists: what is left is the geometry the
// proposer found, normalised to the window, and the provenance to place it.
type Look struct {
	At time.Time
	// Window is which window these regions are of.
	Window WindowRef
	// Regions is what the proposer found, normalised to the window.
	Regions []observe.Region
	// Front says the TARGET was the window in front. A capture by window can return the
	// pixels of whatever is covering it, under the target's own name, so a look taken
	// while something else was in front is not evidence about the application — and
	// treating it as one would let another window's layout become a screen.
	Front bool
	// Missing says no pixels were obtained at all: a failed capture, a minimised window.
	// Not a screen, and never a destination.
	Missing bool
	// Presses are pointer presses observed during this look's interval, normalised to
	// the window.
	Presses []observe.PointerAt
}

// Update is what the observer publishes.
//
// Session-local, provenanced, and never durable. See the package note.
type Update struct {
	At     time.Time              `json:"at"`
	Window WindowRef              `json:"window"`
	Screen observe.VisualScreenID `json:"screen"`
	// Settled says this look promoted a new screen.
	Settled bool `json:"settled,omitempty"`
	// Step is the transition this look completed, with its cause, when it completed one.
	// Nil otherwise — and nil is the ordinary case, because most looks change nothing.
	Step *observe.RouteStep `json:"step,omitempty"`
	// Unobserved says the look saw nothing: a failed capture, a covered window, a blank
	// frame. Published rather than swallowed, because a consumer that cannot tell "the
	// screen is still that one" from "nobody could see" will eventually assume the first.
	Unobserved bool `json:"unobserved,omitempty"`
}

// Stats is what the observer did. Counts and latencies; no pixels and no content.
type Stats struct {
	// Looks offered, Published updates that reached the queue, and Dropped those the
	// queue refused.
	//
	// Dropped is the number that says whether the consumer kept up. It is counted rather
	// than waited on: an observer that blocked on a slow consumer would run at the slow
	// consumer's rate, which is the whole defect this package exists to fix.
	Looks, Published, Dropped int
	// Missing is looks with no pixels, Covered those taken while another window was in
	// front, and Interrupted times the window changed under the observer.
	//
	// Three different reasons a look is not evidence, kept apart because they send a
	// reader to three different places: a capture problem, an occlusion problem, and a
	// window that was replaced.
	Missing, Covered, Interrupted int
	// Promotions is screens settled and Transitions edges recorded.
	Promotions, Transitions int
	// Presses is pointer presses offered to the observer, and PressesUnusable those the
	// correlator was never given because the look carrying them had no geometry — no
	// pixels, another window in front, or nothing proposed in them.
	//
	// The second one exists because a press on such a look is DISCARDED: there is nothing
	// for it to have landed on, and inventing a target for it would be the worst kind of
	// evidence. Discarding it silently is the other worst kind — ADR-169 is the record of
	// a demonstration whose press total could not be accounted for, and a press dropped
	// here would be the same hole in a different wall.
	Presses, PressesUnusable int
	// LatencyMS is per-look fold latency, which is what the observer itself costs once
	// the pixels are in hand.
	LatencyMS []float64
}

// At returns the pth percentile of the fold latency.
func (s Stats) At(p int) float64 {
	if len(s.LatencyMS) == 0 {
		return 0
	}
	v := make([]float64, len(s.LatencyMS))
	copy(v, s.LatencyMS)
	sort.Float64s(v)
	i := p * (len(v) - 1) / 100
	if i < 0 {
		i = 0
	}
	return v[i]
}

// Observer folds looks into session-local screen hypotheses and publishes them.
//
// Not safe for concurrent use. It is folded on one goroutine — the observing one — and what
// crosses to other goroutines is the channel.
type Observer struct {
	settle   int
	interval time.Duration

	screens observe.VisualScreens
	clicks  observe.ClickTargets

	out   chan Update
	stats Stats

	// seen is the window generation the last look was of, so a change can interrupt
	// rather than manufacture.
	seen    uint64
	hasSeen bool
	look    int
}

// New builds an observer for a given observation interval and queue depth.
//
// It REFUSES an interval too coarse to damp. See MinSettleLooks: an observer that cannot fit two
// looks into the settle window would promote on every change, and ADR-175 measured what that
// produces. Refusing is the guard against inheriting the AI path's cadence by accident, and it
// is a refusal rather than a clamp because a clamp would silently observe at a rate whose
// results nobody should trust.
func New(interval time.Duration, queue int) (*Observer, error) {
	if interval <= 0 {
		return nil, fmt.Errorf("fastobserve: an interval of %s is not a cadence", interval)
	}
	settle := SettleLooksFor(interval)
	if settle < MinSettleLooks {
		return nil, fmt.Errorf("fastobserve: at %s a look, the %s settle window holds "+
			"%d look(s) and damping needs %d. One look is no damping at all — it "+
			"produced a promotion every eight looks on the measured recording — so "+
			"this refuses rather than observing at a rate whose results nobody "+
			"should trust. Observe faster than %s",
			interval, SettleWindow, settle, MinSettleLooks,
			SettleWindow/time.Duration(MinSettleLooks-1))
	}
	if queue < 1 {
		return nil, fmt.Errorf("fastobserve: a queue of %d cannot hold an update", queue)
	}
	o := &Observer{settle: settle, interval: interval, out: make(chan Update, queue)}
	o.screens.SettleAfter(settle)
	return o, nil
}

// Updates is the bounded channel the observer publishes to.
//
// Read it or lose updates; that is the contract, and the drops are counted. A consumer that
// cannot keep up is a consumer that will see a later screen rather than every screen, which is
// the right failure for a thing whose job is to notice the CURRENT one.
func (o *Observer) Updates() <-chan Update { return o.out }

// SettleLooks is the rule in force, derived from the interval.
func (o *Observer) SettleLooks() int { return o.settle }

// Stats is what the observer did.
func (o *Observer) Stats() Stats { return o.stats }

// Screens is the session-local screens settled so far.
func (o *Observer) Screens() []observe.VisualScreenSeen { return o.screens.Seen() }

// Transitions is every change observed between settled screens.
func (o *Observer) Transitions() []observe.VisualTransition { return o.screens.Transitions() }

// Clicks is the press evidence, with the three causes intact.
func (o *Observer) Clicks() observe.ClickEvidence { return o.clicks.Evidence() }

// Observe folds one look and publishes what it concluded.
//
// Never blocks. The fold is bounded work and the publication is a non-blocking send.
func (o *Observer) Observe(lk Look) Update {
	t0 := time.Now()
	o.look++
	o.stats.Looks++

	// ── A DIFFERENT WINDOW IS A DISCONTINUITY, NOT A NAVIGATION ──
	//
	// The looks either side of a replaced window are of two different things. Dropping
	// the standing claim means the next promotion has nothing to come from, and a
	// transition with no origin is already refused — so no edge can span the change.
	if o.hasSeen && lk.Window.Generation != o.seen {
		o.screens.Interrupt()
		o.stats.Interrupted++
	}
	o.seen, o.hasSeen = lk.Window.Generation, true

	// ── THREE WAYS A LOOK IS NOT EVIDENCE, COUNTED APART ──
	//
	// No pixels, somebody else's window in front, or nothing found in them. The first
	// two are offered to the stabiliser as an unobserved look — which is what it is —
	// because a run of them should eventually give the screen up and a single one should
	// not.
	regions := lk.Regions
	switch {
	case lk.Missing:
		o.stats.Missing++
		regions = nil
	case !lk.Front:
		// A capture by window can return the covering window's pixels under the
		// target's own name. Letting another window's layout settle as this
		// application's screen is the failure this guards, and it is not
		// hypothetical: Experiment-025 captured the Windows taskbar on one look of
		// 107 with provenance clean.
		o.stats.Covered++
		regions = nil
	}

	// THE PRESSES FIRST, against the screen that was standing when they happened. A
	// press folded after this look's arrangement would, on the one look that both
	// carries a press and completes a promotion, be recorded as made on the screen it
	// arrived at — a learning unit whose source is its own destination.
	standing := o.screens.Screen()
	o.stats.Presses += len(lk.Presses)
	if len(regions) > 0 {
		for _, w := range lk.Presses {
			o.clicks.Press(o.look, w, standing, regions)
		}
	} else {
		// A PRESS WITH NOTHING TO HAVE LANDED ON, counted where it is lost. The
		// look had no geometry, so there is no candidate set and no target; the
		// press is real and unusable, which is a different thing from absent.
		o.stats.PressesUnusable += len(lk.Presses)
	}

	id := o.screens.Observe(observe.ArrangeLook(regions), lk.At)
	up := Update{At: lk.At, Window: lk.Window, Screen: id}
	switch id {
	case observe.VisualScreenUnobserved, observe.VisualScreenUnknown:
		up.Unobserved = true
	}
	if t, ok := o.screens.Promoted(); ok {
		o.stats.Transitions++
		// THE STEP THE CORRELATOR JUST RECORDED, taken from it rather than looked up
		// afterwards. Two steps can share a source and a destination and differ only
		// in cause, and the cause is the field that decides whether anything may ever
		// be pressed — so publishing one found by its endpoints would eventually
		// publish the wrong one.
		st := o.clicks.Arrived(o.look, t)
		up.Step = &st
		up.Settled = true
	}
	if n := o.screens.Stability().Promotions; n != o.stats.Promotions {
		o.stats.Promotions = n
		up.Settled = true
	}

	o.stats.LatencyMS = append(o.stats.LatencyMS,
		float64(time.Since(t0).Microseconds())/1000)
	o.publish(up)
	return up
}

// publish sends without waiting, and counts what the queue would not take.
func (o *Observer) publish(up Update) {
	select {
	case o.out <- up:
		o.stats.Published++
	default:
		// THE WHOLE POINT. A slow consumer costs updates and never costs cadence.
		o.stats.Dropped++
	}
}

// Close ends the session, filing the presses still pending.
//
// A demonstration's last press is the one that opened the thing being demonstrated, and a
// session that ended before the screen settled would otherwise lose it.
func (o *Observer) Close() {
	o.clicks.Close()
	close(o.out)
}
