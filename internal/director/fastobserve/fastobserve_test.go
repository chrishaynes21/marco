package fastobserve_test

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// win is one window's provenance, at a generation.
func win(gen uint64) fastobserve.WindowRef {
	return fastobserve.WindowRef{
		ID: "hwnd:1", Generation: gen,
		Bounds: directorapi.Rect{Width: 1920, Height: 1080},
	}
}

// rows is a column of rectangles, which is the shape a menu makes.
func rows(x float64, n int) []observe.Region {
	out := make([]observe.Region, 0, n)
	for i := range n {
		out = append(out, observe.Region{
			X: x, Y: 0.1 + float64(i)*0.08, Width: 0.2, Height: 0.05,
		})
	}
	return out
}

var t0 = time.Unix(1700000000, 0).UTC()

// look builds a good look at a given millisecond.
func look(ms int, gen uint64, regions []observe.Region) fastobserve.Look {
	return fastobserve.Look{
		At:     t0.Add(time.Duration(ms) * time.Millisecond),
		Window: win(gen), Regions: regions, Front: true,
	}
}

// THE SETTLE RULE COMES FROM THE OBSERVER'S OWN INTERVAL, NEVER FROM THE AI'S.
//
// `observe.VisualSettleLooks` is three because three looks at 10fps span about 200ms. The rule
// is the DURATION; the count is how it is measured at a rate. An observer that inherited the
// count without the rate would damp for eleven seconds at the AI's 3.8-second cadence, which is
// exactly what [[ADR-175-no-settling-rule-survives-a-four-second-cadence]] measured as fatal.
//
// Deleting the derivation must fail this.
func TestTheSettleRuleComesFromTheObserversOwnInterval(t *testing.T) {
	for _, c := range []struct {
		interval time.Duration
		want     int
	}{
		// 10fps is the rate every offline measurement was taken at, and it must come
		// out as the rule those measurements were taken under.
		{100 * time.Millisecond, 3},
		{50 * time.Millisecond, 5},
		{200 * time.Millisecond, 2},
	} {
		if got := fastobserve.SettleLooksFor(c.interval); got != c.want {
			t.Errorf("at %s a look the %s window holds %d look(s), want %d",
				c.interval, fastobserve.SettleWindow, got, c.want)
		}
	}
	o, err := fastobserve.New(100*time.Millisecond, 8)
	if err != nil {
		t.Fatalf("a 10fps observer: %v", err)
	}
	defer o.Close()
	if o.SettleLooks() != observe.VisualSettleLooks {
		t.Errorf("a 10fps observer settles after %d look(s); every offline measurement "+
			"was taken at %d and the two must be the same rule",
			o.SettleLooks(), observe.VisualSettleLooks)
	}

	// AND THE DERIVED RULE REACHES THE STABILISER, which is a different claim from
	// reporting it. At 10fps the derived count happens to equal the default, so an
	// observer that computed the rule and never applied it would look correct at the one
	// rate every offline measurement was taken at — and silently damp for the default at
	// every other. So this checks a rate where the two DIFFER.
	slow, err := fastobserve.New(200*time.Millisecond, 8)
	if err != nil {
		t.Fatalf("a 5fps observer: %v", err)
	}
	defer slow.Close()
	if slow.SettleLooks() != 2 {
		t.Fatalf("a 5fps observer settles after %d look(s), want 2", slow.SettleLooks())
	}
	// TWO looks of one arrangement must settle it there. Under the default of three it
	// would not, and nothing would have settled at all.
	for i := range 2 {
		slow.Observe(look(i*200, 1, rows(0.1, 6)))
	}
	if n := len(slow.Screens()); n != 1 {
		t.Errorf("%d screen(s) after two looks at a 5fps observer whose rule is two, "+
			"want 1. The rule was computed and not applied, so the stabiliser is "+
			"still damping for three", n)
	}
}

