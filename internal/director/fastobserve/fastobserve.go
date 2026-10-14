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
	// Text is what was read on THIS look's pixels, by a source that could answer in
	// time to be folded with them.
	//
	// Look-local, and that is a hard requirement rather than a convenience: see the
	// note beside the fold below on why a channel that arrives late and
	// intermittently must not reach screen identity. A source that cannot answer
	// within a look leaves this zero, and the fold is then the pixel-only one it
	// always was -- nobody read it, which is not a disagreement.
	Text observe.ScreenText
	// Readings is what a SLOW text source has finished reading by this look — each
	// describing an EARLIER one, exactly as Slow does and judged by the same rule.
	//
	// `Text` above is a reading of THIS look and is look-local by construction. No live
	// source can satisfy it: OCR costs hundreds of milliseconds against a cadence of a
	// hundred. These are the readings a live source can actually produce, and what
	// makes them safe is not speed but provenance — see intakeReadings.
	Readings []observe.ScreenReading
	// Slow is detections from a slower source that have ARRIVED by this look — each
	// describing an EARLIER one.
	//
	// The pixels are not here and must not be: a frame costs 600ms to run through the
	// detector and a look is 60ms, so the source that produced these owns the capture,
	// runs off this thread, and hands back rectangles stamped with the context they
	// were computed in. This observer's whole job with them is to decide whether that
	// context is still the one it is in — see admitSlow.
	Slow []observe.Candidate
}

