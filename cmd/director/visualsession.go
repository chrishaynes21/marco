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
	"github.com/chaynes-simpleclouds/marco/internal/director/preview"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
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
	// told, when set, is called after every look has been FOLDED, with how many seconds
	// of the ceiling remain.
	//
	// The opposite ordering from `each`, for the opposite reason. `each` must not see
	// what a look taught, because it measures a saved routine against evidence the
	// observer has never seen. This must: what it reports is whether the screen has been
	// read and whether a press has an outcome yet, and neither of those exists until the
	// look is in. See demonstratevisual.go, where it becomes a learnstate stream.
	told func(left int)
	// pictures says to keep one thumbnail per screen and one frame per press, so a
	// person reviewing the demonstration can SEE what it will press and what it expects.
	//
	// Off unless somebody asked. `demonstrate-visual` prints "No frames and no
	// screenshots are kept", and keeping crops of a desktop is a change to what a
	// recording does with it — so it is opt-in at the command and the surface that asks
	// for it says what is kept. See internal/director/preview.
	pictures bool
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
	// window is the window every look of this demonstration was taken in, and resized
	// says it was not ONE window.
	//
	// Kept because a demonstration that produces a replayable coordinate has to be able
	// to say what the coordinate was relative to. Every reading in the artifact is
	// normalised to its window, so the arithmetic produces a confident-looking point at
	// any window size whatever — and the only thing that makes the demonstrated point and
	// the pressed point the same point is that the window has not changed.
	//
	// Kept OUTSIDE the `pictures` licence, because a size is not a picture of anybody's
	// screen. A demonstration recorded without previews still records what it was
	// demonstrated in.
	//
	// `resized` is a refusal rather than a choice between two sizes: somebody who resized
	// the window mid-demonstration did not demonstrate a fixed layout, and picking the
	// first or the last would be this layer deciding which half of that they meant.
	window  directorapi.Rect
	resized bool
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
	// screens is one thumbnail per screen the session saw, and pressed one full frame
	// per look that carried a press. Empty unless `pictures` was asked for.
	//
	// # Why a thumbnail for a screen and a whole frame for a press
	//
	// Because a screen is looked at whole and a control is cropped out of one. A frame is
	// 1920x1080 of RGBA — eight megabytes — so keeping one per screen would be a hundred
	// megabytes for a long demonstration, and a thumbnail is a few hundred kilobytes.
	// A press needs full resolution because the crop comes out of it, and there are a
	// handful of presses in a demonstration rather than a handful per second.
	//
	// Both are bounded: `previewScreens` and `previewPresses`. A recording that ran for
	// five minutes over thirty screens keeps the first dozen and says so by having no
	// picture for the rest, which reads as a missing preview rather than as a lie.
	screens map[observe.VisualScreenID]image.Image
	pressed map[int]image.Image
	// evidence is the looks kept to calibrate each pressed control, by the look its
	// press arrived on — the same key `pressed` uses, and the one `keepPreviews`
	// already resolves a control to.
	//
	// A band of grey levels around the control rather than a frame: the calibration
	// never looks outside it, and a frame is eight megabytes against a couple of
	// hundred kilobytes. Measured at the END of the session; see visualevidence.go
	// for why it cannot be measured in this loop.
	evidence map[int]*pressEvidence
	// recent is the last few looks, as grey levels, so that the looks BEFORE a press
	// are available at the moment the press arrives. Bounded and constant: the ring
	// is `evidenceLooks` frames whatever happens, and what each press takes out of it
	// is a band around its own control. See visualevidence.go.
	recent []recentLook
	// application is the executable image name of the window actually watched.
	//
	// OBSERVED, never the selector the person typed: a selector is what somebody asked
	// to watch and the image is what they watched, and the one time those differ is the
	// one time it matters. It is what the saved workflow is scoped to — see
	// workflowstore.Scope.
	application string
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

// previewScreens and previewPresses bound what a session holds in memory for review.
//
// Twelve screens at a few hundred kilobytes each, and eight full frames at eight megabytes:
// about seventy megabytes at the very worst, for the length of one recording, freed as soon as
// it is saved. A demonstration that visits more than twelve screens or makes more than eight
// presses is not one somebody is going to review a picture at a time.
const (
	previewScreens = 12
	previewPresses = 8
)

// keep holds a picture of this look, if it is one worth holding.
//
// The FIRST thumbnail of each screen rather than the last: the first settled look of a screen is
// the screen as it was arrived at, and the last is whatever it looked like as the person left
// it — half-way through the next transition, which is the picture nobody wants.
func (r *visualResult) keep(screen observe.VisualScreenID, look int, frame image.Image,
	press bool) {

	if frame == nil {
		return
	}
	if press && len(r.pressed) < previewPresses {
		if r.pressed == nil {
			r.pressed = map[int]image.Image{}
		}
		// THE WHOLE FRAME, because the crop of the control comes out of it and
		// the control is a few hundred pixels across.
		r.pressed[look] = frame
	}
	if screen == "" {
		return
	}
	if _, had := r.screens[screen]; had || len(r.screens) >= previewScreens {
		return
	}
	// SHRUNK ON THE WAY IN, so what is held is a thumbnail and not a desktop.
	thumb, err := preview.Thumbnail(frame)
	if err != nil {
		return
	}
	if r.screens == nil {
		r.screens = map[observe.VisualScreenID]image.Image{}
	}
	r.screens[screen] = thumb
}