// AN OBSERVER TOO SLOW TO DAMP REFUSES TO RUN.
//
// At the AI path's ~3.8-second cadence the settle window holds one look, and one look is no
// damping at all: ADR-175 measured settle=1 on the real recording at 113 promotions over 900
// looks, with 72 of the causes unknown. A transition storm looks like evidence, which makes it
// worse than seeing nothing.
//
// So this refuses rather than clamping. A clamp would observe at a rate whose results nobody
// should trust and say nothing about it — and silently inheriting the slow cadence is the exact
// accident the whole package exists to prevent.
//
// Deleting the refusal must fail this.
func TestAnObserverTooSlowToDampRefusesToRun(t *testing.T) {
	if _, err := fastobserve.New(3800*time.Millisecond, 8); err == nil {
		t.Error("an observer at the AI path's 3.8-second cadence was accepted. One " +
			"look in the settle window is no damping, and it produced a promotion " +
			"every eight looks on the measured recording")
	}
	// The boundary: the slowest interval that still fits two looks is the settle window
	// itself, and anything slower is refused.
	if _, err := fastobserve.New(fastobserve.SettleWindow, 8); err != nil {
		t.Errorf("an observer at exactly the settle window was refused: %v", err)
	}
	if _, err := fastobserve.New(fastobserve.SettleWindow+time.Millisecond, 8); err == nil {
		t.Error("an observer slower than the settle window was accepted, and it can " +
			"fit only one look")
	}
	if _, err := fastobserve.New(0, 8); err == nil {
		t.Error("an observer with no interval was accepted")
	}
	if _, err := fastobserve.New(100*time.Millisecond, 0); err == nil {
		t.Error("an observer with no queue was accepted; it could publish nothing")
	}
}

// A SLOW CONSUMER COSTS UPDATES AND NEVER COSTS CADENCE.
//
//	I would also require a bounded queue between the fast observer and slower consumers. If
//	the semantic path falls behind, it should not cause an unbounded backlog or slow down
//	visual observation.
//
// So the queue is small, the send never waits, and what it refuses is counted. An observer that
// blocked would run at the slow consumer's rate — which is the defect this package was built to
// fix, arriving by a different door.
//
// Deleting the non-blocking send must hang this test, which is a failure of the loudest kind.
func TestASlowConsumerCostsUpdatesAndNotCadence(t *testing.T) {
	o, err := fastobserve.New(100*time.Millisecond, 2)
	if err != nil {
		t.Fatal(err)
	}
	// Nobody reads the channel. Far more looks than the queue can hold.
	const n = 20
	done := make(chan struct{})
	go func() {
		for i := range n {
			o.Observe(look(i*100, 1, rows(0.1, 6)))
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the observer blocked on a consumer that was not reading. A fast " +
			"observer that waits runs at the slow consumer's rate")
	}
	s := o.Stats()
	if s.Looks != n {
		t.Errorf("%d look(s) folded of %d", s.Looks, n)
	}
	if s.Published > 2 {
		t.Errorf("%d update(s) reached a queue of 2", s.Published)
	}
	if s.Dropped == 0 {
		t.Error("nothing was counted as dropped, so a consumer that fell behind would " +
			"look like a screen that never changed")
	}
	if s.Published+s.Dropped != s.Looks {
		t.Errorf("%d published and %d dropped of %d looks: every look must be in one "+
			"of the two or the report cannot be added up",
			s.Published, s.Dropped, s.Looks)
	}
}

