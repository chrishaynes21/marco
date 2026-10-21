package main

// THE OBSERVATION LOOP, IN ONE PLACE, WITH MORE THAN ONE CALLER.
//
// # Why it moved out of `runPerformWorkflow`
//
// Because a second caller arrived and there must not be a second loop. `director
// perform-workflow` walks a saved artifact from the front, with `workflowrun.Runner`'s cursor as
// the order. A demonstrated PLAY takes its order from its own sentences instead
// ([[ADR-243-a-door-of-its-own-and-the-order-in-the-file]]), so a host answering one sentence has
// to drive looks until that one interaction has an answer — the same looks, built the same way,
// handed to the same `Advance`.
//
// Copying the body would have been the worst available outcome: two places that build a
// `Sighting`, drifting apart, with the safety-critical ordering in each. This file is the body,
// unchanged, with the state it carried between iterations turned into fields.
//
// # What the ordering means, because it is the whole of the care here
//
//	acquire          the window, every look — a target that went away ends the run
//	capture          no pixels is MISSING, which `worldMoved` refuses, not "unchanged"
//	propose          the regions of THIS look
//	offer/drain      the slow sources, non-blocking, carrying the stamps that date them
//	reacquire        BEFORE the fold, so nothing this look teaches helps recognise it
//	observe          the fold, which is what mints or merges a screen identity
//	arrange          the arrangement of this LOOK, not of the screen standing
//	admit readings   only while `observe.Current` holds of their stamps
//
// Each line of that is somebody's measurement and several are ADRs. Moving it verbatim is the
// only safe way to move it, so that is what was done: the comments travelled with the code they
// are about.
//
// # It performs nothing
//
// A looker looks. It cannot press anything, cannot ask anybody, and has no executor — what it
// returns is one `workflowrun.Sighting`, and the coordinator decides what to do with it.

import (
	"context"
	"fmt"
	"image"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/structure"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/platform/wincapture"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
)

// looker builds one observation per call, and keeps what a run carries between them.
type looker struct {
	// ── what it looks with ──
	tracker *windowref.Tracker
	target  string
	shots   *wincapture.Capture
	windows *winprovider.Provider
	obs     *fastobserve.Observer
	opt     structure.Options
	// eyes and reader are the slow sources, each on its own clock. Either may be nil,
	// which is the ordinary case: both are opt-in flags.
	eyes   *learnedEyes
	reader *readingEyes
	// probe is handed the frame a plan was made from, so it can compare the control
	// against itself a quarter of a second later.
	probe *hoverProbe
	// repairs is told what this look found, because it cannot look for itself. Nil when
	// nothing may be re-grounded.
	repairs *regrounder
	// ground is told this look's arrangement and what was read, for the same reason.
	ground *promptGrounder
	// art and session are what reacquisition searches with.
	art     workflowstore.Artifact
	session *workflowstore.SessionBinding

	// ── what it carries between looks ──
	look int
	// blind is how many looks obtained no pixels, and blindWhy the first reason.
	//
	// Kept because discarding it turned a capture that failed 410 times out of 417 into
	// "no pixels were obtained", which names the symptom and not one cause.
	blind    int
	blindWhy string
	// said is what was last READ, with the context that keeps it current.
	//
	// Held across looks because OCR answers far less often than the loop looks, and
	// dropped the moment `observe.Current` stops being true of it — which is the moment a
	// screen change is recorded, not the moment a different screen is standing.
	said               []observe.ReadWord
	saidGen, saidEpoch uint64
	// saidSeq is WHICH reading `said` is, and saidLook the look it describes.
	//
	// OCR answers about twice a second against a loop that looks ten times, so the last
	// reading stands for the looks that follow it — and a run once counted that one
	// reading once per look. Measured live: a press whose panel had not arrived yet was
	// contradicted 572ms later on "5 fresh reading(s)", which were five copies of the
	// single reading taken before the press. A verdict needs five readings, not one
	// reading five times.
	saidSeq, saidLook int
	// saidText is what it said AS A VOCABULARY, which is a different question from where
	// each word was. `said` carries positions and is what a control is found by; this is
	// what a SCREEN is recognised by, and the destination test needs the second. Kept on
	// the same stamps so one admission rule covers both.
	saidText observe.ScreenText
	// held says a reading is standing, so that DROPPING it is counted once.
	//
	// Without it the drop was counted again on every look after it: the dropped reading
	// keeps its stamps, so `observe.Current` goes on being false of it, and a stationary
	// screen waiting for OCR accumulated a stale reading ten times a second. The number
	// that came out of that said the screen was moving when nothing was.
	held bool
	// stale is how many DISTINCT readings this run has discarded because the observation
	// had moved on, and staleRun how many it has discarded in a row with none admitted
	// in between.
	//
	// Two numbers because they answer different questions and only the second one is
	// about now. One stale reading is the ordinary cost of a source that answers half a
	// second behind a loop that looks ten times: a single transition discards whatever
	// was being held and the screen then sits still. A screen that keeps changing UNDER
	// the reader discards every reading it will ever take, and that is the one no amount
	// of patience reaches. A total cannot tell them apart, because the first one leaves
	// its total standing for the rest of the run. Any admitted reading resets the run:
	// the screen demonstrably held still long enough to be read once. See whyNoReading.
	stale, staleRun int
	// lost is why looking stopped, empty while it has not.
	lost string
	// within is whether looking may continue AT ALL, and why not when it may not.
	//
	// # Why the clock lives here
	//
	// `--seconds` is the run's advertised limit, and it used to be the cursor loop's own
	// `for time.Now().Before(deadline)` — which the play driver never saw. A play could
	// therefore keep observing, and keep asking for actions, past the time a person was
	// told the run would take. An observation count is not a wall clock: it bounds how
	// many looks one interaction may consume and says nothing about how long they take.
	//
	// So the bound sits where the looks are produced, and both drivers inherit it. Nil
	// means unbounded, which no production caller passes.
	within func() (bool, string)
}

