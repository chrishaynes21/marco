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
// read. This is the case no amount of patience reaches: a screen that keeps changing under a
// source that takes half a second invalidates every reading it will ever take, and a run told to
// keep waiting would wait out its whole budget.
func TestAReadingThatArrivesAboutAnOlderScreenSaysWaitingWillNotHelp(t *testing.T) {
	l := &looker{saidLook: -1, ground: &promptGrounder{}}
	at := aLookWorthReading()

	l.admit([]observe.ScreenReading{
		readingOf(3, 2, 3, "profile", "sound", "video"),
		readingOf(4, 2, 3, "profile", "sound", "video"),
	}, &at, 2, 4)

	if l.ground.read.Said() {
		t.Fatal("a reading of an observation that had moved on was admitted")
	}
	for _, want := range []string{"2 reading(s)", "waiting longer will not change that"} {
		if !strings.Contains(l.ground.readWhy, want) {
			t.Errorf("the reason does not say %q: %s", want, l.ground.readWhy)
		}
	}
	if l.stale != 2 {
		t.Errorf("%d stale reading(s) counted, want 2", l.stale)
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
	if !strings.Contains(l.ground.readWhy, "keeps moving") {
		t.Errorf("dropping a held reading is not reported as the screen moving: %s", l.ground.readWhy)
	}
	if len(next.Words) != 0 {
		t.Error("the step was given words from an observation that had moved on")
	}
}