// Update is what the observer publishes.
//
// Session-local, provenanced, and never durable. See the package note.
type Update struct {
	At     time.Time              `json:"at"`
	Window WindowRef              `json:"window"`
	Screen observe.VisualScreenID `json:"screen"`
	// Look is which look this was, and Epoch the observation generation it was folded
	// under.
	//
	// Both are published because a caller that wants richer perception has to stamp its
	// request with the context it was made in, and it cannot be trusted to work that
	// out for itself: an epoch inferred from a screen NAME would be wrong in exactly
	// the case this exists for, because a screen name recurs and an epoch does not.
	Look  int    `json:"look"`
	Epoch uint64 `json:"epoch"`
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
	// ReadingsOffered is how many delayed READINGS were handed to the observer, and the
	// rest of this group what became of them. The same shape as the Slow counters
	// below, because it is the same rule being applied to a different kind of result.
	//
	// ReadingsRefusedMerge is the number that says whether any of this did anything: a
	// run that admitted readings and refused nothing is a run where OCR cost hundreds
	// of milliseconds a frame and changed no decision.
	ReadingsOffered      int
	ReadingsAdmitted     int
	ReadingsUnusable     int
	ReadingsStaleWindow  int
	ReadingsStaleEpoch   int
	ReadingsSilent       int
	ReadingsRefusedMerge int
	// SlowOffered is slow detections handed to the observer and SlowAdmitted those it
	// accepted as candidates for this look. The three refusals are counted apart
	// because they mean different things and only one of them is a defect:
	//
	//	SlowStaleEpoch	the screen moved while inference ran. ORDINARY.
	//	SlowStaleWindow	another window's frame. Should be rare.
	//	SlowUnusable	the look itself was not evidence.
	//
	// A deployment where SlowAdmitted is near zero and SlowStaleEpoch is large is one
	// where inference is slower than the interface moves — which is a finding about
	// the cadence and not a bug to be worked around by relaxing the rule.
	SlowOffered, SlowAdmitted       int
	SlowStaleEpoch, SlowStaleWindow int
	SlowUnusable                    int
	// SlowExpired is held candidates dropped because the observation moved on,
	// and SlowOfferedLooks the looks on which at least one held candidate was part
	// of the set a press would be judged against. The second is the one that says
	// whether the second source was ever actually available when it mattered.
	SlowExpired, SlowOfferedLooks int
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

	// epoch is the OBSERVATION GENERATION: a counter that moves whenever the standing
	// screen changes, and whenever the window underneath is replaced.
	//
	// It exists because an arrangement RECURS and a moment does not. Settle on A,
	// ask for inference, navigate to B, come back to A, and the answer arrives
	// describing a frame from the first visit while the screen in front looks
	// identical. Nothing about the arrangement can tell those apart; a counter can.
	// Returning to the same screen is a NEW epoch, so the old rectangle is refused
	// rather than silently reused — which is the whole difference between a delayed
	// result and a wrong one.
	epoch     uint64
	lastID    observe.VisualScreenID
	hasLastID bool

	// held is the slow candidates admitted and not yet outrun.
	//
	// A detection arrives on ONE look and is evidence about a SCREEN, so holding it
	// is not an optimisation — it is the difference between the integration working
	// and not. The press that needs the TRAINING tile lands about twenty looks after
	// the menu settled; an answer offered only on the look it arrived on would be
	// gone by then, and every counter would still report it admitted.
	//
	// Held, never refreshed and never reprojected. When the epoch moves the whole set
	// goes at once, because what expired is the observation and not the rectangles.
	held []observe.Candidate
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

// Epoch is the observation generation a request made now should be stamped with.
func (o *Observer) Epoch() uint64 { return o.epoch }

// Screens is the session-local screens settled so far.
func (o *Observer) Screens() []observe.VisualScreenSeen { return o.screens.Seen() }

// Stability is what the stabiliser did, and the numbers that say whether to trust it.
//
// Exposed for the measurements that characterise the fold. `Stats` is this observer's own
// ledger — looks, presses, slow admissions — and this is the screen model's.
func (o *Observer) Stability() observe.VisualStability { return o.screens.Stability() }

// SaidBy is what a screen was read to say, stable words only.
//
// Unread when nobody supplied text for its looks, which is a different answer from a
// screen with no words on it. Saved beside the arrangement by the learner.
func (o *Observer) SaidBy(id observe.VisualScreenID) observe.ScreenText {
	return o.screens.SaidBy(id)
}

// Arrangement is one settled screen's stored arrangement.
//
// For measurement, like the method it forwards to: an arrangement is what a cross-session
// binding would have to carry, and comparing two observers' screens needs it in hand. A caller
// that reconstructed a screen identity from this would be a second statement of what "the same
// screen" means, which is why the relation stays in observe.
func (o *Observer) Arrangement(id observe.VisualScreenID) observe.VisualArrangement {
	return o.screens.Arrangement(id)
}

// Transitions is every change observed between settled screens.
func (o *Observer) Transitions() []observe.VisualTransition { return o.screens.Transitions() }

// Clicks is the press evidence, with the three causes intact.
func (o *Observer) Clicks() observe.ClickEvidence { return o.clicks.Evidence() }

// Walk is the order the session visited its screens.
//
// The order, not the set. A workflow is a sequence somebody performed, and the transition list is
// sorted most-repeated first — assembling a draft from that would produce a set of edges in
// popularity order, which is not a thing anybody demonstrated.
func (o *Observer) Walk() []observe.VisualScreenID { return o.screens.Walk() }

// Screen is the screen standing now, which is what a reacquisition is scoped to.
func (o *Observer) Screen() observe.VisualScreenID { return o.screens.Screen() }

// Registry is every target demonstrated this session, with its provenance.
func (o *Observer) Registry() []observe.TargetRecord { return o.clicks.Registry() }

// Reacquire looks for the targets demonstrated on the screen standing NOW among the proposals of
// the look in front of us.
//
// The screen comes from the observer rather than the caller on purpose: a reacquisition scoped to
// a screen somebody else believed was standing would be a target found on the wrong page. And it
// is a read — nothing here folds, remembers or presses, and
// [[ADR-180-finding-a-demonstrated-target-again]] is explicit that a match is not permission.
func (o *Observer) Reacquire(lk Look) []observe.Reacquisition {
	return o.clicks.Reacquire(o.screens.Screen(), observe.Regions(o.Candidates(lk)))
}

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
		// AND THE EPOCH MOVES. A result computed on the old window describes
		// pixels this observer is no longer looking at, whatever it says.
		o.epoch++
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

	// THE CANDIDATE SET A PRESS IS JUDGED AGAINST: this look's own geometry, plus
	// whatever a slower source has returned that is still about this screen.
	//
	// Built even when there are no presses, because the admission ledger is a
	// measurement of the integration and would otherwise only be collected on the
	// seventeen looks of a nine-hundred-look recording that happen to carry one.
	cands := observe.Merge(
		observe.Proposed(regions, o.look, lk.Window.Generation, o.epoch),
		o.intakeSlow(lk, len(regions) > 0))
	if len(o.held) > 0 && len(regions) > 0 {
		o.stats.SlowOfferedLooks++
	}

	if len(regions) > 0 {
		for _, w := range lk.Presses {
			o.clicks.Press(o.look, w, standing, cands)
		}
	} else {
		// A PRESS WITH NOTHING TO HAVE LANDED ON, counted where it is lost. The
		// look had no geometry, so there is no candidate set and no target; the
		// press is real and unusable, which is a different thing from absent.
		o.stats.PressesUnusable += len(lk.Presses)
	}

	// THE ARRANGEMENT IS PIXEL-ONLY, DELIBERATELY.
	//
	// `regions`, never `cands`. A screen's identity is a count of rectangles in nine
	// boxes, and feeding it a channel that arrives ten looks late and intermittently
	// would make the signature of one screen depend on whether inference had landed
	// yet — the same screen would be two screens, or two screens one, according to
	// timing. Screen identity must be a function of the look alone.
	//
	// `lk.Text` IS A FUNCTION OF THE LOOK ALONE, which is why it may be here and
	// `cands` may not. It is what a source read of THESE pixels, not what arrived
	// while they were being folded, and the type cannot express anything else.
	//
	// It also cannot alter a stored arrangement. The most it does is decline a
	// merge the arrangement proposed, so the signature of every screen stays a
	// count of rectangles whether anybody read it or not, and a run with no text
	// source folds exactly as it did before text existed. See
	// observe/screentext.go for which direction that asymmetry protects.
	// THE SLOW TEXT SOURCE, admitted before the fold so a refusal it raises is in
	// force for this look. See intakeReadings.
	o.intakeReadings(lk, len(regions) > 0 && lk.Front)

	id := o.screens.ObserveWith(observe.ArrangeLook(regions), lk.Text, lk.At)
	if o.hasLastID && id != o.lastID {
		o.epoch++
	}
	o.lastID, o.hasLastID = id, true
	up := Update{At: lk.At, Window: lk.Window, Screen: id, Look: o.look, Epoch: o.epoch}
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

// intakeReadings decides which delayed readings are evidence about the look in front of us,
// and hands the ones that are to the screen model.
//
// THE SAME THREE RULES AS intakeSlow, and deliberately not a second statement of them:
// `observe.Current` decides, and the counters only say which part of the context moved. The
// argument for the third rule is written out below for detections and is the same argument
// here — an epoch moves on every screen change, so an unchanged epoch means no screen change
// has happened since the frame was captured, which is a demonstrably continuous interval
// rather than the weaker "the same screen is standing".
//
// What a reading may do once admitted is in `observe.VisualScreens.Read`, and it is one
// thing: refuse to believe the standing screen is the screen the arrangement says. It never
// mints a screen, moves the standing claim, or reaches a stored arrangement — so a run with
// no text source, or one whose readings all arrive stale, folds exactly as it always did.
//
// NOTHING HERE REACHES AN ACTION ALREADY TAKEN. A refusal changes which screen later looks
// are folded onto; it does not revisit a press, an authorisation, or an arrival already
// established, because none of those is stored here and all of them were decided against
// the observation they happened in.
func (o *Observer) intakeReadings(lk Look, usable bool) {
	for _, r := range lk.Readings {
		o.stats.ReadingsOffered++
		switch {
		case !usable:
			// A look that cannot host a pixel proposal cannot host a reading
			// either. Admitting one would let a covered window's words in by
			// the back door, and a covered window's words are another
			// application's.
			o.stats.ReadingsUnusable++
		case !r.Admissible(lk.Window.Generation, o.epoch):
			if r.Generation != lk.Window.Generation {
				o.stats.ReadingsStaleWindow++
			} else {
				o.stats.ReadingsStaleEpoch++
			}
		default:
			o.stats.ReadingsAdmitted++
			refused, _ := o.screens.Read(r)
			switch {
			case refused:
				o.stats.ReadingsRefusedMerge++
			case !r.Text.Said():
				// READ AND SILENT IS NOT MISSING. A source that answered
				// without reading anything is a fact about the screen or
				// about OCR; a source that never answered is neither, and
				// conflating them would make a frame nobody read look like
				// a screen that went quiet.
				o.stats.ReadingsSilent++
			}
		}
	}
}

// admitSlow decides which delayed detections are evidence about the look in front of us.
//
// # Three rules, and the one that is easy to get wrong
//
// A detection is admissible only when ALL of:
//
//  1. the look it arrives on is itself evidence — pixels were obtained and the target window
//     was in front. A look that cannot host a pixel proposal cannot host a detection either,
//     and admitting one would let a covered window's geometry in by the back door;
//  2. it was computed under the window generation standing now. A frame of a window that has
//     since been replaced describes something else;
//  3. it was computed under the OBSERVATION GENERATION standing now.
//
// The third is the one that matters, and an earlier draft of this had it wrong. It said a
// detection stayed admissible while its SCREEN was still standing — which sounds equivalent and
// is not, because a screen can be left and returned to:
//
//	settle on A  ->  request  ->  navigate to B  ->  return to A  ->  result arrives
//
// Under the screen rule that result is admitted: the arrangement matches, because it is the
// same menu. But between the request and the answer the interface was somewhere else, and
// anything could have changed on the way back — a list scrolled, a dialog opened, a selection
// moved. The rectangle is from a frame nobody is looking at any more and it would be offered as
// though it were current.
//
// An epoch cannot be mistaken that way. It moves on every screen change, so leaving A and
// coming back costs two increments, and the returning detection is refused for the reason it
// should be: not that the screen is different, but that the OBSERVATION is.
//
// Refusals are counted, never logged and never retried. There is nothing to recover: the frame
// is gone, and the next settle will ask again.
func (o *Observer) intakeSlow(lk Look, usable bool) []observe.Candidate {
	for _, c := range lk.Slow {
		o.stats.SlowOffered++
		switch {
		case !usable:
			o.stats.SlowUnusable++
		case !observe.Admissible(c, lk.Window.Generation, o.epoch):
			// ONE DECISION, TWO LABELS. An earlier version asked about the
			// window here and about the epoch in `observe`, which read as
			// thoroughness and was a second copy of half the rule — a mutation
			// run found the copy in `observe` could be deleted with every test
			// still passing, because nothing here was reaching it.
			//
			// So the rule decides, and the counters only say which part of the
			// context moved: a window replaced under the observer and a screen
			// that moved on are different findings about a deployment.
			if c.Generation != lk.Window.Generation {
				o.stats.SlowStaleWindow++
			} else {
				o.stats.SlowStaleEpoch++
			}
		default:
			o.stats.SlowAdmitted++
			o.held = append(o.held, c)
		}
	}
	// WHAT THE OBSERVATION HAS OUTRUN GOES, all of it, the look it stops being current.
	if len(o.held) > 0 {
		kept := o.held[:0]
		for _, c := range o.held {
			if observe.Admissible(c, lk.Window.Generation, o.epoch) {
				kept = append(kept, c)
			}
		}
		o.stats.SlowExpired += len(o.held) - len(kept)
		o.held = kept
	}
	if !usable {
		return nil
	}
	return o.held
}

// Candidates is what a press on this look would be judged against, computed without changing
// anything: the look's own proposals, plus every slow candidate still admissible — held from an
// earlier look, or arriving on this one.
//
// Exported because a RELOADED workflow needs it too. A workflow read from a file has stored
// readings and no session registry, and matching them against a narrower set than a press is
// judged against would mean Marco could learn a control it could never re-acquire. One set, one
// statement of what is on this look.
//
// Reacquisition reads it, so a target the detector made findable can actually be found again.
// Searching a narrower set than a press is judged against would mean Marco could learn a
// control it could never re-acquire, which is the quietest possible way to waste the evidence.
func (o *Observer) Candidates(lk Look) []observe.Candidate {
	if lk.Missing || !lk.Front {
		return nil
	}
	var slow []observe.Candidate
	for _, set := range [][]observe.Candidate{o.held, lk.Slow} {
		for _, c := range set {
			if observe.Admissible(c, lk.Window.Generation, o.epoch) {
				slow = append(slow, c)
			}
		}
	}
	return observe.Merge(
		observe.Proposed(lk.Regions, o.look, lk.Window.Generation, o.epoch), slow)
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