// newLooker wires a loop over a target. `saidLook` starts at -1: no reading has been counted.
//
// `within` is the wall clock, and it is not optional for a production caller: see the field.
func newLooker(tracker *windowref.Tracker, target string, shots *wincapture.Capture,
	windows *winprovider.Provider, obs *fastobserve.Observer, opt structure.Options,
	eyes *learnedEyes, reader *readingEyes, probe *hoverProbe, repairs *regrounder,
	ground *promptGrounder, art workflowstore.Artifact,
	session *workflowstore.SessionBinding, within func() (bool, string)) *looker {

	return &looker{
		tracker: tracker, target: target, shots: shots, windows: windows, obs: obs,
		opt: opt, eyes: eyes, reader: reader, probe: probe, repairs: repairs,
		ground: ground, art: art, session: session, within: within, saidLook: -1,
	}
}

// admit takes this look’s readings and answers with the one that is current, or with why
// none is — which is the sentence a person watching a paused run reads.
//
// A method rather than part of `next` because `next` cannot be stood up in a test: it holds a
// window tracker, a desktop capture and a window provider, all concrete. This holds the part
// that DECIDES, and a test enters it through the production call with nothing faked but the
// readings themselves.
//
// It writes to the sighting AND to the grounder rather than returning, because the words a step
// may use and the reason a screen could not be identified are the same admission asked twice,
// and a caller that assigned one of the two was a caller that could drop the other. The
// grounder is the only thing that has to say this to a person: see promptGrounder.readWhy.
func (l *looker) admit(readings []observe.ScreenReading, at *workflowrun.Sighting,
	generation, epoch uint64) {

	// WHETHER THESE PIXELS WERE EVIDENCE AT ALL, asked once.
	//
	// This used to be inside the loop below, `break`ing on the same answer every time
	// around: the sighting does not change while its readings are drained. What is
	// new is that the answer is KEPT — a look whose pixels were not evidence about the
	// application is not a look that has not been read yet, and the grounder’s pause
	// sentence said the second about the first for as long as the run lasted.
	why := ""
	if ok, unusable := at.Usable(); !ok {
		why = unusable
	}
	wordless := 0
	for _, rd := range readings {
		if why != "" {
			break
		}
		if !rd.Admissible(generation, epoch) {
			l.stale++
			l.staleRun++
			continue
		}
		if len(rd.Words) == 0 {
			wordless++
			continue
		}
		if rd.FromLook != l.saidLook {
			l.saidSeq++
			l.saidLook = rd.FromLook
		}
		l.said, l.saidGen, l.saidEpoch = rd.Words, rd.Generation, rd.Epoch
		l.saidText = rd.Text
		// AND THE SCREEN HELD STILL LONG ENOUGH TO BE READ ONCE, which is the
		// fact that ends a run of staleness. Whatever came before it was a
		// screen that moved and then stopped, not a screen that keeps moving.
		l.held, l.staleRun = true, 0
	}
	if observe.Current(l.saidGen, l.saidEpoch, generation, epoch) {
		at.Words, at.WordsSeq = l.said, l.saidSeq
	} else {
		if l.held {
			// A READING WE WERE HOLDING, DROPPED. The screen changed since it
			// was taken, which is the same fact as a reading arriving stale
			// and is counted with it — ONCE, which is what `held` is for.
			l.stale++
			l.staleRun++
			l.held = false
		}
		l.said, l.saidText = nil, observe.ScreenText{}
	}
	if !l.saidText.Said() {
		why = whyNoReading(why, wordless, l.stale, l.staleRun)
	}
	l.ground.read, l.ground.readWhy = l.saidText, why
}

