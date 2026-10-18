package main

// ONE LIVE RUN OF THE FAST VISUAL OBSERVER, shared by the two commands that need one.
//
// # Why this is a file of its own
//
// `director observe-screen` watches a window and prints what it concluded, and its own wiring
// test asserts over its TEXT that it names none of the file-writing or image-encoding calls --
// it watches and keeps nothing. That is a property worth keeping and it is also the reason a
// second command had to exist: capturing a demonstration means writing one down.
//
// The test reads THIS file too, so the same refusal covers the shared loop. Anything that
// writes belongs in the demonstrating command, where a reader looking for the thing that writes
// will find it.
//
// So the loop lives here, writes nothing itself, and is called by both. `observe-screen` keeps
// its own observer construction and its own report, which is what its test reads it for; the
// demonstrating command adds the save, in its own file, where a reader looking for the thing
// that writes will find it.
//
// # What it does, and the three things it is careful about
//
// Each of them is a defect that was found live and is now load-bearing:
//
//   - **A frame before the subscription.** `navsource` refuses a press it cannot place, and it
//     can only place one against bounds pushed BEFORE the press. Without this every press in
//     the first interval comes back `unplaceable_pointer` — a perception failure reported for a
//     press nobody had told the pointer path where to put. See ADR-168.
//   - **Reacquisition before the fold.** Nothing a look teaches may help recognise what that
//     look was meant to recognise, and the targets this look's own presses belong to are
//     excluded for the same reason.
//   - **A consumer on the update channel.** The queue has to be read or the drop count means
//     nothing, and the consumer must never make the observer wait.

import (
	"context"
	"fmt"
	"image"
	"io"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/structure"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
)

// visualSession is what to watch, how fast, and for how long.
type visualSession struct {
	app string
	// title narrows the window when an application has several.
	//
	// Needed the first time either command was pointed at a real desktop: Notepad and
	// Settings both report more than one window, and `--application` alone is refused as
	// ambiguous -- correctly, because watching "whichever one it picked" is how a
	// demonstration gets recorded against the wrong surface. The same two selectors
	// `director learn` has, spelled the same way.
	title    string
	seconds  int
	interval time.Duration
	delay    time.Duration
	// announce is what the session prints before it starts. A command that watches
	// somebody's screen says so, and says when it will begin.
	announce func(ref string, settle int)
	// each, when set, is given every look BEFORE it is folded.
	//
	// Before, which is the whole reason it is a hook rather than a loop over what the
	// observer ended up knowing: nothing a look teaches may help recognise what that
	// look was meant to recognise. The observer's own reacquisition is ordered the same
	// way, inside this loop, and a caller measuring a SAVED routine needs the same
	// ordering against evidence the observer has never seen.
	each func(lk fastobserve.Look)
	// stop, when set, ends the session early -- the person saying "that's it".
	//
	// A demonstration is a thing somebody DOES, and a fixed duration makes them watch a
	// clock instead. The bound stays: --seconds is still the ceiling, and a forgotten
	// terminal still stops on its own.
	stop <-chan struct{}
	// reader, when set, reads the words on each look BESIDE the loop.
	//
	// Beside, never inside: OCR costs hundreds of milliseconds against a cadence of a
	// hundred, so no live source can ever fill `Look.Text` -- which is look-local by
	// construction. What it fills is `Look.Readings`, the delayed-arrival channel, and
	// the observer refuses a reading whose observation generation has moved on.
	//
	// `readingEyes` was built for exactly this and `director perform-workflow` was the
	// only caller. The first human game-menu demonstration found out what that cost:
	// three clicks, no control read at all, and the one control learned was a 650x214
	// rectangle with no name. See
	// [[Experiment-079-the-first-human-game-menu-demonstration]].
	reader *readingEyes
}

// visualResult is everything the loop learned that is not in the observer itself.
type visualResult struct {
	// seen is how many updates the consumer drained, and lost why the loop stopped early.
	seen int
	lost string
	// in is what the mouse producer itself did, taken before its Close tears it down.
	// Without it a report has an attribution count and no account of what reached it.
	in    observe.InputStats
	keys  int
	stage stageTimes
	found reacquireTally
	// ended says the session stopped before its deadline because the person said so.
	// Empty when it ran to time, which is a different thing from `lost`.
	ended string
	// looks counts the looks taken, and readings how many OCR results arrived in time to
	// be offered to the observer. A reading that arrived after its observation had moved
	// on is refused by the observer and counted there, not here.
	looks    int
	readings int
	// moments is every look that carried input, with what arrived on it.
	//
	// The raw demonstration. `splitNav` took the pointer positions out of each drain and
	// counted the rest, and the events themselves — every keypress, every release, every
	// repeat, with its own timestamp and its own intent — were dropped on the floor one
	// look at a time. So the production teaching path kept counters while
	// `capture-game-session` kept a timeline, and the owner's architecture needs the
	// timeline on the path a person actually teaches through:
	//
	//	Marco remembers what it thinks the user taught it better than it remembers
	//	exactly what the user did.
	//
	// Collected here and WRITTEN nowhere: this file's own wiring test asserts that the
	// shared loop names nothing that writes. See demonstratevisual.go.
	moments []workflowstore.Moment
	// generations is the distinct window identities the session ran against.
	//
	// More than one means the window was replaced mid-demonstration, which invalidates
	// comparing geometry across the join. Recorded, never repaired.
	generations map[uint64]bool
}

