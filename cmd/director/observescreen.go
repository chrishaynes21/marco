package main

// `director observe-screen` — the fast visual observer, live, watching and saying nothing else.
//
// # What it is
//
// The fast half of Marco's perception, on its own clock. It captures the watched window at its
// own interval, runs the model-free pixel proposer, folds the arrangement into session-local
// screen hypotheses, correlates the mouse presses it observes with the changes that follow, and
// prints what it concluded. See `internal/director/fastobserve`.
//
// Its cadence is its own. [[ADR-175-no-settling-rule-survives-a-four-second-cadence]] measured
// that the screen model needs about ten looks a second and that the AI discovery path's ~3.8
// seconds cannot support it at all — not with a different threshold, not with any threshold. The
// observer takes its interval as an argument and DERIVES its settle rule from it, and refuses to
// start at a rate too coarse to damp.
//
// [[ADR-177-acquisition-is-a-frame-interval-not-a-cost]] made it affordable: a full-resolution
// look is about 50ms, so ten a second uses half the budget on this machine.
//
// # What it does not do
//
// It does not act, it does not remember, and it does not write. No pixels, no frames, no
// screenshots: the output is the terminal and it is hypotheses and counts. The observer package
// cannot reach the durable store, the execution pipeline, or anything that can touch a desktop,
// and its own tests walk its imports to prove it rather than promising it.
//
// It is bounded by `--seconds` and will not run without being told which application to watch.

import (
	"context"
	"flag"
	"fmt"
	"image"
	"io"
	"os"
	"sort"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/structure"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
)

// maxObserveSeconds bounds a session.
//
// Five minutes. Long enough to walk a menu several times and short enough that a forgotten
// terminal does not become persistent visual observation, which is explicitly not authorised.
const maxObserveSeconds = 300

// observeQueue is how many updates may wait for a consumer.
//
// Small on purpose. The point of the queue is that a slow consumer costs UPDATES and never
// costs cadence; a deep one would hide a consumer falling behind for seconds at a time, which
// is the thing the drop count exists to show.
const observeQueue = 32

