package main

// WHAT THE LOOP ADMITS AS A READING, AND WHAT IT SAYS WHEN IT ADMITS NOTHING.
//
// The owner's first live Act refused twice in a row on the screen it starts on. The shape agreed
// at 0.897 and 0.935; the words — which is what a granted run identifies a screen BY, because
// there is nobody to ask — never arrived, and both runs said "nothing has been read on it yet"
// for the four seconds they lasted. That sentence is true of a source that has not answered yet,
// of one whose answers keep arriving about a screen that has already changed, and of a look
// whose pixels were never the application's, and only the first of those is fixed by waiting.
//
// `admit` is the part that knows which. It is entered here through the production call, with
// nothing faked but the readings: `looker` is held by `next`, which cannot be stood up at all —
// a window tracker, a desktop capture and a window provider, all concrete.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// aLookWorthReading is a sighting that passes every term of `Usable`, so what a test changes is
// the readings and not whether the look was evidence.
func aLookWorthReading() workflowrun.Sighting {
	return workflowrun.Sighting{
		Screen: "visual_1", Epoch: 4, Generation: 2, Front: true,
		Bounds: directorapi.Rect{Width: 1920, Height: 1080},
	}
}

// readingOf is one reading of a look, stamped with the context it was computed in.
func readingOf(fromLook int, generation, epoch uint64, words ...string) observe.ScreenReading {
	rd := observe.ScreenReading{
		FromLook: fromLook, Generation: generation, Epoch: epoch,
		Text: observe.TextOf(directorapi.SourceOCR, words),
	}
	for _, w := range words {
		rd.Words = append(rd.Words, observe.ReadWord{Text: w})
	}
	return rd
}

// A reading taken under this observation is the reading, and nothing is said about an absence.
func TestAReadingOfThisObservationIsWhatTheScreenSays(t *testing.T) {
	l := &looker{saidLook: -1, ground: &promptGrounder{}}
	at := aLookWorthReading()

	l.admit([]observe.ScreenReading{
		readingOf(3, 2, 4, "profile", "sound", "video", "controls"),
	}, &at, 2, 4)

	if !l.ground.read.Said() {
		t.Fatal("a reading taken under this very observation was not admitted")
	}
	if l.ground.readWhy != "" {
		t.Errorf("a look that WAS read carries a reason it was not: %s", l.ground.readWhy)
	}
	if len(at.Words) == 0 {
		t.Error("the step was not given the words of the reading the screen was")
	}
}

// AND A READING OF AN OBSERVATION THAT HAS MOVED ON SAYS SO, rather than saying nothing has been
// read.
//
// Said two ways, because a screen that MOVED and a screen that KEEPS moving have opposite
// remedies. Two readings arriving late is the ordinary cost of a source three to six looks
// behind a transition, and waiting is the whole of it. Three in a row with none admitted between
// them is a reader that has not once caught the screen still, and that is the case no amount of
// patience reaches: a run told to keep waiting would wait out its whole budget.
func TestAReadingThatArrivesAboutAnOlderScreenSaysWaitingWillNotHelp(t *testing.T) {
	l := &looker{saidLook: -1, ground: &promptGrounder{}}
	at := aLookWorthReading()

	// TWO, ACROSS ONE TRANSITION. Not yet a claim about the screen.
	l.admit([]observe.ScreenReading{
		readingOf(3, 2, 3, "profile", "sound", "video"),
		readingOf(4, 2, 3, "profile", "sound", "video"),
	}, &at, 2, 4)

	if l.ground.read.Said() {
		t.Fatal("a reading of an observation that had moved on was admitted")
	}
	if !strings.Contains(l.ground.readWhy, "an earlier reading became outdated") {
		t.Errorf("two late readings do not say what is actually known: %s",
			l.ground.readWhy)
	}
	if strings.Contains(l.ground.readWhy, "will not change") {
		t.Errorf("two late readings are reported as a screen that keeps moving: %s",
			l.ground.readWhy)
	}

	// AND A THIRD, STILL WITH NONE ADMITTED. Now it is.
	l.admit([]observe.ScreenReading{
		readingOf(5, 2, 3, "profile", "sound", "video"),
	}, &at, 2, 4)

	for _, want := range []string{"3 reading(s)", "waiting longer will not change that"} {
		if !strings.Contains(l.ground.readWhy, want) {
			t.Errorf("the reason does not say %q: %s", want, l.ground.readWhy)
		}
	}
	if l.stale != 3 || l.staleRun != 3 {
		t.Errorf("%d stale reading(s) counted, %d in a row; want 3 and 3",
			l.stale, l.staleRun)
	}
}

