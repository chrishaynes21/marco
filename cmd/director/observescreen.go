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
	"os"
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

	if navs != nil {
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
	for time.Now().Before(deadline) {
		lk := fastobserve.Look{At: time.Now()}

		var presses []observe.PointerAt
		if navSub != nil {
			for _, e := range navSub.Drain() {
				if e.Intent == observe.NavPoint {
					presses = append(presses, e.Where)
				}
			}
		}
		lk.Presses = presses

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

		img, err := shots.CaptureWindow(ctx, windowRefToWindow(v.Ref))
		if err != nil {
			lk.Missing = true
		} else {
			lk.Front = frontnessOf(frontCandidate(ctx, windows), v.Ref) ==
				observe.FrontTarget
			res := structure.Propose(img.Image, opt)
			// THE SAME NORMALISATION THE REPLAY USES. One statement of it, in
			// observe, so the live observer and the offline measurement are the
			// same arithmetic on the same proposals.
			rects := make([]image.Rectangle, 0, len(res.Candidates))
			for _, c := range res.Candidates {
				rects = append(rects, c.Bounds)
			}
			lk.Regions = observe.NormaliseRects(img.Image.Bounds(), rects)
		}
		obs.Observe(lk)
		<-ticker.C
	}
	obs.Close()
	time.Sleep(50 * time.Millisecond) // let the consumer drain

	reportObserve(obs, seen, lost)
	return 0
}

// reportObserve prints what the observer concluded. Hypotheses and counts.
func reportObserve(obs *fastobserve.Observer, consumed int, lost string) {
	s := obs.Stats()
	fmt.Printf("\n%d look(s): %d published, %d dropped, %d consumed\n",
		s.Looks, s.Published, s.Dropped, consumed)
	fmt.Printf("  not evidence: %d with no pixels, %d behind another window, "+
		"%d window change(s)\n", s.Missing, s.Covered, s.Interrupted)
	fmt.Printf("  fold latency: median %.2fms  p95 %.2fms  max %.2fms\n",
		s.At(50), s.At(95), s.At(100))
	if s.Dropped > 0 {
		fmt.Printf("  %d update(s) were dropped because the consumer fell behind. The\n"+
			"  observer did not wait for it, which is the point.\n", s.Dropped)
	}

	screens := obs.Screens()
	fmt.Printf("\n%d session-local screen(s) settled (never durable):\n", len(screens))
	for _, sc := range screens {
		fmt.Printf("  %-10s %4d look(s), settled at %d, %d region(s) over %d cell(s)\n",
			sc.ID, sc.Looks, sc.First, sc.Total, sc.Cells)
	}
	tr := obs.Transitions()
	fmt.Printf("\n%d distinct transition(s):\n", len(tr))
	for _, x := range tr {
		fmt.Printf("  %-10s -> %-10s x%d\n", x.From, x.To, x.Count)
	}
	e := obs.Clicks()
	byCause := map[observe.TransitionCause]int{}
	for _, st := range e.Steps {
		byCause[st.Cause] += st.Count
	}
	fmt.Printf("\npresses: %d landed on nothing, %d inside more than one region\n",
		e.Blind, e.AmbiguousPresses)
	fmt.Printf("changes by cause: %d click, %d automatic, %d unknown\n",
		byCause[observe.CauseClick], byCause[observe.CauseAutomatic],
		byCause[observe.CauseUnknown])
	for _, st := range e.Steps {
		fmt.Printf("  %-10s --%-10s--> %-10s %-9s x%d\n",
			st.From, st.Target, st.To, st.Cause, st.Count)
	}
	if lost != "" {
		fmt.Printf("\nStopped early: %s\n", lost)
	}
	fmt.Printf("\nNothing was remembered and nothing was written.\n")
}