func runObserveScreen(args []string) int {
	fs := flag.NewFlagSet("observe-screen", flag.ExitOnError)
	app := fs.String("application", "", "the application whose window to watch")
	seconds := fs.Int("seconds", 60, "how long to observe")
	fps := fs.Int("fps", 10, "observations per second")
	delay := fs.Duration("delay", 5*time.Second, "wait before starting, to switch to the app")
	_ = fs.Parse(flagsFirst(args))

	if *app == "" {
		fmt.Fprintln(os.Stderr,
			"director: --application is required\n"+
				"  example: director observe-screen --application rocketleague "+
				"--seconds 60 --fps 10")
		return 2
	}
	if *seconds < 1 || *seconds > maxObserveSeconds {
		fmt.Fprintf(os.Stderr, "director: --seconds must be between 1 and %d\n",
			maxObserveSeconds)
		return 2
	}
	if *fps < 1 || *fps > 60 {
		fmt.Fprintln(os.Stderr, "director: --fps must be between 1 and 60")
		return 2
	}
	interval := time.Second / time.Duration(*fps)
	obs, err := fastobserve.New(interval, observeQueue)
	if err != nil {
		// The refusal an observer gives when its cadence cannot damp. Printed as the
		// reason it is, because "too slow to be trusted" is a conclusion somebody
		// should read rather than a silent degradation.
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 2
	}

	windows := winprovider.New()
	tracker := windowref.NewTracker(windows)
	shots := newCapture(windows)
	ctx := context.Background()
	v := tracker.AcquireBy(ctx, windows, windowref.NewDirectory(),
		windowref.Selector{Application: *app})
	if !v.State.OK() {
		fmt.Fprintf(os.Stderr, "director: %s (%s)\n", v.Reason, v.State)
		return 1
	}

	// THE MOUSE HALF. navsource owns its own blocking message pump and this only opens a
	// subscription and drains it, exactly as the observation sampler does. Nothing here
	// touches the hook thread, and nothing here can inject.
	navs, navWhy := navsource.New()
	var navSub *navsource.Subscription
	if navWhy != "" {
		fmt.Printf("WARNING: %s\n         Presses will NOT be correlated.\n\n", navWhy)
	}

	fmt.Printf("Observing %s at %d/second for %ds\n", v.Ref.Describe(), *fps, *seconds)
	fmt.Printf("  settle rule %d consecutive looks, derived from the %s interval\n",
		obs.SettleLooks(), interval)
	fmt.Printf("  Nothing is injected, nothing is remembered, nothing is written to disk.\n")
	fmt.Printf("  Starting in %s.\n\n", *delay)
	time.Sleep(*delay)

	// THE FRAME BEFORE THE SUBSCRIPTION, so the first press of the session is placeable.
	//
	// navsource refuses a press it cannot place, and it can only place one against bounds
	// pushed BEFORE the press happened — the loop below pushes them, but not until after
	// its first capture, so every press in that first interval would come back
	// `unplaceable_pointer`. That is ADR-168's defect at one look's scale: a press
	// reported as a perception failure when the truth is that nothing had told the
	// pointer path where the window was yet.
	//
	// Re-acquired rather than reused from before the delay. The rectangle from eight
	// seconds ago is not necessarily where the window is now, and pushing it stamped
	// `now` would be a small lie the pointer path would then trust.
	if navs != nil {
		if w := tracker.Acquire(ctx, *app); w.State.OK() &&
			w.Ref.Bounds.Width > 0 && w.Ref.Bounds.Height > 0 {
			navs.SetWindowBounds(w.Ref.Bounds.X, w.Ref.Bounds.Y,
				w.Ref.Bounds.Width, w.Ref.Bounds.Height, time.Now())
		}
		navSub = navs.Open(time.Now())
		defer navs.CloseSession(navSub)
		defer func() { _ = navs.Close() }()
	}

	// A CONSUMER, so the queue is read and the drop count means something. It stands in
	// for the slow semantic path and does nothing but count — which is the shape the real
	// one has to have: read, or lose updates, and never make the observer wait.
	seen := 0
	go func() {
		for range obs.Updates() {
			seen++
		}
	}()

	opt := structure.DefaultOptions()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	deadline := time.Now().Add(time.Duration(*seconds) * time.Second)
	lost := ""
	keys := 0
	var found reacquireTally
	// THE STAGES, TIMED SEPARATELY.
	//
	// The observer reports its own fold latency, which is the one number that can be
	// measured without a screen — and the first live run showed exactly why that is not
	// enough. Six hundred looks in sixty seconds proves the whole look fitted inside its
	// 100ms slot and says nothing about whether it took twenty or ninety-nine, so the
	// acquisition figure every cadence decision rests on could not be confirmed against
	// the thing it was a prediction about.
	//
	// [[ADR-177-acquisition-is-a-frame-interval-not-a-cost]] measured 16.7ms for the blit
	// on a bench and attributed it to a 60Hz compositor. That is a hypothesis about a live
	// window, so a live window should answer it.
	var stage stageTimes
	for time.Now().Before(deadline) {
		lk := fastobserve.Look{At: time.Now()}

		if navSub != nil {
			presses, k := splitNav(navSub.Drain())
			lk.Presses = presses
			keys += k
		}

		v := tracker.Acquire(ctx, *app)
		if !v.State.OK() {
			lost = fmt.Sprintf("target lost (%s)", v.State)
			break
		}
		lk.Window = fastobserve.WindowRef{
			ID: v.Ref.ID, Generation: v.Ref.Generation, Bounds: v.Ref.Bounds,
		}
		// THE POINTER NEEDS A FRAME, the same as the recorder: navsource refuses a
		// press it cannot place, and it cannot place one until somebody pushes the
		// window's rectangle. See ADR-168.
		if navs != nil && v.Ref.Bounds.Width > 0 && v.Ref.Bounds.Height > 0 {
			navs.SetWindowBounds(v.Ref.Bounds.X, v.Ref.Bounds.Y,
				v.Ref.Bounds.Width, v.Ref.Bounds.Height, time.Now())
		}

		whole := time.Now()
		shot := time.Now()
		img, err := shots.CaptureWindow(ctx, windowRefToWindow(v.Ref))
		stage.capture = append(stage.capture, msSince(shot))
		if err != nil {
			lk.Missing = true
		} else {
			lk.Front = frontnessOf(frontCandidate(ctx, windows), v.Ref) ==
				observe.FrontTarget
			prop := time.Now()
			res := structure.Propose(img.Image, opt)
			stage.propose = append(stage.propose, msSince(prop))
			// THE SAME NORMALISATION THE REPLAY USES. One statement of it, in
			// observe, so the live observer and the offline measurement are the
			// same arithmetic on the same proposals.
			rects := make([]image.Rectangle, 0, len(res.Candidates))
			for _, c := range res.Candidates {
				rects = append(rects, c.Bounds)
			}
			lk.Regions = observe.NormaliseRects(img.Image.Bounds(), rects)
		}
		// REACQUISITION BEFORE THE FOLD, so nothing this look teaches can help
		// recognise what this look was meant to recognise. The targets this look's
		// own presses belong to are excluded from the tally for the same reason.
		if len(obs.Registry()) > 0 {
			pressed := map[observe.VisualTargetID]bool{}
			for _, t := range obs.Registry() {
				for _, w := range lk.Presses {
					if t.Screen == obs.Screen() && containsAny(t.Where, w) {
						pressed[t.ID] = true
					}
				}
			}
			found.fold(obs.Reacquire(lk.Regions), pressed)
		}
		obs.Observe(lk)
		stage.whole = append(stage.whole, msSince(whole))
		<-ticker.C
	}
	obs.Close()
	time.Sleep(50 * time.Millisecond) // let the consumer drain

	// WHAT THE MOUSE PRODUCER ITSELF DID, taken before the deferred Close tears it down.
	// Without it the report has an attribution count and no account of what reached it.
	var in observe.InputStats
	if navs != nil {
		in = navs.Stats()
	} else {
		in.Unavailable = navWhy
		if in.Unavailable == "" {
			in.Unavailable = "no mouse producer was started"
		}
	}
	reportObserve(os.Stdout, obs, seen, lost, in, keys, stage, found)
	return 0
}