// AND A LOOK WHOSE PIXELS WERE NOT THE APPLICATION'S IS NOT A LOOK NOBODY HAS READ YET.
//
// The one of these a person can fix in a click, so it is the one that gets said — and the
// readings are not consulted at all, because pixels that may belong to whatever is covering the
// window are not evidence about the window.
func TestALookThatWasNotEvidenceSaysThatAndNotThatNothingWasRead(t *testing.T) {
	l := &looker{saidLook: -1, ground: &promptGrounder{}}
	at := aLookWorthReading()
	at.Front = false

	l.admit([]observe.ScreenReading{
		readingOf(3, 2, 4, "profile", "sound", "video"),
	}, &at, 2, 4)

	if l.ground.read.Said() {
		t.Fatal("a reading of pixels that were not the application's was admitted")
	}
	if !strings.Contains(l.ground.readWhy, "not the window in front") {
		t.Errorf("the reason does not say the pixels were not evidence: %s", l.ground.readWhy)
	}
	if strings.Contains(l.ground.readWhy, "nothing has been read on it yet") {
		t.Errorf("a look that was not evidence is reported as one not read yet: %s", l.ground.readWhy)
	}
	if len(at.Words) != 0 {
		t.Error("a step was given words read off pixels that were not the window's")
	}
}

// AND NO READINGS AT ALL IS THE PLAIN CASE, which waiting does fix.
func TestNoReadingYetSaysNothingMoreThanThat(t *testing.T) {
	l := &looker{saidLook: -1, ground: &promptGrounder{}}
	at := aLookWorthReading()

	l.admit(nil, &at, 2, 4)

	if l.ground.read.Said() {
		t.Fatal("something was read out of no readings")
	}
	if l.ground.readWhy != "nothing has been read on it yet" {
		t.Errorf("the reason for no readings at all is %q", l.ground.readWhy)
	}
}

// AND A READING THIS LOOP WAS HOLDING IS DROPPED WHEN THE OBSERVATION MOVES, and dropping it
// counts as the same fact an arriving stale reading is.
//
// The held reading is the whole reason the slow source works at all: OCR answers twice a second
// against ten looks, so the last reading stands for the looks after it. What makes that safe is
// that it stops standing the moment a screen change is recorded — and a run whose readings keep
// being dropped that way is in the same situation as one whose readings keep arriving late.
func TestAHeldReadingStopsStandingWhenTheScreenChanges(t *testing.T) {
	l := &looker{saidLook: -1, ground: &promptGrounder{}}
	at := aLookWorthReading()

	l.admit([]observe.ScreenReading{
		readingOf(3, 2, 4, "profile", "sound", "video"),
	}, &at, 2, 4)
	if !l.ground.read.Said() {
		t.Fatal("the reading was not admitted in the observation it was taken in")
	}
	// THE SAME LOOKER, ONE OBSERVATION LATER, with no new reading.
	next := aLookWorthReading()
	next.Epoch = 5
	l.admit(nil, &next, 2, 5)

	if l.ground.read.Said() {
		t.Fatal("a reading of the previous observation still stood after it moved")
	}
	if !strings.Contains(l.ground.readWhy, "an earlier reading became outdated") {
		t.Errorf("dropping one held reading is not reported as one reading going "+
			"out of date: %s", l.ground.readWhy)
	}
	if strings.Contains(l.ground.readWhy, "keeps moving") {
		t.Errorf("one transition is reported as a screen that keeps moving: %s",
			l.ground.readWhy)
	}
	if len(next.Words) != 0 {
		t.Error("the step was given words from an observation that had moved on")
	}
}