// note keeps one look's inputs as a moment of the raw demonstration.
//
// Only looks that CARRIED input become moments: a demonstration is thirty seconds of looks and
// four seconds of hands, and a record with a moment per look would be mostly empty objects.
// The look index is the reference into the visual evidence, and it is kept whether or not
// anything was made of what arrived.
func (r *visualResult) note(look int, since time.Duration, screen observe.VisualScreenID,
	front bool, in []observe.InputEvent) {

	if len(in) == 0 {
		return
	}
	r.moments = append(r.moments, workflowstore.Moment{
		// THE SESSION ID, held in memory only. It is translated into the
		// routine's own label when the record is written, because a durable
		// file may not carry a session-local visual identity. See
		// demonstratevisual.go's rawDemonstration.
		Look: look, Screen: string(screen), AtMS: since.Milliseconds(),
		Front: front, Inputs: in,
	})
}

// watch runs one session, folding every look into obs. It writes nothing.
//
// Returns the result and a process exit code: non-zero when the window could not be acquired at
// all, which is a different thing from losing it partway through — that comes back in `lost`,
// because a session that watched for forty seconds and then lost its window has evidence.
func (s visualSession) watch(ctx context.Context, obs *fastobserve.Observer,
	errOut io.Writer) (visualResult, int) {

	var res visualResult
	windows := winprovider.New()
	tracker := windowref.NewTracker(windows)
	shots := newCapture(windows)
	// EXACTLY ONE SELECTOR, which is what `Selector.Validate` requires and why: a
	// request naming two is a request with nothing to disambiguate it, and resolving it
	// by precedence would be this layer deciding which half of a person's sentence to
	// believe. A title is the narrower of the two, so a title given wins outright and
	// the application is then only in the message.
	sel := windowref.Selector{Application: s.app}
	if s.title != "" {
		sel = windowref.Selector{Title: s.title}
	}
	v := tracker.AcquireBy(ctx, windows, windowref.NewDirectory(), sel)
	if !v.State.OK() {
		fmt.Fprintf(errOut, "director: %s (%s)\n", v.Reason, v.State)
		return res, 1
	}

	// THE MOUSE HALF. navsource owns its own blocking message pump and this only opens a
	// subscription and drains it, exactly as the observation sampler does. Nothing here
	// touches the hook thread, and nothing here can inject.
	navs, navWhy := navsource.New()
	var navSub *navsource.Subscription
	if navWhy != "" {
		fmt.Fprintf(errOut, "WARNING: %s\n         Presses will NOT be correlated.\n\n",
			navWhy)
	}

	if s.announce != nil {
		s.announce(v.Ref.Describe(), obs.SettleLooks())
	}
	time.Sleep(s.delay)

	// THE FRAME BEFORE THE SUBSCRIPTION, re-acquired rather than reused from before the
	// delay: the rectangle from eight seconds ago is not necessarily where the window is
	// now, and pushing it stamped `now` would be a small lie the pointer path would trust.
	if navs != nil {
		if w := tracker.Acquire(ctx, s.app); w.State.OK() &&
			w.Ref.Bounds.Width > 0 && w.Ref.Bounds.Height > 0 {
			navs.SetWindowBounds(w.Ref.Bounds.X, w.Ref.Bounds.Y,
				w.Ref.Bounds.Width, w.Ref.Bounds.Height, time.Now())
		}
		navSub = navs.Open(time.Now())
		defer navs.CloseSession(navSub)
		defer func() { _ = navs.Close() }()
	}

	// A CONSUMER, so the queue is read and the drop count means something. It stands in
	// for the slow semantic path and does nothing but count.
	done := make(chan int, 1)
	go func() {
		n := 0
		for range obs.Updates() {
			n++
		}
		done <- n
	}()

	opt := structure.DefaultOptions()
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	// THE SESSION'S OWN ZERO, so every time in the raw record is relative to the start of
	// the demonstration and correlates with nothing outside it. The same rule
	// `navsource.Subscription` applies to its own stamps, and the same reason.
	t0 := time.Now()
	deadline := t0.Add(time.Duration(s.seconds) * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-s.stop:
			res.ended = "you ended it"
		default:
		}
		if res.ended != "" {
			break
		}
		lk := fastobserve.Look{At: time.Now()}
		// WHAT ARRIVED ON THIS LOOK, and the screen that was standing when it
		// did. Kept here and written down below, once frontness is known.
		var arrived []observe.InputEvent
		var standing observe.VisualScreenID
		look := res.looks

		if navSub != nil {
			// THE NAVIGATION, AND THE EDGES OF IT, merged into one timeline.
			//
			// Two drains because they are two collections: an edge carries its
			// press's own intent and must never reach a correlator, which is why
			// `DrainEdges` is separate. `lk.Presses` is unchanged — only the
			// pointer positions fold into the look — and the whole batch is kept
			// as the moment it arrived on.
			nav := navSub.Drain()
			presses, k := splitNav(nav)
			lk.Presses = presses
			res.keys += k
			// THE SCREEN THAT WAS STANDING, read BEFORE this look is folded:
			// the inputs arrived while the previous screen was up, and
			// attributing them to the screen they may have just caused would
			// be this layer deciding the causality it exists to record.
			//
			// The moment itself is kept further down, once this look's
			// frontness is known — a press made while something covered the
			// target is a different fact from one made on the application.
			arrived, standing = observe.MergeInputs(nav, navSub.DrainEdges()),
				obs.Screen()
		}

		v := tracker.Acquire(ctx, s.app)
		if !v.State.OK() {
			res.lost = fmt.Sprintf("target lost (%s)", v.State)
			break
		}
		lk.Window = fastobserve.WindowRef{
			ID: v.Ref.ID, Generation: v.Ref.Generation, Bounds: v.Ref.Bounds,
		}
		if res.generations == nil {
			res.generations = map[uint64]bool{}
		}
		res.generations[v.Ref.Generation] = true
		if navs != nil && v.Ref.Bounds.Width > 0 && v.Ref.Bounds.Height > 0 {
			navs.SetWindowBounds(v.Ref.Bounds.X, v.Ref.Bounds.Y,
				v.Ref.Bounds.Width, v.Ref.Bounds.Height, time.Now())
		}

		whole := time.Now()
		shot := time.Now()
		img, err := shots.CaptureWindow(ctx, windowRefToWindow(v.Ref))
		res.stage.capture = append(res.stage.capture, msSince(shot))
		if err != nil {
			lk.Missing = true
		} else {
			lk.Front = frontnessOf(frontCandidate(ctx, windows), v.Ref) ==
				observe.FrontTarget
			prop := time.Now()
			out := structure.Propose(img.Image, opt)
			res.stage.propose = append(res.stage.propose, msSince(prop))
			// THE SAME NORMALISATION THE REPLAY USES. One statement of it, in
			// observe, so the live observer and the offline measurement are the
			// same arithmetic on the same proposals.
			rects := make([]image.Rectangle, 0, len(out.Candidates))
			for _, c := range out.Candidates {
				rects = append(rects, c.Bounds)
			}
			lk.Regions = observe.NormaliseRects(img.Image.Bounds(), rects)
			// AND WHAT THE WORDS SAY, offered to the reader and collected
			// whenever it has finished one. Non-blocking in both directions:
			// `offer` drops the frame when a reading is still running, which
			// is the ordinary case and the number that says how far behind
			// OCR is.
			if s.reader.working() {
				s.reader.offer(readingFrame{
					img: img.Image, look: res.looks,
					epoch:      obs.Epoch(),
					generation: v.Ref.Generation,
				})
				lk.Readings = s.reader.ready()
				res.readings += len(lk.Readings)
			}
		}
		res.looks++
		// THE MOMENT, with the frontness of the look the inputs arrived on.
		res.note(look, lk.At.Sub(t0), standing, lk.Front, arrived)
		if s.each != nil {
			s.each(lk)
		}
		// REACQUISITION BEFORE THE FOLD.
		if len(obs.Registry()) > 0 {
			pressed := map[observe.VisualTargetID]bool{}
			for _, t := range obs.Registry() {
				for _, w := range lk.Presses {
					if t.Screen == obs.Screen() && containsAny(t.Where, w) {
						pressed[t.ID] = true
					}
				}
			}
			res.found.fold(obs.Reacquire(lk), pressed)
		}
		obs.Observe(lk)
		res.stage.whole = append(res.stage.whole, msSince(whole))
		<-ticker.C
	}
	obs.Close()
	res.seen = <-done

	if navs != nil {
		res.in = navs.Stats()
	} else {
		res.in.Unavailable = navWhy
		if res.in.Unavailable == "" {
			res.in.Unavailable = "no mouse producer was started"
		}
	}
	return res, 0
}