// A WINDOW THAT CHANGED UNDER THE OBSERVER DOES NOT MANUFACTURE A TRANSITION.
//
//	Missing frames and window changes must not manufacture transitions.
//
// The looks either side of a replaced window are of two different things. Dropping the standing
// claim leaves the next promotion with nothing to come from, and a transition with no origin is
// already refused — so no edge can span the change.
//
// Deleting the interrupt must fail this.
func TestAWindowChangeDoesNotManufactureATransition(t *testing.T) {
	o, err := fastobserve.New(100*time.Millisecond, 64)
	if err != nil {
		t.Fatal(err)
	}
	// Settle on one screen in generation 1...
	for i := range 4 {
		o.Observe(look(i*100, 1, rows(0.1, 6)))
	}
	// ...then the window is replaced and the layout is different.
	for i := range 4 {
		o.Observe(look(400+i*100, 2, rows(0.8, 3)))
	}
	s := o.Stats()
	if s.Interrupted != 1 {
		t.Errorf("%d interruption(s) over one window change, want 1", s.Interrupted)
	}
	// TWO SCREENS SETTLED AND NO EDGE BETWEEN THEM. The screens are kept — coming back
	// to one is still a return — and the pair across the discontinuity is not a
	// navigation anybody made.
	if n := len(o.Screens()); n != 2 {
		t.Errorf("%d screen(s) settled over two windows, want 2: %+v", n, o.Screens())
	}
	if tr := o.Transitions(); len(tr) != 0 {
		t.Errorf("%d transition(s) recorded across a window change: %+v. The looks "+
			"either side are of two different things", len(tr), tr)
	}
}

// A LOOK WITH NO PIXELS, OR OF SOMEBODY ELSE'S WINDOW, IS NOT A SCREEN.
//
// Three different reasons a look is not evidence, and they are counted apart because they send a
// reader to three different places: a capture that failed, a window that was covered, and a
// window that was replaced.
//
// The covered case is not hypothetical. A capture by window can return the pixels of whatever is
// in front, under the target's own name — Experiment-025 captured the Windows taskbar on one
// look of 107 with provenance clean — so letting another window's layout settle as this
// application's screen is a real failure and not a theoretical one.
//
// Deleting either guard must fail this.
func TestALookWithNoPixelsOrOfAnotherWindowIsNotAScreen(t *testing.T) {
	o, err := fastobserve.New(100*time.Millisecond, 64)
	if err != nil {
		t.Fatal(err)
	}
	// ENOUGH OF EACH TO SETTLE, which is what makes this a test rather than a tally.
	// One covered look cannot settle whatever rule is in force, so a version that
	// offered one would pass with the guard deleted. Three of each, of one layout.
	//
	// A missing look is given regions too: `Missing` means "these pixels are not
	// evidence", and a caller that set both must not be trusted into a screen.
	for i := range 3 {
		o.Observe(fastobserve.Look{
			At: t0.Add(time.Duration(i*100) * time.Millisecond), Window: win(1),
			Regions: rows(0.45, 12), Missing: true})
	}
	for i := range 3 {
		o.Observe(fastobserve.Look{
			At: t0.Add(time.Duration(300+i*100) * time.Millisecond), Window: win(1),
			Regions: rows(0.8, 9), Front: false})
	}
	s := o.Stats()
	if s.Missing != 3 {
		t.Errorf("%d look(s) counted as missing pixels, want 3", s.Missing)
	}
	if s.Covered != 3 {
		t.Errorf("%d look(s) counted as taken behind another window, want 3", s.Covered)
	}
	if n := len(o.Screens()); n != 0 {
		t.Errorf("%d screen(s) settled from a failed capture and a covered window: %+v",
			n, o.Screens())
	}
	// AND NEITHER LAYOUT MUST HAVE BEEN REMEMBERED. Nine rows and twelve rows were
	// offered, three looks each — enough to settle anything. If either appears below,
	// the guard counted the look and used it anyway.
	for i := range 4 {
		o.Observe(look(600+i*100, 1, rows(0.1, 6)))
	}
	seen := o.Screens()
	if len(seen) != 1 {
		t.Fatalf("%d screen(s) after four good looks, want 1: %+v", len(seen), seen)
	}
	if seen[0].Total != 6 {
		t.Errorf("the settled screen holds %d region(s), want 6. Nine is the covered "+
			"window's layout, and it must not have become this application's screen",
			seen[0].Total)
	}
}

