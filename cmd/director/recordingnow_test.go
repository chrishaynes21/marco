package main

// WHAT THE RECORDER SAYS IT HAS, WHICH IS THE THING A PERSON ACTS ON.
//
// The window shows this ten times a second while somebody is demonstrating, and it is the only
// account they get of whether the demonstration is worth anything. Three things in it were
// read off the correlator wrongly, and each of them told a person to do something that could
// not work:
//
//	watched counted outcome WATCHES        a watch is opened for a blind press too, so
//	                                      `watched` equalled `presses` always, `presses -
//	                                      watched` was always zero, and the Blind phase was
//	                                      unreachable: a person clicking BESIDE a control was
//	                                      told to wait for a result that could never arrive
//	the starting screen came from Walk[0]  which is where the SESSION began, not where an Act
//	                                      built from it would start
//	and it asked ScreenText.Read          "did anybody look", which a blank frame satisfies,
//	                                      rather than the term the RUN applies to the same
//	                                      screen later
//
// `recordingFrom` is the whole of that reading. It is entered here through the production
// function, with nothing faked but the three answers the observer gives it.

import (
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/learnstate"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// saidOn is a stand-in for the observer's accumulated vocabulary per screen.
func saidOn(by map[observe.VisualScreenID][]string) func(observe.VisualScreenID) observe.ScreenText {
	return func(id observe.VisualScreenID) observe.ScreenText {
		words, ok := by[id]
		if !ok {
			return observe.ScreenText{}
		}
		return observe.TextOf(directorapi.SourceOCR, words)
	}
}

// aCapturedOutcome is a press whose result Marco has the evidence for: five independent words
// that appeared and stood, with readings on both sides. The same four terms `Verified` applies.
func aCapturedOutcome(screen observe.VisualScreenID,
	target observe.VisualTargetID) observe.LocalOutcome {
	return observe.LocalOutcome{
		Target: target, Screen: screen, Look: 7, Settled: true,
		Appeared:    []string{"advanced", "bloom", "brightness", "gamma", "shadows"},
		BeforeLooks: 2, AfterLooks: 3,
	}
}

// A PRESS THAT LANDED ON NOTHING IS NOT A PRESS BEING CHECKED.
//
// It can never become a step — nothing was found under it to re-ground — so waiting cannot help
// and the phase that says so has to be reachable. It was not: the recorder counted outcome
// watches, and the correlator opens one for every press it is given, blind or not.
func TestTheRecorderCountsOnlyThePressesThatCouldBecomeSteps(t *testing.T) {
	said := saidOn(map[observe.VisualScreenID][]string{
		"visual_1": {"audio", "video", "display", "controls"},
	})
	// ONE PRESS, ON NOTHING. The correlator reports it, and reports a watch for it
	// whose outcome can never be anybody's evidence.
	ev := observe.ClickEvidence{Presses: 1, Blind: 1,
		Outcomes: []observe.LocalOutcome{{Screen: "visual_1", Look: 4}}}

	at := recordingFrom(ev, "visual_1", said, 12)
	if at.Watched != 0 {
		t.Errorf("%d of 1 press(es) is being checked, and it landed on nothing",
			at.Watched)
	}
	if at.Phase != learnstate.Blind {
		t.Errorf("the phase is %q, want %q — anything else tells somebody to wait "+
			"for a result that cannot arrive", at.Phase, learnstate.Blind)
	}

	// AND A PRESS ON SOMETHING, BESIDE IT, IS THE ONE IN FLIGHT. One of each is the
	// ordinary case: a click beside a control and then one on it.
	ev.Presses = 2
	ev.Outcomes = append(ev.Outcomes,
		observe.LocalOutcome{Target: "t-audio", Screen: "visual_1", Look: 9,
			BeforeLooks: 1})
	at = recordingFrom(ev, "visual_1", said, 11)
	switch {
	case at.Watched != 1:
		t.Errorf("%d press(es) of 2 are being checked, and one of them was blind",
			at.Watched)
	case at.Phase != learnstate.Checking:
		t.Errorf("the phase is %q, want %q: the press being measured is the one "+
			"that landed on something", at.Phase, learnstate.Checking)
	}
	if at.Missing == "" {
		t.Error("nothing says what is missing from the press in flight")
	}
}

// AND THE STARTING SCREEN IS JUDGED BY WHAT THE RUN WILL ASK OF IT.
//
// `observe.ScreenText.Identifies`, the same call `workflowstore.Standing` makes: enough stable
// words to tell that screen from another one. `Read` — did anybody look at all — says yes about
// a screen the run then refuses, and the person finds that out by pressing Play.
func TestTheRecorderAsksWhatTheRunWillAskOfTheStartingScreen(t *testing.T) {
	for _, c := range []struct {
		why   string
		words []string
		known bool
		phase learnstate.Phase
	}{
		{"nothing read on it", nil, false, learnstate.Reading},
		// A READING THAT FOUND TWO WORDS IS A READING, and two words cannot
		// tell two screens apart. `Read` and `Said` are both true here.
		{"two words", []string{"audio", "video"}, false, learnstate.Reading},
		{"three words", []string{"audio", "video", "display"}, true,
			learnstate.Ready},
	} {
		at := recordingFrom(observe.ClickEvidence{}, "visual_1",
			saidOn(map[observe.VisualScreenID][]string{"visual_1": c.words}), 20)
		if at.StartKnown != c.known {
			t.Errorf("with %s, the starting screen is reported as "+
				"recognisable=%v", c.why, at.StartKnown)
		}
		if at.Phase != c.phase {
			t.Errorf("with %s, the phase is %q and should be %q",
				c.why, at.Phase, c.phase)
		}
	}
}

// AND IT IS THE SCREEN AN ACT WOULD START ON, NOT THE ONE THE SESSION STARTED ON.
//
// The first press that can become a step becomes step 1, so the screen it was made on is the
// screen a granted run has to recognise before it presses anything. A person who opened the menu
// with a key before clicking has a first screen the Act never starts on, and judging its
// vocabulary judged a screen nothing will ever look for.
func TestTheStartingScreenIsTheOneAnActWouldStartOn(t *testing.T) {
	said := saidOn(map[observe.VisualScreenID][]string{
		// WHERE THE SESSION BEGAN, read well. Nothing presses here.
		"visual_1": {"continue", "options", "credits", "quit"},
		// AND WHERE THE FIRST PRESS WAS MADE, barely read. This is the one that
		// has to be judged.
		"visual_2": {"audio"},
		// AND THE SCREEN IN FRONT OF THEM NOW, which the press opened and which is
		// read well — so a recording that judged the screen STANDING rather than
		// the screen an Act starts on would call this demonstration fine. A
		// fixture where it happened to be unread would be satisfied by the wrong
		// answer, and this one was.
		"visual_3": {"advanced", "bloom", "brightness", "gamma"},
	})
	ev := observe.ClickEvidence{Presses: 1,
		Outcomes: []observe.LocalOutcome{aCapturedOutcome("visual_2", "t-audio")}}

	at := recordingFrom(ev, "visual_3", said, 8)
	if at.StartKnown {
		t.Error("the starting screen is reported as recognisable, and the screen " +
			"the first press was made on had one word on it — the words of " +
			"some other screen were judged instead")
	}

	// AND BEFORE ANY PRESS IT IS THE SCREEN IN FRONT OF THEM, because that is the
	// screen the next press would be made on.
	none := recordingFrom(observe.ClickEvidence{}, "visual_1", said, 9)
	if !none.StartKnown {
		t.Error("before a press, the screen they are looking at is the one an Act " +
			"would start on, and that one was read well")
	}
}

// AND A CAPTURED RESULT DOES NOT MAKE AN UNRECOGNISABLE STARTING SCREEN GO AWAY.
//
// The defect reported against the first version of this: the phase reached `captured`, the
// window said "result captured — click the next control, or finish", and nothing anywhere
// mentioned that an Act built from it would pause on a screen it could not name. They are two
// faults with two remedies, so the recording carries both.
func TestACapturedResultDoesNotHideAnUnnameableStartingScreen(t *testing.T) {
	at := recordingFrom(
		observe.ClickEvidence{Presses: 1,
			Outcomes: []observe.LocalOutcome{
				aCapturedOutcome("visual_1", "t-audio")}},
		"visual_2",
		saidOn(map[observe.VisualScreenID][]string{"visual_1": {"audio"}}), 6)

	if at.Phase != learnstate.Captured || at.Captured != 1 {
		t.Fatalf("the press's own evidence is not reported as captured: "+
			"phase=%q captured=%d", at.Phase, at.Captured)
	}
	if at.StartKnown {
		t.Error("a captured result reported the starting screen as recognisable, " +
			"and one word was read on it")
	}
}

// AND THE RECORDER'S LOOP IS WHAT SAYS IT.
//
// Over the source, because the alternative is a capture loop with a real window behind it. What
// is asserted is the join: the stream is written from `recordingNow`, which is the reading above
// — a loop that assembled its own event would be a second answer to the question this file
// exists to answer once.
func TestTheProgressStreamIsWrittenFromWhatTheRecorderKnows(t *testing.T) {
	src, err := os.ReadFile("demonstratevisual.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "told.Say(recordingNow(obs, left))") {
		t.Error("the recorder does not say what it knows as the session runs")
	}
	if !strings.Contains(string(src), "last := recordingNow(obs, 0)") {
		t.Error("the last event of a recording is not what the recorder knew at " +
			"the end of it")
	}
	loop, err := os.ReadFile("visualsession.go")
	if err != nil {
		t.Fatal(err)
	}
	// AFTER THE OBSERVATION AND NOT BEFORE IT. The event is a reading of what the
	// observer has, so saying it before folding this look reports the one before.
	at := strings.Index(string(loop), "obs.Observe(lk)")
	told := strings.Index(string(loop), "s.told(")
	if at < 0 || told < 0 || told < at {
		t.Errorf("the loop observes at %d and says what it knows at %d, so it "+
			"reports the look before the one it just took", at, told)
	}
}