// whyNoReading is the reason there is no current reading of the look in front of us.
//
// A pure function because it is the part that can be WRONG, and the loop it is called from
// cannot be stood up in a test: it holds a window tracker, a desktop capture and a provider. One
// sentence per situation a missing reading can be in, and a test that walks every one.
//
// `unusable` is why this look was not evidence at all, empty when it was; `wordless` how many
// admissible readings of it came back with no words on them; `stale` how many DISTINCT readings
// this run has discarded as out of date; and `staleRun` how many it has discarded in a row with
// none admitted in between.
//
// The order is the order of what a person can do about it. A look that was not evidence is the
// thing to say first and the only one of these they can fix in a click. Words that came back
// empty is the screen, now.
//
// # Why staleness is said two ways, and why the stronger one needs a run
//
// Because a total cannot tell a screen that MOVED from a screen that KEEPS moving, and the two
// have opposite remedies. A single transition discards whatever reading was being held and the
// screen then sits still waiting for the next one — ordinary, resolves by waiting, and leaves
// its total standing for the rest of the run. Telling that person their screen keeps moving and
// that waiting will not help would send them to hold still while the only thing wrong was that
// OCR had not answered yet. So the stronger sentence waits for `staleSustained` discards with
// nothing admitted between them, and anything short of that says what is actually known: a
// reading went out of date and the current screen is being read.
func whyNoReading(unusable string, wordless, stale, staleRun int) string {
	switch {
	case unusable != "":
		return unusable
	case wordless > 0:
		return "reading the words on it found none, which is what an empty or " +
			"still-drawing screen looks like from here"
	case staleRun >= staleSustained:
		return fmt.Sprintf("%d reading(s) in a row have come back about a screen "+
			"that had already changed — a screen that keeps moving cannot be "+
			"read, and waiting longer will not change that", staleRun)
	case stale > 0:
		return "an earlier reading became outdated, and the screen in front of you " +
			"is being read now"
	}
	return "nothing has been read on it yet"
}

// staleSustained is how many readings must be discarded IN A ROW, with none admitted between
// them, before a run says the screen keeps moving and waiting will not help.
//
// Three. One is a transition: whatever was held when the screen changed. Two is a transition
// whose next reading was still in flight across it, which Experiment-043 measured as the
// ordinary cost of a source three to six looks behind. Three in a row with nothing admitted is
// a reader that has not once caught the screen still, and that is the claim the sentence makes.
const staleSustained = 3