// An unobserved look SAYS it was unobserved rather than repeating the last screen.
//
// A consumer that cannot tell "the screen is still that one" from "nobody could see" will
// eventually assume the first, and then act on a screen nobody has looked at.
func TestAnUnobservedLookSaysSoRatherThanRepeatingTheLastScreen(t *testing.T) {
	o, err := fastobserve.New(100*time.Millisecond, 64)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	for i := range 4 {
		o.Observe(look(i*100, 1, rows(0.1, 6)))
	}
	up := o.Observe(fastobserve.Look{
		At: t0.Add(500 * time.Millisecond), Window: win(1), Missing: true})
	if !up.Unobserved {
		t.Errorf("a look with no pixels published %+v without saying it was "+
			"unobserved", up)
	}
	// The provenance travels with it, because a hypothesis nobody can place is not a
	// hypothesis.
	if up.Window.Generation != 1 || up.Window.ID == "" {
		t.Errorf("the update carries no window provenance: %+v", up.Window)
	}
	if up.At.IsZero() {
		t.Error("the update carries no timestamp, so nothing can say when it was true")
	}
}

// A settled change publishes its CAUSE, and an automatic one carries no target.
//
// The three-cause model is preserved through the observer, because the cause is the field that
// decides what may ever be done: a click may authorise a press, an automatic transition says to
// wait, and an unknown one says nothing may be executed.
func TestASettledChangePublishesItsCause(t *testing.T) {
	o, err := fastobserve.New(100*time.Millisecond, 64)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	// Settle on a screen, press something on it, then let the layout change.
	for i := range 4 {
		o.Observe(look(i*100, 1, rows(0.1, 6)))
	}
	press := look(400, 1, rows(0.1, 6))
	press.Presses = []observe.PointerAt{{X: 0.2, Y: 0.15}}
	o.Observe(press)
	var step *observe.RouteStep
	for i := range 4 {
		up := o.Observe(look(500+i*100, 1, rows(0.8, 3)))
		if up.Step != nil {
			step = up.Step
		}
	}
	if step == nil {
		t.Fatal("a settled change published no step, so a consumer cannot tell what " +
			"caused it")
	}
	if step.Cause != observe.CauseClick {
		t.Errorf("the step's cause is %q, want click: a press was made on the screen "+
			"it left, inside the window", step.Cause)
	}
	if !step.Cause.AuthorizesInput() {
		t.Error("a click-caused step says it may not produce a press")
	}
	if step.Target == "" {
		t.Error("a click-caused step names no target, so nothing could be re-grounded")
	}
	// And the click evidence is intact behind it.
	if n := len(o.Clicks().Steps); n == 0 {
		t.Error("the observer publishes steps and keeps no evidence of them")
	}
}

// A PRESS ON THE LOOK THAT COMPLETES A PROMOTION BELONGS TO THE SCREEN IT LEFT.
//
// The one look in a session that both carries a press and settles a change is where the order
// matters: folding the press after the arrangement would record it as made on the screen it
// ARRIVED at, and a learning unit whose source is its own destination is a step nothing can
// walk.
//
// Hard to hit by accident and trivial to break, so it gets its own test.
//
// Deleting the standing-screen read before the fold must fail this.
func TestAPressOnThePromotingLookBelongsToTheScreenItLeft(t *testing.T) {
	o, err := fastobserve.New(100*time.Millisecond, 64)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	for i := range 4 {
		o.Observe(look(i*100, 1, rows(0.1, 6)))
	}
	// Two looks of the new layout, then a third that BOTH carries a press and settles.
	o.Observe(look(400, 1, rows(0.8, 3)))
	o.Observe(look(500, 1, rows(0.8, 3)))
	settling := look(600, 1, rows(0.8, 3))
	settling.Presses = []observe.PointerAt{{X: 0.85, Y: 0.15}}
	up := o.Observe(settling)

	if up.Step == nil {
		t.Fatal("the settling look published no step")
	}
	if up.Step.From == up.Step.To {
		t.Errorf("the step reads %s -> %s: a unit whose source is its own destination "+
			"means the press was folded AFTER the arrangement that promoted",
			up.Step.From, up.Step.To)
	}
	// The press was made on the screen being LEFT, so it cannot be the target of the
	// step that arrives — and this press cannot have caused this change anyway, since
	// the change had already been settling for two looks.
	for _, st := range o.Clicks().Steps {
		if st.From == st.To {
			t.Errorf("a step reads %s -> %s with target %q", st.From, st.To, st.Target)
		}
	}
}