// reportObserve prints what the observer concluded. Hypotheses and counts.
// reacquireTally is how often each demonstrated target was found again.
//
// Counted live because [[ADR-180-finding-a-demonstrated-target-again]]'s numbers come from one
// recording and five repeat presses. A rate needs a session that walks the same route twice on
// purpose, and the command has to be able to report one before such a session is worth asking
// for.
type reacquireTally struct {
	// byTarget is one record per identity, keyed in first-seen order by `order`.
	byTarget map[observe.VisualTargetID]*targetTally
	order    []observe.VisualTargetID
}

type targetTally struct {
	screen                           observe.VisualScreenID
	looks                            int
	matched, ambiguous, unobservable int
	indistinct                       int
	uncertain                        bool
}

// fold records one look's outcomes.
//
// `pressed` is the targets this look's presses belong to, which are excluded: the owner's
// standard is that a click's own observation is not a reacquisition, and a matcher scored on the
// frame it learned from would report its own memory.
func (r *reacquireTally) fold(got []observe.Reacquisition, pressed map[observe.VisualTargetID]bool) {
	if r.byTarget == nil {
		r.byTarget = map[observe.VisualTargetID]*targetTally{}
	}
	for _, o := range got {
		if pressed[o.Target] {
			continue
		}
		t, ok := r.byTarget[o.Target]
		if !ok {
			t = &targetTally{screen: o.Screen}
			r.byTarget[o.Target] = t
			r.order = append(r.order, o.Target)
		}
		t.looks++
		t.uncertain = t.uncertain || o.Uncertain
		switch o.Result {
		case observe.ReacquireMatched:
			t.matched++
		case observe.ReacquireAmbiguous:
			t.ambiguous++
		default:
			t.unobservable++
			if o.Indistinct > 0 {
				t.indistinct++
			}
		}
	}
}