// fold folds one look into the observer and files everything this look is remembered by under
// the number the observer gives back.
//
// ── ONE LOOK NUMBER, AND IT IS THE OBSERVER'S ──
//
// Everything a demonstration keeps per look — the raw moment, the picture of the screen, the
// band of pixels a control will be calibrated from — is read back later by matching it against
// the look the OBSERVER says a press arrived on, because the observer is the only thing that
// attributes a press to a control at all. So there is one counter, the observer keeps it, and it
// is taken from the fold that advanced it.
//
// This used to be a second counter the loop kept beside it, read BEFORE the fold, so it ran one
// behind: the observer counts from one and the loop counted from zero. Nothing reported a
// disagreement, because both numbers were plausible and each half worked perfectly against
// itself — the pictures were filed, the evidence was measured, and then every lookup between
// them missed by one and returned nothing. Across three of the owner's recordings that meant NO
// control crop and NO control evidence was ever written, with not a word said about it, while
// the raw record carried the same press as moment 59 and press 60.
//
// It is a function rather than four lines in the loop so that the numbering can be DRIVEN by a
// test. The loop itself cannot be: it builds its own window provider, capture path and
// navigation hook, so what a test could reach before was two hand-numbered halves — which is
// exactly the fixture that agreed with itself and missed this.
//
// Returns the look number, for a caller that wants to say which look something happened on.
func (r *visualResult) fold(obs *fastobserve.Observer, lk fastobserve.Look, frame image.Image,
	since time.Duration, standing observe.VisualScreenID, arrived []observe.InputEvent,
	pictures bool) int {

	up := obs.Observe(lk)
	look := up.Look
	r.looks = look
	// AND THE WINDOW THIS LOOK WAS TAKEN IN. Before the pictures licence, because a
	// size is not a picture; and only for a look that HAS a window, so a failed
	// capture does not read as a resize.
	if w := lk.Window.Bounds; w.Width > 0 && w.Height > 0 {
		switch {
		case r.window.Width == 0:
			r.window = w
		case r.window.Width != w.Width || r.window.Height != w.Height:
			r.resized = true
		}
	}
	// THE MOMENT, with the frontness of the look the inputs arrived on, and the screen
	// that was standing when they did.
	r.note(look, since, standing, lk.Front, arrived)
	if pictures && !lk.Missing && lk.Front {
		// AFTER THE FOLD, so the screen this frame is a picture OF is the screen
		// the observer says is standing. Before it, the frame belongs to whatever
		// was standing a look ago, and a thumbnail filed under the wrong screen is
		// worse than none: it is a picture that disagrees with the words beside it.
		//
		// And only when the window was in front. A capture of a covered window
		// returns the covering window's pixels under the target's name, which would
		// put somebody else's screen in this person's store. See the memory note of
		// that name.
		r.keep(obs.Screen(), look, frame, len(lk.Presses) > 0)
		// AND THE EVIDENCE FOR A GUARD, which needs the looks BEFORE a press as
		// much as the press itself: a control seen once has never been seen to STAY
		// the same, and what happens after a press is the outcome's question. Kept
		// here and measured at the end — see visualevidence.go.
		r.watchPressed(look, frame, lk.Presses, lk.Regions)
	}
	return look
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

	// ANNOUNCED BEFORE THE HOOKS GO ON, or not started. See recordingnow.go.
	clearRecording, announced := announceRecording(errOut, "a demonstration in "+s.app)
	if !announced {
		return res, 1
	}
	defer clearRecording()

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
		// AND WHAT THE WINDOW LAYER SAYS THIS APPLICATION IS, which is what the
		// saved workflow will be scoped to.
		res.application = v.Ref.Application
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
				// THE LOOK THIS FRAME IS ABOUT TO BE FOLDED AS. One more
				// than the last one the observer counted, because the
				// fold has not happened yet — and in the observer's
				// numbering, which is the only one this loop uses.
				s.reader.offer(readingFrame{
					img: img.Image, look: res.looks + 1,
					epoch:      obs.Epoch(),
					generation: v.Ref.Generation,
				})
				lk.Readings = s.reader.ready()
				res.readings += len(lk.Readings)
			}
		}
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
		res.fold(obs, lk, img.Image, lk.At.Sub(t0), standing, arrived, s.pictures)
		if s.told != nil {
			// WHAT THE RECORDER NOW KNOWS. Called with the look in, and on
			// every look: the thing on the other end emits only on CHANGE,
			// which is where that decision belongs — this loop shares a thread
			// with nothing and the writer shares its cadence with a hook.
			left := int(time.Until(deadline).Seconds())
			if left < 0 {
				left = 0
			}
			s.told(left)
		}
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