// next is one look, or false when there is nothing more to look at.
//
// False means the run is over for a reason `lost` records: the context ended, or the window went
// away. It is NOT "nothing happened" — a look that obtained no pixels still returns a sighting,
// marked missing, because a look nobody could take is evidence that was not obtained rather than
// evidence of no change.
func (l *looker) next(ctx context.Context) (workflowrun.Sighting, bool) {
	if ctx.Err() != nil {
		return workflowrun.Sighting{}, false
	}
	if l.within != nil {
		if ok, why := l.within(); !ok {
			l.lost = why
			return workflowrun.Sighting{}, false
		}
	}
	v := l.tracker.Acquire(ctx, l.target)
	if !v.State.OK() {
		l.lost = fmt.Sprintf("the target was lost (%s)", v.State)
		return workflowrun.Sighting{}, false
	}
	lk := fastobserve.Look{
		At: time.Now(),
		Window: fastobserve.WindowRef{
			ID: v.Ref.ID, Generation: v.Ref.Generation, Bounds: v.Ref.Bounds,
		},
	}
	img, cerr := l.shots.CaptureWindow(ctx, windowRefToWindow(v.Ref))
	if cerr != nil {
		// WHY THE PIXELS DID NOT ARRIVE. Discarding this turned a capture
		// that failed 410 times out of 417 into "no pixels were obtained",
		// which names the symptom and not one cause.
		lk.Missing = true
		l.blind++
		if l.blindWhy == "" {
			l.blindWhy = cerr.Error()
		}
	} else {
		// THE FRAME THE PLAN IS MADE FROM, kept so the hover probe can
		// compare the control against itself a quarter of a second later.
		l.probe.before = img.Image
		lk.Front = frontnessOf(frontCandidate(ctx, l.windows), v.Ref) ==
			observe.FrontTarget
		res := structure.Propose(img.Image, l.opt)
		rects := make([]image.Rectangle, 0, len(res.Candidates))
		for _, c := range res.Candidates {
			rects = append(rects, c.Bounds)
		}
		lk.Regions = observe.NormaliseRects(img.Image.Bounds(), rects)

		// THE SLOW SOURCE. Offering is non-blocking and the queue holds
		// one frame, so a busy detector costs detections and never
		// costs cadence; the candidates it returns describe a frame
		// about ten looks old and carry the epoch and generation that
		// says so. See [[ADR-184-two-speeds-of-looking]].
		if l.eyes.working() {
			l.eyes.offer(learnedFrame{
				img: img.Image, look: l.look,
				epoch:      l.obs.Epoch(),
				generation: v.Ref.Generation,
			})
			lk.Slow = l.eyes.ready()
		}

		// THE SLOW TEXT SOURCE, offered and drained exactly as the detector
		// is, and for the same reason: OCR costs hundreds of milliseconds
		// against a cadence of a hundred, so what comes back describes a
		// frame several looks old and carries the window generation and
		// observation epoch that say which. The observer refuses one whose
		// context has moved -- see fastobserve.intakeReadings -- and an
		// admitted one may do exactly one thing: refuse to believe the
		// standing screen is the screen the arrangement says.
		if l.reader.working() {
			l.reader.offer(readingFrame{
				img: img.Image, look: l.look,
				epoch:      l.obs.Epoch(),
				generation: v.Ref.Generation,
			})
			lk.Readings = l.reader.ready()
		}
	}
	l.look++
	// RE-ACQUISITION BEFORE THE FOLD, so nothing this look teaches can help
	// recognise what this look was meant to recognise.
	outcomes := reacquireStored(l.art, l.session, l.obs, lk)
	up := l.obs.Observe(lk)

	at := workflowrun.Sighting{
		Screen: up.Screen, Epoch: up.Epoch, Generation: v.Ref.Generation,
		Window: v.Ref.ID, Bounds: v.Ref.Bounds,
		Front: lk.Front, Missing: lk.Missing, Unobserved: up.Unobserved,
		Outcomes: outcomes, At: lk.At,
	}
	// THE ARRANGEMENT OF THIS LOOK, not of the screen standing.
	//
	// `obs.Arrangement(id)` is a signature fixed when that screen was minted and
	// never updated, so standing on a page that merged onto an older identity it
	// answers with the OLDER PAGE's arrangement -- which is what
	// [[Experiment-048-the-destination-a-workflow-can-see-for-itself]] measured
	// the destination being compared against for sixty observations. The regions
	// of the look in front of us are right here, and the saved Training page
	// agrees with a Training look at 0.600 and with a Play look at 0.448.
	l.ground.arrangement = observe.ArrangeLook(lk.Regions)

	// WHAT THE SOURCES FOUND ON THIS LOOK, handed over because neither the
	// repair nor the coordinator can look for itself. Both sources, and only
	// regions that locate something: a band is never a candidate for
	// replacing a band.
	at.Candidates = append(append([]observe.Region{}, lk.Regions...),
		fresh(lk.Slow, l.obs.Epoch(), v.Ref.Generation)...)
	if l.repairs != nil {
		l.repairs.candidates = at.Candidates
	}

	// AND WHAT WAS READ ON IT, WHICH IS OLDER THAN IT AND STILL CURRENT.
	//
	// OCR answers about twice a second against a loop that looks ten times, so
	// most looks have no reading of their own and the last one stands. What
	// makes that safe is the same rule the readings themselves go through:
	// `observe.Current` is true only while NO SCREEN CHANGE has happened since
	// the frame was captured, which is a demonstrably continuous interval and
	// not the weaker "the same screen is standing".
	// See [[ADR-195-a-reading-that-arrives-after-the-look-it-describes]].
	//
	// The sighting is handed over so the words a STEP may use are admitted by the
	// same rule, and the reason there is none goes to the grounder, which is the
	// thing that has to say it to somebody. See admit.
	l.admit(lk.Readings, &at, v.Ref.Generation, up.Epoch)

	return at, true
}