// stageTimes is per-look milliseconds for each stage of a look.
//
// Kept apart because they are three different claims about three different pieces of the
// machine, and a combined figure answers none of them — the mistake
// [[ADR-177-acquisition-is-a-frame-interval-not-a-cost]] was written to avoid.
type stageTimes struct {
	capture, propose, whole []float64
}

// msSince is one stage's cost in milliseconds, at microsecond resolution.
func msSince(t time.Time) float64 { return float64(time.Since(t).Microseconds()) / 1000 }

// at returns the pth percentile, or zero when nothing was measured.
func pctile(v []float64, p int) float64 {
	if len(v) == 0 {
		return 0
	}
	c := make([]float64, len(v))
	copy(c, v)
	sort.Float64s(c)
	return c[p*(len(c)-1)/100]
}

func reportObserve(w io.Writer, obs *fastobserve.Observer, consumed int, lost string,
	in observe.InputStats, keys int, stage stageTimes, found reacquireTally) {
	s := obs.Stats()
	fmt.Fprintf(w, "\n%d look(s): %d published, %d dropped, %d consumed\n",
		s.Looks, s.Published, s.Dropped, consumed)
	fmt.Fprintf(w, "  not evidence: %d with no pixels, %d behind another window, "+
		"%d window change(s)\n", s.Missing, s.Covered, s.Interrupted)
	fmt.Fprintf(w, "  fold latency: median %.2fms  p95 %.2fms  max %.2fms\n",
		s.At(50), s.At(95), s.At(100))
	for _, st := range []struct {
		name string
		v    []float64
	}{
		{"acquisition", stage.capture},
		{"proposer   ", stage.propose},
		{"whole look ", stage.whole},
	} {
		fmt.Fprintf(w, "  %s:  median %6.2fms  p95 %6.2fms  max %6.2fms  (%d measured)\n",
			st.name, pctile(st.v, 50), pctile(st.v, 95), pctile(st.v, 100), len(st.v))
	}
	if len(stage.whole) > 0 {
		parts := pctile(stage.capture, 50) + pctile(stage.propose, 50)
		if pctile(stage.whole, 50) < parts {
			fmt.Fprintf(w, "  STAGE TIMINGS DO NOT ADD UP: the parts median %.2fms\n  against a whole look of %.2fms. A timer is measuring something other than\n  its own stage, so read none of the three.\n",
				parts, pctile(stage.whole, 50))
		}
	}
	if s.Dropped > 0 {
		fmt.Fprintf(w, "  %d update(s) were dropped because the consumer fell behind. The\n"+
			"  observer did not wait for it, which is the point.\n", s.Dropped)
	}

	screens := obs.Screens()
	fmt.Fprintf(w, "\n%d session-local screen(s) settled (never durable):\n", len(screens))
	for _, sc := range screens {
		fmt.Fprintf(w, "  %-10s %4d look(s), settled at %d, %d region(s) over %d cell(s)\n",
			sc.ID, sc.Looks, sc.First, sc.Total, sc.Cells)
	}
	tr := obs.Transitions()
	fmt.Fprintf(w, "\n%d distinct transition(s):\n", len(tr))
	for _, x := range tr {
		fmt.Fprintf(w, "  %-10s -> %-10s x%d\n", x.From, x.To, x.Count)
	}
	e := obs.Clicks()
	byCause := map[observe.TransitionCause]int{}
	for _, st := range e.Steps {
		byCause[st.Cause] += st.Count
	}

	// ── THE MOUSE, ACCOUNTED FOR END TO END ──
	//
	// Five times now a perception diagnosis has been lost to a count with no denominator,
	// so this reconciles: what the hook saw, what it placed inside the window, what the
	// observer was offered, and what became of each one. "Zero clicks attributed" has at
	// least four causes and they live in four different places — no hook, no press inside
	// the window, a press on a look that was not evidence, a press on nothing proposed —
	// and a reader must not have to guess which.
	fmt.Fprintf(w, "\nthe mouse:\n")
	if in.Unavailable != "" {
		fmt.Fprintf(w, "  NOT OBSERVED: %s\n", in.Unavailable)
		fmt.Fprintf(w, "  Every change below is therefore automatic or unknown BY "+
			"CONSTRUCTION,\n  and the absence of click causes is not evidence "+
			"about the screens.\n")
	} else {
		fmt.Fprintf(w, "  hook: %d event(s) received, %d placed inside the window\n",
			in.Received, in.Classified)
		if len(in.Ignored) > 0 {
			fmt.Fprintf(w, "  not placed:")
			for _, r := range sortedReasons(in.Ignored) {
				fmt.Fprintf(w, " %s=%d", r, in.Ignored[r])
			}
			fmt.Fprintf(w, "\n")
		}
		if in.PointerDeclined > 0 || in.PointerInjected > 0 || in.Dropped > 0 {
			fmt.Fprintf(w, "  refused before the queue: %d not a navigation button, "+
				"%d injected; %d dropped by a full queue\n",
				in.PointerDeclined, in.PointerInjected, in.Dropped)
		}
		fmt.Fprintf(w, "  observer: %d press(es) offered, %d on a look that was not "+
			"evidence\n", s.Presses, s.PressesUnusable)
		fmt.Fprintf(w, "  correlator: %d press(es) folded, %d landed on nothing, "+
			"%d inside more than one region\n",
			e.Presses, e.Blind, e.AmbiguousPresses)
		fmt.Fprintf(w, "  %d press(es) were followed by no screen change\n", len(e.Inert))
		if keys > 0 {
			fmt.Fprintf(w, "  %d keyboard event(s) were seen and not used: this path "+
				"correlates presses only\n", keys)
		}
	}
	fmt.Fprintf(w, "changes by cause: %d click, %d automatic, %d unknown\n",
		byCause[observe.CauseClick], byCause[observe.CauseAutomatic],
		byCause[observe.CauseUnknown])
	for _, st := range e.Steps {
		fmt.Fprintf(w, "  %-10s --%-10s--> %-10s %-9s x%d\n",
			st.From, st.Target, st.To, st.Cause, st.Count)
	}
	reportReacquire(w, obs, found)
	if lost != "" {
		fmt.Fprintf(w, "\nStopped early: %s\n", lost)
	}
	fmt.Fprintf(w, "\nNothing was remembered and nothing was written.\n")
}