// A TRANSITION, THEN A SCREEN THAT SITS STILL, THEN A LATE READING THAT LANDS.
//
// The sequence the whole diagnosis has to get right, and the one the first version got wrong. A
// screen changes; whatever reading was being held goes out of date; the screen then holds
// perfectly still for a second while OCR works. Every look in that second found no current
// reading, and the first version counted the SAME discarded reading again on each of them — ten
// a second — and then reported the total as proof that the screen kept moving and that waiting
// would not help. Waiting was the entire remedy, and it worked.
func TestAScreenThatMovedOnceAndThenHeldStillIsNotAScreenThatKeepsMoving(t *testing.T) {
	l := &looker{saidLook: -1, ground: &promptGrounder{}}

	// ── 1. A READING OF WHERE WE WERE ──
	was := aLookWorthReading()
	l.admit([]observe.ScreenReading{
		readingOf(3, 2, 4, "play", "create", "campaign"),
	}, &was, 2, 4)
	if !l.ground.read.Said() {
		t.Fatal("the reading of the screen we started on was not admitted")
	}

	// ── 2. THE SCREEN CHANGES, AND NOTHING HAS BEEN READ OF THE NEW ONE ──
	//
	// Thirteen looks of it: one where the held reading is dropped, and twelve more
	// where there is simply no reading yet. OCR is working on a frame from look 14.
	for i := 0; i < 13; i++ {
		at := aLookWorthReading()
		at.Epoch = 5
		l.admit(nil, &at, 2, 5)
		if l.ground.read.Said() {
			t.Fatalf("look %d: a reading of the previous screen still stood", i)
		}
		if strings.Contains(l.ground.readWhy, "keeps moving") {
			t.Fatalf("look %d of a stationary screen is reported as a screen "+
				"that keeps moving: %s", i, l.ground.readWhy)
		}
		if !strings.Contains(l.ground.readWhy, "an earlier reading became outdated") {
			t.Fatalf("look %d does not say what is actually known: %s",
				i, l.ground.readWhy)
		}
	}
	// AND ONE DISCARDED READING IS ONE, not one per look that followed it.
	if l.stale != 1 || l.staleRun != 1 {
		t.Errorf("thirteen looks after one transition counted %d discarded "+
			"reading(s), %d in a row; want 1 and 1", l.stale, l.staleRun)
	}

	// ── 3. AND THE LATE READING ARRIVES, AND IS THE SCREEN ──
	now := aLookWorthReading()
	now.Epoch = 5
	l.admit([]observe.ScreenReading{
		readingOf(14, 2, 5, "profile", "sound", "video", "controls"),
	}, &now, 2, 5)

	if !l.ground.read.Said() {
		t.Fatal("the reading that arrived about the screen in front of us was not " +
			"admitted, so waiting would never have worked")
	}
	if l.ground.readWhy != "" {
		t.Errorf("a look that WAS read carries a reason it was not: %s",
			l.ground.readWhy)
	}
	if len(now.Words) == 0 {
		t.Error("the step was not given the words of the reading that landed")
	}
	// AND THE RUN OF STALENESS IS OVER, because the screen demonstrably held still
	// long enough to be read. A later transition starts from one again, not from two.
	if l.staleRun != 0 {
		t.Errorf("a reading was admitted and %d discard(s) are still counted in a "+
			"row", l.staleRun)
	}
}

// AND A SCREEN THAT NEVER HOLDS STILL IS SAID TO BE THAT, once there is evidence of it: three
// readings in a row, each about an observation that had already moved on, with none admitted
// between them. This is the case patience cannot reach, and the only one where waiting is the
// wrong thing to tell somebody.
func TestAScreenThatNeverHoldsStillSaysSoOnceItHasBeenSeenToThreeTimes(t *testing.T) {
	l := &looker{saidLook: -1, ground: &promptGrounder{}}

	for i, want := range []string{
		"an earlier reading became outdated",
		"an earlier reading became outdated",
		"keeps moving",
	} {
		at := aLookWorthReading()
		at.Epoch = uint64(10 + i)
		// EACH READING IS OF THE LOOK BEFORE IT, which is what a source three to
		// six looks behind a moving screen produces every time.
		l.admit([]observe.ScreenReading{
			readingOf(i, 2, uint64(9+i), "profile", "sound", "video"),
		}, &at, 2, uint64(10+i))

		if l.ground.read.Said() {
			t.Fatalf("reading %d was admitted about an observation that had "+
				"moved on", i)
		}
		if !strings.Contains(l.ground.readWhy, want) {
			t.Errorf("reading %d says %q, which does not mention %q",
				i, l.ground.readWhy, want)
		}
	}
	if l.staleRun != 3 {
		t.Errorf("%d discard(s) counted in a row, want 3", l.staleRun)
	}
}