// The observer reports its own fold latency, in percentiles.
//
// Separate from the capture cost and from the proposer's: this is what the observer ITSELF adds
// once the pixels are in hand, and it is the one number that can be measured without a screen.
// A cadence has to survive its p95.
func TestTheObserverReportsItsOwnFoldLatency(t *testing.T) {
	o, err := fastobserve.New(100*time.Millisecond, 64)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	for i := range 10 {
		o.Observe(look(i*100, 1, rows(0.1, 6)))
	}
	s := o.Stats()
	if len(s.LatencyMS) != 10 {
		t.Fatalf("%d latency sample(s) over 10 looks", len(s.LatencyMS))
	}
	if s.At(50) < 0 || s.At(95) < s.At(50) {
		t.Errorf("the latency percentiles are median %.3f p95 %.3f", s.At(50), s.At(95))
	}
	var none fastobserve.Stats
	if none.At(50) != 0 {
		t.Error("an observer that folded nothing reported a latency")
	}
}

// EVERY PRESS IS ACCOUNTED FOR, INCLUDING THE ONES NOTHING COULD BE DONE WITH.
//
// A press offered on a look that carried no geometry — no pixels, another window in front,
// nothing proposed — cannot be correlated: there is no candidate set, so there is no target and
// no screen it was made on. Discarding it is right. Discarding it SILENTLY is the defect
// [[ADR-169-a-recording-that-could-not-explain-its-own-total]] is about, and it would be worse
// here than in the recorder, because the live observer's press count is the only thing that
// separates "no clicks were attributed" from "no clicks were seen".
//
// So: the total the observer was offered, and the subset it could not use, both counted.
func TestAPressOnALookThatIsNotEvidenceIsCountedAndNotCorrelated(t *testing.T) {
	o, err := fastobserve.New(100*time.Millisecond, 64)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	at := observe.PointerAt{X: 0.2, Y: 0.15}

	// One press on a look with no pixels, one on a look behind another window, and one
	// on a look that is ordinary evidence.
	blindLook := fastobserve.Look{
		At: time.Unix(1700000000, 0).UTC(), Window: win(1),
		Regions: rows(0.1, 6), Missing: true, Presses: []observe.PointerAt{at},
	}
	o.Observe(blindLook)
	covered := fastobserve.Look{
		At: time.Unix(1700000000, 0).UTC().Add(100 * time.Millisecond), Window: win(1),
		Regions: rows(0.1, 6), Front: false, Presses: []observe.PointerAt{at},
	}
	o.Observe(covered)
	good := look(200, 1, rows(0.1, 6))
	good.Presses = []observe.PointerAt{at}
	o.Observe(good)

	s := o.Stats()
	if s.Presses != 3 {
		t.Errorf("%d press(es) offered, want 3: the total is the denominator every "+
			"other press count is a subset of", s.Presses)
	}
	if s.PressesUnusable != 2 {
		t.Errorf("%d press(es) counted as unusable, want 2 — one on a look with no "+
			"pixels and one taken behind another window", s.PressesUnusable)
	}
	if n := o.Clicks().Presses; n != 1 {
		t.Errorf("the correlator folded %d press(es), want 1. A press on a look with "+
			"no geometry has no candidate set and must not become evidence", n)
	}
	// AND THE TWO ACCOUNTS AGREE. Offered, minus unusable, is what the correlator got;
	// a version that counted the discard and folded it anyway would pass both checks
	// above and fail this one.
	if got := s.Presses - s.PressesUnusable; got != o.Clicks().Presses {
		t.Errorf("%d press(es) offered less %d unusable is %d, but the correlator "+
			"holds %d: the account does not close",
			s.Presses, s.PressesUnusable, got, o.Clicks().Presses)
	}
}