// splitNav separates the presses this path can use from the events it only counts.
//
// The fast observer correlates POINTER presses. A keyboard event is observed and not used, and
// counting it is what distinguishes "nobody navigated" from "they navigated with a device this
// path does not read" — which is the mistake
// [[ADR-169-a-recording-that-could-not-explain-its-own-total]] records: four minutes of
// controller navigation reported as a player who pressed nothing.
//
// Extracted so the classification can be tested without a window. Inside the observation loop it
// was reachable only by watching a real screen, which is the one thing this command may not do
// unasked.
func splitNav(events []observe.InputEvent) ([]observe.PointerAt, int) {
	var presses []observe.PointerAt
	keys := 0
	for _, e := range events {
		if e.Intent == observe.NavPoint {
			presses = append(presses, e.Where)
			continue
		}
		keys++
	}
	return presses, keys
}

// containsAny reports whether any of a target's readings contains a press.
func containsAny(where []observe.Region, at observe.PointerAt) bool {
	for _, r := range where {
		if r.Width <= 0 || r.Height <= 0 {
			continue
		}
		if at.X >= r.X && at.Y >= r.Y && at.X < r.X+r.Width && at.Y < r.Y+r.Height {
			return true
		}
	}
	return false
}

// sortedReasons orders the ignore reasons so two runs print them the same way.
func sortedReasons(m map[navsource.IgnoreReason]int) []navsource.IgnoreReason {
	out := make([]navsource.IgnoreReason, 0, len(m))
	for r := range m {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
